package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"call-my-agent/pkg/codex"
	"call-my-agent/pkg/config"
	"call-my-agent/pkg/model"
	"call-my-agent/pkg/session"
	"call-my-agent/pkg/store"
	"call-my-agent/pkg/whisper"
)

const heartbeatSummaryLimit = 5

// MessageProvider describes Telegram interactions.
type MessageProvider interface {
	Receive(ctx context.Context) (<-chan model.InboundMessage, <-chan error)
	Send(ctx context.Context, chatID string, text string) error
}

type transcriber interface {
	Transcribe(ctx context.Context, audio whisper.Audio) (string, error)
}

type logLevel int

const (
	levelSilent logLevel = iota
	levelError
	levelWarn
	levelInfo
	levelDebug
)

// Service coordinates polling, Codex calls, and heartbeats.
type Service struct {
	cfg         config.Config
	provider    MessageProvider
	ai          codex.AIClient
	sessions    *session.Manager
	clock       session.Clock
	logger      *log.Logger
	store       store.SessionStore
	logLevel    logLevel
	transcriber transcriber
	// transcriberMu protects lazy initialization of the transcriber client.
	transcriberMu sync.Mutex

	// commandQueues coordinates Codex command-mode calls per chat so that
	// multiple inbound messages received while a command is running are
	// batched into the next Codex run.
	commandQueuesMu sync.Mutex
	commandQueues   map[string]*commandQueue
}

type commandQueue struct {
	pending []string
	running bool
}

// New creates a service instance.
func New(cfg config.Config, provider MessageProvider, ai codex.AIClient, clock session.Clock, sessionStore store.SessionStore) (*Service, error) {
	if clock == nil {
		clock = session.RealClock{}
	}
	level, err := parseLogLevel(cfg.Logging.Level)
	if err != nil {
		return nil, err
	}
	writer, err := openLogWriter(cfg.Logging.File)
	if err != nil {
		return nil, err
	}

	srv := &Service{
		cfg:           cfg,
		provider:      provider,
		ai:            ai,
		sessions:      session.NewManager(cfg.Inbound.Reply.Session.IdleMinutes, cfg.Inbound.Reply.Session.ResetTriggers, cfg.Inbound.Reply.Session.MaxMessages, clock),
		clock:         clock,
		logger:        log.New(writer, "cma ", log.LstdFlags),
		store:         sessionStore,
		logLevel:      level,
		commandQueues: make(map[string]*commandQueue),
	}

	if sessionStore != nil {
		loaded, err := sessionStore.Load()
		if err != nil {
			return nil, err
		}
		for _, sess := range loaded {
			srv.sessions.Restore(sess)
		}
	}

	return srv, nil
}

// Start runs the main polling loop until context cancellation.
func (s *Service) Start(ctx context.Context) error {
	if s.provider == nil {
		return fmt.Errorf("message provider is not configured")
	}
	s.logf(levelInfo, "service starting (heartbeat=%d min, logLevel=%s)", s.cfg.Inbound.HeartbeatMinutes, s.logLevel.String())
	defer s.logf(levelInfo, "service stopped")
	msgCh, errCh := s.provider.Receive(ctx)
	heartbeatDone := make(chan struct{})
	if s.cfg.Inbound.HeartbeatMinutes > 0 {
		go s.startHeartbeatScheduler(ctx, heartbeatDone)
	} else {
		close(heartbeatDone)
	}
	s.logf(levelInfo, "service started and listening for messages")

	for {
		select {
		case <-ctx.Done():
			<-heartbeatDone
			return nil
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			s.logf(levelWarn, "provider error: %v", err)
		case msg, ok := <-msgCh:
			if !ok {
				msgCh = nil
				continue
			}
			s.logf(levelInfo, "received message chat=%s text=%q", msg.ChatID, msg.Text)
			s.dispatchMessage(ctx, msg)
		}
		if msgCh == nil && errCh == nil {
			<-heartbeatDone
			return nil
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg model.InboundMessage) error {
	body, err := s.normalizeMessage(ctx, msg)
	if err != nil || body == "" {
		return err
	}
	return s.processBatch(ctx, msg.ChatID, body)
}

func (s *Service) transcribeAudio(ctx context.Context, audio *model.Audio) (string, error) {
	s.transcriberMu.Lock()
	if s.transcriber == nil {
		client, err := whisper.NewClientFromEnv()
		if err != nil {
			s.transcriberMu.Unlock()
			return "", err
		}
		s.transcriber = client
	}
	tr := s.transcriber
	s.transcriberMu.Unlock()

	start := time.Now()
	text, err := tr.Transcribe(ctx, whisper.Audio{
		Data:     audio.Data,
		FileName: audio.FileName,
		MimeType: audio.MimeType,
	})
	elapsed := time.Since(start)
	s.logf(levelInfo, "audio transcription completed in %s", elapsed)
	return text, err
}

// dispatchMessage routes inbound messages either through the existing
// synchronous handler (for static mode) or through the per-chat command
// queues (for command mode) so that Codex calls can be batched.
func (s *Service) dispatchMessage(ctx context.Context, msg model.InboundMessage) {
	switch strings.ToLower(s.cfg.Inbound.Reply.Mode) {
	case "command":
		if err := s.enqueueCommandMessage(ctx, msg); err != nil {
			s.logf(levelError, "enqueue command message: %v", err)
		}
	default:
		if err := s.handleMessage(ctx, msg); err != nil {
			s.logf(levelError, "handle message: %v", err)
		}
	}
}

// normalizeMessage enforces access control and resolves the text body,
// including optional audio transcription. It returns an empty body when
// the message should be ignored.
func (s *Service) normalizeMessage(ctx context.Context, msg model.InboundMessage) (string, error) {
	if !model.AllowedChat(msg.ChatID, s.cfg.Inbound.AllowFrom) {
		return "", nil
	}
	body := msg.Text
	if body == "" && msg.Audio != nil {
		transcribed, err := s.transcribeAudio(ctx, msg.Audio)
		if err != nil {
			s.logf(levelError, "transcribe audio: %v", err)
			// Best-effort apology; ignore send errors to match previous behaviour.
			_ = s.provider.Send(ctx, msg.ChatID, "Sorry, I couldn't transcribe that voice message.")
			return "", nil
		}
		body = transcribed
	}
	if body == "" {
		return "", nil
	}
	return body, nil
}

// processBatch executes the core reply logic for a single (possibly
// aggregated) user body.
func (s *Service) processBatch(ctx context.Context, chatID, body string) error {
	sess, reset := s.sessions.Get(chatID, body)
	if reset {
		s.saveSessions()
	}
	task := codex.BuildTask(s.cfg.Inbound.Reply.BodyPrefix, sess, body)
	s.sessions.Append(sess, "user", body)
	s.saveSessions()

	var reply string
	switch strings.ToLower(s.cfg.Inbound.Reply.Mode) {
	case "static":
		reply = s.cfg.Inbound.Reply.StaticText
	case "command":
		data := codex.PrepareTemplateData(chatID, body, task)
		var err error
		reply, err = s.ai.Run(ctx, data, s.cfg.Timeout())
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported mode %q", s.cfg.Inbound.Reply.Mode)
	}

	// Suppress bare HEARTBEAT_OK replies so that accidental heartbeat-style
	// outputs from Codex are not forwarded to Telegram chats.
	if strings.TrimSpace(reply) == "HEARTBEAT_OK" {
		s.logf(levelDebug, "suppressing HEARTBEAT_OK reply for chat=%s", chatID)
		return nil
	}

	s.sessions.Append(sess, "assistant", reply)
	s.saveSessions()
	return s.provider.Send(ctx, chatID, reply)
}

func (s *Service) enqueueCommandMessage(ctx context.Context, msg model.InboundMessage) error {
	body, err := s.normalizeMessage(ctx, msg)
	if err != nil || body == "" {
		return err
	}

	s.commandQueuesMu.Lock()
	q := s.commandQueues[msg.ChatID]
	if q == nil {
		q = &commandQueue{}
		s.commandQueues[msg.ChatID] = q
	}
	if q.running {
		q.pending = append(q.pending, body)
		s.commandQueuesMu.Unlock()
		return nil
	}
	// First message for this chat starts a new worker and is processed
	// immediately; subsequent messages accumulate in the pending queue
	// while Codex is running and will be batched on the next run.
	q.running = true
	s.commandQueuesMu.Unlock()

	go s.processCommandQueue(ctx, msg.ChatID, body)
	return nil
}

func (s *Service) processCommandQueue(ctx context.Context, chatID, firstBody string) {
	if ctx.Err() != nil {
		return
	}
	if err := s.processBatch(ctx, chatID, firstBody); err != nil {
		s.logf(levelError, "handle command batch chat=%s: %v", chatID, err)
	}

	for {
		bodies := s.dequeuePending(chatID)
		if len(bodies) == 0 || ctx.Err() != nil {
			return
		}
		combined := strings.Join(bodies, "\n\n")
		if err := s.processBatch(ctx, chatID, combined); err != nil {
			s.logf(levelError, "handle command batch chat=%s: %v", chatID, err)
		}
	}
}

func (s *Service) dequeuePending(chatID string) []string {
	s.commandQueuesMu.Lock()
	defer s.commandQueuesMu.Unlock()

	q := s.commandQueues[chatID]
	if q == nil || len(q.pending) == 0 {
		if q != nil {
			q.running = false
			// Remove idle queues to avoid unbounded growth for transient chats.
			delete(s.commandQueues, chatID)
		}
		return nil
	}
	bodies := q.pending
	q.pending = nil
	return bodies
}

func (s *Service) startHeartbeatScheduler(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Duration(s.cfg.Inbound.HeartbeatMinutes) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RunHeartbeat(ctx); err != nil {
				s.logf(levelWarn, "heartbeat: %v", err)
			}
		}
	}
}

// RunHeartbeat executes a single heartbeat pass.
func (s *Service) RunHeartbeat(ctx context.Context) error {
	if s.provider == nil {
		return fmt.Errorf("message provider is not configured")
	}
	now := s.clock.Now()
	idleLimit := time.Duration(s.cfg.Inbound.Reply.Session.HeartbeatIdleMinutes) * time.Minute
	if idleLimit <= 0 {
		idleLimit = time.Duration(s.cfg.Inbound.Reply.Session.IdleMinutes) * time.Minute
	}
	changed := false
	var firstErr error
	for _, sess := range s.sessions.Snapshot() {
		if now.Sub(sess.UpdatedAt) > idleLimit {
			continue
		}
		heartbeatBody := buildHeartbeatPrompt(sess)
		task := codex.BuildTask(s.cfg.Inbound.Reply.BodyPrefix, sess, heartbeatBody)
		data := codex.PrepareTemplateData(sess.ChatID, heartbeatBody, task)
		resp, err := s.ai.Run(ctx, data, s.cfg.Timeout())
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.logf(levelWarn, "heartbeat ai.Run chat=%s: %v", sess.ChatID, err)
			continue
		}
		if strings.TrimSpace(resp) == "HEARTBEAT_OK" {
			continue
		}
		if s.sessions.AppendByID(sess.ChatID, sess.ID, "system", heartbeatBody) {
			changed = true
		}
		if s.sessions.AppendByID(sess.ChatID, sess.ID, "assistant", resp) {
			changed = true
		}
		if err := s.provider.Send(ctx, sess.ChatID, resp); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			s.logf(levelWarn, "heartbeat send chat=%s: %v", sess.ChatID, err)
			continue
		}
		changed = true
	}
	if s.sessions.ExpireIdleSessions() {
		changed = true
	}
	if changed {
		s.saveSessions()
	}
	return firstErr
}

// StatusOptions controls status output.
type StatusOptions struct {
	JSON  bool
	Limit int
}

// StatusEntry describes one session for status reporting.
type StatusEntry struct {
	SessionID     string    `json:"sessionId"`
	ChatID        string    `json:"chatId"`
	UpdatedAt     time.Time `json:"updatedAt"`
	MessageCount  int       `json:"messageCount"`
	LastUser      string    `json:"lastUser"`
	LastAssistant string    `json:"lastAssistant"`
}

// Status prints a status report to the writer.
func (s *Service) Status(w io.Writer, opts StatusOptions) error {
	sessions := s.sessions.Snapshot()
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	entries := make([]StatusEntry, 0, len(sessions))
	for _, sess := range sessions {
		entry := summarizeSession(sess)
		entries = append(entries, entry)
		if opts.Limit > 0 && len(entries) >= opts.Limit {
			break
		}
	}
	if opts.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	for _, entry := range entries {
		if _, err := fmt.Fprintf(
			w,
			"Session %s chat %s updated=%s messages=%d lastUser=%q lastAssistant=%q\n",
			entry.SessionID,
			entry.ChatID,
			entry.UpdatedAt.Format(time.RFC3339),
			entry.MessageCount,
			entry.LastUser,
			entry.LastAssistant,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveSessions() {
	if s.store == nil {
		return
	}
	if err := s.store.Save(s.sessions.Snapshot()); err != nil {
		s.logf(levelError, "save sessions: %v", err)
	}
}

func (s *Service) logf(level logLevel, format string, args ...interface{}) {
	if s.logLevel == levelSilent {
		return
	}
	if level <= s.logLevel {
		s.logger.Printf("[%s] %s", level.String(), fmt.Sprintf(format, args...))
	}
}

func (l logLevel) String() string {
	switch l {
	case levelError:
		return "error"
	case levelWarn:
		return "warn"
	case levelInfo:
		return "info"
	case levelDebug:
		return "debug"
	default:
		return "silent"
	}
}

func parseLogLevel(level string) (logLevel, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return levelInfo, nil
	case "debug":
		return levelDebug, nil
	case "warn":
		return levelWarn, nil
	case "error":
		return levelError, nil
	case "silent":
		return levelSilent, nil
	default:
		return levelInfo, fmt.Errorf("unknown log level %q", level)
	}
}

func openLogWriter(path string) (io.Writer, error) {
	if path == "" || path == "-" {
		return os.Stdout, nil
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func buildHeartbeatPrompt(sess *model.Session) string {
	var b strings.Builder
	b.WriteString("HEARTBEAT TELEGRAM\n\n")
	b.WriteString("Recent messages:\n")
	messages := sess.Messages
	if len(messages) > heartbeatSummaryLimit {
		messages = messages[len(messages)-heartbeatSummaryLimit:]
	}
	for _, msg := range messages {
		b.WriteString("- ")
		b.WriteString(roleLabel(msg.Role))
		b.WriteString(": ")
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}
	b.WriteString("\nIf there is nothing useful to tell the user, reply with exactly HEARTBEAT_OK.")
	return b.String()
}

func roleLabel(role string) string {
	if role == "" {
		return role
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func summarizeSession(sess *model.Session) StatusEntry {
	var lastUser, lastAssistant string
	for i := len(sess.Messages) - 1; i >= 0; i-- {
		msg := sess.Messages[i]
		if lastUser == "" && msg.Role == "user" {
			lastUser = msg.Content
		}
		if lastAssistant == "" && msg.Role == "assistant" {
			lastAssistant = msg.Content
		}
		if lastUser != "" && lastAssistant != "" {
			break
		}
	}
	return StatusEntry{
		SessionID:     sess.ID,
		ChatID:        sess.ChatID,
		UpdatedAt:     sess.UpdatedAt,
		MessageCount:  len(sess.Messages),
		LastUser:      lastUser,
		LastAssistant: lastAssistant,
	}
}
