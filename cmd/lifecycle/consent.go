package main

import "os"

// isTerminal reports whether f is an interactive character device. Used
// to pick between prompting and refusing under the strict consent
// policy (internal/approve.ConsentGate) — mirrors the sibling
// adr-sourced-constitution primitive's own cmd/constitution/writepath.go
// helper of the same name.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
