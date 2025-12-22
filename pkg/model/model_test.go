package model

import "testing"

func TestAllowedChatExactMatch(t *testing.T) {
	if !AllowedChat("123", []string{"123", "456"}) {
		t.Fatalf("expected chat 123 to be allowed")
	}
	if AllowedChat("789", []string{"123", "456"}) {
		t.Fatalf("expected chat 789 to be rejected")
	}
}

func TestAllowedChatWildcard(t *testing.T) {
	if !AllowedChat("any-chat", []string{"*"}) {
		t.Fatalf("expected wildcard to allow any chat")
	}
	if !AllowedChat("123", []string{"*", "456"}) {
		t.Fatalf("expected wildcard mixed with other IDs to allow any chat")
	}
}
