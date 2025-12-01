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

func TestAppendByID(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, nil, 0, clk)
	sess, _ := mgr.Get("chat", "hello")
	if ok := mgr.AppendByID("chat", sess.ID, "assistant", "reply"); !ok {
		t.Fatalf("expected append to succeed")
	}
	snapshot := mgr.Snapshot()
	if len(snapshot) != 1 || len(snapshot[0].Messages) != 1 {
		t.Fatalf("expected one recorded message, got %+v", snapshot)
	}
	if ok := mgr.AppendByID("chat", "other", "assistant", "nope"); ok {
		t.Fatalf("expected append to fail for mismatched session")
	}
}

func TestMaxMessagesBound(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, nil, 2, clk)
	sess, _ := mgr.Get("chat", "hello")
	mgr.Append(sess, "user", "m1")
	mgr.Append(sess, "assistant", "m2")
	mgr.Append(sess, "user", "m3")
	snapshot := mgr.Snapshot()
	if len(snapshot[0].Messages) != 2 {
		t.Fatalf("expected messages to be bounded, got %d", len(snapshot[0].Messages))
	}
	if snapshot[0].Messages[0].Content != "m2" || snapshot[0].Messages[1].Content != "m3" {
		t.Fatalf("unexpected messages %+v", snapshot[0].Messages)
	}
}

func TestSnapshotDeepCopy(t *testing.T) {
	clk := &fakeClock{now: time.Now()}
	mgr := NewManager(10, nil, 0, clk)
	sess, _ := mgr.Get("chat", "hello")
	mgr.Append(sess, "user", "hi")
	snapshot := mgr.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot")
	}
	snapshot[0].Messages = nil
	if len(mgr.Snapshot()[0].Messages) == 0 {
		t.Fatalf("modifying snapshot should not mutate manager state")
	}
}
