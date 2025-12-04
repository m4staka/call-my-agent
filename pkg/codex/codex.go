package codex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"call-my-agent/pkg/model"
)

// AIClient executes Codex CLI tasks.
type AIClient interface {
	Run(ctx context.Context, data TemplateData, timeout time.Duration) (string, error)
}

// ExecClient runs the Codex CLI as an external process.
type ExecClient struct {
	CommandTemplate []string
	WorkingDir      string
}

// TemplateData defines templated fields for commands.
type TemplateData struct {
	Body         string
	BodyStripped string
	ChatID       string
	Task         string
}

// Run executes the configured Codex CLI command with the provided template data.
func (c ExecClient) Run(ctx context.Context, data TemplateData, timeout time.Duration) (string, error) {
	if len(c.CommandTemplate) == 0 {
		return "", fmt.Errorf("command template is empty")
	}
	args, err := renderArgs(c.CommandTemplate, data)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if resolved := c.resolveWorkingDir(); resolved != "" {
		cmd.Dir = resolved
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("codex exec failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c ExecClient) resolveWorkingDir() string {
	if c.WorkingDir == "" {
		return ""
	}
	resolved, err := resolvePath(c.WorkingDir)
	if err != nil {
		return ""
	}
	return resolved
}

func renderArgs(tmpl []string, data TemplateData) ([]string, error) {
	rendered := make([]string, 0, len(tmpl))
	for _, arg := range tmpl {
		t, err := template.New("cmd").Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("parse command template: %w", err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("render command template: %w", err)
		}
		rendered = append(rendered, buf.String())
	}
	return rendered, nil
}

// BuildTask composes the Codex task text with prefix and conversation context.
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

// PrepareTemplateData constructs the template data for command rendering.
func PrepareTemplateData(chatID, body, task string) TemplateData {
	return TemplateData{
		Body:         body,
		BodyStripped: strings.TrimSpace(body),
		ChatID:       chatID,
		Task:         task,
	}
}

func resolvePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand home: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		path = filepath.Join(wd, path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory")
	}
	return path, nil
}
