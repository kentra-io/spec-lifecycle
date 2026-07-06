package validate

// Stage is one of the three gated stages spec-lifecycle.md §3 defines for
// the default feature flow.
type Stage string

// The three recognized Stage values, in gate order.
const (
	StageRefine Stage = "refine"
	StageDesign Stage = "design"
	StagePlan   Stage = "plan"
)

// Stages lists every recognized Stage value, in gate order — the set
// `lifecycle validate --stage` accepts.
var Stages = []Stage{StageRefine, StageDesign, StagePlan}

// Severity distinguishes a hard validation failure (fails the artifact,
// and — at the CLI — the process) from an advisory that never does.
type Severity string

// The two recognized Severity values.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding is one precise, position-anchored validation result: which
// file, which line (0 if not tied to a single line), a stable machine
// Kind a caller can switch on without matching prose, and a
// human-readable Message.
type Finding struct {
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Kind     string   `json:"kind"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

// ArtifactsForStage returns the artifact path(s) — relative to a change
// folder — that `stage` gates (the package doc's Stage -> artifact table).
// Useful for a caller (e.g. the CLI) that wants to name what it's about to
// check without duplicating the mapping.
func ArtifactsForStage(stage Stage) []string {
	switch stage {
	case StageRefine:
		return []string{proposalFile, specsDir + "/**/spec.md"}
	case StageDesign:
		return []string{designFile}
	case StagePlan:
		return []string{tasksFile}
	default:
		return nil
	}
}
