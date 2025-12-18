package agent

import (
	"context"
	"strings"
	"time"

	"call-my-agent/pkg/model"
)

// Runner executes coding agents (Codex, Pi, etc.).
type Runner interface {
	Run(ctx context.Context, req Request) (string, error)
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
	if sess != nil && sess.ID != "" {
		b.WriteString("Session ID: ")
		b.WriteString(sess.ID)
		b.WriteString("\n")
	}
	b.WriteString("User: ")
	b.WriteString(userText)
	return b.String()
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	r := []rune(text)
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
