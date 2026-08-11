package schema

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/testutil"
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
		"living-spec.schema.json",
		"spec-delta.schema.json",
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
	if len(mismatches) != 6 { // schema.yaml + 3 templates + 2 published JSON Schemas
		t.Fatalf("Verify on an uninstalled dir found %d mismatches, want 6", len(mismatches))
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
	testutil.SkipUnlessPermissionEnforcement(t)
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
	testutil.SkipUnlessPermissionEnforcement(t)
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

// TestPublishedSchemasAreWellFormed guards the two published JSON Schemas
// (change 007, design D4): each must be present under PublishedSchema and
// parse as JSON carrying the $id the delta schema's cross-file $ref and the
// in-process validator both resolve against.
func TestPublishedSchemasAreWellFormed(t *testing.T) {
	for _, name := range []string{LivingSpecSchemaName, SpecDeltaSchemaName} {
		data, err := PublishedSchema(name)
		if err != nil {
			t.Fatalf("PublishedSchema(%q): %v", name, err)
		}
		var doc struct {
			Schema string `json:"$schema"`
			ID     string `json:"$id"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s is not valid JSON: %v", name, err)
		}
		if doc.Schema != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("%s $schema = %q, want draft 2020-12", name, doc.Schema)
		}
		if doc.ID == "" {
			t.Errorf("%s is missing an $id (needed for $ref resolution)", name)
		}
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
		// change 007 M6 retired the top-level apply: block; it must be
		// absent now (the machine plan surface is milestoned-plan-dag).
		Apply *struct {
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
	if len(doc.Artifacts) != 3 {
		t.Fatalf("schema.yaml has %d artifacts, want 3 (proposal, specs, design)", len(doc.Artifacts))
	}
	wantIDs := []string{"proposal", "specs", "design"}
	for i, id := range wantIDs {
		if doc.Artifacts[i].ID != id {
			t.Errorf("artifact[%d].id = %q, want %q", i, doc.Artifacts[i].ID, id)
		}
	}
	if doc.Apply != nil {
		t.Errorf("schema.yaml still declares an apply: block (%+v); change 007 M6 retired it", *doc.Apply)
	}
}

// TestInstalledDescriptorMatchesEmbedded dogfoods the descriptor: this repo
// plans itself through its own openspec/ tree, so its installed descriptor
// must equal what the binary ships. `lifecycle init` installs the descriptor
// only when its marker file is absent and never refreshes it, so without this
// guard the repo's own tree silently rots — which is exactly what happened
// after change 007 (it kept a pre-flip `tasks` artifact and a tasks.md
// template, violating openspec/specs/plan-integration's own requirement).
func TestInstalledDescriptorMatchesEmbedded(t *testing.T) {
	mismatches, err := Verify("../..")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	for _, m := range mismatches {
		t.Errorf("openspec/schemas/%s/%s: %s — re-copy it from internal/schema/", Name, m.Rel, m.Reason)
	}

	// Verify reports "missing" and "modified" only — it never reports a file
	// the descriptor no longer ships. templates/tasks.md is exactly that case
	// (retired by change 007, still installed), so check for extras directly.
	embedded := map[string]bool{}
	rels, err := relPaths()
	if err != nil {
		t.Fatalf("relPaths: %v", err)
	}
	for _, r := range rels {
		embedded[r] = true
	}
	installed := Dir("../..")
	err = filepath.WalkDir(installed, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, rerr := filepath.Rel(installed, p)
		if rerr != nil {
			return rerr
		}
		if rel = filepath.ToSlash(rel); !embedded[rel] {
			t.Errorf("openspec/schemas/%s/%s: installed but not shipped — delete it", Name, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", installed, err)
	}
}
