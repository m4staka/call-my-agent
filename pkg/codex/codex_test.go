package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"call-my-agent/pkg/model"
)

func TestBuildTaskIncludesPrefixAndHistory(t *testing.T) {
	sess := &model.Session{
		Messages: []model.Message{{Role: "assistant", Content: "Hi"}},
	}
	task := BuildTask("system prefix", sess, "new question")
	if !strings.Contains(task, "system prefix") || !strings.Contains(task, "Assistant: Hi") || !strings.Contains(task, "User: new question") {
		t.Fatalf("unexpected task content: %s", task)
	}
}

func TestPrepareTemplateData(t *testing.T) {
	data := PrepareTemplateData("123", "  body  ", "task")
	if data.BodyStripped != "body" {
		t.Fatalf("expected stripped body, got %q", data.BodyStripped)
	}
	if data.ChatID != "123" || data.Task != "task" {
		t.Fatalf("unexpected data fields %+v", data)
	}
}

func TestExecClientRunUsesWorkingDir(t *testing.T) {
	client := ExecClient{
		CommandTemplate: []string{"/bin/sh", "-c", "pwd"},
		WorkingDir:      t.TempDir(),
	}
	ctx := context.Background()
	output, err := client.Run(ctx, TemplateData{}, time.Second)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if output != client.WorkingDir {
		t.Fatalf("expected output %q, got %q", client.WorkingDir, output)
	}
}

func TestExecClientRunInvalidWorkingDir(t *testing.T) {
	client := ExecClient{
		CommandTemplate: []string{"/bin/sh", "-c", "pwd"},
		WorkingDir:      "/path/does/not/exist",
	}
	ctx := context.Background()
	output, err := client.Run(ctx, TemplateData{}, time.Second)
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
		CommandTemplate: []string{"/bin/sh", "-c", "pwd"},
		WorkingDir:      "project",
	}

	ctx := context.Background()
	output, err := client.Run(ctx, TemplateData{}, time.Second)
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
		CommandTemplate: []string{"/bin/sh", "-c", "pwd"},
		WorkingDir:      "~",
	}
	output, err = homeClient.Run(ctx, TemplateData{}, time.Second)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	cleanedHome := filepath.Clean(homeDir)
	if output != cleanedHome {
		t.Fatalf("expected home directory %q, got %q", cleanedHome, output)
	}
}
