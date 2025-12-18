package pi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"call-my-agent/pkg/agent"
)

// Client runs the Pi CLI in non-interactive mode.
type Client struct {
	WorkingDir     string
	commandBuilder func(req agent.Request) ([]string, error)
}

// Run invokes the Pi CLI.
func (c Client) Run(ctx context.Context, req agent.Request) (agent.Result, error) {
	builder := c.commandBuilder
	if builder == nil {
		builder = buildPiCommand
	}
	args, err := builder(req)
	if err != nil {
		return agent.Result{}, err
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, args[0], args[1:]...)
	if c.WorkingDir != "" {
		cmd.Dir = c.WorkingDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return agent.Result{}, fmt.Errorf("pi exec failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return agent.Result{Reply: strings.TrimSpace(stdout.String()), SessionID: strings.TrimSpace(req.SessionID)}, nil
}

func buildPiCommand(req agent.Request) ([]string, error) {
	if strings.TrimSpace(req.Task) == "" {
		return nil, fmt.Errorf("task is required")
	}
	args := []string{"pi", "-p"}
	if req.SessionID != "" {
		path, err := sessionFilePath(req.SessionID)
		if err != nil {
			return nil, err
		}
		args = append(args, "--session", path)
	}
	args = append(args, req.Task)
	return args, nil
}

func sessionFilePath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("session id is required")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".pi", "agent", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create pi session dir: %w", err)
	}
	return filepath.Join(dir, fmt.Sprintf("%s.jsonl", id)), nil
}
