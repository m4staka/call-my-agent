package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"call-my-agent/pkg/model"
)

func TestFileSessionStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	store := NewFileSessionStore(path)

	sessions := []*model.Session{{
		ID:        "sess",
		ChatID:    "123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []model.Message{
			{Role: "user", Content: "hi"},
		},
	}}
	if err := store.Save(sessions); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ChatID != "123" || len(loaded[0].Messages) != 1 {
		t.Fatalf("unexpected sessions %+v", loaded)
	}
}

func TestFileSessionStoreLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	store := NewFileSessionStore(path)
	sessions, err := store.Load()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be absent, stat err=%v", err)
	}
}

