package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"call-my-agent/pkg/agent"
)

func TestBuildExecCommand(t *testing.T) {
	args, err := buildExecCommand(agent.Request{Task: "do thing"})
	if err != nil {
		t.Fatalf("buildExecCommand returned error: %v", err)
	}
	want := []string{"codex", "exec", "do thing"}
	if len(args) != len(want) {
		t.Fatalf("expected args %v, got %v", want, args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("expected args %v, got %v", want, args)
		}
	}
}

func TestBuildExecCommandResume(t *testing.T) {
	args, err := buildExecCommand(agent.Request{Task: "next", Resume: true, SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("buildExecCommand returned error: %v", err)
	}
	want := []string{"codex", "exec", "resume", "sess-1", "next"}
	if len(args) != len(want) {
		t.Fatalf("expected args %v, got %v", want, args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("expected args %v, got %v", want, args)
		}
	}
}

func TestExecClientRunUsesWorkingDir(t *testing.T) {
	client := ExecClient{
		WorkingDir: t.TempDir(),
		commandBuilder: func(req agent.Request) ([]string, error) {
			return []string{"/bin/sh", "-c", "pwd"}, nil
		},
	}
	ctx := context.Background()
	output, err := client.Run(ctx, agent.Request{Timeout: time.Second, Task: "pwd"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if output != client.WorkingDir {
		t.Fatalf("expected output %q, got %q", client.WorkingDir, output)
	}
}

func TestExecClientRunInvalidWorkingDir(t *testing.T) {
	client := ExecClient{
		WorkingDir: "/path/does/not/exist",
		commandBuilder: func(req agent.Request) ([]string, error) {
			return []string{"/bin/sh", "-c", "pwd"}, nil
		},
	}
	ctx := context.Background()
	output, err := client.Run(ctx, agent.Request{Timeout: time.Second, Task: "pwd"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	cwd, _ := os.Getwd()
	if output != cwd {
		t.Fatalf("expected fallback to process cwd %q, got %q", cwd, output)
	}
}

func TestResolveWorkingDirSupportsRelativeAndHome(t *testing.T) {
	temp := t.TempDir()
	nested := filepath.Join(temp, "project")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWD) })
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	client := ExecClient{
		WorkingDir: "project",
		commandBuilder: func(req agent.Request) ([]string, error) {
			return []string{"/bin/sh", "-c", "pwd"}, nil
		},
	}

	ctx := context.Background()
	output, err := client.Run(ctx, agent.Request{Timeout: time.Second, Task: "pwd"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if output != nested {
		t.Fatalf("expected resolved cwd %q, got %q", nested, output)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot resolve home dir")
	}
	homeClient := ExecClient{
		WorkingDir: "~",
		commandBuilder: func(req agent.Request) ([]string, error) {
			return []string{"/bin/sh", "-c", "pwd"}, nil
		},
	}
	output, err = homeClient.Run(ctx, agent.Request{Timeout: time.Second, Task: "pwd"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	cleanedHome := filepath.Clean(homeDir)
	if output != cleanedHome {
		t.Fatalf("expected home directory %q, got %q", cleanedHome, output)
	}
}
