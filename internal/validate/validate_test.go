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

// validYAMLDelta is the change-007 authoritative delta form: structured YAML
// (spec.yaml), read by the refine stage in place of any markdown file.
const validYAMLDelta = `capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system SHALL allow a registered user to authenticate with a username and password.
      scenarios:
        - name: Successful login
          given:
            - a registered user
          when:
            - they submit correct credentials
          then:
            - the system grants a session
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
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.yaml"), validYAMLDelta)
	// A stray markdown file must NOT be read as authoritative: change 007
	// reads the YAML delta, not any markdown file (spec-format scenario "a
	// spec delta is authored and read as YAML"). Garbage markdown here must
	// be ignored.
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.md"), "not a real delta at all\n")

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

// The plan-stage happy path now delegates to milestoned-plan-dag over
// plan.yaml — see plan_gate_test.go (TestValidatePlanValidPasses).

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

// --- specs/**/spec.yaml delegation (change 007) ---

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
	// Missing SHALL/MUST keyword — an internal/spec.ParseDeltaYAML error
	// (KindMissingRFC2119), not something this package re-implements.
	writeFile(t, filepath.Join(dir, "specs", "auth", "spec.yaml"), `capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system lets a user log in.
      scenarios:
        - name: ok
          then:
            - c
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

// TestSpecsDeltaMalformedYAMLNamedByPath asserts the change-007 refine-read
// contract: a spec.yaml that is not valid YAML fails with an error naming the
// offending file path.
func TestSpecsDeltaMalformedYAMLNamedByPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	yamlPath := filepath.Join(dir, "specs", "auth", "spec.yaml")
	writeFile(t, yamlPath, "capability: auth\ndeltas: [unterminated\n")

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	var bad *Finding
	for i := range findings {
		if findings[i].Kind == "malformed_yaml" {
			bad = &findings[i]
		}
	}
	if bad == nil {
		t.Fatalf("findings = %+v, want a malformed_yaml finding", findings)
	}
	if bad.File != yamlPath {
		t.Errorf("malformed_yaml finding File = %q, want the offending path %q", bad.File, yamlPath)
	}
	if !strings.Contains(bad.Message, yamlPath) {
		t.Errorf("malformed_yaml message = %q, want it to name the offending path %q", bad.Message, yamlPath)
	}
	if bad.Severity != SeverityError {
		t.Errorf("malformed_yaml severity = %q, want error", bad.Severity)
	}
}

// TestSpecsDeltaSchemaViolationNamedByPath asserts a spec.yaml that decodes but
// violates the published spec-delta schema fails naming the offending path.
func TestSpecsDeltaSchemaViolationNamedByPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "proposal.md"), validProposal)
	yamlPath := filepath.Join(dir, "specs", "auth", "spec.yaml")
	// op is not one of the four enum values — a schema violation.
	writeFile(t, yamlPath, `capability: auth
deltas:
  - op: BOGUS
    requirement:
      name: X
`)

	findings, err := Change(dir, StageRefine)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	var bad *Finding
	for i := range findings {
		if findings[i].Kind == "delta_schema_error" {
			bad = &findings[i]
		}
	}
	if bad == nil {
		t.Fatalf("findings = %+v, want a delta_schema_error finding", findings)
	}
	if !strings.Contains(bad.Message, yamlPath) {
		t.Errorf("delta_schema_error message = %q, want it to name the offending path %q", bad.Message, yamlPath)
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

// --- plan stage (plan.yaml, via milestoned-plan-dag) ---
//
// The plan-stage gate delegates to milestoned-plan-dag over plan.yaml
// (change 007, Milestone 5). A change with no plan.yaml is a
// missing_artifact; the valid / invalid delegation paths are covered in
// plan_gate_test.go.

func TestPlanMissingArtifact(t *testing.T) {
	dir := t.TempDir()
	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "missing_artifact") {
		t.Errorf("findings = %v, want missing_artifact (no plan.yaml)", kinds)
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
