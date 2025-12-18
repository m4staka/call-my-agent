package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"call-my-agent/pkg/model"
)

// SessionStore persists sessions between CLI invocations.
type SessionStore interface {
	Load() ([]*model.Session, error)
	Save([]*model.Session) error
}

// FileSessionStore stores sessions in a JSON file refreshed on each save.
type FileSessionStore struct {
	Path string
}

// NewFileSessionStore returns a FileSessionStore pointing at path.
func NewFileSessionStore(path string) *FileSessionStore {
	return &FileSessionStore{Path: path}
}

// Load reads all sessions from disk.
func (s *FileSessionStore) Load() ([]*model.Session, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session store: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var sessions []*model.Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parse session store: %w", err)
	}
	return sessions, nil
}

// Save writes all sessions to disk atomically.
func (s *FileSessionStore) Save(sessions []*model.Session) error {
	if s.Path == "" {
		return fmt.Errorf("session store path is empty")
	}
	dir := filepath.Dir(s.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create session store dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp session store: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("replace session store: %w", err)
	}
	return nil
}
