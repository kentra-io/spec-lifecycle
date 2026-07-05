package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureProjectConfig_FreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	changed, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "hello context")
	if err != nil {
		t.Fatalf("EnsureProjectConfig: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true for a freshly created file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	got := string(data)
	if !strings.Contains(got, "schema: kentra-spec-lifecycle") {
		t.Errorf("config.yaml missing schema key:\n%s", got)
	}
	if !strings.Contains(got, "hello context") {
		t.Errorf("config.yaml missing seeded context:\n%s", got)
	}
}

func TestEnsureProjectConfig_PreservesUnknownKeysAndComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "" +
		"# a user comment above rules\n" +
		"rules:\n" +
		"  proposal:\n" +
		"    - keep it short\n" +
		"store: my-store\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "seeded context")
	if err != nil {
		t.Fatalf("EnsureProjectConfig: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true (schema/context keys were added)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{"# a user comment above rules", "rules:", "keep it short", "store: my-store", "schema: kentra-spec-lifecycle", "seeded context"} {
		if !strings.Contains(got, want) {
			t.Errorf("config.yaml missing %q after edit:\n%s", want, got)
		}
	}
}

func TestEnsureProjectConfig_PreservesInlineCommentOnOverwrittenSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "" +
		"schema: some-other-schema  # user had a different schema\n" +
		"customKey: kept\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "seeded context")
	if err != nil {
		t.Fatalf("EnsureProjectConfig: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true (schema value needed overwriting)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "schema: kentra-spec-lifecycle") {
		t.Errorf("schema value was not overwritten:\n%s", got)
	}
	if !strings.Contains(got, "# user had a different schema") {
		t.Errorf("overwriting schema's value silently dropped the user's inline comment:\n%s", got)
	}
	if !strings.Contains(got, "customKey: kept") {
		t.Errorf("config.yaml missing unrelated unknown key after edit:\n%s", got)
	}
}

func TestEnsureProjectConfig_SchemaAlwaysSetContextOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "original context"); err != nil {
		t.Fatal(err)
	}

	// Simulate a user hand-editing the seeded context and changing the
	// schema to something else (as if they briefly pointed at a different
	// schema) — a re-run must restore schema but leave the user's context
	// edit alone.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), "schema: kentra-spec-lifecycle", "schema: something-else", 1)
	tampered = strings.Replace(tampered, "original context", "user-edited context", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "original context")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false, want true (schema needed restoring)")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "schema: kentra-spec-lifecycle") {
		t.Errorf("schema key was not restored:\n%s", got)
	}
	if strings.Contains(string(got), "something-else") {
		t.Errorf("stale schema value survived:\n%s", got)
	}
	if !strings.Contains(string(got), "user-edited context") {
		t.Errorf("user's context edit was clobbered:\n%s", got)
	}
}

func TestEnsureProjectConfig_IdempotentNoOpSkipsWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "context text"); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	changed, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "context text")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("changed = true on a byte-identical re-run, want false")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("file was rewritten on a no-op re-run")
	}
}

func TestEnsureProjectConfig_RefusesNonMappingRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("- just\n- a\n- list\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "context")
	if err == nil {
		t.Fatal("expected an error for a non-mapping root")
	}
	if !strings.Contains(err.Error(), "must be a YAML mapping") {
		t.Errorf("error = %q, want it to mention the mapping requirement", err.Error())
	}
}

func TestEnsureProjectConfig_RefusesInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("schema: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := EnsureProjectConfig(path, "kentra-spec-lifecycle", "context")
	if err == nil {
		t.Fatal("expected an error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "not valid YAML") {
		t.Errorf("error = %q, want it to mention \"not valid YAML\"", err.Error())
	}
}
