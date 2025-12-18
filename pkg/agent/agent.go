package agent

import (
	"context"
	"strings"
	"time"

	"call-my-agent/pkg/model"
)

// Runner executes coding agents (Codex, Pi, etc.).
type Runner interface {
	Run(ctx context.Context, req Request) (Result, error)
}

// Request captures the data sent to an agent invocation.
type Request struct {
	Body         string
	BodyStripped string
	ChatID       string
	SessionID    string
	Resume       bool
	Task         string
	Timeout      time.Duration
}

// Result captures the response from an agent invocation.
type Result struct {
	Reply     string
	SessionID string
}

// PrepareRequest constructs an agent request payload.
func PrepareRequest(chatID, body, task, sessionID string, resume bool, timeout time.Duration) Request {
	return Request{
		Body:         body,
		BodyStripped: strings.TrimSpace(body),
		ChatID:       chatID,
		SessionID:    sessionID,
		Resume:       resume,
		Task:         task,
		Timeout:      timeout,
	}
}

// BuildTask composes the agent task text with prefix and conversation context.
func BuildTask(prefix string, sess *model.Session, userText string) string {
	var b strings.Builder
	if prefix != "" {
		b.WriteString(strings.TrimSpace(prefix))
		b.WriteString("\n\n")
	}
	if sess != nil {
		id := strings.TrimSpace(sess.AgentSessionID)
		if id == "" {
			id = strings.TrimSpace(sess.ID)
		}
		if id != "" {
			b.WriteString("Session ID: ")
			b.WriteString(id)
			b.WriteString("\n")
		}
	}
	b.WriteString("User: ")
	b.WriteString(userText)
	return b.String()
}
