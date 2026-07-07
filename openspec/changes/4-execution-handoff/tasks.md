## Milestone 1: Structured validation contract on a milestone
**Goal** — let a milestone declare an optional, machine-readable acceptance-check command, criteria, and allowed path-set.
**Deliverables** — `internal/validate/plan.go` (Contract/contractYAML types, `parseContractBlock`, `validateContractBlock`, `dedent`); `tasks.go` refactored to share `splitMilestoneBlocks`/`labelSection`; the ````contract` block documented in the `tasks.md` template and `spec-lifecycle.md` §4.2.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/validate/...` passes, including the new contract-block cases (well-formed, malformed YAML, missing fields, absolute/traversal paths, duplicate block).
  - A milestone with no `` ```contract `` block still validates with zero findings — discharges **"A contract-less milestone still validates"**.

  ```contract
  check: go test ./internal/validate/...
  criteria: All validate package tests pass, including the contract-block well-formed/malformed cases and the pre-existing checkbox-free Steps case (TestTasksMultipleMilestonesOneBad) unmodified.
  paths:
    - internal/validate/**
    - openspec/schemas/kentra-spec-lifecycle/templates/tasks.md
  ```
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. [x] Refactor `tasks.go`'s milestone-block splitting into shared `splitMilestoneBlocks`/`labelSection` helpers (no behavior change — proven by the pre-existing test suite passing unmodified).
  2. [x] Add `internal/validate/plan.go`: `Contract`/`Step`/`Milestone` types, `parseContractBlock` (fence detection, YAML decode, field + path-safety validation), `validateContractBlock` (Finding adaptation), `ParseMilestones` (extraction for archive/apply).
  3. [x] Add `internal/validate/plan_test.go` covering both validation (well-formed/malformed/duplicate/absolute-path/traversal) and extraction (steps untracked-by-default, checkbox-tracked, no-tasks-file, id/title).
  4. [x] Update the `tasks.md` template and `spec-lifecycle.md` §4.2 to document the block.

## Milestone 2: Archive tasks-completion gate
**Goal** — refuse to archive a change with outstanding, self-declared work.
**Deliverables** — `internal/archive/tasks_gate.go` (`checkTasksComplete`); `Request.ForceIncompleteTasks` / `Result.TasksIncompleteOverridden` / `Record.TasksIncompleteOverridden`; the gate wired into `Archive`'s pipeline; `cmd/lifecycle/archive.go`'s `--force-incomplete-tasks` flag.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/archive/...` passes, including `TestArchiveRefusesOnIncompleteTasks`, `TestArchiveForceIncompleteTasksOverride`, `TestArchiveAllowsAllStepsChecked`, `TestArchiveAllowsNoTasksFile`, `TestArchiveAllowsUntrackedSteps` — discharges **"Archive refuses on incomplete tracked tasks"** (all three scenarios).
  - `cmd/lifecycle/testdata/script/archive_tasks_incomplete_gate.txtar` passes — the real CLI, both the refusal and `--force-incomplete-tasks` paths, with `tasksIncompleteOverridden:true` verified on the ledger record.

  ```contract
  check: go test ./internal/archive/... ./cmd/lifecycle/...
  criteria: The tasks-completion gate refuses archive on any unchecked tracked step, is a no-op for a tasks.md-less or fully-untracked change, and its override is recorded (never silent) on every ledger record.
  paths:
    - internal/archive/**
    - cmd/lifecycle/archive.go
    - cmd/lifecycle/testdata/script/archive_tasks_incomplete_gate.txtar
  ```
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. [x] Add `tasks_gate.go`'s `checkTasksComplete`, reusing `validate.ParseMilestones`.
  2. [x] Wire the gate into `Archive` (types.go's new Request/Result/Record fields + Archive's step 1b), mirroring the existing gate-check/conflict-check override posture.
  3. [x] Add the `--force-incomplete-tasks` CLI flag + exit-code mapping (`ErrTasksIncomplete` → exit 1).
  4. [x] Unit tests (both paths) + one real-CLI txtar test proving refuse → override → success end to end.

## Milestone 3: Machine-readable `apply` surface
**Goal** — surface a change's milestones + contracts as JSON without bespoke markdown parsing.
**Deliverables** — `cmd/lifecycle/apply.go` (`applyCommand`, `runApply`, `applyResult`); registered in `cmd/lifecycle/main.go`; documented in `spec-lifecycle.md` §9.1.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go build ./... && go vet ./...` clean; `cmd/lifecycle/testdata/script/apply_json_and_text.txtar` passes (text + JSON output, and the refusal path on a malformed contract) — discharges **"lifecycle apply surfaces milestones and contracts as JSON"** (both scenarios).
  - `lifecycle apply <change> --format json` on a fixture with a well-formed contract emits `id`/`title`/`steps`/`contract` (with `check`/`criteria`/`paths`) per milestone.

  ```contract
  check: go test ./cmd/lifecycle/...
  criteria: apply emits the documented JSON shape for a well-formed plan and refuses (exit 1) for a plan that fails plan-stage validation.
  paths:
    - cmd/lifecycle/apply.go
    - cmd/lifecycle/main.go
    - cmd/lifecycle/testdata/script/apply_json_and_text.txtar
  ```
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. [x] Implement `apply.go`: re-run plan-stage validation, refuse on error findings, else project `ParseMilestones` + proposal meta as `applyResult`.
  2. [x] Register the verb in `main.go`; text + JSON writers.
  3. [x] Add the txtar test (well-formed text/JSON, malformed-contract refusal, missing-change could-not-run).
  4. [x] Document the JSON shape in `spec-lifecycle.md` §9.1 and the CLI's own `--help` description.
