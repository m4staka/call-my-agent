package service

import (
	"context"
	"testing"
	"time"

	"call-my-agent/pkg/codex"
	"call-my-agent/pkg/config"
	"call-my-agent/pkg/model"
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
}

func (f *fakeAI) Run(ctx context.Context, data codex.TemplateData, timeout time.Duration) (string, error) {
	if f.idx >= len(f.responses) {
		return "", nil
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp, nil
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

func TestHandleMessageCommandMode(t *testing.T) {
	cfg := config.Config{
		Inbound: config.InboundConfig{
			AllowFrom: []string{"123"},
			Reply:     config.ReplyConfig{Mode: "command", BodyPrefix: "system", Command: []string{"cmd", "{{.Task}}"}, Session: config.SessionConfig{IdleMinutes: 60}},
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
	if len(sessions) != 1 || len(sessions[0].Messages) != 2 {
		t.Fatalf("expected session messages recorded, got %+v", sessions)
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
	if len(sessions) != 1 || len(sessions[0].Messages) != 3 {
		t.Fatalf("expected heartbeat messages recorded, got %+v", sessions)
	}
}
