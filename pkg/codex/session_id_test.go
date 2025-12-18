package codex

import "testing"

func TestExtractSessionID(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	jsonl := `{"type":"session_started","session_id":"` + id + `"}
{"type":"message","role":"assistant","content":"hello"}`
	got := extractSessionID(jsonl)
	if got != id {
		t.Fatalf("expected %q, got %q", id, got)
	}
}

func TestExtractSessionIDNested(t *testing.T) {
	id := "22222222-2222-2222-2222-222222222222"
	jsonl := `{"event":{"session":{"session_id":"` + id + `"}}}`
	got := extractSessionID(jsonl)
	if got != id {
		t.Fatalf("expected %q, got %q", id, got)
	}
}
