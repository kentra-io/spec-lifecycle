package validate

import (
	"fmt"
	"sort"
)

// Change validates the artifact(s) a stage gates inside a single change
// folder (dir — e.g. "openspec/changes/042-user-auth", absolute or
// relative; every Finding's File is derived from dir exactly as given).
// Findings are returned in a stable order (by File, then Line); a
// non-nil error means validation could not even be attempted (e.g. a
// change directory that exists but isn't readable) — distinct from
// findings, which are validation *results*, not failures to run.
func Change(dir string, stage Stage) ([]Finding, error) {
	var (
		findings []Finding
		err      error
	)
	switch stage {
	case StageRefine:
		findings, err = validateRefine(dir)
	case StageDesign:
		findings, err = validateDesign(dir)
	case StagePlan:
		findings, err = validatePlan(dir)
	default:
		return nil, fmt.Errorf("validate: unrecognized stage %q (want one of %v)", stage, Stages)
	}
	if err != nil {
		return nil, err
	}
	sortFindings(findings)
	return findings, nil
}

func validateRefine(dir string) ([]Finding, error) {
	var findings []Finding

	proposalFindings, err := validateProposal(dir)
	if err != nil {
		return nil, err
	}
	findings = append(findings, proposalFindings...)

	deltaFindings, err := validateSpecsDeltas(dir)
	if err != nil {
		return nil, err
	}
	findings = append(findings, deltaFindings...)

	return findings, nil
}

func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})
}
