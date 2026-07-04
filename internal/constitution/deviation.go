package constitution

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// DeviationExit mirrors `constitution deviation validate`'s exit-code
// contract (doc.go's spike note; ADR-0009 in the companion primitive).
type DeviationExit int

// The three exit codes `constitution deviation validate` reports.
const (
	DeviationValid       DeviationExit = 0
	DeviationInvalid     DeviationExit = 1
	DeviationCouldNotRun DeviationExit = 2
)

// DeviationResult is one `constitution deviation validate` invocation's
// outcome: the exit code plus the captured streams (Stderr carries both
// the advisory staleness note, printed regardless of exit code, and — on
// exit 1 — the schema/citation/tally error lines).
type DeviationResult struct {
	ExitCode DeviationExit
	Stdout   string
	Stderr   string
}

// Valid reports whether the deviation.json passed (exit 0). A valid
// result MAY still carry an advisory in Stderr (e.g. a stale
// constitutionHash) — Valid() does not inspect that.
func (r DeviationResult) Valid() bool { return r.ExitCode == DeviationValid }

// DeviationValidate runs `<bin> deviation validate <path>` with its
// working directory set to root. Per the verb's own contract (doc.go's
// spike note), root MUST be "a constitution project root" — a directory
// containing constitution.yml and constitution/adr/ — and path SHOULD be
// absolute so it resolves correctly regardless of root (callers are not
// required to pass an absolute path, but a relative one resolves against
// root, not the caller's own cwd, exactly like running the command by
// hand from that directory would).
//
// A non-nil error means the process itself could not be started/run (e.g.
// bin is not executable) — distinct from DeviationCouldNotRun, which is
// the verb's OWN exit-2 "could not run" outcome (unreadable path, root
// isn't a constitution project root, ADR log unreadable) and is reported
// as a normal (err=nil) DeviationResult so callers can inspect Stderr for
// why.
func DeviationValidate(bin, root, path string) (DeviationResult, error) {
	cmd := exec.Command(bin, "deviation", "validate", path) //nolint:gosec // bin/root/path are caller-resolved (Locate + project layout), not arbitrary user input
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res := DeviationResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		res.ExitCode = DeviationValid
		return res, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = DeviationExit(exitErr.ExitCode())
		return res, nil
	}
	return DeviationResult{}, fmt.Errorf("constitution deviation validate: %w", err)
}
