package archive

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/plandag"
)

// planFile is the plan-stage source artifact the step-completion gate reads
// milestone done-states from (change 007, design D6).
const planFile = "plan.yaml"

// checkTasksComplete implements the step-completion gate (doc.go's
// "Gate-check reuse"): a change may not be archived while its plan reports
// any milestone not done. Milestone done-states are read from
// `milestoned-plan-dag resolve <change>/plan.yaml` — a CLI/YAML process
// boundary, no Go import (design D6) — replacing the previous tasks.md
// checkbox parse. Returns one human-readable violation string per
// outstanding milestone (nil means nothing is outstanding).
//
// The gate is deliberately silent — zero violations, not an error — when
// the change carries no plan.yaml at all (a delta-less bug fix, or any
// change that predates the plan-dag integration): nothing to resolve,
// nothing to gate. This preserves the same backward-compatible posture the
// tasks.md gate had for a change with no tasks.md.
//
// When a plan.yaml IS present, the milestoned-plan-dag binary is required:
// a change that authored a plan but whose plan primitive can't be run is a
// could-not-run error, not a silent pass (`lifecycle init` preflights the
// binary and warns when it is absent).
func checkTasksComplete(changeDir string) ([]string, error) {
	planPath := filepath.Join(changeDir, planFile)
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", planPath, err)
	}

	bin, err := plandag.Locate("")
	if err != nil {
		return nil, err
	}

	milestones, err := plandag.Resolve(bin, planPath)
	if err != nil {
		return nil, err
	}

	var violations []string
	for _, m := range milestones {
		if !m.Done {
			violations = append(violations, fmt.Sprintf(
				"milestone %d (%q) is not done", m.ID, m.Title,
			))
		}
	}
	return violations, nil
}
