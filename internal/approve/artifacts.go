package approve

import (
	"fmt"

	"github.com/kentra-io/spec-lifecycle/internal/schema"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// featureArtifactIDs maps each feature-flow stage to the schema.yaml
// artifact id(s) it gates (spec-lifecycle.md §4's Stage column).
var featureArtifactIDs = map[Stage][]string{
	StageRefine: {"proposal", "specs"},
	StageDesign: {"design"},
	StagePlan:   {"tasks"},
}

// ArtifactGlobs returns the generates: glob pattern(s) that gate stage,
// resolved from the embedded kentra-spec-lifecycle schema
// (implementation-plan.md §2.6: "resolve the stage's artifact set via the
// schema's generates: globs"). See doc.go's "Bug-flow artifact reuse" for
// why StageRepro/StageFix resolve to the SAME globs as StageRefine's
// proposal(+specs)/StagePlan's tasks rather than a second hand-typed
// literal.
func ArtifactGlobs(def *schema.Definition, stage Stage) ([]string, error) {
	switch stage {
	case StageRefine, StageDesign, StagePlan:
		ids := featureArtifactIDs[stage]
		globs := make([]string, 0, len(ids))
		for _, id := range ids {
			g := def.Generates(id)
			if g == "" {
				return nil, fmt.Errorf("approve: schema has no generates: glob for artifact %q", id)
			}
			globs = append(globs, g)
		}
		return globs, nil
	case StageRepro:
		globs := []string{def.Generates("proposal")}
		if g := def.Generates("specs"); g != "" {
			globs = append(globs, g)
		}
		return globs, nil
	case StageFix:
		return []string{def.Generates("tasks")}, nil
	default:
		return nil, fmt.Errorf("approve: unrecognized stage %q (want one of %v)", stage, Stages)
	}
}

// requiresDeviation reports whether stage is one of gates 2/3 (design,
// plan) — the ONLY stages that require + validate deviation.json
// (spec-lifecycle.md §3.3/§7 item 5), regardless of the change's type. A
// promoted bug literally inserts stages NAMED "design"/"plan"
// (spec-lifecycle.md §8's promotion hatch), so keying on the stage name
// alone — not the change type — is exactly right: repro/fix never run
// the plan-gate, design/plan always do, promoted or not.
func requiresDeviation(stage Stage) bool {
	return stage == StageDesign || stage == StagePlan
}

// validateForStage runs the SAME validation code path `lifecycle
// validate` uses (implementation-plan.md §2.6: "never approve an invalid
// artifact"). For the three feature stages this is exactly
// validate.Change; for the bug flow's repro/fix, see doc.go's "Bug-flow
// artifact reuse" — repro always checks proposal.md, and additionally the
// specs/ delta ONLY when one is present (a promoted bug); fix checks
// tasks.md's structure only when tasks.md exists at all (spec-lifecycle.md
// §8: "tasks.md optional" — nothing to validate when it's absent).
func validateForStage(dir string, stage Stage) ([]validate.Finding, error) {
	switch stage {
	case StageRefine:
		return validate.Change(dir, validate.StageRefine)
	case StageDesign:
		return validate.Change(dir, validate.StageDesign)
	case StagePlan:
		return validate.Change(dir, validate.StagePlan)
	case StageRepro:
		findings, err := validate.Proposal(dir)
		if err != nil {
			return nil, err
		}
		if validate.HasSpecsDeltas(dir) {
			deltaFindings, err := validate.SpecsDeltas(dir)
			if err != nil {
				return nil, err
			}
			findings = append(findings, deltaFindings...)
		}
		return findings, nil
	case StageFix:
		if !validate.HasArtifact(dir, "tasks.md") {
			return nil, nil
		}
		return validate.Plan(dir)
	default:
		return nil, fmt.Errorf("approve: unrecognized stage %q (want one of %v)", stage, Stages)
	}
}

// hasError reports whether findings contains at least one error-severity
// Finding (warnings never block a write).
func hasError(findings []validate.Finding) bool {
	for _, f := range findings {
		if f.Severity == validate.SeverityError {
			return true
		}
	}
	return false
}
