package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/config"
	"github.com/kentra-io/spec-lifecycle/internal/constitution"
	"github.com/kentra-io/spec-lifecycle/internal/schema"
)

// TestMain implements the classic os/exec "fake subprocess" idiom (verbatim
// mirror of internal/constitution's own main_test.go): when re-exec'd with
// GO_WANT_HELPER_PROCESS=1, this test binary behaves like a `constitution`
// binary instead of running `go test`, so RunInit's constitution-preflight
// step can be exercised without depending on the real companion binary
// being installed.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT")) //nolint:errcheck
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR")) //nolint:errcheck
		code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeConstitutionBin configures the current test binary to behave, when
// re-exec'd as a subprocess, like `constitution --version` printing stdout
// and exiting 0; the returned path (os.Args[0], absolute) is passed
// straight to InitOptions.ConstitutionBinOverride, which resolves via
// internal/constitution.Locate's "contains a path separator ⇒ stat it
// directly" rule — no PATH lookup involved.
func fakeConstitutionBin(t *testing.T, versionLine string) string {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", "0")
	t.Setenv("HELPER_STDOUT", versionLine)
	t.Setenv("HELPER_STDERR", "")
	return os.Args[0]
}

// noConstitutionEnv isolates a test from any real `constitution` binary
// that might happen to be on the sandbox's PATH, so "missing binary"
// assertions are deterministic regardless of the host environment.
func noConstitutionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", "")
	t.Setenv(constitution.EnvBinOverride, "")
}

func newInitOpts(root string) InitOptions {
	return InitOptions{
		Root:   root,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}

func TestRunInit_FreshRepoWritesEverything(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	res, err := RunInit(newInitOpts(root))
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if !res.Fresh {
		t.Error("Fresh = false, want true for a brand-new repo")
	}

	// step a: openspec/{changes,specs}
	for _, dir := range []string{"openspec/changes", "openspec/specs"} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir))); err != nil || !info.IsDir() {
			t.Errorf("%s: not created (%v)", dir, err)
		}
	}

	// step b: schema descriptor
	if mismatches, err := schema.Verify(root); err != nil || len(mismatches) != 0 {
		t.Errorf("schema.Verify(root) = %v, %v; want no mismatches", mismatches, err)
	}

	// steps c/d: openspec/config.yaml
	cfgYAML, err := os.ReadFile(filepath.Join(root, "openspec", "config.yaml"))
	if err != nil {
		t.Fatalf("reading openspec/config.yaml: %v", err)
	}
	if !strings.Contains(string(cfgYAML), "schema: kentra-spec-lifecycle") {
		t.Errorf("openspec/config.yaml missing schema key:\n%s", cfgYAML)
	}
	if !strings.Contains(string(cfgYAML), defaultProjectContext) {
		t.Errorf("openspec/config.yaml missing seeded context:\n%s", cfgYAML)
	}

	// step e: lifecycle.yml
	if res.ConfigPath != filepath.Join(root, "lifecycle.yml") {
		t.Errorf("ConfigPath = %q, want %s/lifecycle.yml", res.ConfigPath, root)
	}
	cfg, err := config.Load(res.ConfigPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.ConsentPolicy != config.ConsentStrict {
		t.Errorf("ConsentPolicy = %q, want %q", cfg.ConsentPolicy, config.ConsentStrict)
	}
	if cfg.SpecFormat.Convention != config.ConventionOpenSpec || cfg.SpecFormat.Grammar != "1.5.0" {
		t.Errorf("SpecFormat = %+v", cfg.SpecFormat)
	}
	if len(cfg.Runtimes) != 3 {
		t.Errorf("Runtimes = %v, want the 3 defaults", cfg.Runtimes)
	}
	if cfg.SourceTracking.Type != config.SourceTrackingNone {
		t.Errorf("SourceTracking.Type = %q, want %q (default)", cfg.SourceTracking.Type, config.SourceTrackingNone)
	}

	// step f: constitution preflight — no binary on PATH ⇒ a warning, never
	// a failure.
	if !anyWarningContains(res.Warnings, "constitution binary not found") {
		t.Errorf("Warnings = %v, want a missing-constitution-binary notice", res.Warnings)
	}

	// steps g/h: skills fan-out + managed pointer blocks
	for _, dir := range []string{".claude", ".agents", ".cursor"} {
		for _, skill := range []string{"lifecycle-refine", "lifecycle-design", "lifecycle-plan", "lifecycle-bug", "lifecycle-archive", "lifecycle-new-feature"} {
			p := filepath.Join(root, dir, "skills", skill, "SKILL.md")
			if _, err := os.Stat(p); err != nil {
				t.Errorf("missing fanned-out skill %s: %v", p, err)
			}
		}
	}
	for _, f := range []string{"CLAUDE.md", "AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Errorf("%s not written: %v", f, err)
			continue
		}
		if !bytes.Contains(data, []byte(BlockBegin)) {
			t.Errorf("%s missing the managed block markers", f)
		}
	}
}

func TestRunInit_IdempotentByteIdenticalReRun(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	if _, err := RunInit(newInitOpts(root)); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}
	before := readAll(t, root)

	if _, err := RunInit(newInitOpts(root)); err != nil {
		t.Fatalf("second RunInit: %v", err)
	}
	after := readAll(t, root)

	if len(before) != len(after) {
		t.Fatalf("file count changed on re-run: before=%d after=%d", len(before), len(after))
	}
	for name, b := range before {
		a, ok := after[name]
		if !ok {
			t.Errorf("%s disappeared on re-run", name)
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s changed on idempotent re-run", name)
		}
	}
}

func TestRunInit_ExistingLifecycleYMLWinsOnReRun(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	if _, err := RunInit(newInitOpts(root)); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}

	// Hand-edit lifecycle.yml to a narrower runtimes list, as a human would
	// after init. A second init with different --runtimes flags must
	// leave it alone (matching the constitution's own "existing config
	// wins" stance).
	lifecyclePath := filepath.Join(root, "lifecycle.yml")
	custom := "schemaVersion: 1\nconsentPolicy: strict\nruntimes: [cursor]\n"
	if err := os.WriteFile(lifecyclePath, []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := newInitOpts(root)
	opts.Runtimes = []string{config.RuntimeClaudeCode, config.RuntimeCodex}
	res, err := RunInit(opts)
	if err != nil {
		t.Fatalf("second RunInit: %v", err)
	}
	if res.Fresh {
		t.Error("Fresh = true on a re-run against an existing lifecycle.yml")
	}

	cfg, err := config.Load(lifecyclePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Runtimes) != 1 || cfg.Runtimes[0] != config.RuntimeCursor {
		t.Errorf("Runtimes = %v, want the hand-edited [cursor] to survive", cfg.Runtimes)
	}
}

func TestRunInit_ManagedBlockDriftRefusedThenForced(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	if _, err := RunInit(newInitOpts(root)); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}

	claudePath := filepath.Join(root, "CLAUDE.md")
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	drifted, err := ApplyBlock(content, "@HACKED.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath, drifted, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := RunInit(newInitOpts(root)); err == nil {
		t.Fatal("expected a refusal for drifted CLAUDE.md without --force")
	} else if !strings.Contains(err.Error(), "drifted") {
		t.Errorf("error = %q, want it to mention drift", err.Error())
	}
	got, _ := os.ReadFile(claudePath)
	if !bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("a refused init must not have rewritten CLAUDE.md")
	}

	forced := newInitOpts(root)
	forced.Force = true
	if _, err := RunInit(forced); err != nil {
		t.Fatalf("forced RunInit: %v", err)
	}
	got, _ = os.ReadFile(claudePath)
	if bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("--force must overwrite the drifted interior")
	}
}

func TestRunInit_MarkerErrorRefusesOutright(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# x\n"+BlockBegin+"\nno end\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := RunInit(newInitOpts(root))
	var me *MarkerError
	if !errors.As(err, &me) {
		t.Fatalf("expected a *MarkerError, got %v", err)
	}
}

func TestRunInit_ConstitutionPreflight_PresentMatchingPin(t *testing.T) {
	root := t.TempDir()
	bin := fakeConstitutionBin(t, "constitution version 0.3.1 (abcdef012345)\n")

	opts := newInitOpts(root)
	opts.ConstitutionBinOverride = bin
	res, err := RunInit(opts)
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	cfg, err := config.Load(res.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Constitution.Version != "0.3.x" {
		t.Errorf("Constitution.Version = %q, want the detected 0.3.x pin", cfg.Constitution.Version)
	}
	if anyWarningContains(res.Warnings, "constitution") {
		t.Errorf("Warnings = %v, want no constitution warnings for a matching, freshly-detected pin", res.Warnings)
	}
}

// TestRunInit_ConstitutionPreflight_VersionMismatchOnReRun covers the
// other half of implementation-plan.md §2.7's preflight — a binary that
// IS present but whose reported version does not satisfy an EXISTING
// lifecycle.yml's constitution.version pin (as opposed to
// PresentMatchingPin above, which only exercises a freshly-detected pin
// that trivially matches itself). A pre-existing lifecycle.yml wins on a
// re-run (ensureLifecycleConfig), so this is the only way a mismatch
// pin/binary pair can arise in practice: the pin was set on an earlier
// `init` (or by hand) against a constitution version that has since
// moved on. Per spec-lifecycle.md §7 item 5, a mismatch is a WARNING,
// never a failure.
func TestRunInit_ConstitutionPreflight_VersionMismatchOnReRun(t *testing.T) {
	root := t.TempDir()
	bin := fakeConstitutionBin(t, "constitution version 0.9.0 (abcdef012345)\n")

	lifecyclePath := filepath.Join(root, "lifecycle.yml")
	pinned := "schemaVersion: 1\nconsentPolicy: strict\nconstitution:\n  version: 0.1.x\n"
	if err := os.WriteFile(lifecyclePath, []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := newInitOpts(root)
	opts.ConstitutionBinOverride = bin
	res, err := RunInit(opts)
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if res.Fresh {
		t.Error("Fresh = true against a pre-existing lifecycle.yml")
	}
	if !anyWarningContains(res.Warnings, `does not satisfy the pinned version "0.1.x"`) {
		t.Errorf("Warnings = %v, want a version-mismatch notice against the pinned 0.1.x", res.Warnings)
	}

	// The pin itself is untouched — a re-run never silently rewrites a
	// human-set constitution.version to match whatever binary happens to
	// be installed; that would defeat the point of pinning it.
	cfg, err := config.Load(lifecyclePath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Constitution.Version != "0.1.x" {
		t.Errorf("Constitution.Version = %q, want the pre-existing 0.1.x pin preserved", cfg.Constitution.Version)
	}
}

func TestRunInit_ConstitutionPreflight_MissingBinary(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	res, err := RunInit(newInitOpts(root))
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if !anyWarningContains(res.Warnings, "constitution binary not found") {
		t.Errorf("Warnings = %v, want a missing-binary notice", res.Warnings)
	}
	// init must still succeed end to end even with the companion absent.
	if _, err := os.Stat(filepath.Join(root, "lifecycle.yml")); err != nil {
		t.Errorf("lifecycle.yml not written despite the missing constitution binary: %v", err)
	}
}

func TestRunInit_SourceTrackingDisagreementWarning(t *testing.T) {
	root := t.TempDir()
	noConstitutionEnv(t)

	if err := os.WriteFile(filepath.Join(root, "constitution.yml"), []byte("sourceTracking:\n  type: jira\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := newInitOpts(root)
	opts.SourceTrackingType = config.SourceTrackingGitHubIssue
	res, err := RunInit(opts)
	if err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	if !anyWarningContains(res.Warnings, "disagrees") {
		t.Errorf("Warnings = %v, want a sourceTracking-disagreement notice", res.Warnings)
	}
}

func anyWarningContains(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}
