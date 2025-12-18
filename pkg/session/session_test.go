package session

import (
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestSessionResetOnIdle(t *testing.T) {
	clk := &fakeClock{now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	mgr := NewManager(1, nil, 0, clk)
	sess1, _ := mgr.Get("chat", "hello")
	clk.now = clk.now.Add(2 * time.Minute)
	sess2, reset := mgr.Get("chat", "hi again")
	if !reset {
		t.Fatalf("expected reset after idle")
	}
	if sess1.ID == sess2.ID {
		t.Fatalf("expected new session ID after reset")
	}
}

func TestResetTrigger(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, []string{"/new"}, 0, clk)
	sess1, _ := mgr.Get("chat", "hello")
	sess2, reset := mgr.Get("chat", "/new extra text")
	if !reset {
		t.Fatalf("expected reset due to trigger")
	}
	if sess1.ID == sess2.ID {
		t.Fatalf("expected new session after trigger")
	}
	sess3, reset := mgr.Get("chat", "   /new again")
	if !reset {
		t.Fatalf("expected reset for whitespace-prefixed trigger")
	}
	if sess2.ID == sess3.ID {
		t.Fatalf("expected new session after whitespace trigger")
	}
}

func TestResetTriggerInBatchedMessage(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, []string{"/new"}, 0, clk)
	sess1, _ := mgr.Get("chat", "first pending message")
	if sess1.ID == "" {
		t.Fatalf("expected initial session")
	}
	combined := "status update\n\n/new start over\n\nnext task"
	sess2, reset := mgr.Get("chat", combined)
	if !reset {
		t.Fatalf("expected reset when later line starts with trigger")
	}
	if sess1.ID == sess2.ID {
		t.Fatalf("expected new session ID, got same %q", sess1.ID)
	}
}

func TestAppendByID(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, nil, 0, clk)
	sess, _ := mgr.Get("chat", "hello")
	updatedAt := sess.UpdatedAt
	clk.now = clk.now.Add(time.Minute)
	if ok := mgr.AppendByID("chat", sess.ID, "assistant", "reply"); !ok {
		t.Fatalf("expected append to succeed")
	}
	snapshot := mgr.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected one recorded session, got %+v", snapshot)
	}
	if !snapshot[0].UpdatedAt.After(updatedAt) {
		t.Fatalf("expected UpdatedAt to change after append")
	}
	if ok := mgr.AppendByID("chat", "other", "assistant", "nope"); ok {
		t.Fatalf("expected append to fail for mismatched session")
	}
}

func TestSnapshotDeepCopy(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, nil, 0, clk)
	_, _ = mgr.Get("chat", "hello")
	snapshot := mgr.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot")
	}
	snapshot[0].ID = "different"
	if mgr.Snapshot()[0].ID == "different" {
		t.Fatalf("modifying snapshot should not mutate manager state")
	}
}
