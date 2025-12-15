package pi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"call-my-agent/pkg/agent"
)

// Client runs the Pi CLI in non-interactive mode.
type Client struct {
	WorkingDir     string
	commandBuilder func(req agent.Request) ([]string, error)
}

// Run invokes the Pi CLI.
func (c Client) Run(ctx context.Context, req agent.Request) (string, error) {
	builder := c.commandBuilder
	if builder == nil {
		builder = buildPiCommand
	}
	args, err := builder(req)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("pi exec failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func buildPiCommand(req agent.Request) ([]string, error) {
	if strings.TrimSpace(req.Task) == "" {
		return nil, fmt.Errorf("task is required")
	}
	args := []string{"pi", "-p", "--no-session"}
	args = append(args, req.Task)
	return args, nil
}
