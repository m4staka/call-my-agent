package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"call-my-agent/pkg/codex"
	"call-my-agent/pkg/config"
	"call-my-agent/pkg/model"
	"call-my-agent/pkg/session"
)

// MessageProvider describes Telegram interactions.
type MessageProvider interface {
	Receive(ctx context.Context) (<-chan model.InboundMessage, <-chan error)
	Send(ctx context.Context, chatID string, text string) error
}

// Service coordinates polling, Codex calls, and heartbeats.
type Service struct {
	cfg      config.Config
	provider MessageProvider
	ai       codex.AIClient
	sessions *session.Manager
	clock    session.Clock
	logger   *log.Logger
}

// New creates a service instance.
func New(cfg config.Config, provider MessageProvider, ai codex.AIClient, clock session.Clock) *Service {
	if clock == nil {
		clock = session.RealClock{}
	}
	return &Service{
		cfg:      cfg,
		provider: provider,
		ai:       ai,
		sessions: session.NewManager(cfg.Inbound.Reply.Session.IdleMinutes, cfg.Inbound.Reply.Session.ResetTriggers, clock),
		clock:    clock,
		logger:   log.New(os.Stdout, "cma ", log.LstdFlags),
	}
}

// Start runs the main polling loop until context cancellation.
func (s *Service) Start(ctx context.Context) error {
	msgCh, errCh := s.provider.Receive(ctx)
	heartbeatDone := make(chan struct{})
	if s.cfg.Inbound.HeartbeatMinutes > 0 {
		go s.startHeartbeatScheduler(ctx, heartbeatDone)
	} else {
		close(heartbeatDone)
	}

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
			s.logger.Printf("provider error: %v", err)
		case msg, ok := <-msgCh:
			if !ok {
				msgCh = nil
				continue
			}
			if err := s.handleMessage(ctx, msg); err != nil {
				s.logger.Printf("handle message: %v", err)
			}
		}
		if msgCh == nil && errCh == nil {
			<-heartbeatDone
			return nil
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg model.InboundMessage) error {
	if !model.AllowedChat(msg.ChatID, s.cfg.Inbound.AllowFrom) {
		return nil
	}
	sess, _ := s.sessions.Get(msg.ChatID, msg.Text)
	task := codex.BuildTask(s.cfg.Inbound.Reply.BodyPrefix, sess, msg.Text)
	s.sessions.Append(sess, "user", msg.Text)

	var reply string
	switch strings.ToLower(s.cfg.Inbound.Reply.Mode) {
	case "static":
		reply = s.cfg.Inbound.Reply.StaticText
	case "command":
		data := codex.PrepareTemplateData(msg.ChatID, msg.Text, task)
		var err error
		reply, err = s.ai.Run(ctx, data, s.cfg.Timeout())
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported mode %q", s.cfg.Inbound.Reply.Mode)
	}

	s.sessions.Append(sess, "assistant", reply)
	return s.provider.Send(ctx, msg.ChatID, reply)
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
				s.logger.Printf("heartbeat: %v", err)
			}
		}
	}
}

// RunHeartbeat executes a single heartbeat pass.
func (s *Service) RunHeartbeat(ctx context.Context) error {
	now := s.clock.Now()
	idleLimit := time.Duration(s.cfg.Inbound.Reply.Session.HeartbeatIdleMinutes) * time.Minute
	if idleLimit == 0 {
		idleLimit = time.Duration(s.cfg.Inbound.Reply.Session.IdleMinutes) * time.Minute
	}
	for _, sess := range s.sessions.Snapshot() {
		if now.Sub(sess.UpdatedAt) > idleLimit {
			continue
		}
		heartbeatBody := "[HEARTBEAT]"
		task := codex.BuildTask(s.cfg.Inbound.Reply.BodyPrefix, sess, heartbeatBody)
		data := codex.PrepareTemplateData(sess.ChatID, heartbeatBody, task)
		resp, err := s.ai.Run(ctx, data, s.cfg.Timeout())
		if err != nil {
			return err
		}
		if strings.TrimSpace(resp) == "HEARTBEAT_OK" {
			continue
		}
		s.sessions.Append(sess, "system", heartbeatBody)
		s.sessions.Append(sess, "assistant", resp)
		if err := s.provider.Send(ctx, sess.ChatID, resp); err != nil {
			return err
		}
	}
	s.sessions.ExpireIdleSessions()
	return nil
}

// Status prints a simple status report.
func (s *Service) Status(w io.Writer) {
	for _, sess := range s.sessions.Snapshot() {
		fmt.Fprintf(w, "Session %s chat %s messages=%d updated=%s\n", sess.ID, sess.ChatID, len(sess.Messages), sess.UpdatedAt.Format(time.RFC3339))
	}
}
