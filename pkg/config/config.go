package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config describes the Call-my-agent configuration.
type Config struct {
	Telegram TelegramConfig `json:"telegram"`
	Inbound  InboundConfig  `json:"inbound"`
	Logging  LoggingConfig  `json:"logging"`
}

// TelegramConfig controls polling behavior.
type TelegramConfig struct {
	PollIntervalSeconds int `json:"pollIntervalSeconds"`
}

// InboundConfig holds inbound-processing configuration.
type InboundConfig struct {
	AllowFrom        []string    `json:"allowFrom"`
	Reply            ReplyConfig `json:"reply"`
	HeartbeatMinutes int         `json:"heartbeatMinutes"`
}

// ReplyConfig captures auto-reply settings.
type ReplyConfig struct {
	Mode           string        `json:"mode"`
	StaticText     string        `json:"staticText"`
	BodyPrefix     string        `json:"bodyPrefix"`
	Command        []string      `json:"command"`
	TimeoutSeconds int           `json:"timeoutSeconds"`
	Session        SessionConfig `json:"session"`
}

// SessionConfig controls session lifecycle.
type SessionConfig struct {
	Scope                string   `json:"scope"`
	IdleMinutes          int      `json:"idleMinutes"`
	ResetTriggers        []string `json:"resetTriggers"`
	HeartbeatIdleMinutes int      `json:"heartbeatIdleMinutes"`
}

// LoggingConfig controls log outputs.
type LoggingConfig struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// Load reads configuration from a JSON file path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, cfg.Validate()
}

// Validate checks required fields and sets defaults.
func (c *Config) Validate() error {
	if c.Inbound.Reply.Mode == "" {
		c.Inbound.Reply.Mode = "command"
	}
	if c.Telegram.PollIntervalSeconds <= 0 {
		c.Telegram.PollIntervalSeconds = 2
	}
	if c.Inbound.Reply.TimeoutSeconds <= 0 {
		c.Inbound.Reply.TimeoutSeconds = 600
	}
	if c.Inbound.Reply.Session.IdleMinutes <= 0 {
		c.Inbound.Reply.Session.IdleMinutes = 60
	}
	if c.Inbound.Reply.Session.Scope == "" {
		c.Inbound.Reply.Session.Scope = "per-chat"
	}
	if c.Inbound.Reply.Session.HeartbeatIdleMinutes == 0 {
		c.Inbound.Reply.Session.HeartbeatIdleMinutes = 240
	}
	if c.Inbound.HeartbeatMinutes < 0 {
		return fmt.Errorf("heartbeatMinutes cannot be negative")
	}
	if len(c.Inbound.Reply.Command) == 0 && c.Inbound.Reply.Mode == "command" {
		c.Inbound.Reply.Command = []string{"codex", "exec", "{{.Task}}"}
	}
	return nil
}

// Timeout returns the configured command timeout.
func (c Config) Timeout() time.Duration {
	return time.Duration(c.Inbound.Reply.TimeoutSeconds) * time.Second
}

// PollInterval returns the polling interval as a duration.
func (c Config) PollInterval() time.Duration {
	return time.Duration(c.Telegram.PollIntervalSeconds) * time.Second
}
