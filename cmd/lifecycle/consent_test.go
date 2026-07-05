package main

import (
	"os"
	"testing"
)

// TestIsTerminal_DevNull is the regression guard for the /dev/null prompt
// leak: os.DevNull is a character device (on Unix), not a terminal, so
// isTerminal must report false for it. A bare os.ModeCharDevice check gets
// this wrong (character devices are not necessarily terminals), which is
// what wired up a spurious interactive confirm prompt — and, for a
// genuinely-blocking character device, would hang on read — during
// non-interactive runs (e.g. `lifecycle init < /dev/null`).
func TestIsTerminal_DevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()

	if isTerminal(f) {
		t.Errorf("isTerminal(%s) = true, want false: not a terminal", os.DevNull)
	}
}

// TestIsTerminal_RegularFile mirrors the non-interactive case exercised via
// stdin redirection from a file; a regular file must never be treated as a
// terminal.
func TestIsTerminal_RegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "isterminal")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if isTerminal(f) {
		t.Errorf("isTerminal(regular file) = true, want false")
	}
}
