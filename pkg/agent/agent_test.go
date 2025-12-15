package agent

import (
	"strings"
	"testing"

	"call-my-agent/pkg/model"
)

func TestBuildTask(t *testing.T) {
	sess := &model.Session{
		Messages: []model.Message{
			{Role: "assistant", Content: "Hi"},
		},
	}
	task := BuildTask("system prefix", sess, "new question")
	if task == "" {
		t.Fatalf("expected task content")
	}
	if want := "system prefix"; !strings.Contains(task, want) {
		t.Fatalf("expected prefix %q in task %q", want, task)
	}
	if want := "Assistant: Hi"; !strings.Contains(task, want) {
		t.Fatalf("expected assistant history %q in task %q", want, task)
	}
	if want := "User: new question"; !strings.Contains(task, want) {
		t.Fatalf("expected user text %q in task %q", want, task)
	}
}

func TestPrepareRequest(t *testing.T) {
	req := PrepareRequest("123", "  body  ", "task", 0)
	if req.BodyStripped != "body" {
		t.Fatalf("expected stripped body, got %q", req.BodyStripped)
	}
	if req.ChatID != "123" || req.Task != "task" {
		t.Fatalf("unexpected request fields %+v", req)
	}
}
