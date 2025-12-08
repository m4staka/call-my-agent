package config

import "testing"

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
