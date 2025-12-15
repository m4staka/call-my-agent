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
	Task         string
	Timeout      time.Duration
}

// PrepareRequest constructs an agent request payload.
func PrepareRequest(chatID, body, task string, timeout time.Duration) Request {
	return Request{
		Body:         body,
		BodyStripped: strings.TrimSpace(body),
		ChatID:       chatID,
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
	for _, msg := range sess.Messages {
		b.WriteString(capitalize(msg.Role))
		b.WriteString(": ")
		b.WriteString(msg.Content)
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
