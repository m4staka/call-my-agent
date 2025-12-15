package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsEmptyAllowFrom(t *testing.T) {
	cfg := Config{
		Inbound: InboundConfig{
			AllowFrom: nil,
			Reply: ReplyConfig{
				Mode: "static",
				Session: SessionConfig{
					IdleMinutes: 1,
					MaxMessages: 1,
				},
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error when inbound.allowFrom is empty, got nil")
	}
}

func TestValidateAcceptsNonEmptyAllowFrom(t *testing.T) {
	cfg := Config{
		Inbound: InboundConfig{
			AllowFrom: []string{"123"},
			Reply: ReplyConfig{
				Mode: "static",
				Session: SessionConfig{
					IdleMinutes: 1,
					MaxMessages: 1,
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nil error for non-empty inbound.allowFrom, got %v", err)
	}
}

func TestValidateResolvesPaths(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	cfg := Config{
		Inbound: InboundConfig{
			AllowFrom: []string{"123"},
			Reply: ReplyConfig{
				Mode: "static",
				Session: SessionConfig{
					IdleMinutes: 1,
					MaxMessages: 1,
				},
			},
		},
		Logging: LoggingConfig{File: "logs/cma.log"},
		// Intentionally use a tilde path to ensure expansion.
		SessionStorePath: "~/.cma/sessions.json",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	expectedStore := filepath.Join(tmpHome, ".cma", "sessions.json")
	if cfg.SessionStorePath != expectedStore {
		t.Fatalf("expected sessionStorePath %q, got %q", expectedStore, cfg.SessionStorePath)
	}

	expectedLog := filepath.Join(wd, "logs", "cma.log")
	if cfg.Logging.File != expectedLog {
		t.Fatalf("expected logging.file %q, got %q", expectedLog, cfg.Logging.File)
	}
}

func TestValidateSetsDefaultAgent(t *testing.T) {
	cfg := Config{
		Inbound: InboundConfig{
			AllowFrom: []string{"123"},
			Reply: ReplyConfig{
				Mode: "static",
				Session: SessionConfig{
					IdleMinutes: 1,
					MaxMessages: 1,
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if cfg.Inbound.Reply.Agent != "codex" {
		t.Fatalf("expected default agent codex, got %q", cfg.Inbound.Reply.Agent)
	}
}

func TestValidateRejectsUnknownAgent(t *testing.T) {
	cfg := Config{
		Inbound: InboundConfig{
			AllowFrom: []string{"123"},
			Reply: ReplyConfig{
				Mode:  "static",
				Agent: "unknown",
				Session: SessionConfig{
					IdleMinutes: 1,
					MaxMessages: 1,
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for unknown runner")
	}
}
