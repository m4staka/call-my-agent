package codex

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

// ExecClient runs the Codex CLI as an external process.
type ExecClient struct {
	WorkingDir     string
	commandBuilder func(req agent.Request) ([]string, error)
}

// Run executes the Codex CLI with the provided request data.
func (c ExecClient) Run(ctx context.Context, req agent.Request) (string, error) {
	builder := c.commandBuilder
	if builder == nil {
		builder = buildExecCommand
	}
	args, err := builder(req)
	if err != nil {
		return "", err
	}
	if len(args) == 0 {
		return "", fmt.Errorf("codex command is empty")
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, args[0], args[1:]...)
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
	return []string{"codex", "exec", task}, nil
}
