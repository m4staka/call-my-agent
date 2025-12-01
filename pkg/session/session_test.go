package session

import (
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestSessionResetOnIdle(t *testing.T) {
	clk := &fakeClock{now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
	mgr := NewManager(1, nil, clk)
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
	mgr := NewManager(10, []string{"/new"}, clk)
	sess1, _ := mgr.Get("chat", "hello")
	sess2, reset := mgr.Get("chat", "/new")
	if !reset {
		t.Fatalf("expected reset due to trigger")
	}
	if sess1.ID == sess2.ID {
		t.Fatalf("expected new session after trigger")
	}
}
