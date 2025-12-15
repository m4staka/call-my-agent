package main

import "testing"

// Test_run_help verifies that help-style invocations do not error.
func Test_run_help(t *testing.T) {
	if err := run([]string{"help"}); err != nil {
		t.Fatalf("run(help) returned error: %v", err)
	}
	if err := run([]string{"-h"}); err != nil {
		t.Fatalf("run(-h) returned error: %v", err)
	}
	if err := run([]string{"--help"}); err != nil {
		t.Fatalf("run(--help) returned error: %v", err)
	}
}

// Test_run_unknownCommand ensures unknown commands surface a clear error.
func Test_run_unknownCommand(t *testing.T) {
	if err := run([]string{"wat"}); err == nil {
		t.Fatalf("expected error for unknown command, got nil")
	}
}

// Test_run_noArgs prints usage and returns an error.
func Test_run_noArgs(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatalf("expected error when no args are provided")
	}
}
