package validate

import (
	"path/filepath"
	"testing"
)

// --- ```contract block: validatePlan structural checks ---

func TestTasksNoContractBlockStillValid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Password login
**Goal** — implement login.
**Deliverables** — login handler.
**Validation contract** — checkable:
  - go test ./... passes
**Steps** — s:
  1. step one
`)
	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none (a contract-less milestone must still validate — backward compatible)", findings)
	}
}

func TestTasksWellFormedContractBlockValid(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — implement login.\n"+
		"**Deliverables** — login handler.\n"+
		"**Validation contract** — checkable:\n"+
		"  - go test ./... passes\n"+
		"\n"+
		"  ```contract\n"+
		"  check: go test ./internal/auth/...\n"+
		"  criteria: All auth tests pass; password-login scenario is discharged.\n"+
		"  paths:\n"+
		"    - internal/auth/**\n"+
		"    - cmd/auth/**\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none (well-formed contract block)", findings)
	}

	ms, ok, err := ParseMilestones(dir)
	if err != nil || !ok {
		t.Fatalf("ParseMilestones: ok=%v err=%v", ok, err)
	}
	if len(ms) != 1 {
		t.Fatalf("len(milestones) = %d, want 1", len(ms))
	}
	c := ms[0].Contract
	if c == nil {
		t.Fatal("Contract = nil, want a parsed contract")
	}
	if c.Check != "go test ./internal/auth/..." {
		t.Errorf("Check = %q", c.Check)
	}
	if c.Criteria == "" {
		t.Error("Criteria is empty")
	}
	if len(c.Paths) != 2 || c.Paths[0] != "internal/auth/**" || c.Paths[1] != "cmd/auth/**" {
		t.Errorf("Paths = %v", c.Paths)
	}
}

func TestTasksMalformedContractBlockYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — implement login.\n"+
		"**Deliverables** — login handler.\n"+
		"**Validation contract** — checkable:\n"+
		"  - go test ./... passes\n"+
		"\n"+
		"  ```contract\n"+
		"  check: [this is not: valid: yaml\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "malformed_contract") {
		t.Errorf("findings = %v, want malformed_contract", kinds)
	}
}

func TestTasksMalformedContractMissingFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — implement login.\n"+
		"**Deliverables** — login handler.\n"+
		"**Validation contract** — checkable:\n"+
		"  - go test ./... passes\n"+
		"\n"+
		"  ```contract\n"+
		"  check: \"\"\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "malformed_contract") {
		t.Errorf("findings = %v, want malformed_contract (missing check/criteria/paths)", kinds)
	}

	ms, ok, err := ParseMilestones(dir)
	if err != nil || !ok {
		t.Fatalf("ParseMilestones: ok=%v err=%v", ok, err)
	}
	if ms[0].Contract != nil {
		t.Errorf("Contract = %+v, want nil (malformed contract is never trusted)", ms[0].Contract)
	}
}

func TestTasksContractAbsolutePathRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — g.\n"+
		"**Deliverables** — d.\n"+
		"**Validation contract** — checkable:\n"+
		"  - x\n"+
		"\n"+
		"  ```contract\n"+
		"  check: go test ./...\n"+
		"  criteria: fine\n"+
		"  paths:\n"+
		"    - /etc/passwd\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "malformed_contract") {
		t.Errorf("findings = %v, want malformed_contract (absolute path)", kinds)
	}
}

func TestTasksContractTraversalPathRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — g.\n"+
		"**Deliverables** — d.\n"+
		"**Validation contract** — checkable:\n"+
		"  - x\n"+
		"\n"+
		"  ```contract\n"+
		"  check: go test ./...\n"+
		"  criteria: fine\n"+
		"  paths:\n"+
		"    - ../../etc/passwd\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "malformed_contract") {
		t.Errorf("findings = %v, want malformed_contract (parent traversal)", kinds)
	}
}

func TestTasksDuplicateContractBlockRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), "## Milestone 1: Password login\n"+
		"**Goal** — g.\n"+
		"**Deliverables** — d.\n"+
		"**Validation contract** — checkable:\n"+
		"  - x\n"+
		"\n"+
		"  ```contract\n"+
		"  check: go test ./...\n"+
		"  criteria: fine\n"+
		"  paths: [internal/a/**]\n"+
		"  ```\n"+
		"\n"+
		"  ```contract\n"+
		"  check: go test ./...\n"+
		"  criteria: fine\n"+
		"  paths: [internal/b/**]\n"+
		"  ```\n"+
		"**Steps** — s:\n"+
		"  1. step one\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	kinds := findingKinds(findings)
	if !contains(kinds, "duplicate_contract") {
		t.Errorf("findings = %v, want duplicate_contract", kinds)
	}
}

// --- Steps checkbox tracking (opt-in, ParseMilestones) ---

func TestParseMilestonesStepsUntrackedByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Legacy
**Goal** — g.
**Deliverables** — d.
**Validation contract** — checkable:
  - x
**Steps** — s:
  1. step one
  2. step two
`)
	ms, ok, err := ParseMilestones(dir)
	if err != nil || !ok {
		t.Fatalf("ParseMilestones: ok=%v err=%v", ok, err)
	}
	if len(ms[0].Steps) != 2 {
		t.Fatalf("Steps = %+v, want 2", ms[0].Steps)
	}
	for _, s := range ms[0].Steps {
		if s.Tracked {
			t.Errorf("step %+v: Tracked = true, want false (no checkbox in source)", s)
		}
	}
}

func TestParseMilestonesStepsCheckboxTracked(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: Tracked
**Goal** — g.
**Deliverables** — d.
**Validation contract** — checkable:
  - x
**Steps** — s:
  1. [x] done step
  2. [ ] not done step
`)
	ms, ok, err := ParseMilestones(dir)
	if err != nil || !ok {
		t.Fatalf("ParseMilestones: ok=%v err=%v", ok, err)
	}
	steps := ms[0].Steps
	if len(steps) != 2 {
		t.Fatalf("Steps = %+v, want 2", steps)
	}
	if !steps[0].Tracked || !steps[0].Checked {
		t.Errorf("step[0] = %+v, want tracked+checked", steps[0])
	}
	if !steps[1].Tracked || steps[1].Checked {
		t.Errorf("step[1] = %+v, want tracked+unchecked", steps[1])
	}
	if steps[0].Text != "done step" || steps[1].Text != "not done step" {
		t.Errorf("Text = %q / %q", steps[0].Text, steps[1].Text)
	}
}

func TestParseMilestonesNoTasksFile(t *testing.T) {
	dir := t.TempDir()
	ms, ok, err := ParseMilestones(dir)
	if err != nil {
		t.Fatalf("ParseMilestones: %v", err)
	}
	if ok {
		t.Error("ok = true, want false (no tasks.md at all)")
	}
	if ms != nil {
		t.Errorf("milestones = %v, want nil", ms)
	}
}

func TestParseMilestonesIDAndTitle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.md"), `## Milestone 1: First
**Goal** — g.
**Deliverables** — d.
**Validation contract** — checkable:
  - x
**Steps** — s:
  1. step

## Milestone 2: Second
**Goal** — g.
**Deliverables** — d.
**Validation contract** — checkable:
  - x
**Steps** — s:
  1. step
`)
	ms, ok, err := ParseMilestones(dir)
	if err != nil || !ok {
		t.Fatalf("ParseMilestones: ok=%v err=%v", ok, err)
	}
	if len(ms) != 2 {
		t.Fatalf("len = %d, want 2", len(ms))
	}
	if ms[0].ID != 1 || ms[0].Title != "First" {
		t.Errorf("ms[0] = %+v", ms[0])
	}
	if ms[1].ID != 2 || ms[1].Title != "Second" {
		t.Errorf("ms[1] = %+v", ms[1])
	}
}
