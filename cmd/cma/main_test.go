package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test_run_help verifies that help-style invocations do not error.
func Test_run_help(t *testing.T) {
	if err := run([]string{"help"}); err != nil {
		t.Fatalf("run(help) returned error: %v", err)
	}
	if err := run([]string{"-h"}); err != nil {
		t.Fatalf("run(-h) returned error: %v", err)
	}
	if err := run([]string{"--help"}); err != nil {
		t.Fatalf("run(--help) returned error: %v", err)
	}
}

// Test_run_unknownCommand ensures unknown commands surface a clear error.
func Test_run_unknownCommand(t *testing.T) {
	if err := run([]string{"wat"}); err == nil {
		t.Fatalf("expected error for unknown command, got nil")
	}
}

// Test_run_noArgs prints usage and returns an error.
func Test_run_noArgs(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatalf("expected error when no args are provided")
	}
}

// Test_run_status_smoke exercises the status command wiring using a temporary config.
// It does not assert on output, only that no error is returned.
func Test_run_status_smoke(t *testing.T) {
	dir := t.TempDir()

	cfgPath := filepath.Join(dir, "config.json")
	sessionsPath := filepath.Join(dir, "sessions.json")
	logPath := filepath.Join(dir, "cma.log")

	cfg := `{
  "telegram": { "pollIntervalSeconds": 1 },
  "inbound": {
    "allowFrom": ["123"],
    "reply": {
      "mode": "static",
      "staticText": "ok",
      "bodyPrefix": "",
      "command": [],
      "cwd": "",
      "timeoutSeconds": 1,
      "session": {
        "scope": "per-chat",
        "idleMinutes": 1,
        "resetTriggers": [],
        "heartbeatIdleMinutes": 0,
        "maxMessages": 10
      }
    },
    "heartbeatMinutes": 0
  },
  "logging": {
    "level": "silent",
    "file": "` + logPath + `"
  },
  "sessionStorePath": "` + sessionsPath + `"
}
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := run([]string{"status", "-config", cfgPath}); err != nil {
		t.Fatalf("run(status) returned error: %v", err)
	}
}


