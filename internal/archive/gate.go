package archive

import (
	"fmt"

	"github.com/kentra-io/spec-lifecycle/internal/status"
)

// checkGates implements step 1 (doc.go's gate-check reuse): it derives the
// change's full gate-state report via internal/status.Change — the SAME
// bug-vs-feature base stage set, unioned with whatever promoted stage
// names actually appear (spec-lifecycle.md §8/§2.11) — and returns one
// human-readable violation string per stage that is neither approved nor
// (legitimately) skipped. A nil/empty result means every required gate is
// satisfied.
func checkGates(changeDir string) ([]string, error) {
	cs, err := status.Change(changeDir)
	if err != nil {
		return nil, err
	}

	var violations []string
	for _, g := range cs.Gates {
		switch g.State {
		case status.StateApproved, status.StateSkipped:
			continue
		default:
			violations = append(violations, fmt.Sprintf("%s: %s", g.Stage, g.State))
		}
	}
	return violations, nil
}
