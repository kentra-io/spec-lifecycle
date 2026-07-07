package archive

import (
	"fmt"

	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// checkTasksComplete implements the tasks-completion gate (doc.go's
// "Gate-check reuse" companion — harness orchestration.md §5.5): a
// change whose tasks.md declares checkbox-tracked Steps
// ("<n>. [ ]"/"<n>. [x]", internal/validate's opt-in addendum to
// spec-lifecycle.md §4.2) must have every tracked step checked before it
// can be archived. Returns one human-readable violation string per
// unchecked tracked step (nil means nothing is outstanding).
//
// This gate is deliberately silent — zero violations, not an error — for
// the two cases that predate this addendum and must keep archiving
// exactly as before (same backward-compatibility posture as
// internal/validate's optional ```contract block):
//
//   - No tasks.md at all (e.g. a delta-less bug fix, or any change that
//     simply hasn't adopted checkbox tracking).
//   - A tasks.md whose Steps are plain "<n>. <text>" lines with no
//     checkbox at all — untracked, exactly as every Steps line behaved
//     before this addendum.
//
// The gate only ever activates for a milestone that opts in by tracking
// at least one step; from that point every tracked step in that
// milestone must be checked.
func checkTasksComplete(changeDir string) ([]string, error) {
	milestones, ok, err := validate.ParseMilestones(changeDir)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	var violations []string
	for _, m := range milestones {
		for _, s := range m.Steps {
			if s.Tracked && !s.Checked {
				violations = append(violations, fmt.Sprintf(
					"milestone %d (%q): step %q is not checked", m.ID, m.Title, s.Text,
				))
			}
		}
	}
	return violations, nil
}
