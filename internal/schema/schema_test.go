package schema

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestInstallWritesExpectedTree(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	root := Dir(dir)
	wantFiles := []string{
		"schema.yaml",
		"templates/proposal.md",
		"templates/spec.md",
		"templates/design.md",
		"templates/tasks.md",
	}
	for _, rel := range wantFiles {
		path := filepath.Join(root, filepath.FromSlash(rel))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading installed %s: %v", rel, err)
		}
		want, err := assets.ReadFile(rel)
		if err != nil {
			t.Fatalf("reading embedded %s: %v", rel, err)
		}
		if string(got) != string(want) {
			t.Errorf("installed %s does not match embedded asset byte-for-byte", rel)
		}
	}

	if got := filepath.Join(dir, "openspec", "schemas", Name); root != got {
		t.Errorf("Dir(%q) = %q, want %q", dir, root, got)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	if err := Install(dir); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	mismatches, err := Verify(dir)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("Verify after two Installs found mismatches: %v", mismatches)
	}
}

func TestVerifyCleanAfterInstall(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	mismatches, err := Verify(dir)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("Verify on a freshly installed tree found mismatches: %v", mismatches)
	}
}

func TestVerifyDetectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := os.Remove(filepath.Join(Dir(dir), "templates", "design.md")); err != nil {
		t.Fatalf("removing installed file: %v", err)
	}

	mismatches, err := Verify(dir)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(mismatches) != 1 || mismatches[0].Rel != "templates/design.md" || mismatches[0].Reason != "missing" {
		t.Fatalf("Verify mismatches = %v, want exactly one {templates/design.md missing}", mismatches)
	}
}

func TestVerifyDetectsModifiedFile(t *testing.T) {
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	path := filepath.Join(Dir(dir), "schema.yaml")
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tampering with installed file: %v", err)
	}

	mismatches, err := Verify(dir)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(mismatches) != 1 || mismatches[0].Rel != "schema.yaml" || mismatches[0].Reason != "modified" {
		t.Fatalf("Verify mismatches = %v, want exactly one {schema.yaml modified}", mismatches)
	}
}

func TestVerifyReportsMissingDescriptorEntirely(t *testing.T) {
	dir := t.TempDir()
	mismatches, err := Verify(dir)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(mismatches) != 5 { // schema.yaml + 4 templates
		t.Fatalf("Verify on an uninstalled dir found %d mismatches, want 5", len(mismatches))
	}
	for _, m := range mismatches {
		if m.Reason != "missing" {
			t.Errorf("mismatch %+v: want Reason=missing", m)
		}
	}
}

// TestInstallFailsWhenParentPathIsNotADirectory exercises Install's
// os.MkdirAll error branch: a regular file sitting where a path component
// of the descriptor root should be a directory makes MkdirAll fail.
func TestInstallFailsWhenParentPathIsNotADirectory(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "openspec")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("creating blocking file: %v", err)
	}

	if err := Install(dir); err == nil {
		t.Fatal("Install: want error when a descriptor path component is a regular file, got nil")
	}
}

// TestInstallFailsWhenRootDirIsReadOnly exercises Install's
// atomicwrite.WriteFile error branch: MkdirAll succeeds (the root already
// exists) but the directory lacks write permission, so creating the
// temp file for the atomic write fails.
func TestInstallFailsWhenRootDirIsReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks are bypassed")
	}
	dir := t.TempDir()
	root := Dir(dir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("pre-creating root: %v", err)
	}
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatalf("chmod root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	if err := Install(dir); err == nil {
		t.Fatal("Install: want error when descriptor root is not writable, got nil")
	}
}

// TestVerifyPropagatesNonNotExistReadError exercises Verify's error
// branch for a read failure that is not "file does not exist" — e.g. a
// permission error on an installed-but-unreadable file.
func TestVerifyPropagatesNonNotExistReadError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks are bypassed")
	}
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	path := filepath.Join(Dir(dir), "schema.yaml")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod installed file unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := Verify(dir); err == nil {
		t.Fatal("Verify: want error when an installed file cannot be read, got nil")
	}
}

func TestMismatchString(t *testing.T) {
	m := Mismatch{Rel: "templates/tasks.md", Reason: "modified"}
	if got, want := m.String(), "templates/tasks.md: modified"; got != want {
		t.Errorf("Mismatch.String() = %q, want %q", got, want)
	}
}

// TestSchemaYAMLIsWellFormed guards against a hand-edit that breaks the
// descriptor's own YAML syntax (nothing else in this repo parses it back —
// see the package doc — so this is the only check that would catch that).
func TestSchemaYAMLIsWellFormed(t *testing.T) {
	data, err := assets.ReadFile("schema.yaml")
	if err != nil {
		t.Fatalf("reading embedded schema.yaml: %v", err)
	}
	var doc struct {
		Name        string `yaml:"name"`
		Version     int    `yaml:"version"`
		Description string `yaml:"description"`
		Artifacts   []struct {
			ID        string   `yaml:"id"`
			Generates string   `yaml:"generates"`
			Template  string   `yaml:"template"`
			Requires  []string `yaml:"requires"`
		} `yaml:"artifacts"`
		Apply struct {
			Requires []string `yaml:"requires"`
			Tracks   string   `yaml:"tracks"`
		} `yaml:"apply"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("schema.yaml is not valid YAML: %v", err)
	}
	if doc.Name != Name {
		t.Errorf("schema.yaml name = %q, want %q", doc.Name, Name)
	}
	if len(doc.Artifacts) != 4 {
		t.Fatalf("schema.yaml has %d artifacts, want 4 (proposal, specs, design, tasks)", len(doc.Artifacts))
	}
	wantIDs := []string{"proposal", "specs", "design", "tasks"}
	for i, id := range wantIDs {
		if doc.Artifacts[i].ID != id {
			t.Errorf("artifact[%d].id = %q, want %q", i, doc.Artifacts[i].ID, id)
		}
	}
	if doc.Apply.Tracks != "tasks.md" {
		t.Errorf("apply.tracks = %q, want %q", doc.Apply.Tracks, "tasks.md")
	}
}
