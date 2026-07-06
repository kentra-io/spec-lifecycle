package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func findingKinds(findings []Finding) []string {
	var kinds []string
	for _, f := range findings {
		kinds = append(kinds, f.Kind)
	}
	return kinds
}

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

func TestChangeRefineHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Change(refine) on a well-formed change = %+v, want no findings", findings)
	}
}

func TestChangeDesignHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "design.md"), validDesign)

	findings, err := Change(dir, StageDesign)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Change(design) on a well-formed design.md = %+v, want no findings", findings)
	}
}

func TestChangePlanHappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), validTasks)

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("Change(plan) on a well-formed tasks.md = %+v, want no findings", findings)
	}
}

func TestChangeUnrecognizedStage(t *testing.T) {
	dir := t.TempDir()
	if _, err := Change(dir, Stage("bogus")); err == nil {
		t.Error("Change with an unrecognized stage: want error, got nil")
	}
}

// --- proposal.md ---

func TestProposalMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_artifact") {
		t.Errorf("findings = %v, want missing_artifact", kinds)
	}
}

func TestProposalMissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), "# No frontmatter here\n")
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_frontmatter") {
		t.Errorf("findings = %v, want missing_frontmatter", kinds)
	}
}

func TestProposalMalformedFrontmatterYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\nissue: [unterminated\n---\nbody\n")
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "malformed_frontmatter") {
		t.Errorf("findings = %v, want malformed_frontmatter", kinds)
	}
}

func TestProposalMissingIssueField(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\nissue: \"\"\n---\n\n# Title\n")
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_issue_ref") {
		t.Errorf("findings = %v, want missing_issue_ref", kinds)
	}
}

func TestProposalIssueFieldAbsentEntirely(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ndesignSkip: false\n---\n\n# Title\n")
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), validDelta)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_issue_ref") {
		t.Errorf("findings = %v, want missing_issue_ref", kinds)
	}
}

// --- specs/**/spec.md delegation ---

func TestSpecsDeltaMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_artifact") {
		t.Errorf("findings = %v, want missing_artifact", kinds)
	}
}

func TestSpecsDeltaMalformedGrammarDelegatesToInternalSpec(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	// Missing SHALL/MUST keyword — an internal/spec.ParseDelta error
	// (KindMissingRFC2119), not something this package re-implements.
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), `## ADDED Requirements

### Requirement: Password login
The system lets a user log in.

#### Scenario: ok
- **GIVEN** a
- **WHEN** b
- **THEN** c
`)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_rfc2119_keyword") {
		t.Errorf("findings = %v, want missing_rfc2119_keyword (the internal/spec.Error Kind passed through verbatim)", kinds)
	}
	for _, f := range findings {
		if f.Kind == "missing_rfc2119_keyword" && f.Severity != SeverityError {
			t.Errorf("missing_rfc2119_keyword finding severity = %q, want error", f.Severity)
		}
	}
}

func TestSpecsDeltaUnrecognizedSectionWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), `## ADDED Requirements

### Requirement: Password login
The system SHALL allow a user to log in.

#### Scenario: ok
- **GIVEN** a
- **WHEN** b
- **THEN** c

## Notes

### Requirement: Orphaned under Notes
This one is silently dropped by the fold.
`)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	var warn *Finding
	for i := range findings {
		if findings[i].Kind == "requirement_under_unrecognized_section" {
			warn = &findings[i]
		}
	}
	if warn == nil {
		t.Fatalf("findings = %+v, want a requirement_under_unrecognized_section warning", findings)
	}
	if warn.Severity != SeverityWarning {
		t.Errorf("requirement_under_unrecognized_section severity = %q, want warning", warn.Severity)
	}
}

// --- design.md ---

func TestDesignMissingFile(t *testing.T) {
	dir := t.TempDir()
	findings, err := Change(dir, StageDesign)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_artifact") {
		t.Errorf("findings = %v, want missing_artifact", kinds)
	}
}

func TestDesignMissingNFRDischarge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "design.md"), "# Title — Design\n\n## Context\nBackground.\n")

	findings, err := Change(dir, StageDesign)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_nfr_discharge") {
		t.Errorf("findings = %v, want missing_nfr_discharge", kinds)
	}
}

func TestDesignNFRDischargeHyphenatedHeadingAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "design.md"), "# Title — Design\n\n## NFR-Discharge\n(none declared)\n")

	findings, err := Change(dir, StageDesign)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none (hyphenated heading should be accepted)", findings)
	}
}

// --- tasks.md ---

func TestTasksMissingFile(t *testing.T) {
	dir := t.TempDir()
	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_artifact") {
		t.Errorf("findings = %v, want missing_artifact", kinds)
	}
}

func TestTasksNoMilestoneHeadings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "- [ ] just a checkbox, no milestone heading\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "no_milestone_headings") {
		t.Errorf("findings = %v, want no_milestone_headings", kinds)
	}
}

func TestTasksMissingLabel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Steps** — do it.
`)

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	var found bool
	for _, f := range findings {
		if f.Kind == "missing_milestone_label" {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %+v, want a missing_milestone_label finding for the omitted Validation contract label", findings)
	}
}

func TestTasksEmptyValidationContract(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Validation contract** —
**Steps** — do it.
`)

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "empty_validation_contract") {
		t.Errorf("findings = %v, want empty_validation_contract", kinds)
	}
}

func TestTasksMultipleMilestonesOneBad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Good
**Goal** — g.
**Deliverables** — d.
**Validation contract** — checkable:
  - x passes
**Steps** — s:
  1. step

## Milestone 2: Bad
**Goal** — g.
**Deliverables** — d.
**Steps** — s.
`)

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("want at least one finding for Milestone 2's missing Validation contract label")
	}
	for _, f := range findings {
		if !strings.Contains(f.Message, "Milestone 2") {
			t.Errorf("finding %+v: want it to name Milestone 2 (Milestone 1 is well-formed and shouldn't be flagged)", f)
		}
	}
}

// --- ArtifactsForStage ---

func TestArtifactsForStage(t *testing.T) {
	cases := map[Stage]string{
		StageRefine: proposalFile,
		StageDesign: designFile,
		StagePlan:   tasksFile,
	}
	for stage, want := range cases {
		got := ArtifactsForStage(stage)
		if len(got) == 0 {
			t.Errorf("ArtifactsForStage(%s) = empty, want to include %q", stage, want)
			continue
		}
		if got[0] != want {
			t.Errorf("ArtifactsForStage(%s)[0] = %q, want %q", stage, got[0], want)
		}
	}
	if got := ArtifactsForStage(Stage("bogus")); got != nil {
		t.Errorf("ArtifactsForStage(bogus) = %v, want nil", got)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
