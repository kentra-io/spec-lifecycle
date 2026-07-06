package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/approve"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const validProposal = `---
issue: "kentra-io/kafka-dq#42"
designSkip: false
---

# Add password login

## Why
Users need to authenticate.

## What Changes
- **auth:** ADDED - password login.

## Impact
- New capability: auth.
`

func offGate() approve.ConsentGate { return approve.ConsentGate{Policy: "off"} }

func newFeatureChange(t *testing.T) (root, changeDir string) {
	t.Helper()
	root = t.TempDir()
	changeDir = filepath.Join(root, "openspec", "changes", "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), `## ADDED Requirements

### Requirement: Password login
The system SHALL allow a registered user to authenticate.

#### Scenario: Successful login
- **GIVEN** a registered user
- **WHEN** they submit correct credentials
- **THEN** the system SHALL grant a session
`)
	return root, changeDir
}

func TestChangeAllPendingBeforeAnyGate(t *testing.T) {
	_, changeDir := newFeatureChange(t)
	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if cs.Type != "feature" {
		t.Errorf("Type = %q, want feature", cs.Type)
	}
	if len(cs.Gates) != 3 {
		t.Fatalf("len(Gates) = %d, want 3 (refine/design/plan)", len(cs.Gates))
	}
	for _, g := range cs.Gates {
		if g.State != StatePending {
			t.Errorf("Gates[%s].State = %q, want pending", g.Stage, g.State)
		}
	}
}

func TestChangeApprovedAndPending(t *testing.T) {
	root, changeDir := newFeatureChange(t)
	_, err := approve.Approve(approve.Request{
		Root: root, Change: "042-user-auth", Stage: approve.StageRefine, Consent: offGate(),
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	states := map[approve.Stage]StageState{}
	for _, g := range cs.Gates {
		states[g.Stage] = g.State
	}
	if states[approve.StageRefine] != StateApproved {
		t.Errorf("refine state = %q, want approved", states[approve.StageRefine])
	}
	if states[approve.StageDesign] != StatePending {
		t.Errorf("design state = %q, want pending", states[approve.StageDesign])
	}
}

func TestChangeDesignSkippedIsNotPending(t *testing.T) {
	root, changeDir := newFeatureChange(t)
	_, err := approve.Approve(approve.Request{
		Root: root, Change: "042-user-auth", Stage: approve.StageRefine, DesignSkip: true, Consent: offGate(),
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	for _, g := range cs.Gates {
		if g.Stage == approve.StageDesign {
			if g.State != StateSkipped {
				t.Errorf("design state = %q, want skipped", g.State)
			}
			return
		}
	}
	t.Fatal("no design gate reported")
}

func TestChangeLatestEntryWins(t *testing.T) {
	root, changeDir := newFeatureChange(t)
	if _, err := approve.Approve(approve.Request{
		Root: root, Change: "042-user-auth", Stage: approve.StageRefine, Consent: offGate(),
	}); err != nil {
		t.Fatalf("first Approve: %v", err)
	}
	if _, err := approve.Approve(approve.Request{
		Root: root, Change: "042-user-auth", Stage: approve.StageRefine, Reject: true, Consent: offGate(),
	}); err != nil {
		t.Fatalf("second Approve: %v", err)
	}

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	for _, g := range cs.Gates {
		if g.Stage == approve.StageRefine && g.State != StateRejected {
			t.Errorf("refine state = %q, want rejected (latest entry wins)", g.State)
		}
	}
}

func TestChangeFlagsPostGateDrift(t *testing.T) {
	root, changeDir := newFeatureChange(t)
	if _, err := approve.Approve(approve.Request{
		Root: root, Change: "042-user-auth", Stage: approve.StageRefine, Consent: offGate(),
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal+"\nedited after approval\n")

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	for _, g := range cs.Gates {
		if g.Stage == approve.StageRefine {
			if len(g.Drifted) == 0 {
				t.Error("Drifted is empty, want proposal.md flagged")
			}
			return
		}
	}
	t.Fatal("no refine gate reported")
}

func TestChangeBugFlow(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "openspec", "changes", "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`)

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if cs.Type != "bug" {
		t.Errorf("Type = %q, want bug", cs.Type)
	}
	if len(cs.Gates) != 2 {
		t.Fatalf("len(Gates) = %d, want 2 (repro/fix)", len(cs.Gates))
	}
	if cs.Gates[0].Stage != approve.StageRepro || cs.Gates[1].Stage != approve.StageFix {
		t.Errorf("Gates = %+v, want [repro, fix]", cs.Gates)
	}
}

func TestChangePromotedBugMixesStageNames(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "openspec", "changes", "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`)

	if _, err := approve.Approve(approve.Request{
		Root: root, Change: "007-fix-panic", Stage: approve.StageRepro, Consent: offGate(),
	}); err != nil {
		t.Fatalf("Approve(repro): %v", err)
	}

	// Simulate the promotion hatch (spec-lifecycle.md §8): a "design"
	// gate entry recorded under the same bug change folder, mixing stage
	// names. Constructed directly (not via approve.Approve, which would
	// need a real deviation.json + constitution binary for a design
	// gate) since this test is only about status's own stage-set-union
	// display logic, not the write path itself.
	sf, err := approve.ReadState(changeDir)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	sf.Gates = append(sf.Gates, approve.Entry{
		Stage: approve.StageDesign, Status: approve.StatusApproved,
		Artifacts: map[string]string{}, ApprovedAt: "2026-07-04T00:00:00Z",
	})
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, approve.StatePath(changeDir), string(data))

	cs, err := Change(changeDir)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	states := map[approve.Stage]StageState{}
	for _, g := range cs.Gates {
		states[g.Stage] = g.State
	}
	if states[approve.StageRepro] != StateApproved {
		t.Errorf("repro state = %q, want approved", states[approve.StageRepro])
	}
	if states[approve.StageFix] != StatePending {
		t.Errorf("fix state = %q, want pending", states[approve.StageFix])
	}
	if states[approve.StageDesign] != StateApproved {
		t.Errorf("design state = %q, want approved (promoted, mixed into a bug's gates[])", states[approve.StageDesign])
	}
	if len(cs.Gates) != 3 {
		t.Errorf("len(Gates) = %d, want 3 (repro, fix, design)", len(cs.Gates))
	}
}
