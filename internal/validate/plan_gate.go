package validate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/plandag"
)

// planFile is the plan-stage source artifact (change 007, design D6): the
// plan is authored as YAML and validated by the milestoned-plan-dag
// primitive, not parsed by lifecycle.
const planFile = "plan.yaml"

// validatePlan is the plan-stage gate. It delegates plan validation to
// `milestoned-plan-dag validate <change>/plan.yaml` (a CLI/YAML process
// boundary, no Go import — design D6), passing the gate only when the plan
// primitive reports the plan valid and surfacing the primitive's own error
// verbatim when it reports the plan invalid.
//
// Failure taxonomy (mirrors Change's findings-vs-error split):
//   - plan.yaml absent            → a "missing_artifact" Finding (the plan
//     stage requires a plan, exactly as it required a tasks.md before).
//   - primitive reports invalid   → an "invalid_plan" Finding carrying the
//     primitive's message verbatim.
//   - primitive can't be run      → a non-nil error (could-not-run): the
//     binary isn't installed, or the exec itself failed. This is NOT a
//     validation result; `lifecycle init` preflights the binary and warns
//     when it is absent (see internal/scaffold).
func validatePlan(dir string) ([]Finding, error) {
	path := filepath.Join(dir, planFile)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: path, Kind: "missing_artifact",
				Message:  planFile + " not found",
				Severity: SeverityError,
			}}, nil
		}
		return nil, fmt.Errorf("validate: reading %s: %w", path, err)
	}

	bin, err := plandag.Locate("")
	if err != nil {
		return nil, fmt.Errorf("validate: plan stage: %w", err)
	}

	report, valid, err := plandag.Validate(bin, path)
	if err != nil {
		return nil, fmt.Errorf("validate: plan stage: %w", err)
	}
	if !valid {
		return []Finding{{
			File: path, Kind: "invalid_plan",
			Message:  fmt.Sprintf("milestoned-plan-dag reported the plan invalid: %s", report),
			Severity: SeverityError,
		}}, nil
	}
	return nil, nil
}
