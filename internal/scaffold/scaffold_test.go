package scaffold

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// testInterior and testSkill are stand-ins for the config-driven interior
// text / embedded skill content the constitution derives from its
// config.Config + go:embed skills FS (TODO(M6/M7) — see scaffold.go). Here
// they're supplied directly so the drift-engine mechanics can be exercised
// without that wiring existing yet.
const testInterior = "@openspec/lifecycle.md"

var testSkillContent = []byte("---\nname: lifecycle-refine\n---\nbody\n")

// newTestRoot returns a scratch repo root with openspec/ present (so
// .state can be written) and one block target + one skill item — enough to
// exercise both a managed block and a fanned-out file.
func newTestRoot(t *testing.T) (root string, blocks []BlockTarget, skills []SkillItem) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}
	blocks = []BlockTarget{{Rel: "CLAUDE.md", Interior: testInterior}}
	skills = []SkillItem{{Rel: filepath.ToSlash(filepath.Join(".claude", "skills", "lifecycle-refine", "SKILL.md")), Content: testSkillContent}}
	return root, blocks, skills
}

func initOpts(root string, blocks []BlockTarget, skills []SkillItem) Options {
	return Options{
		Root: root, Mode: ModeInit,
		BlockTargets: blocks, SkillItems: skills,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}
}

func claudePath(root string) string { return filepath.Join(root, "CLAUDE.md") }
func skillPath(root string) string {
	return filepath.Join(root, ".claude", "skills", "lifecycle-refine", "SKILL.md")
}
func statePathT(root string) string { return filepath.Join(root, "openspec", ".state") }

func TestRefresh_InitWritesAndIsIdempotent(t *testing.T) {
	root, blocks, skills := newTestRoot(t)

	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	claude, err := os.ReadFile(claudePath(root))
	if err != nil {
		t.Fatalf("CLAUDE.md not written: %v", err)
	}
	if !bytes.Contains(claude, []byte(testInterior)) {
		t.Fatalf("CLAUDE.md missing the managed interior:\n%s", claude)
	}
	if _, err := os.Stat(skillPath(root)); err != nil {
		t.Fatalf("skill not fanned out: %v", err)
	}
	if _, err := os.Stat(statePathT(root)); err != nil {
		t.Fatalf(".state not written: %v", err)
	}

	// Snapshot, refresh again, assert byte-identical (idempotent no-op).
	before := readAll(t, root)
	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	after := readAll(t, root)
	for name, b := range before {
		if !bytes.Equal(after[name], b) {
			t.Fatalf("%s changed on idempotent re-run", name)
		}
	}
}

func TestRefresh_NoManagedTargetsSkipsState(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Refresh(Options{Root: root, Mode: ModeInit}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := os.Stat(statePathT(root)); !os.IsNotExist(err) {
		t.Fatal("a repo that manages nothing must not get a .state file")
	}
}

// driftInterior rewrites CLAUDE.md's managed-block interior to simulate a
// user hand-edit.
func driftInterior(t *testing.T, root string) {
	t.Helper()
	content, err := os.ReadFile(claudePath(root))
	if err != nil {
		t.Fatal(err)
	}
	out, err := ApplyBlock(content, "@openspec/HACKED.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudePath(root), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRefresh_WarnLeavesDrift(t *testing.T) {
	root, blocks, skills := newTestRoot(t)
	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatal(err)
	}
	driftInterior(t, root)

	var stderr bytes.Buffer
	err := Refresh(Options{Root: root, Mode: ModeWarn, BlockTargets: blocks, SkillItems: skills, Stderr: &stderr})
	if err != nil {
		t.Fatalf("warn mode must never fail on drift: %v", err)
	}
	got, _ := os.ReadFile(claudePath(root))
	if !bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("warn mode must leave the drifted file untouched")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("drifted")) {
		t.Fatalf("expected a drift warning, got %q", stderr.String())
	}
}

func TestRefresh_InitRefusesDriftWithoutForce(t *testing.T) {
	root, blocks, skills := newTestRoot(t)
	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatal(err)
	}
	driftInterior(t, root)

	err := Refresh(initOpts(root, blocks, skills))
	if err == nil {
		t.Fatal("init mode must refuse a drifted target without --force")
	}
	got, _ := os.ReadFile(claudePath(root))
	if !bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("a refused init must not have rewritten the file")
	}
}

func TestRefresh_InitForceOverwritesDrift(t *testing.T) {
	root, blocks, skills := newTestRoot(t)
	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatal(err)
	}
	driftInterior(t, root)

	o := initOpts(root, blocks, skills)
	o.Force = true
	if err := Refresh(o); err != nil {
		t.Fatalf("force refresh: %v", err)
	}
	got, _ := os.ReadFile(claudePath(root))
	if bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("--force must overwrite the drifted interior")
	}
	if !bytes.Contains(got, []byte(testInterior)) {
		t.Fatal("--force must restore the managed interior")
	}
}

func TestRefresh_InitConfirmOverwritesDrift(t *testing.T) {
	root, blocks, skills := newTestRoot(t)
	if err := Refresh(initOpts(root, blocks, skills)); err != nil {
		t.Fatal(err)
	}
	driftInterior(t, root)

	o := initOpts(root, blocks, skills)
	o.Confirm = func(string) (bool, error) { return true, nil } // user says yes
	if err := Refresh(o); err != nil {
		t.Fatalf("confirmed refresh: %v", err)
	}
	got, _ := os.ReadFile(claudePath(root))
	if bytes.Contains(got, []byte("HACKED")) {
		t.Fatal("an accepted confirm must overwrite the drifted interior")
	}
}

func TestPreflightBlocks_BrokenMarkers(t *testing.T) {
	root, blocks, _ := newTestRoot(t)
	if err := os.WriteFile(claudePath(root), []byte("# x\n"+BlockBegin+"\nno end\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := PreflightBlocks(root, blocks)
	var me *MarkerError
	if !errors.As(err, &me) {
		t.Fatalf("expected a *MarkerError, got %v", err)
	}
	if me.Path != "CLAUDE.md" {
		t.Fatalf("MarkerError.Path = %q, want CLAUDE.md", me.Path)
	}
}

func TestRefresh_WarnToleratesBrokenMarkers(t *testing.T) {
	root, blocks, skills := newTestRoot(t)
	if err := os.WriteFile(claudePath(root), []byte("# x\n"+BlockBegin+"\nno end\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if err := Refresh(Options{Root: root, Mode: ModeWarn, BlockTargets: blocks, SkillItems: skills, Stderr: &stderr}); err != nil {
		t.Fatalf("warn mode must not fail on broken markers: %v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("BEGIN marker present without")) {
		t.Fatalf("expected a marker warning, got %q", stderr.String())
	}
}

// readAll returns the bytes of every regular file under root, keyed by path
// relative to root.
func readAll(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
