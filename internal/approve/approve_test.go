package approve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kentra-io/spec-lifecycle/internal/constitution"
	"github.com/kentra-io/spec-lifecycle/internal/schema"
)

// --- fake constitution subprocess (classic os/exec TestHelperProcess
// idiom — hermetic: no real binary is built or executed for these unit
// tests; the real-binary path is exercised by cmd/lifecycle's testscript
// e2e suite instead, per implementation-plan.md M3's DoD). ---

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
// re-exec'd as a subprocess, like `constitution deviation validate <path>`
// exiting with code and printing stdout/stderr.
func fakeConstitutionBin(t *testing.T, code int, stdout, stderr string) string {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", strconv.Itoa(code))
	t.Setenv("HELPER_STDOUT", stdout)
	t.Setenv("HELPER_STDERR", stderr)
	return os.Args[0]
}

// --- fixtures ---

const validProposal = `---
issue: "kentra-io/kafka-dq#42"
designSkip: false
---

# Add password login

## Why
Users need to authenticate.

## What Changes
- **auth:** ADDED - password login.

## Impact
- New capability: auth.
`

const validDelta = `## ADDED Requirements

### Requirement: Password login
The system SHALL allow a registered user to authenticate with a username and password.

#### Scenario: Successful login
- **GIVEN** a registered user
- **WHEN** they submit correct credentials
- **THEN** the system SHALL grant a session
`

const validDesign = `# Add password login — Design

## Context
Background.

## Goals / Non-Goals
**Goals:**
Ship login.

**Non-Goals:**
SSO.

## Decisions
Use bcrypt.

## NFR Discharge
- Latency: p99 under 200ms, verified by benchmark.

## ADR proposals
(none)

## Risks / Trade-offs
None known.
`

const validTasks = `## Milestone 1: Password login
**Goal** — implement password-based login.
**Deliverables** — login handler, session cookie.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - ` + "`go test ./auth/...`" + ` passes
  - Scenario "Successful login" passes
**Steps** — ordered breakdown, sized per ` + "`planGranularity`" + `:
  1. Implement login handler
  2. Write scenario test
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// newProject creates <tmp>/openspec/changes/<change>/ and returns (root,
// changeDir).
func newProject(t *testing.T, change string) (root, changeDir string) {
	t.Helper()
	root = t.TempDir()
	changeDir = filepath.Join(root, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, changeDir
}

func offGate() ConsentGate { return ConsentGate{Policy: "off"} }

// --- consent ---

func TestApproveConsentStrictRefusesWithoutApprove(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageRefine,
		Consent: ConsentGate{Policy: "strict", IsTTY: false},
	}
	_, err := Approve(req)
	if !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("Approve() error = %v, want ErrConsentRequired", err)
	}
	if _, statErr := os.Stat(StatePath(changeDir)); statErr == nil {
		t.Error("approval-state.json was written despite refused consent")
	}
}

func TestApproveConsentStrictWithApproveFlagSucceeds(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageRefine,
		Consent: ConsentGate{Policy: "strict", Approved: true},
	}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if res.Entry.Status != StatusApproved {
		t.Errorf("Status = %q, want approved", res.Entry.Status)
	}
}

func TestApproveConsentOffProceedsWithoutApprove(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	if _, err := Approve(req); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
}

// --- artifact validation / refusal ---

func TestApproveRefusesInvalidArtifact(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	// No proposal.md at all.

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	res, err := Approve(req)
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("Approve() error = %v, want ErrInvalidArtifact", err)
	}
	if len(res.Findings) == 0 {
		t.Error("Result.Findings is empty, want the missing_artifact finding")
	}
	if _, statErr := os.Stat(StatePath(changeDir)); statErr == nil {
		t.Error("approval-state.json was written despite invalid artifact")
	}
}

func TestApproveRejectBypassesValidation(t *testing.T) {
	root, _ := newProject(t, "042-user-auth")
	// No proposal.md — would fail validation, but --reject should still record.

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Reject: true, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if res.Entry.Status != StatusRejected {
		t.Errorf("Status = %q, want rejected", res.Entry.Status)
	}
	if len(res.Entry.Artifacts) != 0 {
		t.Errorf("Artifacts = %v, want empty (nothing exists to hash)", res.Entry.Artifacts)
	}
}

func TestApproveDesignSkipOnlyValidOnRefine(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(""))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, DesignSkip: true,
		Consent: offGate(), ConstitutionBin: fakeConstitutionBin(t, 0, "ok\n", ""),
	}
	_, err := Approve(req)
	if !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

func TestApproveUnrecognizedStage(t *testing.T) {
	root, _ := newProject(t, "042-user-auth")
	req := Request{Root: root, Change: "042-user-auth", Stage: Stage("bogus"), Consent: offGate()}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

func TestApproveMissingChangeDir(t *testing.T) {
	root := t.TempDir()
	req := Request{Root: root, Change: "does-not-exist", Stage: StageRefine, Consent: offGate()}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

// --- hashing / artifacts ---

func TestApproveHashesRealFileContent(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	want, err := hashFile(filepath.Join(changeDir, "proposal.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Entry.Artifacts["proposal.md"]; got != want {
		t.Errorf("Artifacts[proposal.md] = %q, want %q", got, want)
	}
	if _, ok := res.Entry.Artifacts["specs/auth/spec.md"]; !ok {
		t.Errorf("Artifacts = %v, want a specs/auth/spec.md entry", res.Entry.Artifacts)
	}
}

func TestApproveConstitutionHashOmittedWithWarningWhenAbsent(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if res.Entry.ConstitutionHash != "" {
		t.Errorf("ConstitutionHash = %q, want empty (no constitution/constitution.md)", res.Entry.ConstitutionHash)
	}
	if len(res.Warnings) == 0 {
		t.Error("Warnings is empty, want a note about the missing constitution.md")
	}
}

func TestApproveConstitutionHashPopulatedWhenPresent(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)
	writeFile(t, filepath.Join(root, "constitution", "constitution.md"), "# Constitution\n\nNo rules yet.\n")

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if !strings.HasPrefix(res.Entry.ConstitutionHash, "sha256:") {
		t.Errorf("ConstitutionHash = %q, want a sha256: hash", res.Entry.ConstitutionHash)
	}
}

// --- bug flow: repro / fix ---

func TestApproveReproPlainBugNoSpecsRequired(t *testing.T) {
	root, changeDir := newProject(t, "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`)

	req := Request{Root: root, Change: "007-fix-panic", Stage: StageRepro, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if _, ok := res.Entry.Artifacts["proposal.md"]; !ok {
		t.Errorf("Artifacts = %v, want proposal.md hashed", res.Entry.Artifacts)
	}
	for k := range res.Entry.Artifacts {
		if strings.HasPrefix(k, "specs/") {
			t.Errorf("Artifacts unexpectedly includes %q for a non-promoted bug", k)
		}
	}
}

func TestApproveReproPromotedBugValidatesSpecsDelta(t *testing.T) {
	root, changeDir := newProject(t, "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), `---
issue: "kentra-io/kafka-dq#7"
type: bug
---

# Fix panic on empty input
`)
	writeFile(t, filepath.Join(changeDir, "specs", "parser", "spec.md"), "## ADDED Requirements\n\nnot a valid requirement heading\n")

	req := Request{Root: root, Change: "007-fix-panic", Stage: StageRepro, Consent: offGate()}
	res, err := Approve(req)
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("Approve() error = %v, want ErrInvalidArtifact (malformed promoted-bug delta)", err)
	}
	if len(res.Findings) == 0 {
		t.Error("Findings is empty, want the delta parse error")
	}
}

func TestApproveFixOptionalTasksAbsent(t *testing.T) {
	root, _ := newProject(t, "007-fix-panic")
	req := Request{Root: root, Change: "007-fix-panic", Stage: StageFix, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v, want success (tasks.md is optional for fix)", err)
	}
	if len(res.Entry.Artifacts) != 0 {
		t.Errorf("Artifacts = %v, want empty (no tasks.md present)", res.Entry.Artifacts)
	}
}

func TestApproveFixValidatesTasksWhenPresent(t *testing.T) {
	root, changeDir := newProject(t, "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), "## Milestone 1: incomplete\n**Goal** — x\n")

	req := Request{Root: root, Change: "007-fix-panic", Stage: StageFix, Consent: offGate()}
	res, err := Approve(req)
	if !errors.Is(err, ErrInvalidArtifact) {
		t.Fatalf("Approve() error = %v, want ErrInvalidArtifact (tasks.md missing required labels)", err)
	}
	if len(res.Findings) == 0 {
		t.Error("Findings is empty, want missing-label findings")
	}
}

func TestApproveFixWellFormedTasks(t *testing.T) {
	root, changeDir := newProject(t, "007-fix-panic")
	writeFile(t, filepath.Join(changeDir, "tasks.md"), validTasks)

	req := Request{Root: root, Change: "007-fix-panic", Stage: StageFix, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if _, ok := res.Entry.Artifacts["tasks.md"]; !ok {
		t.Errorf("Artifacts = %v, want tasks.md hashed", res.Entry.Artifacts)
	}
}

// --- deviation gate (design/plan) ---

func validDeviationJSON(hash string) string {
	doc := map[string]any{
		"generatedAt":      "2026-07-03T00:00:00Z",
		"constitutionHash": hash,
		"plan":             "design.md",
		"deviations":       []any{},
		"summary":          map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0},
	}
	data, _ := json.Marshal(doc)
	return string(data)
}

func TestApproveDesignRequiresDeviationJSON(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	// No deviation.json.

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: fakeConstitutionBin(t, 0, "ok\n", ""),
	}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

func TestApproveDesignRequiresConstitutionBin(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(""))

	req := Request{Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(), ConstitutionBin: ""}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

func TestApproveDesignDeviationValid(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(root, "constitution", "constitution.md"), "# Constitution\n\nNo rules yet.\n")

	hash, ok, err := constitution.Hash(root)
	if !ok || err != nil {
		t.Fatalf("constitution.Hash: ok=%v err=%v", ok, err)
	}
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(hash))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: fakeConstitutionBin(t, 0, "deviation validate: ok\n", ""),
	}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if res.Entry.DeviationRef == nil || *res.Entry.DeviationRef != "deviation.json" {
		t.Errorf("DeviationRef = %v, want %s", res.Entry.DeviationRef, "deviation.json")
	}
	if res.Entry.DeviationConstitutionHash == nil || *res.Entry.DeviationConstitutionHash != hash {
		t.Errorf("DeviationConstitutionHash = %v, want %s", res.Entry.DeviationConstitutionHash, hash)
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, "constitutionHash changed") {
			t.Errorf("unexpected mismatch warning when hashes agree: %v", res.Warnings)
		}
	}
}

func TestApproveDesignDeviationHashMismatchWarns(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(root, "constitution", "constitution.md"), "# Constitution\n\nNo rules yet.\n")
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON("sha256:deadbeef"))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: fakeConstitutionBin(t, 0, "deviation validate: ok\n", ""),
	}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "constitutionHash changed") {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want a constitutionHash-mismatch warning", res.Warnings)
	}
	if res.Entry.DeviationConstitutionHash == nil || *res.Entry.DeviationConstitutionHash != "sha256:deadbeef" {
		t.Error("DeviationConstitutionHash should still be recorded even on mismatch (both kept, spec §7.5)")
	}
}

func TestApproveDesignDeviationInvalidRefuses(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(""))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: fakeConstitutionBin(t, 1, "", "deviation validate: at /deviations/0/adrId: missing property 'adrId'\n"),
	}
	if _, err := Approve(req); !errors.Is(err, ErrDeviationInvalid) {
		t.Fatalf("Approve() error = %v, want ErrDeviationInvalid", err)
	}
	if _, statErr := os.Stat(StatePath(changeDir)); statErr == nil {
		t.Error("approval-state.json was written despite an invalid deviation.json")
	}
}

func TestApproveDesignDeviationCouldNotRun(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(""))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: fakeConstitutionBin(t, 2, "", "deviation validate: no constitution.yml in .../empty; run from a constitution project root\n"),
	}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

func TestApproveRejectSkipsDeviationGate(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	// No deviation.json, and no constitution binary — a --reject must not need either.

	req := Request{Root: root, Change: "042-user-auth", Stage: StageDesign, Reject: true, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if res.Entry.Status != StatusRejected {
		t.Errorf("Status = %q, want rejected", res.Entry.Status)
	}
}

// --- append / latest-per-stage ---

func TestAppendEntryIsAppendOnly(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	if _, err := Approve(req); err != nil {
		t.Fatalf("first Approve() error = %v", err)
	}
	rejectReq := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Reject: true, Consent: offGate()}
	if _, err := Approve(rejectReq); err != nil {
		t.Fatalf("second Approve() error = %v", err)
	}

	sf, err := ReadState(changeDir)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if len(sf.Gates) != 2 {
		t.Fatalf("len(Gates) = %d, want 2 (append-only)", len(sf.Gates))
	}
	if sf.Gates[0].Status != StatusApproved || sf.Gates[1].Status != StatusRejected {
		t.Errorf("Gates = %+v, want [approved, rejected] in order", sf.Gates)
	}

	latest := LatestPerStage(sf.Gates)
	if latest[StageRefine].Status != StatusRejected {
		t.Errorf("LatestPerStage[refine].Status = %q, want rejected (the later entry wins)", latest[StageRefine].Status)
	}
}

func TestLatestPerStageTieBreaksOnArrayOrder(t *testing.T) {
	same := time.Now().UTC().Format(time.RFC3339)
	gates := []Entry{
		{Stage: StageRefine, Status: StatusApproved, ApprovedAt: same},
		{Stage: StageRefine, Status: StatusRejected, ApprovedAt: same},
	}
	latest := LatestPerStage(gates)
	if latest[StageRefine].Status != StatusRejected {
		t.Errorf("tie-break winner = %q, want the later array entry (rejected)", latest[StageRefine].Status)
	}
}

// --- drift ---

func TestHashDriftDetectsEditAndDeletion(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	res, err := Approve(req)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if drift := HashDrift(changeDir, res.Entry.Artifacts); len(drift) != 0 {
		t.Fatalf("HashDrift right after approval = %v, want none", drift)
	}

	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal+"\nextra line\n")
	if err := os.Remove(filepath.Join(changeDir, "specs", "auth", "spec.md")); err != nil {
		t.Fatal(err)
	}

	drift := HashDrift(changeDir, res.Entry.Artifacts)
	if len(drift) != 2 {
		t.Fatalf("HashDrift after edit+delete = %v, want 2 entries", drift)
	}
}

// --- ArtifactGlobs sanity (schema-sourced, not hand-duplicated) ---

func TestArtifactGlobsResolvesFromSchema(t *testing.T) {
	def, err := schema.Load()
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}
	tests := []struct {
		stage Stage
		want  []string
	}{
		{StageRefine, []string{"proposal.md", "specs/**/spec.md"}},
		{StageDesign, []string{"design.md"}},
		{StagePlan, []string{"tasks.md"}},
		{StageRepro, []string{"proposal.md", "specs/**/spec.md"}},
		{StageFix, []string{"tasks.md"}},
	}
	for _, tt := range tests {
		got, err := ArtifactGlobs(def, tt.stage)
		if err != nil {
			t.Errorf("ArtifactGlobs(%s): %v", tt.stage, err)
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("ArtifactGlobs(%s) = %v, want %v", tt.stage, got, tt.want)
		}
	}
}

func TestArtifactGlobsUnrecognizedStage(t *testing.T) {
	def, err := schema.Load()
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}
	if _, err := ArtifactGlobs(def, Stage("bogus")); err == nil {
		t.Error("ArtifactGlobs(bogus) error = nil, want an unrecognized-stage error")
	}
}

// --- validateForStage direct coverage (StagePlan + the unrecognized-stage
// default; both are unreachable through Approve() itself since Approve
// refuses an unrecognized Stage before ever calling validateForStage). ---

func TestValidateForStagePlan(t *testing.T) {
	changeDir := t.TempDir()
	writeFile(t, filepath.Join(changeDir, "tasks.md"), validTasks)
	findings, err := validateForStage(changeDir, StagePlan)
	if err != nil {
		t.Fatalf("validateForStage(plan): %v", err)
	}
	if hasError(findings) {
		t.Errorf("validateForStage(plan) findings = %v, want none error-severity", findings)
	}
}

func TestValidateForStageUnrecognized(t *testing.T) {
	if _, err := validateForStage(t.TempDir(), Stage("bogus")); err == nil {
		t.Error("validateForStage(bogus) error = nil, want an unrecognized-stage error")
	}
}

// --- readDeviationConstitutionHash direct coverage ---

func TestReadDeviationConstitutionHashInvalidJSON(t *testing.T) {
	if _, ok := readDeviationConstitutionHash([]byte("not json")); ok {
		t.Error("readDeviationConstitutionHash(invalid JSON) ok = true, want false")
	}
}

func TestReadDeviationConstitutionHashEmptyField(t *testing.T) {
	if _, ok := readDeviationConstitutionHash([]byte(`{"constitutionHash":""}`)); ok {
		t.Error("readDeviationConstitutionHash(empty field) ok = true, want false")
	}
}

// --- hashFiles direct coverage ---

func TestHashFilesMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := hashFiles(dir, []string{"missing.md"}); err == nil {
		t.Error("hashFiles(missing file) error = nil, want a read error")
	}
}

// --- resolveArtifactFiles direct coverage ---

func TestResolveArtifactFilesUnsupportedGlobShape(t *testing.T) {
	dir := t.TempDir()
	// A second "/" inside the segment after "/**/" is not one of the two
	// supported shapes (glob.go: a literal path, or exactly one "**"
	// segment with no further "/" in the trailing name).
	if _, err := resolveArtifactFiles(dir, "specs/**/nested/spec.md"); err == nil {
		t.Error("resolveArtifactFiles(unsupported glob shape) error = nil, want an error")
	}
}

// chmodBlocked removes all permissions from path, restoring them via
// t.Cleanup so t.TempDir() can still remove the tree afterward. Callers
// still need their own Skip when running as an unprivileged process would
// normally get a permission error but this platform doesn't enforce it
// (e.g. running as root).
func chmodBlocked(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func TestResolveArtifactFilesLiteralPathStatErrorNotNotExist(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	writeFile(t, filepath.Join(sub, "file.md"), "x")
	chmodBlocked(t, sub)
	if _, err := os.Stat(filepath.Join(sub, "file.md")); err == nil || os.IsNotExist(err) {
		t.Skip("this platform's filesystem doesn't enforce directory permissions for an unprivileged process")
	}

	if _, err := resolveArtifactFiles(dir, "sub/file.md"); err == nil {
		t.Error("resolveArtifactFiles(unreadable parent dir) error = nil, want a permission error")
	}
}

func TestResolveArtifactFilesGlobRootStatErrorNotNotExist(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	chmodBlocked(t, blocked)
	if _, err := os.Stat(filepath.Join(blocked, "nested")); err == nil || os.IsNotExist(err) {
		t.Skip("this platform's filesystem doesn't enforce directory permissions for an unprivileged process")
	}

	if _, err := resolveArtifactFiles(dir, "blocked/nested/**/spec.md"); err == nil {
		t.Error("resolveArtifactFiles(unreadable glob root parent) error = nil, want a permission error")
	}
}

func TestResolveArtifactFilesWalkErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "prefix")
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(blocked, "spec.md"), "x")
	chmodBlocked(t, blocked)
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("this platform's filesystem doesn't enforce directory permissions for an unprivileged process")
	}

	if _, err := resolveArtifactFiles(dir, "prefix/**/spec.md"); err == nil {
		t.Error("resolveArtifactFiles(unreadable nested dir during walk) error = nil, want the propagated walk error")
	}
}

// --- ReadState direct coverage: error paths beyond the "file absent" case ---

func TestReadStateFileIsDirectoryErrors(t *testing.T) {
	changeDir := t.TempDir()
	if err := os.MkdirAll(StatePath(changeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadState(changeDir); err == nil {
		t.Error("ReadState(approval-state.json is a directory) error = nil, want a read error")
	}
}

func TestReadStateMalformedJSONErrors(t *testing.T) {
	changeDir := t.TempDir()
	writeFile(t, StatePath(changeDir), "{not valid json")
	if _, err := ReadState(changeDir); err == nil {
		t.Error("ReadState(malformed JSON) error = nil, want a parse error")
	}
}

func TestReadStateUnsupportedSchemaVersionErrors(t *testing.T) {
	changeDir := t.TempDir()
	writeFile(t, StatePath(changeDir), `{"schemaVersion":2,"change":"x","gates":[]}`)
	if _, err := ReadState(changeDir); err == nil {
		t.Error("ReadState(unsupported schemaVersion) error = nil, want a refusal")
	}
}

// --- appendEntry direct coverage: propagates a ReadState error rather than
// silently overwriting a corrupt approval-state.json. ---

func TestAppendEntryPropagatesReadStateError(t *testing.T) {
	changeDir := t.TempDir()
	writeFile(t, StatePath(changeDir), "{not valid json")
	if err := appendEntry(changeDir, Entry{Stage: StageRefine, Status: StatusApproved}); err == nil {
		t.Error("appendEntry() error = nil, want the underlying ReadState parse error")
	}
}

// --- ConsentGate.Confirm interactive-prompt direct coverage ---

func TestConsentGateConfirmInteractiveYes(t *testing.T) {
	var out strings.Builder
	g := ConsentGate{Policy: "strict", IsTTY: true, In: strings.NewReader("yes\n"), Out: &out}
	if err := g.Confirm("approve change X"); err != nil {
		t.Errorf("Confirm() error = %v, want nil after an interactive \"yes\"", err)
	}
	if !strings.Contains(out.String(), "approve change X") {
		t.Errorf("prompt output = %q, want it to mention the verb", out.String())
	}
}

func TestConsentGateConfirmInteractiveDeclined(t *testing.T) {
	var out strings.Builder
	g := ConsentGate{Policy: "strict", IsTTY: true, In: strings.NewReader("no\n"), Out: &out}
	if err := g.Confirm("approve change X"); !errors.Is(err, ErrConsentRequired) {
		t.Errorf("Confirm() error = %v, want ErrConsentRequired after an interactive decline", err)
	}
}

// --- constitution.Hash real-error path (not just "absent") ---

func TestApproveConstitutionHashReadErrorSurfacesAsErrCouldNotRun(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(changeDir, "specs", "auth", "spec.md"), validDelta)
	// constitution/constitution.md as a directory makes os.ReadFile fail
	// with something other than IsNotExist, so constitution.Hash returns a
	// genuine error rather than its (hash="", ok=false, err=nil) "absent" case.
	if err := os.MkdirAll(filepath.Join(root, "constitution", "constitution.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := Request{Root: root, Change: "042-user-auth", Stage: StageRefine, Consent: offGate()}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}

// --- runDeviationGate: constitution binary that fails to exec at all
// (a genuine derr, distinct from the DeviationCouldNotRun exit-code case
// exercised by TestApproveDesignDeviationCouldNotRun). ---

func TestApproveDesignConstitutionBinNotExecutable(t *testing.T) {
	root, changeDir := newProject(t, "042-user-auth")
	writeFile(t, filepath.Join(changeDir, "design.md"), validDesign)
	writeFile(t, filepath.Join(changeDir, "deviation.json"), validDeviationJSON(""))

	req := Request{
		Root: root, Change: "042-user-auth", Stage: StageDesign, Consent: offGate(),
		ConstitutionBin: filepath.Join(t.TempDir(), "no-such-binary"),
	}
	if _, err := Approve(req); !errors.Is(err, ErrCouldNotRun) {
		t.Fatalf("Approve() error = %v, want ErrCouldNotRun", err)
	}
}
