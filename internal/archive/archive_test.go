package archive

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/approve"
	"github.com/kentra-io/spec-lifecycle/internal/testutil"
)

// --- fixtures / helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGates writes changeDir's approval-state.json directly (bypassing
// approve.Approve, and therefore the constitution deviation-gate
// machinery it exercises at gates 2/3) — this package's gate-check only
// ever reads records back via internal/status, so a hand-built,
// already-approved StateFile is a legitimate, hermetic fixture for
// exercising Archive's OWN logic in isolation.
func writeGates(t *testing.T, changeDir string, entries ...approve.Entry) {
	t.Helper()
	sf := approve.StateFile{SchemaVersion: approve.SchemaVersion, Change: filepath.Base(changeDir), Gates: entries}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, approve.StatePath(changeDir), string(data))
}

func approved(stage approve.Stage) approve.Entry {
	return approve.Entry{Stage: stage, Status: approve.StatusApproved, ApprovedAt: "2026-07-04T00:00:00Z"}
}

func approvedRefineDesignSkipped() approve.Entry {
	e := approved(approve.StageRefine)
	e.DesignSkipped = true
	return e
}

const validProposal = `---
issue: "kentra-io/kafka-dq#42"
---

# Add password login

## Why
Users need to authenticate.
`

const validBugProposal = `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`

func addedRequirement(name, body, scenario string) string {
	return "## ADDED Requirements\n\n### Requirement: " + name + "\n" + body +
		"\n\n#### Scenario: " + scenario + "\n- **GIVEN** a precondition\n- **WHEN** an action\n- **THEN** the system SHALL respond\n"
}

func modifiedRequirement(name, body, scenario string) string {
	return "## MODIFIED Requirements\n\n### Requirement: " + name + "\n" + body +
		"\n\n#### Scenario: " + scenario + "\n- **GIVEN** a precondition\n- **WHEN** an action\n- **THEN** the system SHALL respond\n"
}

// newFeatureChange scaffolds a minimal feature change folder under a
// fresh temp project root: proposal.md + a single-capability ADDED delta.
func newFeatureChange(t *testing.T, root, name, capability, reqName string) string {
	t.Helper()
	changeDir := filepath.Join(root, "openspec", "changes", name)
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", capability, "spec.md"),
		addedRequirement(reqName, "The system SHALL "+reqName+".", "Successful "+reqName))
	return changeDir
}

func newProjectRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// --- gate-check ---

func TestArchiveRefusesWithoutApprovedGates(t *testing.T) {
	root := newProjectRoot(t)
	newFeatureChange(t, root, "001-add-login", "auth", "Password login")

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrGatesNotApproved) {
		t.Fatalf("err = %v, want ErrGatesNotApproved", err)
	}

	// Nothing should have moved or been written.
	if _, statErr := os.Stat(filepath.Join(root, "openspec", "changes", "001-add-login")); statErr != nil {
		t.Errorf("change folder was relocated despite refusal: %v", statErr)
	}
	if _, statErr := os.Stat(LedgerPath(root)); !os.IsNotExist(statErr) {
		t.Errorf("ledger file exists despite refusal (stat err = %v)", statErr)
	}
}

func TestArchiveForceGatesOverride(t *testing.T) {
	root := newProjectRoot(t)
	newFeatureChange(t, root, "001-add-login", "auth", "Password login")

	res, err := Archive(Request{Root: root, Change: "001-add-login", ForceGates: true})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if !res.GatesOverridden {
		t.Error("Result.GatesOverridden = false, want true")
	}
	if len(res.Records) != 1 || !res.Records[0].GatesOverridden {
		t.Fatalf("Records = %+v, want exactly one with GatesOverridden = true", res.Records)
	}
	if len(res.Warnings) == 0 {
		t.Error("Warnings is empty, want a note about the override")
	}
}

// --- feature fold + relocate + ledger ---

func TestArchiveFeatureFold(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	res, err := Archive(Request{Root: root, Change: "001-add-login"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.Type != "feature" {
		t.Errorf("Type = %q, want feature", res.Type)
	}
	if res.GatesOverridden || res.ConflictsOverridden {
		t.Errorf("unexpected override flags: %+v", res)
	}
	if len(res.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(res.Records))
	}

	rec := res.Records[0]
	if rec.Seq != 1 {
		t.Errorf("Seq = %d, want 1", rec.Seq)
	}
	if rec.Capability != "auth" {
		t.Errorf("Capability = %q, want auth", rec.Capability)
	}
	if rec.PreImageSha != emptyImageSHA {
		t.Errorf("PreImageSha = %q, want the empty-image sentinel %q (brand-new capability)", rec.PreImageSha, emptyImageSHA)
	}
	if len(rec.DeltaOps) != 1 || rec.DeltaOps[0].Op != "ADDED" || rec.DeltaOps[0].Requirement != "Password login" {
		t.Errorf("DeltaOps = %+v, want a single ADDED \"Password login\" op", rec.DeltaOps)
	}

	// The change folder must have relocated.
	if _, err := os.Stat(changeDir); !os.IsNotExist(err) {
		t.Errorf("change folder still present at %s (stat err = %v)", changeDir, err)
	}
	archiveDir := filepath.Join(root, "openspec", "changes", "archive", "001-add-login")
	if _, err := os.Stat(archiveDir); err != nil {
		t.Errorf("archived folder missing at %s: %v", archiveDir, err)
	}

	// The live spec must exist and hash to PostImageSha.
	specPath := filepath.Join(root, "openspec", "specs", "auth", "spec.md")
	got, err := hashFile(specPath)
	if err != nil {
		t.Fatalf("hashFile(%s): %v", specPath, err)
	}
	if got != rec.PostImageSha {
		t.Errorf("live spec hash %q != ledger PostImageSha %q", got, rec.PostImageSha)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "### Requirement: Password login") {
		t.Errorf("folded spec.md missing the requirement block:\n%s", data)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// --- delta-less bug ---

func TestArchiveBugDeltaless(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := filepath.Join(root, "openspec", "changes", "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validBugProposal)
	writeGates(t, changeDir, approved(approve.StageRepro), approved(approve.StageFix))

	res, err := Archive(Request{Root: root, Change: "007-fix-panic"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.Type != "bug" {
		t.Errorf("Type = %q, want bug", res.Type)
	}
	if len(res.Records) != 1 {
		t.Fatalf("len(Records) = %d, want exactly 1 (delta-less bug)", len(res.Records))
	}
	rec := res.Records[0]
	if rec.Capability != "" {
		t.Errorf("Capability = %q, want \"\" (no capability affected)", rec.Capability)
	}
	if rec.PreImageSha != emptyImageSHA || rec.PostImageSha != emptyImageSHA {
		t.Errorf("PreImageSha/PostImageSha = %q/%q, want both == the empty sentinel %q", rec.PreImageSha, rec.PostImageSha, emptyImageSHA)
	}
	if rec.DeltaOps == nil || len(rec.DeltaOps) != 0 {
		t.Errorf("DeltaOps = %#v, want a non-nil empty slice", rec.DeltaOps)
	}

	// A delta-less archive touches no openspec/specs/ content at all.
	entries, err := os.ReadDir(filepath.Join(root, "openspec", "specs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("openspec/specs/ has entries after a delta-less bug archive: %v", entries)
	}
}

// --- tasks-completion gate ---

const tasksWithUncheckedStep = `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Validation contract** — checkable:
  - go test ./... passes
**Steps** — s:
  1. [x] write the handler
  2. [ ] write the tests
`

const tasksAllChecked = `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Validation contract** — checkable:
  - go test ./... passes
**Steps** — s:
  1. [x] write the handler
  2. [x] write the tests
`

func TestArchiveRefusesOnIncompleteTasks(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), tasksWithUncheckedStep)
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrTasksIncomplete) {
		t.Fatalf("err = %v, want ErrTasksIncomplete", err)
	}
	if !contains(err.Error(), "write the tests") {
		t.Errorf("error message %q should name the unchecked step", err.Error())
	}

	// Nothing should have moved or been written.
	if _, statErr := os.Stat(changeDir); statErr != nil {
		t.Errorf("change folder was relocated despite refusal: %v", statErr)
	}
	if _, statErr := os.Stat(LedgerPath(root)); !os.IsNotExist(statErr) {
		t.Errorf("ledger file exists despite refusal (stat err = %v)", statErr)
	}
}

func TestArchiveForceIncompleteTasksOverride(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), tasksWithUncheckedStep)
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	res, err := Archive(Request{Root: root, Change: "001-add-login", ForceIncompleteTasks: true})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if !res.TasksIncompleteOverridden {
		t.Error("Result.TasksIncompleteOverridden = false, want true")
	}
	if len(res.Records) != 1 || !res.Records[0].TasksIncompleteOverridden {
		t.Fatalf("Records = %+v, want exactly one with TasksIncompleteOverridden = true", res.Records)
	}
}

func TestArchiveAllowsAllStepsChecked(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), tasksAllChecked)
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	res, err := Archive(Request{Root: root, Change: "001-add-login"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.TasksIncompleteOverridden {
		t.Error("TasksIncompleteOverridden = true, want false (nothing to override)")
	}
}

func TestArchiveAllowsNoTasksFile(t *testing.T) {
	// No tasks.md at all — the gate must stay a no-op (backward
	// compatible with every change that predates checkbox tracking).
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if err != nil {
		t.Fatalf("Archive: %v (tasks-completion gate must not block a change with no tasks.md)", err)
	}
}

func TestArchiveAllowsUntrackedSteps(t *testing.T) {
	// Legacy-style Steps (no checkboxes at all) must not be gated either.
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Validation contract** — checkable:
  - go test ./... passes
**Steps** — s:
  1. write the handler
  2. write the tests
`)
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if err != nil {
		t.Fatalf("Archive: %v (untracked Steps must not be gated)", err)
	}
}

// --- conflict-check ---

func TestArchiveConflictRefusesThenForceOverrideResolves(t *testing.T) {
	root := newProjectRoot(t)

	// Seed a base capability spec both changes will MODIFY.
	writeFile(t, filepath.Join(root, "openspec", "specs", "auth", "spec.md"), `# auth Specification

## Purpose
Authentication.

## Requirements
### Requirement: Password login
The system SHALL allow login.

#### Scenario: Successful login
- **GIVEN** a user
- **WHEN** they log in
- **THEN** the system SHALL grant a session
`)

	changeA := filepath.Join(root, "openspec", "changes", "100-change-a")
	writeFile(t, filepath.Join(changeA, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeA, "specs", "auth", "spec.md"),
		modifiedRequirement("Password login", "The system SHALL allow login via A.", "Successful login A"))
	writeGates(t, changeA, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	changeB := filepath.Join(root, "openspec", "changes", "200-change-b")
	writeFile(t, filepath.Join(changeB, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeB, "specs", "auth", "spec.md"),
		modifiedRequirement("Password login", "The system SHALL allow login via B.", "Successful login B"))
	writeGates(t, changeB, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	// Archiving A is refused: B is still in-flight and touches the same
	// requirement.
	_, err := Archive(Request{Root: root, Change: "100-change-a"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Archive(A) err = %v, want ErrConflict", err)
	}

	// Forcing it through resolves and records the override.
	res, err := Archive(Request{Root: root, Change: "100-change-a", ForceConflicts: true})
	if err != nil {
		t.Fatalf("Archive(A, ForceConflicts): %v", err)
	}
	if !res.ConflictsOverridden || !res.Records[0].ConflictsOverridden {
		t.Fatalf("ConflictsOverridden not recorded: %+v", res)
	}

	// Now that A is archived, B is no longer conflicting with anything
	// (A is no longer an in-flight change) — B archives clean, folding
	// on top of A's already-updated live spec.
	res2, err := Archive(Request{Root: root, Change: "200-change-b"})
	if err != nil {
		t.Fatalf("Archive(B): %v", err)
	}
	if res2.Records[0].Seq != 2 {
		t.Errorf("B's Seq = %d, want 2 (A archived first, seq is call-order not name-order)", res2.Records[0].Seq)
	}

	specPath := filepath.Join(root, "openspec", "specs", "auth", "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(data), "via B") {
		t.Errorf("expected B's MODIFIED body to win (last folded), got:\n%s", data)
	}
}

// --- seq monotonicity across unrelated archives, folder-name independent ---

func TestArchiveSeqMonotonicIndependentOfChangeName(t *testing.T) {
	root := newProjectRoot(t)

	changeDir1 := newFeatureChange(t, root, "999-later-name", "billing", "Invoice export")
	writeGates(t, changeDir1, approvedRefineDesignSkipped(), approved(approve.StagePlan))
	res1, err := Archive(Request{Root: root, Change: "999-later-name"})
	if err != nil {
		t.Fatalf("Archive(1): %v", err)
	}

	changeDir2 := newFeatureChange(t, root, "001-earlier-name", "billing", "Invoice refund")
	writeGates(t, changeDir2, approvedRefineDesignSkipped(), approved(approve.StagePlan))
	res2, err := Archive(Request{Root: root, Change: "001-earlier-name"})
	if err != nil {
		t.Fatalf("Archive(2): %v", err)
	}

	if res1.Records[0].Seq != 1 {
		t.Errorf("first-archived change's Seq = %d, want 1", res1.Records[0].Seq)
	}
	if res2.Records[0].Seq != 2 {
		t.Errorf("second-archived change's Seq = %d, want 2 (despite its folder name sorting first alphabetically)", res2.Records[0].Seq)
	}

	all, err := ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 2 || all[0].Seq != 1 || all[1].Seq != 2 {
		t.Errorf("ReadAll = %+v, want seq 1 then 2 in append order", all)
	}
}

func TestArchiveAlreadyArchivedRefuses(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))
	if _, err := Archive(Request{Root: root, Change: "001-add-login"}); err != nil {
		t.Fatalf("first Archive: %v", err)
	}

	// Recreate a same-named change folder (a different capability, so
	// folding itself would succeed) and try again: the archive/
	// destination directory already exists from the first run above.
	changeDir2 := newFeatureChange(t, root, "001-add-login", "billing", "Invoice export")
	writeGates(t, changeDir2, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("err = %v, want ErrCouldNotRun (archive destination collision)", err)
	}
}

// --- request validation / environment-failure branches ---

func TestArchiveRequiresRootAndChange(t *testing.T) {
	if _, err := Archive(Request{Change: "x"}); !errors.Is(err, ErrCouldNotRun) {
		t.Errorf("empty Root: err = %v, want ErrCouldNotRun", err)
	}
	if _, err := Archive(Request{Root: t.TempDir()}); !errors.Is(err, ErrCouldNotRun) {
		t.Errorf("empty Change: err = %v, want ErrCouldNotRun", err)
	}
}

func TestArchiveChangeNotFound(t *testing.T) {
	root := newProjectRoot(t)
	if _, err := Archive(Request{Root: root, Change: "does-not-exist"}); !errors.Is(err, ErrCouldNotRun) {
		t.Errorf("err = %v, want ErrCouldNotRun", err)
	}
}

func TestArchivePropagatesProposalMetaReadError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := filepath.Join(root, "openspec", "changes", "001-bad-proposal")
	// proposal.md is a directory, not a file: ReadProposalMeta's
	// os.ReadFile fails with a real (non-IsNotExist) error.
	if err := os.MkdirAll(filepath.Join(changeDir, "proposal.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Archive(Request{Root: root, Change: "001-bad-proposal"}); !errors.Is(err, ErrCouldNotRun) {
		t.Errorf("err = %v, want ErrCouldNotRun", err)
	}
}

func TestArchivePropagatesCheckGatesError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeFile(t, approve.StatePath(changeDir), "not json at all")

	if _, err := Archive(Request{Root: root, Change: "001-add-login"}); !errors.Is(err, ErrCouldNotRun) {
		t.Errorf("err = %v, want ErrCouldNotRun", err)
	}
}

func TestArchivePropagatesDeltaParseError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := filepath.Join(root, "openspec", "changes", "001-bad-delta")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), "not a valid delta at all\n")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-bad-delta"})
	if !errors.Is(err, ErrFoldFailed) {
		t.Fatalf("err = %v, want ErrFoldFailed", err)
	}
}

func TestArchivePropagatesUnreadableLiveSpecError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	// openspec/specs/auth is a regular file, not a directory: reading the
	// live spec fails with a real (non-IsNotExist) error.
	writeFile(t, filepath.Join(root, "openspec", "specs", "auth"), "not a directory")

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("err = %v, want ErrCouldNotRun", err)
	}
}

func TestArchivePropagatesFoldError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := filepath.Join(root, "openspec", "changes", "001-modify-missing")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"),
		modifiedRequirement("Nonexistent requirement", "Body.", "Scenario"))
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	_, err := Archive(Request{Root: root, Change: "001-modify-missing"})
	if !errors.Is(err, ErrFoldFailed) {
		t.Fatalf("err = %v, want ErrFoldFailed", err)
	}
}

func TestArchivePropagatesArchiveRootMkdirError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	// openspec/changes/archive is a regular file, not a directory.
	writeFile(t, filepath.Join(root, "openspec", "changes", "archive"), "not a directory")

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("err = %v, want ErrCouldNotRun", err)
	}
	if _, statErr := os.Stat(changeDir); statErr != nil {
		t.Errorf("change folder relocated/removed despite the relocate step failing: %v", statErr)
	}
}

func TestArchivePropagatesLedgerAppendError(t *testing.T) {
	root := newProjectRoot(t)
	changeDir := newFeatureChange(t, root, "001-add-login", "auth", "Password login")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))
	writeFile(t, LedgerPath(root), "not json at all\n")

	_, err := Archive(Request{Root: root, Change: "001-add-login"})
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("err = %v, want ErrCouldNotRun", err)
	}
}

// --- RENAMED/REMOVED deltaOps rendering ---

func TestArchiveRendersRenamedAndRemovedDeltaOps(t *testing.T) {
	root := newProjectRoot(t)
	writeFile(t, filepath.Join(root, "openspec", "specs", "auth", "spec.md"), `# auth Specification

## Purpose
Authentication.

## Requirements
### Requirement: Password login
The system SHALL allow login.

#### Scenario: Successful login
- **GIVEN** a user
- **WHEN** they log in
- **THEN** the system SHALL grant a session

### Requirement: Legacy token login
The system SHALL allow legacy token login.

#### Scenario: Legacy login
- **GIVEN** a legacy token
- **WHEN** it is presented
- **THEN** the system SHALL grant a session
`)

	changeDir := filepath.Join(root, "openspec", "changes", "010-rename-and-remove")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"),
		"## RENAMED Requirements\n"+
			"- FROM: `### Requirement: Password login`\n"+
			"- TO: `### Requirement: Username and password login`\n"+
			"\n"+
			"## REMOVED Requirements\n"+
			"### Requirement: Legacy token login\n"+
			"**Reason**: no longer supported.\n"+
			"**Migration**: none.\n")
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	res, err := Archive(Request{Root: root, Change: "010-rename-and-remove"})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(res.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(res.Records))
	}
	ops := res.Records[0].DeltaOps
	if len(ops) != 2 {
		t.Fatalf("DeltaOps = %+v, want exactly 2 ops (RENAMED then REMOVED)", ops)
	}
	if ops[0].Op != "RENAMED" || ops[0].Requirement != "Password login -> Username and password login" {
		t.Errorf("ops[0] = %+v, want RENAMED \"Password login -> Username and password login\"", ops[0])
	}
	if ops[1].Op != "REMOVED" || ops[1].Requirement != "Legacy token login" {
		t.Errorf("ops[1] = %+v, want REMOVED \"Legacy token login\"", ops[1])
	}
}

// --- multi-capability write is one all-or-nothing group ---

// TestArchiveGroupWriteFailureLeavesNoLiveSpecsMutated proves that when a
// multi-capability archive's write phase fails partway through (here:
// billing's write directory is read-only), the OTHER capability's live
// spec (auth, discovered/folded/prepared first) is never committed either
// — the write phase is one all-or-nothing group, not N independent
// per-file atomic writes (doc.go's "prepare, then commit, as one group").
//
// billing's directory is made READ-ONLY, not simply blocked by a
// non-directory entry: a non-directory obstruction at that same path
// would also break the earlier in-memory FOLD step's pre-image read
// (which reads through the identical specPath), failing the archive
// before the write phase is even reached and defeating the point of this
// test. Read-only-but-searchable lets the pre-image read cleanly observe
// "no live spec yet" while still failing the write phase's temp-file
// creation, isolating the failure to exactly the phase this test targets.
func TestArchiveGroupWriteFailureLeavesNoLiveSpecsMutated(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	root := newProjectRoot(t)
	changeDir := filepath.Join(root, "openspec", "changes", "300-two-caps")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"),
		addedRequirement("Password login", "The system SHALL allow login.", "Successful login"))
	writeFile(t, filepath.Join(changeDir, "specs", "billing", "spec.md"),
		addedRequirement("Invoice export", "The system SHALL export invoices.", "Successful export"))
	writeGates(t, changeDir, approvedRefineDesignSkipped(), approved(approve.StagePlan))

	// discoverCapabilities sorts "auth" before "billing", so auth's write
	// is prepared successfully before billing's Prepare fails.
	billingDir := filepath.Join(root, "openspec", "specs", "billing")
	if err := os.MkdirAll(billingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(billingDir, 0o555); err != nil {
		t.Fatalf("chmod billing dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(billingDir, 0o755) })

	_, err := Archive(Request{Root: root, Change: "300-two-caps"})
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("err = %v, want ErrCouldNotRun", err)
	}

	// auth's prepared write must have been discarded, not committed: no
	// leftover spec.md (or stray temp file) under openspec/specs/auth.
	entries, rerr := os.ReadDir(filepath.Join(root, "openspec", "specs", "auth"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(entries) != 0 {
		t.Errorf("openspec/specs/auth has leftover entries %v, want empty (prepared write must be discarded, not committed)", entries)
	}

	// The change folder must not have relocated, and no ledger record
	// must exist — the whole archive is untouched.
	if _, statErr := os.Stat(changeDir); statErr != nil {
		t.Errorf("change folder relocated/removed despite the write failing: %v", statErr)
	}
	if _, statErr := os.Stat(LedgerPath(root)); !os.IsNotExist(statErr) {
		t.Errorf("ledger file exists despite the write failing (stat err = %v)", statErr)
	}
}
