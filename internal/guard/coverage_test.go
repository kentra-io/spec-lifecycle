package guard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
	"github.com/kentra-io/spec-lifecycle/internal/testutil"
)

// TestSortFindings exercises every tie-break level of sortFindings directly
// (Check, then Kind, then Change, then Capability, then Seq) — the
// end-to-end Run-based tests rarely produce enough simultaneous findings
// to hit every branch, so this constructs a deliberately adversarial,
// pre-shuffled slice covering all five levels.
func TestSortFindings(t *testing.T) {
	in := []Finding{
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "y", Seq: 2},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "y", Seq: 1},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "x"},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "a"},
		{Check: CheckDigestChain, Kind: KindChainBreak},
		{Check: CheckDigestChain, Kind: KindProjectionDrift},
		{Check: CheckImmutability, Kind: KindArchiveMutated},
		{Check: CheckLedger, Kind: KindLedgerMissing},
	}
	want := []Finding{
		{Check: CheckDigestChain, Kind: KindChainBreak},
		{Check: CheckDigestChain, Kind: KindProjectionDrift},
		{Check: CheckImmutability, Kind: KindArchiveMutated},
		{Check: CheckLedger, Kind: KindLedgerMissing},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "a"},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "x"},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "y", Seq: 1},
		{Check: CheckReplay, Kind: KindProjectionDrift, Change: "b", Capability: "y", Seq: 2},
	}

	sortFindings(in)
	if len(in) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d", len(in), len(want))
	}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("index %d: got %+v, want %+v (full: %+v)", i, in[i], want[i], in)
		}
	}

	// Sorting an already-sorted slice again must be a no-op.
	again := make([]Finding, len(in))
	copy(again, in)
	sortFindings(again)
	for i := range in {
		if in[i] != again[i] {
			t.Fatalf("sortFindings is not idempotent at index %d: %+v vs %+v", i, in[i], again[i])
		}
	}
}

// TestListArchivedChangeNames_SkipsNonDirEntries covers the branch where a
// stray regular file (not a directory) sits directly under
// openspec/changes/archive/ — it must be ignored, not treated as a change.
func TestListArchivedChangeNames_SkipsNonDirEntries(t *testing.T) {
	b := newFixtureBuilder(t)
	archiveRoot := filepath.Join(b.root, "openspec", "changes", "archive")
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(archiveRoot, "stray.txt"), []byte("not a change"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	names, err := listArchivedChangeNames(archiveRoot)
	if err != nil {
		t.Fatalf("listArchivedChangeNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("want the stray file skipped, got %v", names)
	}
}

// TestListLiveCapabilities_SkipsNonDirAndSpeclessEntries covers both
// skip branches: a stray regular file directly under openspec/specs/, and
// a capability-looking directory with no spec.md inside it.
func TestListLiveCapabilities_SkipsNonDirAndSpeclessEntries(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	specsRoot := filepath.Join(b.root, "openspec", "specs")
	if err := os.WriteFile(filepath.Join(specsRoot, "stray.txt"), []byte("not a capability"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(specsRoot, "empty-dir"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	caps, err := listLiveCapabilities(b.root)
	if err != nil {
		t.Fatalf("listLiveCapabilities: %v", err)
	}
	if len(caps) != 1 || caps[0] != "auth" {
		t.Fatalf("want exactly [auth], got %v", caps)
	}
}

// TestRun_PermissionErrorsMapToCouldNotRun covers the real-I/O-error
// (non-not-exist) branches that only fire on a genuine permission failure
// — skipped on windows/root, where permission bits aren't enforced
// (internal/testutil).
func TestRun_PermissionErrorsMapToCouldNotRun(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	t.Run("archive root unreadable", func(t *testing.T) {
		b := newFixtureBuilder(t)
		b.step("auth", addDeltaText("Password login"))
		archiveRoot := filepath.Join(b.root, "openspec", "changes", "archive")
		if err := os.Chmod(archiveRoot, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		defer os.Chmod(archiveRoot, 0o755) //nolint:errcheck

		if _, err := Run(Options{Root: b.root}); err == nil {
			t.Fatalf("want an error when openspec/changes/archive is unreadable")
		}
	})

	t.Run("live specs directory unreadable", func(t *testing.T) {
		b := newFixtureBuilder(t)
		b.step("auth", addDeltaText("Password login"))
		specsRoot := filepath.Join(b.root, "openspec", "specs")
		if err := os.Chmod(specsRoot, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		defer os.Chmod(specsRoot, 0o755) //nolint:errcheck

		if _, err := Run(Options{Root: b.root}); err == nil {
			t.Fatalf("want an error when openspec/specs is unreadable")
		}
	})

	t.Run("one capability's live spec.md unreadable", func(t *testing.T) {
		b := newFixtureBuilder(t)
		b.step("auth", addDeltaText("Password login"))
		capDir := filepath.Join(b.root, "openspec", "specs", "auth")
		if err := os.Chmod(capDir, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		defer os.Chmod(capDir, 0o755) //nolint:errcheck

		if _, err := Run(Options{Root: b.root}); err == nil {
			t.Fatalf("want an error when a capability's live spec directory is unreadable")
		}
	})
}

// appendBugRecord appends one delta-less bug archive record (Capability ==
// "", the doc.go/archive.go convention) directly via archive.AppendRecords
// — fixtureBuilder.step always sets a real Capability, so this is the only
// way to exercise the "skip delta-less bug records" branch both
// checkDigestChain and checkReplay carry.
func appendBugRecord(t *testing.T, root, change string) {
	t.Helper()
	if _, err := archive.AppendRecords(root, []archive.Record{{
		Change:             change,
		Capability:         "",
		PreImageSha:        archive.EmptyImageSHA,
		PostImageSha:       archive.EmptyImageSHA,
		DeltaOps:           []archive.DeltaOp{},
		ArchiveManifestSha: archive.EmptyImageSHA, // no archived folder needed for this test
	}}); err != nil {
		t.Fatalf("AppendRecords: %v", err)
	}
}

func TestCheckDigestChain_SkipsDeltaLessBugRecords(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	appendBugRecord(t, b.root, "999-bugfix")

	records, err := archive.ReadAll(b.root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	findings, err := checkDigestChain(b.root, records)
	if err != nil {
		t.Fatalf("checkDigestChain: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("want the bug record skipped (no capability to check), got %+v", findings)
	}
}

func TestCheckReplay_SkipsDeltaLessBugRecords(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	appendBugRecord(t, b.root, "999-bugfix")

	records, err := archive.ReadAll(b.root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	findings, err := checkReplay(b.root, records)
	if err != nil {
		t.Fatalf("checkReplay: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("want the bug record skipped (no capability to replay), got %+v", findings)
	}
}

func TestCheckReplay_TaintedByUnparsableArchivedDelta(t *testing.T) {
	b := newFixtureBuilder(t)
	change := b.step("auth", addDeltaText("Password login"))

	deltaPath := filepath.Join(b.root, "openspec", "changes", "archive", change, "specs", "auth", "spec.md")
	if err := os.WriteFile(deltaPath, []byte("not a delta at all, no recognized section"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	records, err := archive.ReadAll(b.root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	findings, err := checkReplay(b.root, records)
	if err != nil {
		t.Fatalf("checkReplay: %v", err)
	}
	if len(findings) != 1 || findings[0].Check != CheckReplay {
		t.Fatalf("want exactly one replay finding, got %+v", findings)
	}
}

func TestCheckReplay_TaintedByUnfoldableArchivedDelta(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	change2 := b.step("auth", addDeltaText("Session expiry"))

	// Corrupt the SECOND archived delta so it MODIFIEs a requirement name
	// that was never ADDED — parses fine, but Fold itself refuses it
	// (spec.KindFoldModifyMissing).
	deltaPath := filepath.Join(b.root, "openspec", "changes", "archive", change2, "specs", "auth", "spec.md")
	if err := os.WriteFile(deltaPath, []byte(modifyDeltaText("Never added", " (v2)")), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	records, err := archive.ReadAll(b.root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	findings, err := checkReplay(b.root, records)
	if err != nil {
		t.Fatalf("checkReplay: %v", err)
	}
	if len(findings) != 1 || findings[0].Check != CheckReplay {
		t.Fatalf("want exactly one replay finding, got %+v", findings)
	}
}

func TestCheckReplay_PermissionErrors(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	t.Run("live spec.md file unreadable", func(t *testing.T) {
		b := newFixtureBuilder(t)
		b.step("auth", addDeltaText("Password login"))
		specPath := filepath.Join(b.root, "openspec", "specs", "auth", "spec.md")
		if err := os.Chmod(specPath, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		defer os.Chmod(specPath, 0o644) //nolint:errcheck

		records, err := archive.ReadAll(b.root)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if _, err := checkReplay(b.root, records); err == nil {
			t.Fatalf("want an error when the live spec.md file itself is unreadable")
		}
	})

	t.Run("specs directory unreadable", func(t *testing.T) {
		b := newFixtureBuilder(t)
		b.step("auth", addDeltaText("Password login"))
		specsRoot := filepath.Join(b.root, "openspec", "specs")
		if err := os.Chmod(specsRoot, 0o000); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		defer os.Chmod(specsRoot, 0o755) //nolint:errcheck

		records, err := archive.ReadAll(b.root)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if _, err := checkReplay(b.root, records); err == nil {
			t.Fatalf("want an error when openspec/specs is unreadable")
		}
	})
}

func TestListLiveCapabilities_PermissionError(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	specsRoot := filepath.Join(b.root, "openspec", "specs")
	if err := os.Chmod(specsRoot, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(specsRoot, 0o755) //nolint:errcheck

	if _, err := listLiveCapabilities(b.root); err == nil {
		t.Fatalf("want an error when openspec/specs is unreadable")
	}
}

func TestCheckImmutability_ManifestSHAPermissionError(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	b := newFixtureBuilder(t)
	change := b.step("auth", addDeltaText("Password login"))
	deltaPath := filepath.Join(b.root, "openspec", "changes", "archive", change, "specs", "auth", "spec.md")
	if err := os.Chmod(deltaPath, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(deltaPath, 0o644) //nolint:errcheck

	if _, err := Run(Options{Root: b.root}); err == nil {
		t.Fatalf("want an error when an archived delta file is unreadable (ManifestSHA can't hash it)")
	}
}

// TestCheckReplay_NoLiveSpecsDirectoryAtAll covers listLiveCapabilities'
// "openspec/specs/ doesn't exist at all" branch as reached FROM
// checkReplay: a project whose only ledger history is delta-less bug
// archives (Capability == "", never creating any capability) legitimately
// has no openspec/specs/ directory yet.
func TestCheckReplay_NoLiveSpecsDirectoryAtAll(t *testing.T) {
	b := newFixtureBuilder(t) // no step() calls: openspec/specs/ never created
	records := []archive.Record{{Change: "999-bugfix", Capability: ""}}

	findings, err := checkReplay(b.root, records)
	if err != nil {
		t.Fatalf("checkReplay: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("want no findings, got %+v", findings)
	}
}

// TestCheckReplay_ListLiveCapabilitiesPermissionError covers
// listLiveCapabilities' real-I/O-error branch AS SURFACED FROM checkReplay
// (replay.go's own "listing %s" wrap), which needs checkReplay's earlier
// per-capability byte-compare loop to complete successfully first: chmod
// only removes READ (list) permission from openspec/specs/, keeping
// EXECUTE (traverse) permission, so `checkReplay` can still directly open
// the already-known "auth" subdirectory's spec.md (no directory listing
// needed for that), and only listLiveCapabilities' own os.ReadDir call
// (which must enumerate entries) fails.
func TestCheckReplay_ListLiveCapabilitiesPermissionError(t *testing.T) {
	testutil.SkipUnlessPermissionEnforcement(t)

	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	specsRoot := filepath.Join(b.root, "openspec", "specs")
	if err := os.Chmod(specsRoot, 0o111); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(specsRoot, 0o755) //nolint:errcheck

	records, err := archive.ReadAll(b.root)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if _, err := checkReplay(b.root, records); err == nil {
		t.Fatalf("want an error when openspec/specs cannot be listed (execute-only)")
	}
}
