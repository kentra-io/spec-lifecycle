package approve

import (
	"fmt"

	"github.com/kentra-io/spec-lifecycle/internal/schema"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// featureArtifactIDs maps each feature-flow stage to the schema.yaml
// artifact id(s) it gates (spec-lifecycle.md §4's Stage column). The plan
// stage is NOT keyed here: change 007 (M6) removed the schema `tasks`
// artifact, so the plan/fix stages resolve to plan.yaml directly (see
// ArtifactGlobs), not via a schema glob.
var featureArtifactIDs = map[Stage][]string{
	StageRefine: {"proposal"},
	StageDesign: {"design"},
}

// specDeltaGlob is the refine/repro-stage delta artifact: the spec delta is
// authored and read as authoritative YAML (change 007, design D2), mirroring
// validate.ArtifactsForStage's "specs/**/spec.yaml". The schema's
// generates:"specs/**/spec.md" names the read-only living-spec markdown
// projection, not the change-folder delta source — so approve resolves the
// delta glob directly here rather than via def.Generates("specs").
const specDeltaGlob = "specs/**/spec.yaml"

// planArtifact is the plan/fix-stage source artifact: plan.yaml, validated
// by milestoned-plan-dag (change 007, design D6). M6 removed the schema
// `tasks` artifact, so approve points these stages at plan.yaml directly,
// mirroring validate.ArtifactsForStage(StagePlan) == "plan.yaml".
const planArtifact = "plan.yaml"

// ArtifactGlobs returns the generates: glob pattern(s) that gate stage. The
// proposal/design artifacts resolve via the embedded kentra-spec-lifecycle
// schema's generates: globs (implementation-plan.md §2.6); the spec delta
// (refine/repro) and the plan (plan/fix) resolve to their YAML source paths
// directly (change 007 — the schema no longer names a `tasks` artifact, and
// its `specs` glob names the markdown projection, not the YAML delta). See
// doc.go's "Bug-flow artifact reuse" for why StageRepro/StageFix reuse the
// SAME globs as StageRefine's proposal(+specs)/StagePlan's plan.
func ArtifactGlobs(def *schema.Definition, stage Stage) ([]string, error) {
	switch stage {
	case StageRefine, StageDesign:
		ids := featureArtifactIDs[stage]
		globs := make([]string, 0, len(ids)+1)
		for _, id := range ids {
			g := def.Generates(id)
			if g == "" {
				return nil, fmt.Errorf("approve: schema has no generates: glob for artifact %q", id)
			}
			globs = append(globs, g)
		}
		if stage == StageRefine {
			globs = append(globs, specDeltaGlob)
		}
		return globs, nil
	case StageRepro:
		g := def.Generates("proposal")
		if g == "" {
			return nil, fmt.Errorf("approve: schema has no generates: glob for artifact %q", "proposal")
		}
		return []string{g, specDeltaGlob}, nil
	case StagePlan, StageFix:
		return []string{planArtifact}, nil
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
// specs/ delta ONLY when one is present (a promoted bug); fix validates the
// plan the same way the plan stage does — via validate.Plan, which delegates
// to milestoned-plan-dag over plan.yaml (change 007, M6 retired tasks.md).
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
		// A promoted bug additionally validates its specs/ delta, ONLY when
		// one is present. Change 007 (design D2): the delta is authoritative
		// YAML, so detection keys on specs/**/spec.yaml (resolveArtifactFiles
		// over the same glob approve hashes), not the retired markdown spec.md.
		deltaFiles, err := resolveArtifactFiles(dir, specDeltaGlob)
		if err != nil {
			return nil, err
		}
		if len(deltaFiles) > 0 {
			deltaFindings, err := validate.SpecsDeltas(dir)
			if err != nil {
				return nil, err
			}
			findings = append(findings, deltaFindings...)
		}
		return findings, nil
	case StageFix:
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
