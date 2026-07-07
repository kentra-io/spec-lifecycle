---
issue: "kentra-io/spec-lifecycle#4"
designSkip: false
type: feature
---

# Execution handoff — the harness `orchestration` module's §5.5 additions

## Why

The harness `orchestration` module (execution leg: implement → verify →
escalate against an approved plan) needs three things from `tasks.md`
that spec-lifecycle does not yet provide: a machine-readable,
pre-committed validation contract per milestone (an executable
acceptance-check command, plain-language criteria, and the allowed
path-set an execution engine confines a milestone's diff to), a real gate
so a change cannot archive with outstanding work, and a data surface so a
consumer doesn't have to hand-parse `tasks.md` itself. This change adds
all three, additively.

## What Changes

- **execution-handoff:** ADDED — an optional, structured
  ` ```contract ` YAML block inside a milestone's Validation contract
  section (`check` / `criteria` / `paths`), enforced well-formed by
  `lifecycle validate --stage plan` when present; absent, a plan still
  validates exactly as before.
- **execution-handoff:** ADDED — opt-in `[ ]`/`[x]` checkbox tracking on
  Steps-list lines, and a new `lifecycle archive` tasks-completion gate:
  once any step in a milestone is tracked this way, every tracked step in
  it must be checked before the change can archive (`--force-incomplete-tasks`
  overrides, recorded on the ledger record). A `tasks.md` with no tracked
  steps, or no `tasks.md` at all, is never gated by this.
- **execution-handoff:** ADDED — a new `lifecycle apply <change>
  [--format json]` verb: a read-only projection of a change's milestones
  (id/title, Steps with checkbox state, the optional contract) as text or
  JSON, refusing (exit 1) if the plan fails the same structural validation
  `validate --stage plan` already runs.

## Impact

- `internal/validate` (new `plan.go`: contract parsing/validation, Steps
  checkbox parsing, `ParseMilestones`), `internal/archive` (new
  `tasks_gate.go` + `Request.ForceIncompleteTasks` /
  `Result.TasksIncompleteOverridden` / `Record.TasksIncompleteOverridden`),
  `cmd/lifecycle` (new `apply.go`, `archive.go`'s new flag).
- `openspec/schemas/kentra-spec-lifecycle/templates/tasks.md`,
  `skills/lifecycle-plan/SKILL.md`, `skills/lifecycle-archive/SKILL.md`
  (+ fanned-out dogfood copies), `spec-lifecycle.md` §4.2/§6.2/§9.1,
  `README.md`.
- No breaking change: both additions are opt-in: an existing plan with no
  ` ```contract ` block and no Steps checkboxes validates and archives
  exactly as it did before this change.
