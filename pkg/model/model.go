package model

import "time"

// InboundMessage represents a message received from Telegram.
type InboundMessage struct {
	ChatID    string
	MessageID int64
	Text      string
	Timestamp time.Time
}

// Message represents a conversation turn stored in a session.
type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// Session captures a per-chat conversation history.
type Session struct {
	ID        string
	ChatID    string
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AllowedChat returns true when the inbound chat ID is permitted.
func AllowedChat(chatID string, allowed []string) bool {
	for _, allowedID := range allowed {
		if allowedID == "*" || chatID == allowedID {
			return true
		}
	}
	return false
}
