package pi

import (
	"os"
	"path/filepath"
	"testing"

	"call-my-agent/pkg/agent"
)

func TestBuildPiCommandWithSession(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	args, err := buildPiCommand(agent.Request{Task: "do it", SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("buildPiCommand returned error: %v", err)
	}
	expectedPath := filepath.Join(tmp, ".pi", "agent", "sessions", "sess-1.jsonl")
	if len(args) != 5 || args[0] != "pi" || args[1] != "-p" || args[2] != "--session" || args[3] != expectedPath || args[4] != "do it" {
		t.Fatalf("unexpected args %+v", args)
	}
	if _, err := os.Stat(filepath.Dir(expectedPath)); err != nil {
		t.Fatalf("expected session dir to be created: %v", err)
	}
}

func TestSessionFilePathRequiresID(t *testing.T) {
	if _, err := sessionFilePath(""); err == nil {
		t.Fatalf("expected error for empty session id")
	}
}
