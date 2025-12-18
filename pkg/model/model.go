package model

import "time"

// InboundMessage represents a message received from Telegram.
type InboundMessage struct {
	ChatID    string
	MessageID int64
	Text      string
	Timestamp time.Time
	Audio     *Audio
}

// Audio contains metadata and content for a received audio clip.
type Audio struct {
	FileID   string
	FileName string
	MimeType string
	Data     []byte
}

// Session captures minimal per-chat metadata.
type Session struct {
	ID             string
	AgentSessionID string
	ChatID         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
