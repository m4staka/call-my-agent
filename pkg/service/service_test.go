package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"call-my-agent/pkg/agent"
	"call-my-agent/pkg/config"
	"call-my-agent/pkg/model"
	"call-my-agent/pkg/whisper"
)

type fakeProvider struct{ sent []model.InboundMessage }

type sentMsg struct {
	chatID string
	text   string
}

func (p *fakeProvider) Receive(ctx context.Context) (<-chan model.InboundMessage, <-chan error) {
	return make(chan model.InboundMessage), make(chan error)
}

func (p *fakeProvider) Send(ctx context.Context, chatID string, text string) error {
	p.sent = append(p.sent, model.InboundMessage{ChatID: chatID, Text: text})
	return nil
}

type fakeAI struct {
	responses []string
	idx       int
	calls     []agent.Request
}

func (f *fakeAI) Run(ctx context.Context, req agent.Request) (string, error) {
	f.calls = append(f.calls, req)
	if f.idx >= len(f.responses) {
		return "", nil
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

type errorAI struct{ err error }

func (e errorAI) Run(ctx context.Context, req agent.Request) (string, error) {
	return "", e.err
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

type fakeTranscriber struct {
	text  string
	err   error
	calls int
}

func (f *fakeTranscriber) Transcribe(ctx context.Context, audio whisper.Audio) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

func TestHandleMessageCommandMode(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"123"},
			Reply:     config.ReplyConfig{Mode: "command", BodyPrefix: "system", Agent: "codex", Session: config.SessionConfig{IdleMinutes: 60}},
		},
	}
	provider := &fakeProvider{}
	ai := &fakeAI{responses: []string{"reply"}}
	srv, err := New(cfg, provider, ai, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := srv.handleMessage(context.Background(), model.InboundMessage{ChatID: "123", Text: "hi"}); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}
	if len(provider.sent) != 1 || provider.sent[0].Text != "reply" {
		t.Fatalf("expected provider to send reply, got %+v", provider.sent)
	}
	sessions := srv.sessions.Snapshot()
	if len(sessions) != 1 || sessions[0].ID == "" {
		t.Fatalf("expected session recorded, got %+v", sessions)
	}
	if len(ai.calls) != 1 || ai.calls[0].SessionID != sessions[0].ID || ai.calls[0].Resume {
		t.Fatalf("unexpected ai call %+v", ai.calls)
	}
}

func TestHandleMessageSuppressesHeartbeatOK(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"123"},
			Reply:     config.ReplyConfig{Mode: "command", BodyPrefix: "system", Agent: "codex", Session: config.SessionConfig{IdleMinutes: 60}},
		},
	}
	provider := &fakeProvider{}
	ai := &fakeAI{responses: []string{"HEARTBEAT_OK"}}
	srv, err := New(cfg, provider, ai, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := srv.handleMessage(context.Background(), model.InboundMessage{ChatID: "123", Text: "hi"}); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}

	if len(provider.sent) != 0 {
		t.Fatalf("expected no messages for HEARTBEAT_OK, got %d", len(provider.sent))
	}

	sessions := srv.sessions.Snapshot()
	if len(sessions) != 1 || sessions[0].ID == "" {
		t.Fatalf("expected session recorded, got %+v", sessions)
	}
}

func TestHeartbeatSuppression(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			HeartbeatMinutes: 1,
			Reply:            config.ReplyConfig{BodyPrefix: "hb", Session: config.SessionConfig{IdleMinutes: 60, HeartbeatIdleMinutes: 60}},
		},
	}
	provider := &fakeProvider{}
	clock := fixedClock{now: time.Now()}

	ai := &fakeAI{responses: []string{"HEARTBEAT_OK"}}
	srv, err := New(cfg, provider, ai, clock, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	sess, _ := srv.sessions.Get("123", "hello")
	srv.sessions.Append(sess, "user", "hello")

	if err := srv.RunHeartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat error: %v", err)
	}
	if len(provider.sent) != 0 {
		t.Fatalf("expected no messages for HEARTBEAT_OK, got %d", len(provider.sent))
	}

	ai.responses = []string{"proactive"}
	ai.idx = 0
	if err := srv.RunHeartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat error: %v", err)
	}
	if len(provider.sent) != 1 || provider.sent[0].Text != "proactive" {
		t.Fatalf("expected proactive message, got %+v", provider.sent)
	}
	sessions := srv.sessions.Snapshot()
	if len(sessions) != 1 || sessions[0].ID == "" {
		t.Fatalf("expected session recorded, got %+v", sessions)
	}
}

func TestHandleMessageTranscribesAudio(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"123"},
			Reply:     config.ReplyConfig{Mode: "command", BodyPrefix: "system", Agent: "codex", Session: config.SessionConfig{IdleMinutes: 60}},
		},
	}
	provider := &fakeProvider{}
	ai := &fakeAI{responses: []string{"reply"}}
	srv, err := New(cfg, provider, ai, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	transcriber := &fakeTranscriber{text: "hello"}
	srv.transcriber = transcriber

	msg := model.InboundMessage{ChatID: "123", Audio: &model.Audio{Data: []byte("voice"), FileName: "clip.ogg", MimeType: "audio/ogg"}}
	if err := srv.handleMessage(context.Background(), msg); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}
	if transcriber.calls != 1 {
		t.Fatalf("expected one transcription call, got %d", transcriber.calls)
	}
	if len(provider.sent) != 1 || provider.sent[0].Text != "reply" {
		t.Fatalf("expected provider to send reply, got %+v", provider.sent)
	}
	if len(srv.sessions.Snapshot()) != 1 {
		t.Fatalf("expected session to be created")
	}
}

func TestHandleMessageTranscribeError(t *testing.T) {
	cfg := config.Config{Inbound: config.InboundConfig{AllowFrom: []string{"123"}, Reply: config.ReplyConfig{Mode: "static", StaticText: "n/a", Session: config.SessionConfig{IdleMinutes: 60}}}}
	provider := &fakeProvider{}
	ai := &fakeAI{responses: []string{"unused"}}
	srv, err := New(cfg, provider, ai, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	srv.transcriber = &fakeTranscriber{err: fmt.Errorf("boom")}
	msg := model.InboundMessage{ChatID: "123", Audio: &model.Audio{Data: []byte("voice"), FileName: "clip.ogg"}}

	if err := srv.handleMessage(context.Background(), msg); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}
	if len(provider.sent) != 1 || provider.sent[0].Text == "" {
		t.Fatalf("expected apology message, got %+v", provider.sent)
	}
	if len(srv.sessions.Snapshot()) != 0 {
		t.Fatalf("expected no session to be stored on transcription failure")
	}
}

func TestHandleMessageSurfacesAIRunErrors(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"chat"},
			Reply: config.ReplyConfig{
				Mode:    "command",
				Agent:   "codex",
				Session: config.SessionConfig{IdleMinutes: 1},
			},
		},
		Logging: config.LoggingConfig{Level: "silent"},
	}
	provider := &fakeProvider{}
	srv, err := New(cfg, provider, errorAI{err: fmt.Errorf("agent missing")}, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := srv.handleMessage(context.Background(), model.InboundMessage{ChatID: "chat", Text: "hi"}); err != nil {
		t.Fatalf("handleMessage returned error: %v", err)
	}

	if len(provider.sent) != 1 {
		t.Fatalf("expected one error reply, got %d", len(provider.sent))
	}
	if !strings.Contains(provider.sent[0].Text, "agent missing") {
		t.Fatalf("unexpected error reply: %q", provider.sent[0].Text)
	}

	sessions := srv.sessions.Snapshot()
	if len(sessions) != 1 || sessions[0].ID == "" {
		t.Fatalf("expected session stored, got %+v", sessions)
	}
}

type queueProvider struct {
	messages []model.InboundMessage
	sent     []sentMsg
}

func (p *queueProvider) Receive(ctx context.Context) (<-chan model.InboundMessage, <-chan error) {
	msgCh := make(chan model.InboundMessage)
	errCh := make(chan error)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		for _, m := range p.messages {
			select {
			case <-ctx.Done():
				return
			case msgCh <- m:
			}
		}
	}()
	return msgCh, errCh
}

func (p *queueProvider) Send(ctx context.Context, chatID string, text string) error {
	p.sent = append(p.sent, sentMsg{chatID: chatID, text: text})
	return nil
}

func TestCommandQueueBatchesMessagesPerChat(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"123"},
			Reply: config.ReplyConfig{
				Mode:  "command",
				Agent: "codex",
				Session: config.SessionConfig{
					IdleMinutes: 60,
				},
			},
		},
		Logging: config.LoggingConfig{Level: "silent"},
	}

	provider := &queueProvider{
		messages: []model.InboundMessage{
			{ChatID: "123", Text: "first"},
			{ChatID: "123", Text: "second"},
			{ChatID: "123", Text: "third"},
		},
	}
	ai := &fakeAI{responses: []string{"r1", "r2"}}
	srv, err := New(cfg, provider, ai, fixedClock{now: time.Now()}, nil)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// Wait for the queued messages to be processed.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(ai.calls) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("service start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("service did not shut down after context cancellation")
	}

	// We expect two Codex invocations: one for the first message, and one
	// batching the second and third messages together.
	if len(ai.calls) != 2 {
		t.Fatalf("expected 2 Codex calls, got %d", len(ai.calls))
	}
	if !strings.Contains(ai.calls[0].Body, "first") {
		t.Fatalf("first call body missing 'first': %q", ai.calls[0].Body)
	}
	if !strings.Contains(ai.calls[1].Body, "second") || !strings.Contains(ai.calls[1].Body, "third") {
		t.Fatalf("second call body should contain batched messages, got %q", ai.calls[1].Body)
	}
	if ai.calls[0].SessionID == "" || ai.calls[0].SessionID != ai.calls[1].SessionID {
		t.Fatalf("expected shared session IDs, got %+v", ai.calls)
	}
	if ai.calls[0].Resume || !ai.calls[1].Resume {
		t.Fatalf("unexpected resume flags %+v", ai.calls)
	}
}
