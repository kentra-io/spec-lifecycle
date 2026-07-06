package archive

import (
	"errors"
	"path/filepath"
	"testing"
)

// These exercise selfCheck (doc.go's post-write self-check) directly,
// rather than via Archive end-to-end: every failure mode here is a
// disk-vs-ledger-record MISMATCH, which by construction never happens on
// Archive's own success path, so a direct unit test is the only way to
// reach these branches without an artificial injection seam.

func TestSelfCheckNoRecordsIsNoop(t *testing.T) {
	if err := selfCheck(t.TempDir(), filepath.Join(t.TempDir(), "missing"), nil, true); err != nil {
		t.Fatalf("selfCheck with no records: %v, want nil", err)
	}
}

func TestSelfCheckPropagatesManifestReadError(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archive-dir-does-not-exist")
	records := []Record{{Seq: 1, ArchiveManifestSha: emptyImageSHA}}

	err := selfCheck(root, archiveDir, records, false)
	if !errors.Is(err, ErrSelfCheckFailed) {
		t.Fatalf("selfCheck = %v, want ErrSelfCheckFailed", err)
	}
}

func TestSelfCheckDetectsManifestMismatch(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archived", "001-change")
	writeFile(t, filepath.Join(archiveDir, "proposal.md"), validProposal)
	records := []Record{{Seq: 1, ArchiveManifestSha: "sha256:not-the-real-manifest"}}

	err := selfCheck(root, archiveDir, records, false)
	if !errors.Is(err, ErrSelfCheckFailed) {
		t.Fatalf("selfCheck = %v, want ErrSelfCheckFailed", err)
	}
}

func TestSelfCheckPropagatesSpecReadError(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archived", "001-change")
	writeFile(t, filepath.Join(archiveDir, "proposal.md"), validProposal)
	manifest, err := ManifestSHA(archiveDir)
	if err != nil {
		t.Fatal(err)
	}
	// hasDelta true, but openspec/specs/auth/spec.md was never written.
	records := []Record{{Seq: 1, Capability: "auth", ArchiveManifestSha: manifest, PostImageSha: emptyImageSHA}}

	err = selfCheck(root, archiveDir, records, true)
	if !errors.Is(err, ErrSelfCheckFailed) {
		t.Fatalf("selfCheck = %v, want ErrSelfCheckFailed", err)
	}
}

func TestSelfCheckDetectsPostImageMismatch(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archived", "001-change")
	writeFile(t, filepath.Join(archiveDir, "proposal.md"), validProposal)
	manifest, err := ManifestSHA(archiveDir)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "openspec", "specs", "auth", "spec.md"), "# auth Specification\n")
	records := []Record{{
		Seq: 1, Capability: "auth", ArchiveManifestSha: manifest,
		PostImageSha: "sha256:not-what-is-on-disk",
	}}

	err = selfCheck(root, archiveDir, records, true)
	if !errors.Is(err, ErrSelfCheckFailed) {
		t.Fatalf("selfCheck = %v, want ErrSelfCheckFailed", err)
	}
}

func TestSelfCheckPassesWhenDiskMatchesRecords(t *testing.T) {
	root := t.TempDir()
	archiveDir := filepath.Join(root, "archived", "001-change")
	writeFile(t, filepath.Join(archiveDir, "proposal.md"), validProposal)
	manifest, err := ManifestSHA(archiveDir)
	if err != nil {
		t.Fatal(err)
	}
	specContent := "# auth Specification\n"
	writeFile(t, filepath.Join(root, "openspec", "specs", "auth", "spec.md"), specContent)
	records := []Record{{
		Seq: 1, Capability: "auth", ArchiveManifestSha: manifest,
		PostImageSha: hashBytes([]byte(specContent)),
	}}

	if err := selfCheck(root, archiveDir, records, true); err != nil {
		t.Fatalf("selfCheck: %v, want nil", err)
	}
}
