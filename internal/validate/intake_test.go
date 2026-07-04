package validate

import (
	"path/filepath"
	"testing"
)

func TestReadProposalMetaDefaultsToFeature(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)

	meta, err := ReadProposalMeta(dir)
	if err != nil {
		t.Fatalf("ReadProposalMeta: %v", err)
	}
	if meta.Type != ChangeTypeFeature {
		t.Errorf("Type = %q, want %q (default)", meta.Type, ChangeTypeFeature)
	}
	if meta.Issue != "kentra-io/kafka-dq#42" {
		t.Errorf("Issue = %q, want kentra-io/kafka-dq#42", meta.Issue)
	}
	if meta.DesignSkip {
		t.Errorf("DesignSkip = true, want false")
	}
}

func TestReadProposalMetaBugType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`)

	meta, err := ReadProposalMeta(dir)
	if err != nil {
		t.Fatalf("ReadProposalMeta: %v", err)
	}
	if meta.Type != ChangeTypeBug {
		t.Errorf("Type = %q, want %q", meta.Type, ChangeTypeBug)
	}
}

func TestReadProposalMetaMissingFile(t *testing.T) {
	dir := t.TempDir()
	meta, err := ReadProposalMeta(dir)
	if err != nil {
		t.Fatalf("ReadProposalMeta: %v", err)
	}
	if meta.Type != ChangeTypeFeature || meta.Issue != "" {
		t.Errorf("ReadProposalMeta on missing file = %+v, want zero-value feature default", meta)
	}
}

func TestReadProposalMetaMalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\nissue: [unterminated\n---\n\nbody\n")

	meta, err := ReadProposalMeta(dir)
	if err != nil {
		t.Fatalf("ReadProposalMeta: %v", err)
	}
	if meta.Type != ChangeTypeFeature {
		t.Errorf("Type = %q, want default %q on malformed frontmatter", meta.Type, ChangeTypeFeature)
	}
}

func TestHasSpecsDeltas(t *testing.T) {
	dir := t.TempDir()
	if HasSpecsDeltas(dir) {
		t.Errorf("HasSpecsDeltas on empty dir = true, want false")
	}
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)
	if !HasSpecsDeltas(dir) {
		t.Errorf("HasSpecsDeltas after writing a delta = false, want true")
	}
}

func TestHasArtifact(t *testing.T) {
	dir := t.TempDir()
	if HasArtifact(dir, "tasks.md") {
		t.Errorf("HasArtifact(tasks.md) on empty dir = true, want false")
	}
	writeFile(t, filepath.Join(dir, "tasks.md"), "# tasks")
	if !HasArtifact(dir, "tasks.md") {
		t.Errorf("HasArtifact(tasks.md) after writing it = false, want true")
	}
}

func TestExportedValidatorsMatchUnexported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	if findings, err := Proposal(dir); err != nil || len(findings) != 0 {
		t.Errorf("Proposal(dir) = %v, %v; want no findings", findings, err)
	}
}
