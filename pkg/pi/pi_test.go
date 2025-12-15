package pi

import (
	"context"
	"strings"
	"testing"
	"time"

	"call-my-agent/pkg/agent"
)

func TestBuildPiCommand(t *testing.T) {
	args, err := buildPiCommand(agent.Request{Task: "do thing"})
	if err != nil {
		t.Fatalf("buildPiCommand returned error: %v", err)
	}
	want := []string{"pi", "-p", "--no-session", "do thing"}
	if len(args) != len(want) {
		t.Fatalf("expected args %v, got %v", want, args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("expected args %v, got %v", want, args)
		}
	}
}

func TestClientRunPropagatesWorkingDir(t *testing.T) {
	client := Client{
		WorkingDir: t.TempDir(),
		commandBuilder: func(req agent.Request) ([]string, error) {
			return []string{"/bin/sh", "-c", "pwd"}, nil
		},
	}
	output, err := client.Run(context.Background(), agent.Request{Timeout: time.Second, Task: "pwd"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(output, client.WorkingDir) {
		t.Fatalf("expected output to include working dir %q, got %q", client.WorkingDir, output)
	}
}
