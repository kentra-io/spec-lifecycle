package main

import (
	"os"

	"golang.org/x/term"
)

// isTerminal reports whether f is an interactive terminal. Used to pick
// between prompting and refusing under the strict consent policy
// (internal/approve.ConsentGate). Uses a real TTY check (golang.org/x/term)
// rather than the ModeCharDevice heuristic: character devices like
// /dev/null and /dev/zero are not terminals, but a bare Mode() check would
// misclassify them as one, wiring up an interactive confirm prompt (and,
// for a genuinely-blocking device, hanging on read) in what is actually a
// non-interactive run.
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
