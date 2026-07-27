package plandag

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Validate runs `<bin> validate <planPath>` (design D6): the plan-stage
// gate delegates plan validation to the milestoned-plan-dag primitive.
//
//   - valid=true, err=nil        — the primitive reported the plan valid
//     (exit 0).
//   - valid=false, err=nil       — the primitive RAN and reported the plan
//     invalid (non-zero exit). report carries the primitive's own
//     validation message (combined stdout+stderr), surfaced verbatim by
//     the plan gate.
//   - err!=nil                    — the primitive could not be run at all
//     (not executable, killed by a signal, …); this is a could-not-run
//     condition, distinct from an invalid-plan result.
func Validate(bin, planPath string) (report string, valid bool, err error) {
	cmd := exec.Command(bin, "validate", planPath) //nolint:gosec // bin is caller-resolved (Locate), not untrusted input
	out, runErr := cmd.CombinedOutput()
	report = strings.TrimSpace(string(out))
	if runErr == nil {
		return report, true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		// Ran to completion with a non-zero exit: an INVALID plan, not a
		// failure to run. Fall back to the exit status when the primitive
		// printed nothing, so the surfaced message is never empty.
		if report == "" {
			report = fmt.Sprintf("%s validate exited %d with no message", BinName, exitErr.ExitCode())
		}
		return report, false, nil
	}
	return report, false, fmt.Errorf("%s validate %s: %w", BinName, planPath, runErr)
}

// Milestone is one milestone's done-state as reported by
// `milestoned-plan-dag resolve` (design D6). id/title identify the
// milestone in a human-readable refusal; done is the completion state the
// archive step-completion gate refuses on.
type Milestone struct {
	ID    int    `yaml:"id"`
	Title string `yaml:"title"`
	Done  bool   `yaml:"done"`
}

// resolveOutput is the YAML document `milestoned-plan-dag resolve` emits on
// stdout — a `milestones:` sequence, matching the YAML-everywhere family
// posture. This is the CLI/YAML boundary contract this package relies on
// (there is no Go import of the primitive); the unit tests exercise it with
// a stub binary emitting exactly this shape.
type resolveOutput struct {
	Milestones []Milestone `yaml:"milestones"`
}

// Resolve runs `<bin> resolve <planPath>` and returns the plan's milestone
// done-states (design D6). A non-zero exit or unparseable output is a
// could-not-run error; the archive gate turns the returned milestones into
// a refusal naming any that are not done.
func Resolve(bin, planPath string) ([]Milestone, error) {
	cmd := exec.Command(bin, "resolve", planPath) //nolint:gosec // bin is caller-resolved (Locate), not untrusted input
	out, runErr := cmd.Output()
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%s resolve %s: %w: %s", BinName, planPath, runErr, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("%s resolve %s: %w", BinName, planPath, runErr)
	}
	var doc resolveOutput
	if err := yaml.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("%s resolve %s: unparseable output: %w", BinName, planPath, err)
	}
	return doc.Milestones, nil
}
