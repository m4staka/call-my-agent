package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"call-my-agent/pkg/agent"
)

// ExecClient runs the Codex CLI as an external process.
type ExecClient struct {
	WorkingDir     string
	commandBuilder func(req agent.Request) ([]string, error)
}

// Run executes the Codex CLI with the provided request data.
func (c ExecClient) Run(ctx context.Context, req agent.Request) (agent.Result, error) {
	builder := c.commandBuilder
	if builder == nil {
		builder = buildExecCommand
	}
	args, err := builder(req)
	if err != nil {
		return agent.Result{}, err
	}
	if len(args) == 0 {
		return agent.Result{}, fmt.Errorf("codex command is empty")
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	useJSONOutput := len(args) >= 2 && args[0] == "codex" && args[1] == "exec"
	outputPath := ""
	cleanup := func() {}
	if useJSONOutput {
		var err error
		outputPath, cleanup, err = newLastMessageFile()
		if err != nil {
			return agent.Result{}, err
		}
		defer cleanup()
		args = injectExecOutputFlags(args, outputPath)
	}

	cmd := exec.CommandContext(runCtx, args[0], args[1:]...)
	if resolved := c.resolveWorkingDir(); resolved != "" {
		cmd.Dir = resolved
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return agent.Result{}, fmt.Errorf("codex exec failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	reply := strings.TrimSpace(stdout.String())
	sessionID := strings.TrimSpace(req.SessionID)
	if useJSONOutput {
		var err error
		reply, err = readLastMessage(outputPath)
		if err != nil {
			return agent.Result{}, err
		}
		if sessionID == "" {
			sessionID = extractSessionID(stdout.String())
		}
	}
	return agent.Result{Reply: reply, SessionID: sessionID}, nil
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

// buildExecCommand builds the default codex exec invocation.
func buildExecCommand(req agent.Request) ([]string, error) {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}
	args := []string{"codex", "exec", "--yolo"}
	if req.Resume {
		if strings.TrimSpace(req.SessionID) == "" {
			return nil, fmt.Errorf("session ID is required when resuming")
		}
		args = append(args, "resume", req.SessionID)
	}
	args = append(args, task)
	return args, nil
}

func injectExecOutputFlags(args []string, outputPath string) []string {
	if len(args) < 2 || args[0] != "codex" || args[1] != "exec" {
		return args
	}
	if outputPath == "" {
		return args
	}
	// Ensure we can extract both the last assistant message and the session id.
	// - `--output-last-message` provides the final reply as plain text.
	// - `--json` prints structured events to stdout where session id is reported.
	flags := []string{"--json", "--output-last-message", outputPath}
	out := make([]string, 0, len(args)+len(flags))
	out = append(out, args[:2]...)
	out = append(out, flags...)
	out = append(out, args[2:]...)
	return out
}

func newLastMessageFile() (string, func(), error) {
	f, err := os.CreateTemp("", "cma-codex-last-message-*.txt")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp output file: %w", err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("close temp output file: %w", err)
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func readLastMessage(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read codex reply file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

var uuidRE = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)

func extractSessionID(jsonl string) string {
	jsonl = strings.TrimSpace(jsonl)
	if jsonl == "" {
		return ""
	}

	var best string
	for _, line := range strings.Split(jsonl, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			continue
		}
		if id := findSessionID(v); id != "" {
			best = id
		}
	}
	if best != "" {
		return best
	}
	// Fallback for unexpected event formats.
	if match := uuidRE.FindString(jsonl); match != "" {
		return match
	}
	return ""
}

func findSessionID(v any) string {
	switch val := v.(type) {
	case map[string]any:
		// Prefer explicit session id keys.
		for _, key := range []string{"session_id", "sessionId", "conversation_id", "conversationId"} {
			if raw, ok := val[key]; ok {
				if s, ok := raw.(string); ok && uuidRE.MatchString(s) {
					return s
				}
			}
		}
		// Search nested maps/slices for session-looking ids.
		for k, raw := range val {
			if s, ok := raw.(string); ok && strings.Contains(strings.ToLower(k), "session") && uuidRE.MatchString(s) {
				return s
			}
			if id := findSessionID(raw); id != "" {
				return id
			}
		}
	case []any:
		for _, item := range val {
			if id := findSessionID(item); id != "" {
				return id
			}
		}
	}
	return ""
}
