package codex

import (
	"strings"
	"testing"

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
