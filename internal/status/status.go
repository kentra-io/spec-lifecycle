package status

import (
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/approve"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// StageState is one gate's derived reporting state (doc.go).
type StageState string

// The four recognized StageState values (doc.go).
const (
	StatePending  StageState = "pending"
	StateApproved StageState = "approved"
	StateRejected StageState = "rejected"
	StateSkipped  StageState = "skipped"
)

// GateStatus is one stage's derived state within a ChangeStatus.
type GateStatus struct {
	Stage         approve.Stage `json:"stage"`
	State         StageState    `json:"state"`
	DesignSkipped bool          `json:"designSkipped,omitempty"`
	ApprovedBy    string        `json:"approvedBy,omitempty"`
	ApprovedAt    string        `json:"approvedAt,omitempty"`
	Notes         string        `json:"notes,omitempty"`
	// Drifted lists artifact paths (relative to the change folder) whose
	// recorded hash no longer matches current content. Only populated for
	// an approved gate; nil means no drift (or the gate isn't approved).
	Drifted []string `json:"drifted,omitempty"`
}

// ChangeStatus is one change folder's full gate-state report.
type ChangeStatus struct {
	Change string       `json:"change"`
	Type   string       `json:"type"`
	Issue  string       `json:"issue"`
	Gates  []GateStatus `json:"gates"`
}

// stageOrder is the canonical global ordering used both for a
// single-flow-type's base stage set and to interleave a promoted bug's
// mixed stage names in a stable, gate-sequence-like order.
var stageOrder = []approve.Stage{
	approve.StageRefine, approve.StageDesign, approve.StagePlan,
	approve.StageRepro, approve.StageFix,
}

var featureStages = map[approve.Stage]bool{
	approve.StageRefine: true, approve.StageDesign: true, approve.StagePlan: true,
}

var bugStages = map[approve.Stage]bool{
	approve.StageRepro: true, approve.StageFix: true,
}

// Change derives the full gate-state report for the change folder at
// dir. dir need not exist yet in the trivial sense of having any gate
// records — a change with only a proposal.md (or nothing at all yet)
// reports every base stage as pending.
func Change(dir string) (ChangeStatus, error) {
	meta, err := validate.ReadProposalMeta(dir)
	if err != nil {
		return ChangeStatus{}, err
	}

	sf, err := approve.ReadState(dir)
	if err != nil {
		return ChangeStatus{}, err
	}
	latest := approve.LatestPerStage(sf.Gates)

	base := featureStages
	if meta.Type == validate.ChangeTypeBug {
		base = bugStages
	}

	var stages []approve.Stage
	for _, s := range stageOrder {
		if base[s] {
			stages = append(stages, s)
			continue
		}
		if _, ok := latest[s]; ok {
			// A promoted change's stage name outside the type's base set
			// (spec-lifecycle.md §8's promotion hatch) — report it too.
			stages = append(stages, s)
		}
	}

	refineEntry, hasRefine := latest[approve.StageRefine]

	gates := make([]GateStatus, 0, len(stages))
	for _, s := range stages {
		gs := GateStatus{Stage: s}
		if entry, ok := latest[s]; ok {
			gs.State = StageState(entry.Status)
			gs.DesignSkipped = entry.DesignSkipped
			gs.ApprovedBy = entry.ApprovedBy
			gs.ApprovedAt = entry.ApprovedAt
			gs.Notes = entry.Notes
			if entry.Status == approve.StatusApproved {
				gs.Drifted = approve.HashDrift(dir, entry.Artifacts)
			}
		} else if s == approve.StageDesign && hasRefine && refineEntry.DesignSkipped {
			gs.State = StateSkipped
		} else {
			gs.State = StatePending
		}
		gates = append(gates, gs)
	}

	return ChangeStatus{
		Change: filepath.Base(dir),
		Type:   meta.Type,
		Issue:  meta.Issue,
		Gates:  gates,
	}, nil
}
