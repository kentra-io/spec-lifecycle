package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/spec"
	"github.com/kentra-io/spec-lifecycle/internal/testutil"
)

// --- checkConflicts ---

func TestCheckConflictsNoChangesDirYieldsNilNotError(t *testing.T) {
	root := t.TempDir() // no openspec/changes at all
	conflicts, warnings, err := checkConflicts(root, "some-change", nil)
	if err != nil {
		t.Fatalf("checkConflicts: %v", err)
	}
	if conflicts != nil || warnings != nil {
		t.Errorf("conflicts=%v warnings=%v, want both nil", conflicts, warnings)
	}
}

func TestCheckConflictsPropagatesNonNotExistReadDirError(t *testing.T) {
	testutil.SkipUnlessUnixFSErrors(t)

	root := t.TempDir()
	// openspec/changes is a regular file, not a directory.
	changesRoot := filepath.Join(root, "openspec", "changes")
	writeFile(t, changesRoot, "not a directory")

	if _, _, err := checkConflicts(root, "some-change", nil); err == nil {
		t.Fatal("checkConflicts: want error when openspec/changes is a regular file, got nil")
	}
}

func TestCheckConflictsSkipsCapabilityThisChangeDoesNotTouch(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "openspec", "changes", "200-other")
	writeFile(t, filepath.Join(other, "specs", "billing", "spec.yaml"),
		deltaYAML("billing", modifiedRequirement("Invoice export", "The system SHALL export.", "Scenario")))

	// ownDeltas only targets "auth" — "billing" (the only capability the
	// sibling touches) is irrelevant to this change.
	ownDeltas := map[string]*spec.Delta{"auth": {}}

	conflicts, warnings, err := checkConflicts(root, "100-this-change", ownDeltas)
	if err != nil {
		t.Fatalf("checkConflicts: %v", err)
	}
	if len(conflicts) != 0 || len(warnings) != 0 {
		t.Errorf("conflicts=%v warnings=%v, want both empty", conflicts, warnings)
	}
}

func TestCheckConflictsWarnsWithoutFailingOnUnreadableSiblingDelta(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	root := t.TempDir()
	other := filepath.Join(root, "openspec", "changes", "200-other")
	writeFile(t, filepath.Join(other, "proposal.md"), validProposal)
	// specs/auth/spec.yaml is a real file (so HasSpecsDeltas/discoverCapabilities
	// both see it) but made unreadable, so os.ReadFile fails on it.
	deltaPath := filepath.Join(other, "specs", "auth", "spec.yaml")
	writeFile(t, deltaPath, deltaYAML("auth", modifiedRequirement("Password login", "The system SHALL log in.", "Scenario")))
	if err := os.Chmod(deltaPath, 0o000); err != nil {
		t.Fatalf("chmod delta unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(deltaPath, 0o644) })

	ownDeltas := map[string]*spec.Delta{"auth": {Deltas: []spec.DeltaEntry{{Op: spec.OpModified, Requirement: &spec.Requirement{Name: "Password login"}}}}}
	conflicts, warnings, err := checkConflicts(root, "100-this-change", ownDeltas)
	if err != nil {
		t.Fatalf("checkConflicts: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none", conflicts)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "200-other/auth") {
		t.Errorf("warnings = %v, want one mentioning 200-other/auth", warnings)
	}
}

func TestCheckConflictsWarnsWithoutFailingOnUnparsableSiblingDelta(t *testing.T) {
	root := t.TempDir()
	other := filepath.Join(root, "openspec", "changes", "200-other")
	writeFile(t, filepath.Join(other, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(other, "specs", "auth", "spec.yaml"), "not a valid delta at all\n")

	ownDeltas := map[string]*spec.Delta{"auth": {Deltas: []spec.DeltaEntry{{Op: spec.OpModified, Requirement: &spec.Requirement{Name: "Password login"}}}}}
	conflicts, warnings, err := checkConflicts(root, "100-this-change", ownDeltas)
	if err != nil {
		t.Fatalf("checkConflicts: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none", conflicts)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unparsable delta") {
		t.Errorf("warnings = %v, want one noting an unparsable delta", warnings)
	}
}

// --- targetedNames ---

func TestTargetedNamesUnionsModifiedRemovedAndRenamedFrom(t *testing.T) {
	d := &spec.Delta{
		Deltas: []spec.DeltaEntry{
			{Op: spec.OpModified, Requirement: &spec.Requirement{Name: "Password login"}},
			{Op: spec.OpRemoved, Requirement: &spec.Requirement{Name: "Legacy token login"}},
			{Op: spec.OpRenamed, From: "Old name", To: "New name"},
		},
	}
	got := targetedNames(d)

	want := map[string]string{
		"password login":     "Password login",
		"legacy token login": "Legacy token login",
		"old name":           "Old name",
	}
	if len(got) != len(want) {
		t.Fatalf("targetedNames = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("targetedNames[%q] = %q, want %q", k, got[k], v)
		}
	}
}
