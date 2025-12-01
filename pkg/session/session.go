package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"call-my-agent/pkg/model"
)

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using time.Now.
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time { return time.Now() }

// Manager manages sessions per chat.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*model.Session
	idle     time.Duration
	resets   []string
	clock    Clock
	maxMsgs  int
}

// NewManager builds a Manager with configuration.
func NewManager(idleMinutes int, resets []string, maxMessages int, clock Clock) *Manager {
	if clock == nil {
		clock = RealClock{}
	}
	if maxMessages < 0 {
		maxMessages = 0
	}
	return &Manager{
		sessions: make(map[string]*model.Session),
		idle:     time.Duration(idleMinutes) * time.Minute,
		resets:   resets,
		clock:    clock,
		maxMsgs:  maxMessages,
	}
}

// Get returns the session for a chat, creating a new one if missing or reset.
func (m *Manager) Get(chatID, text string) (*model.Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.clock.Now()
	if sess, ok := m.sessions[chatID]; ok {
		if m.shouldReset(sess, text, now) {
			sess = m.newSession(chatID, now)
			m.sessions[chatID] = sess
			return sess, true
		}
		return sess, false
	}
	sess := m.newSession(chatID, now)
	m.sessions[chatID] = sess
	return sess, true
}

// Append records a message to a session and updates timestamps.
func (m *Manager) Append(sess *model.Session, role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendMessage(sess, role, content)
}

// AppendByID records a message by chat and session ID.
func (m *Manager) AppendByID(chatID, sessionID, role, content string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[chatID]
	if !ok || sess.ID != sessionID {
		return false
	}
	m.appendMessage(sess, role, content)
	return true
}

// ExpireIdleSessions removes sessions idle beyond the configured limit.
func (m *Manager) ExpireIdleSessions() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := false
	cutoff := m.clock.Now().Add(-m.idle)
	for chatID, sess := range m.sessions {
		if sess.UpdatedAt.Before(cutoff) {
			delete(m.sessions, chatID)
			removed = true
		}
	}
	return removed
}

// Snapshot returns a copy of all sessions.
func (m *Manager) Snapshot() []*model.Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*model.Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		copySess := *sess
		copySess.Messages = append([]model.Message(nil), sess.Messages...)
		out = append(out, &copySess)
	}
	return out
}

// Restore loads an existing session into the manager (used for persisted stores).
func (m *Manager) Restore(sess *model.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copySess := *sess
	copySess.Messages = append([]model.Message(nil), sess.Messages...)
	m.sessions[copySess.ChatID] = &copySess
}

func (m *Manager) shouldReset(sess *model.Session, text string, now time.Time) bool {
	if m.idle > 0 && now.Sub(sess.UpdatedAt) > m.idle {
		return true
	}
	for _, trigger := range m.resets {
		if strings.HasPrefix(strings.TrimSpace(text), trigger) {
			return true
		}
	}
	return false
}

func (m *Manager) newSession(chatID string, now time.Time) *model.Session {
	return &model.Session{
		ID:        newID(),
		ChatID:    chatID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (m *Manager) appendMessage(sess *model.Session, role, content string) {
	now := m.clock.Now()
	sess.Messages = append(sess.Messages, model.Message{Role: role, Content: content, Timestamp: now})
	if m.maxMsgs > 0 && len(sess.Messages) > m.maxMsgs {
		start := len(sess.Messages) - m.maxMsgs
		newMsgs := make([]model.Message, m.maxMsgs)
		copy(newMsgs, sess.Messages[start:])
		sess.Messages = newMsgs
	}
	sess.UpdatedAt = now
}
