package codex

import (
	"context"
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
		CommandTemplate: []string{"/bin/echo", "hello"},
		WorkingDir:      "/path/does/not/exist",
	}
	ctx := context.Background()
	_, err := client.Run(ctx, TemplateData{}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "invalid cwd") {
		t.Fatalf("expected invalid cwd error, got %v", err)
	}
}
