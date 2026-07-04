package validate

import (
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Change type vocabulary (spec-lifecycle.md §8, §0's Stage glossary).
// Recorded at intake in proposal.md's frontmatter (this milestone's own
// decision — see ProposalMeta's doc comment) and consumed by
// internal/approve/internal/status to key the bug-vs-feature stage set
// (implementation-plan.md §2.11).
const (
	ChangeTypeFeature = "feature"
	ChangeTypeBug     = "bug"
)

// ProposalMeta is the intake-time metadata proposal.md's frontmatter
// carries beyond the pass/fail structural check validateProposal (Proposal)
// already performs: the sourceTracking join key, a proposed design-skip,
// and the change type.
//
// Type is spec-lifecycle.md §8/§2.11's "change type recorded at intake" —
// the spec names the concept ("status/guard/archive gate-checks key off
// the change type recorded at intake") but does not pin exactly where
// that's recorded. This package decides: proposal.md's frontmatter, a
// `type: feature|bug` field alongside the already-load-bearing `issue`
// and `designSkip` fields (schema.yaml's proposal artifact documents it
// for the same reason). Absent (or non-string) is ChangeTypeFeature — a
// proposal.md written before this decision landed still parses.
type ProposalMeta struct {
	Issue      string
	DesignSkip bool
	Type       string
}

// ReadProposalMeta reads dir/proposal.md's frontmatter fields. It does
// NOT itself validate structure — call Proposal(dir) (or Change(dir,
// StageRefine)) first if that matters to the caller. A missing file or
// unparsable/absent frontmatter yields a zero ProposalMeta (Type defaults
// to ChangeTypeFeature) and a nil error: this is metadata extraction for
// an already-validated (or not-yet-existing) artifact, not a second
// validation pass.
func ReadProposalMeta(dir string) (ProposalMeta, error) {
	meta := ProposalMeta{Type: ChangeTypeFeature}

	data, err := os.ReadFile(filepath.Join(dir, proposalFile))
	if err != nil {
		if os.IsNotExist(err) {
			return meta, nil
		}
		return ProposalMeta{}, err
	}

	fm, ok := splitFrontmatter(data)
	if !ok {
		return meta, nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(fm), &raw); err != nil {
		return meta, nil //nolint:nilerr // malformed frontmatter is Proposal(dir)'s finding to report, not this extractor's error
	}

	if issue, ok := raw["issue"].(string); ok {
		meta.Issue = issue
	}
	if skip, ok := raw["designSkip"].(bool); ok {
		meta.DesignSkip = skip
	}
	if t, ok := raw["type"].(string); ok && strings.TrimSpace(t) != "" {
		meta.Type = strings.TrimSpace(t)
	}
	return meta, nil
}

// Proposal exports validateProposal for callers outside this package that
// need proposal.md's own structural check without going through Change's
// three-stage dispatch (internal/approve's bug-flow "repro" gate, which
// checks proposal.md but — unlike StageRefine — does not always require a
// specs/ delta; spec-lifecycle.md §8).
func Proposal(dir string) ([]Finding, error) { return validateProposal(dir) }

// Design exports validateDesign — see Proposal's doc comment.
func Design(dir string) ([]Finding, error) { return validateDesign(dir) }

// Plan exports validatePlan — see Proposal's doc comment. Used directly
// by internal/approve's bug-flow "fix" gate when tasks.md is present
// (spec-lifecycle.md §8: "tasks.md optional").
func Plan(dir string) ([]Finding, error) { return validatePlan(dir) }

// SpecsDeltas exports validateSpecsDeltas — see Proposal's doc comment.
func SpecsDeltas(dir string) ([]Finding, error) { return validateSpecsDeltas(dir) }

// HasSpecsDeltas reports whether dir/specs contains at least one spec.md
// delta file — used by internal/approve to decide whether a promoted
// bug's "repro" gate should also validate a specs/ delta (spec-lifecycle.md
// §8: "If the repro reveals mis-specced behavior... gate it like a
// feature refine").
func HasSpecsDeltas(dir string) bool {
	paths, err := findSpecFiles(filepath.Join(dir, specsDir))
	return err == nil && len(paths) > 0
}

// HasArtifact reports whether the named file (e.g. "tasks.md") exists
// directly under dir.
func HasArtifact(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
