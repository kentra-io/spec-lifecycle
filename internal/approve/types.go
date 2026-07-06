package approve

// Stage is one gated stage name — the three feature-flow stages
// internal/validate already knows (M2) plus the two bug-flow stage names
// spec-lifecycle.md §0's Stage glossary reserves (doc.go).
type Stage string

// The five recognized Stage values (see Stages for their canonical order).
const (
	StageRefine Stage = "refine"
	StageDesign Stage = "design"
	StagePlan   Stage = "plan"
	StageRepro  Stage = "repro"
	StageFix    Stage = "fix"
)

// Stages lists every recognized Stage value, in the order shown in the
// CLI's --stage usage string.
var Stages = []Stage{StageRefine, StageDesign, StagePlan, StageRepro, StageFix}

// IsRecognized reports whether s is one of Stages.
func (s Stage) IsRecognized() bool {
	for _, want := range Stages {
		if s == want {
			return true
		}
	}
	return false
}

// Status is a gate entry's persisted outcome. "pending" is deliberately
// NOT a Status value — spec-lifecycle.md §5: "pending is DERIVED, never
// persisted" (internal/status's job, over the absence of an entry).
type Status string

// The two persisted Status values.
const (
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// SchemaVersion is approval-state.json's schemaVersion (spec-lifecycle.md
// §5). Unknown ⇒ refuse, mirroring internal/config's own convention; no
// migration machinery in v1.
const SchemaVersion = 1

// StateFileName is the append-only gate record's filename, one per change
// folder (spec-lifecycle.md §5).
const StateFileName = "approval-state.json"

// Entry is one append-only record in approval-state.json's gates array
// (spec-lifecycle.md §5). DeviationConstitutionHash and DeviationRef are
// pointers so an absent value marshals as JSON null (the spec's own
// example shows `"deviationConstitutionHash": null` for a non-gate-2/3
// entry), not an omitted key or an empty string.
type Entry struct {
	Stage         Stage             `json:"stage"`
	Status        Status            `json:"status"`
	DesignSkipped bool              `json:"designSkipped"`
	Artifacts     map[string]string `json:"artifacts"`
	// ConstitutionHash is lifecycle's OWN recompute at approval time
	// (internal/constitution.Hash) — authoritative. Empty when the
	// constitution companion primitive isn't set up in this project yet
	// (a Warning is surfaced in that case, not a hard failure).
	ConstitutionHash string `json:"constitutionHash,omitempty"`
	// DeviationConstitutionHash is the constitutionHash value the
	// plan-gate stamped INTO deviation.json — set only at gates 2/3
	// (stage design/plan), null otherwise (spec-lifecycle.md §5/§7.5).
	DeviationConstitutionHash *string `json:"deviationConstitutionHash"`
	// DeviationRef is deviation.json's path (relative to the change
	// folder) when the plan-gate ran for this entry, null otherwise.
	DeviationRef *string `json:"deviationRef"`
	ApprovedBy   string  `json:"approvedBy"`
	ApprovedAt   string  `json:"approvedAt"`
	Notes        string  `json:"notes"`
}

// StateFile is the whole approval-state.json document (spec-lifecycle.md
// §5).
type StateFile struct {
	SchemaVersion int     `json:"schemaVersion"`
	Change        string  `json:"change"`
	Issue         string  `json:"issue"`
	Gates         []Entry `json:"gates"`
}
