package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
	"github.com/kentra-io/spec-lifecycle/internal/spec"
)

// --- fixture builder ------------------------------------------------------
//
// fixtureBuilder drives a sequence of single-capability, single-record
// "archive steps" against a scratch project root, using internal/spec's own
// Fold and internal/archive's own AppendRecords/ManifestSHA/HashBytes at
// every step — so every fixture this file builds is byte-for-byte what a
// real `lifecycle archive` run would have produced, and guard's tests are
// exercising guard's OWN logic, never a hand-typed-and-possibly-wrong
// ledger/manifest shape.

type fixtureBuilder struct {
	t       *testing.T
	root    string
	current map[string]*spec.RequirementSet // per capability, nil until first ADD
	seq     int
}

// newFixtureBuilder creates only root/openspec/ itself — deliberately NOT
// openspec/changes/archive/ or openspec/specs/, so a builder with zero
// step calls is a genuinely fresh project (matching a real
// never-yet-archived openspec/ tree, where those directories don't exist
// until the first `lifecycle archive` creates them): step lazily
// MkdirAll's whatever it needs, exactly like internal/archive.Archive
// does.
func newFixtureBuilder(t *testing.T) *fixtureBuilder {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "openspec"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return &fixtureBuilder{t: t, root: root, current: map[string]*spec.RequirementSet{}}
}

func addDeltaText(name string) string {
	return fmt.Sprintf(
		"## ADDED Requirements\n### Requirement: %s\nThe system SHALL do %s work.\n\n#### Scenario: %s happens\n- **GIVEN** a precondition\n- **WHEN** the action happens\n- **THEN** the system SHALL respond\n",
		name, name, name,
	)
}

func modifyDeltaText(name, suffix string) string {
	return fmt.Sprintf(
		"## MODIFIED Requirements\n### Requirement: %s\nThe system SHALL do %s work%s.\n\n#### Scenario: %s happens\n- **GIVEN** a precondition\n- **WHEN** the action happens\n- **THEN** the system SHALL respond\n",
		name, name, suffix, name,
	)
}

func removeDeltaText(name string) string {
	return fmt.Sprintf("## REMOVED Requirements\n### Requirement: %s\n", name)
}

func renameDeltaText(from, to string) string {
	return fmt.Sprintf("## RENAMED Requirements\n- FROM: `### Requirement: %s`\n- TO: `### Requirement: %s`\n", from, to)
}

// step performs one archive-equivalent operation for capability:
// writes the archived change folder (with deltaText verbatim), folds it
// (via internal/spec, exactly like internal/archive does) to produce the
// new live spec.md, appends one ledger record (via
// archive.AppendRecords, so seq/JSON-shape match production), and writes
// the new live spec.md. Returns the change name used.
func (b *fixtureBuilder) step(capability, deltaText string) string {
	b.t.Helper()
	b.seq++
	change := fmt.Sprintf("%03d-change", b.seq)

	changeDir := filepath.Join(b.root, "openspec", "changes", "archive", change, "specs", capability)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		b.t.Fatalf("MkdirAll: %v", err)
	}
	deltaPath := filepath.Join(changeDir, "spec.md")
	if err := os.WriteFile(deltaPath, []byte(deltaText), 0o644); err != nil {
		b.t.Fatalf("WriteFile: %v", err)
	}

	d, err := spec.ParseDelta([]byte(deltaText))
	if err != nil {
		b.t.Fatalf("ParseDelta: %v", err)
	}

	before := b.current[capability]
	preImage := archive.EmptyImageSHA
	if before != nil {
		preImage = archive.HashBytes(before.Render())
	}

	folded, err := spec.Fold(capability, change, before, d)
	if err != nil {
		b.t.Fatalf("Fold: %v", err)
	}
	rendered := folded.Render()
	postImage := archive.HashBytes(rendered)
	b.current[capability] = folded

	specDir := filepath.Join(b.root, "openspec", "specs", capability)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		b.t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), rendered, 0o644); err != nil {
		b.t.Fatalf("WriteFile: %v", err)
	}

	archiveDir := filepath.Join(b.root, "openspec", "changes", "archive", change)
	manifest, err := archive.ManifestSHA(archiveDir)
	if err != nil {
		b.t.Fatalf("ManifestSHA: %v", err)
	}

	if _, err := archive.AppendRecords(b.root, []archive.Record{{
		Change:             change,
		Capability:         capability,
		PreImageSha:        preImage,
		PostImageSha:       postImage,
		DeltaOps:           []archive.DeltaOp{},
		ArchiveManifestSha: manifest,
	}}); err != nil {
		b.t.Fatalf("AppendRecords: %v", err)
	}
	return change
}

// seedBrownfield writes capability's live spec.md directly, bypassing step
// entirely — simulating a capability that existed on disk BEFORE
// lifecycle ever archived anything for it (replay.go/chain.go's
// "brownfield capability" case; mirrors cmd/lifecycle's own
// archive_conformance_case02/archive_conflict fixtures, which pre-seed
// openspec/specs/auth/spec.md the same way). Subsequent step calls for
// capability will compute correct preImageSha against this seeded content, exactly
// as internal/archive's own Archive does against a pre-existing file.
func (b *fixtureBuilder) seedBrownfield(capability, content string) {
	b.t.Helper()
	rs, err := spec.ParseRequirementSet([]byte(content))
	if err != nil {
		b.t.Fatalf("ParseRequirementSet: %v", err)
	}
	specDir := filepath.Join(b.root, "openspec", "specs", capability)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		b.t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644); err != nil {
		b.t.Fatalf("WriteFile: %v", err)
	}
	b.current[capability] = rs
}

// --- tests -----------------------------------------------------------------

func TestRun_CleanEmptyProject(t *testing.T) {
	b := newFixtureBuilder(t)
	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Summary.Clean || len(res.Findings) != 0 {
		t.Fatalf("want clean, got %+v", res)
	}
}

func TestRun_CleanSingleCapabilityHistory(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	b.step("auth", modifyDeltaText("Password login", " (v2)"))
	b.step("auth", addDeltaText("Session expiry"))

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Summary.Clean {
		t.Fatalf("want clean, got findings: %+v", res.Findings)
	}
	if res.Summary.ChangesChecked != 3 || res.Summary.RecordsChecked != 3 {
		t.Fatalf("unexpected summary: %+v", res.Summary)
	}
}

func TestRun_CleanMultiChangeMultiCapability(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	b.step("billing", addDeltaText("Invoice generation"))
	b.step("auth", addDeltaText("Session expiry"))
	b.step("billing", modifyDeltaText("Invoice generation", " (v2)"))
	b.step("auth", removeDeltaText("Session expiry"))
	b.step("billing", renameDeltaText("Invoice generation", "Invoice creation"))

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Summary.Clean {
		t.Fatalf("want clean, got findings: %+v", res.Findings)
	}
}

func TestRun_CleanBrownfieldCapability(t *testing.T) {
	b := newFixtureBuilder(t)
	b.seedBrownfield("auth", "# auth Specification\n\n"+
		"## Purpose\nAuthentication.\n\n"+
		"## Requirements\n"+
		"### Requirement: Password login\n"+
		"The system SHALL allow login.\n\n"+
		"#### Scenario: Successful login\n"+
		"- **GIVEN** a user\n- **WHEN** they log in\n- **THEN** the system SHALL grant a session\n")

	b.step("auth", modifyDeltaText("Password login", " (v2)"))
	b.step("auth", addDeltaText("Session expiry"))

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Summary.Clean {
		t.Fatalf("want a brownfield capability's ledger-tracked history to guard clean, got findings: %+v", res.Findings)
	}
}

func TestRun_LedgerMissingWithArchivesPresent(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	if err := os.Remove(archive.LedgerPath(b.root)); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary.Clean {
		t.Fatalf("want findings, got clean")
	}
	if len(res.Findings) != 1 || res.Findings[0].Kind != KindLedgerMissing {
		t.Fatalf("want exactly one ledger_missing finding, got %+v", res.Findings)
	}
}

func TestRun_LedgerMalformedIsAnError(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	if err := os.WriteFile(archive.LedgerPath(b.root), []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Run(Options{Root: b.root})
	if err == nil {
		t.Fatalf("want an error for a malformed ledger, got nil")
	}
}

func TestRun_ArchiveMutated_EditedArchivedDelta(t *testing.T) {
	b := newFixtureBuilder(t)
	change := b.step("auth", addDeltaText("Password login"))

	deltaPath := filepath.Join(b.root, "openspec", "changes", "archive", change, "specs", "auth", "spec.md")
	data, err := os.ReadFile(deltaPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	mutated := strings.Replace(string(data), "Password login", "Password login HACKED", 1)
	if err := os.WriteFile(deltaPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Summary.Clean {
		t.Fatalf("want findings, got clean")
	}
	var got []Finding
	for _, f := range res.Findings {
		if f.Kind == KindArchiveMutated {
			got = append(got, f)
		}
	}
	if len(got) == 0 {
		t.Fatalf("want at least one archive_mutated finding, got %+v", res.Findings)
	}
}

func TestRun_ArchiveMutated_OrphanArchiveNoLedgerRecord(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	orphanDir := filepath.Join(b.root, "openspec", "changes", "archive", "999-orphan", "specs", "auth")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "spec.md"), []byte(addDeltaText("Orphan requirement")), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range res.Findings {
		if f.Kind == KindArchiveMutated && f.Change == "999-orphan" {
			found = true
		}
	}
	if !found {
		t.Fatalf("want an archive_mutated finding for the orphan change, got %+v", res.Findings)
	}
}

func TestRun_ArchiveMutated_LedgerReferencesMissingFolder(t *testing.T) {
	b := newFixtureBuilder(t)
	change := b.step("auth", addDeltaText("Password login"))

	if err := os.RemoveAll(filepath.Join(b.root, "openspec", "changes", "archive", change)); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, f := range res.Findings {
		if f.Kind == KindArchiveMutated && f.Change == change {
			found = true
		}
	}
	if !found {
		t.Fatalf("want an archive_mutated finding for the missing folder, got %+v", res.Findings)
	}
}

func TestRun_ChainBreak_TamperedPreImageLink(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	b.step("auth", addDeltaText("Session expiry"))

	rewriteLedgerField(t, b.root, 2, "preImageSha", "sha256:0000000000000000000000000000000000000000000000000000000000000")

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []Finding
	for _, f := range res.Findings {
		if f.Kind == KindChainBreak {
			got = append(got, f)
		}
	}
	if len(got) != 1 || got[0].Seq != 2 {
		t.Fatalf("want exactly one chain_break finding at seq 2, got %+v", res.Findings)
	}
}

func TestRun_ProjectionDrift_LiveSpecEdited(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	specPath := filepath.Join(b.root, "openspec", "specs", "auth", "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	mutated := string(data) + "\n### Requirement: Hand-added\nNot part of any ledger record.\n"
	if err := os.WriteFile(specPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var driftChecks []string
	for _, f := range res.Findings {
		if f.Kind == KindProjectionDrift {
			driftChecks = append(driftChecks, f.Check)
		}
	}
	if len(driftChecks) != 2 {
		t.Fatalf("want projection_drift caught by both digest_chain and replay, got %+v", res.Findings)
	}
	hasDigest, hasReplay := false, false
	for _, c := range driftChecks {
		hasDigest = hasDigest || c == CheckDigestChain
		hasReplay = hasReplay || c == CheckReplay
	}
	if !hasDigest || !hasReplay {
		t.Fatalf("want one projection_drift from each of digest_chain and replay, got checks %v", driftChecks)
	}
}

func TestRun_ProjectionDrift_ReplayedCapabilityMissingLiveFile(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	if err := os.Remove(filepath.Join(b.root, "openspec", "specs", "auth", "spec.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range res.Findings {
		if f.Kind != KindProjectionDrift {
			t.Fatalf("want only projection_drift findings, got %+v", res.Findings)
		}
	}
	if len(res.Findings) != 2 {
		t.Fatalf("want digest_chain + replay findings, got %+v", res.Findings)
	}
}

func TestRun_ProjectionDrift_LiveCapabilityWithNoLedgerHistory(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	rogueDir := filepath.Join(b.root, "openspec", "specs", "rogue")
	if err := os.MkdirAll(rogueDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rogueDir, "spec.md"), []byte("# rogue Specification\n\n## Purpose\nHand-written, never archived.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []Finding
	for _, f := range res.Findings {
		if f.Capability == "rogue" {
			got = append(got, f)
		}
	}
	if len(got) != 1 || got[0].Kind != KindProjectionDrift || got[0].Check != CheckReplay {
		t.Fatalf("want exactly one replay/projection_drift finding for capability rogue, got %+v", res.Findings)
	}
}

func TestRun_Idempotent(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))
	b.step("billing", addDeltaText("Invoice generation"))

	specPath := filepath.Join(b.root, "openspec", "specs", "auth", "spec.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(specPath, append(data, []byte("\ntrailing drift\n")...), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	first, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run (1st): %v", err)
	}
	second, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run (2nd): %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("guard is not idempotent:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if first.Summary.Clean {
		t.Fatalf("expected the drifted fixture to be non-clean")
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	b := newFixtureBuilder(t)
	b.step("auth", addDeltaText("Password login"))

	res, err := Run(Options{Root: b.root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out Result
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !strings.Contains(string(data), `"findings":[]`) {
		t.Fatalf(`want a literal "findings":[] (never null) in a clean result, got %s`, data)
	}
}

// rewriteLedgerField does a targeted textual rewrite of one JSON field on
// the ledger record with the given seq, WITHOUT going through
// archive.AppendRecords (which never mutates existing lines) — simulating
// exactly the kind of out-of-band ledger tamper check 2 exists to catch.
func rewriteLedgerField(t *testing.T, root string, seq int, field, newValue string) {
	t.Helper()
	path := archive.LedgerPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("Unmarshal ledger line %d: %v", i, err)
		}
		if int(rec["seq"].(float64)) != seq {
			continue
		}
		rec[field] = newValue
		out, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		lines[i] = string(out)
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		return
	}
	t.Fatalf("no ledger record with seq %d found", seq)
}
