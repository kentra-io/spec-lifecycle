# Execution handoff — Design

## Context

The harness `orchestration` module (the execution leg that drives an
approved `spec-lifecycle` plan to merged code) specified three gaps in
this primitive's `tasks.md`/`archive`/CLI surface it depends on
(harness `orchestration.md` §5.5): a pre-committed, machine-readable
validation contract per milestone; a real gate against archiving
unfinished work; and a JSON surface so a consuming engine's `read_plan`
step doesn't have to hand-parse markdown. This design closes all three
additively, inside the existing `tasks.md` artifact and `archive`
pipeline — no new artifact, no new schema stage.

## Goals / Non-Goals

**Goals:**
- A milestone can declare an executable acceptance-check command,
  plain-language criteria, and a diff-confined-paths declaration, in a
  form `lifecycle validate` can enforce.
- `lifecycle archive` cannot silently fold a change that still has
  outstanding, self-declared work.
- A single, stable, documented JSON contract exposes a plan's milestones
  and contracts — the surface orchestration's `read_plan` step codes
  against.
- Every existing `tasks.md` — including this repo's own already-archived
  changes — keeps validating and archiving exactly as before.

**Non-Goals:**
- Executing a milestone's acceptance-check command, or grading a diff
  against its allowed path-set, is explicitly out of scope — that is
  the `orchestration` module's job. This primitive only stores and
  surfaces the contract.
- No new gate stage, no new schema.yaml artifact entry, no change to the
  refine/design/plan stage set.

## Decisions

### D1: Contract lives in a fenced ` ```contract ` YAML block, not a new artifact or new labels

**Alternatives considered:**
- A fifth fixed milestone label (e.g. `**Contract**`) alongside
  Goal/Deliverables/Validation contract/Steps — rejected: it would make
  the contract mandatory-shaped (every milestone would need the label
  present, even empty), breaking the "optional, additive" requirement
  and complicating `validatePlan`'s existing four-label check.
- A sibling artifact (e.g. `contract.yaml` per change, or per milestone)
  — rejected: multiplies files-per-change for something that is
  logically part of one milestone's Validation contract; `tasks.md`
  staying the single source of a milestone's shape is exactly
  spec-lifecycle.md §4.2's existing design.
- A YAML frontmatter-style block at the top of `tasks.md` — rejected:
  milestones are per-block, not per-file; a top-level block can't express
  "this contract belongs to milestone 2".

**Chosen:** an OPTIONAL fenced ` ```contract ` block nested inside a
milestone's existing Validation contract section, YAML-encoded (this repo
already depends on `go.yaml.in/yaml/v3` for proposal.md frontmatter — no
new dependency). Its presence is entirely orthogonal to the four fixed
labels: a milestone with no block validates exactly as `validatePlan`
already required, and one with a block gets ADDITIONAL structural
checking. `internal/validate/plan.go` shares its milestone-block-splitting
logic with `tasks.go` (refactored into `splitMilestoneBlocks`/
`labelSection`) rather than re-parsing tasks.md a second way.

### D2: Tasks-completion tracking is opt-in per Steps line, not a new required field

**Alternatives considered:**
- Require every Steps line to carry a checkbox (mandatory) — rejected:
  breaks this repo's OWN existing tests (`TestTasksMultipleMilestonesOneBad`
  explicitly asserts a plain `1. step` line is well-formed) and every
  already-archived change's `tasks.md`. `lifecycle validate` would newly
  reject content it accepted yesterday.
- A milestone-level "done: true" field instead of per-step checkboxes —
  rejected: coarser than useful to an execution engine (which wants to
  know WHICH step is outstanding, for its own retry/escalation loop), and
  loses the natural "steps are the execution engine's todo list" mapping.

**Chosen:** an OPTIONAL `[ ]`/`[x]` checkbox right after a Steps line's
ordinal number (GFM task-list convention grafted onto the existing
ordered list). A milestone activates the gate only once it tracks at
least one step; an untracked milestone (or a `tasks.md`-less change, e.g.
a delta-less bug fix) is never gated — verified directly:
`TestArchiveAllowsNoTasksFile`/`TestArchiveAllowsUntrackedSteps` exercise
exactly this, alongside `TestArchiveRefusesOnIncompleteTasks`/
`TestArchiveForceIncompleteTasksOverride`/`TestArchiveAllowsAllStepsChecked`
for the gated path.

The gate sits in `internal/archive` as its own check
(`tasks_gate.go:checkTasksComplete`), run immediately after the existing
gate-check and before the conflict-check — same override posture as
`ForceGates`/`ForceConflicts` (a dedicated `ForceIncompleteTasks` flag,
recorded on `Result` and every `Record`, never silent).

### D3: A new `lifecycle apply` verb, not an extension of `status`

**Alternatives considered:**
- Extend `lifecycle status --format json` with a `milestones` field —
  rejected: `status` is specifically, and only, about
  `approval-state.json` gate state (approved/pending/rejected/skipped +
  drift); folding in unrelated tasks.md content would conflate two
  reporting concerns and bloat a JSON shape other tooling already depends
  on. `status`'s own package doc frames it as "reads each change folder's
  approval-state.json (never writes)" — tasks.md isn't a gate record.
- Name it `lifecycle plan --format json` — rejected as a name: `plan` is
  already used as a `--stage` VALUE (`validate --stage plan`,
  `approve --stage plan`) across three existing verbs; a command also
  named `plan` reads as ambiguous next to those. `apply` is free, and
  `schema.yaml`'s own `apply:` entry ALREADY names this exact concept —
  "read the approved tasks.md, work through its milestones, grade each
  against its pre-committed Validation contract" — as the (until now
  undocumented-as-a-verb) stage after `plan`. Naming the verb after the
  schema's own vocabulary is the better fit, not a coincidence.

**Chosen:** `lifecycle apply <change> [--format text|json]`, read-only,
gate-independent (like `validate`/`status`). It re-runs
`validate.Change(dir, StagePlan)` first and refuses (exit 1) on any
error-severity finding — the milestone/contract data it surfaces is
therefore always already trustworthy; a consumer never has to re-derive
"is this plan well-formed" itself. The JSON shape (`change`/`type`/
`issue`/`milestones[]`, each milestone: `id`/`title`/`steps[]`/
`contract`) is documented in `spec-lifecycle.md` §9.1 and demonstrated in
`cmd/lifecycle/testdata/script/apply_json_and_text.txtar`.

## NFR Discharge

- Backward compatibility (every existing `tasks.md` keeps validating and
  archiving unchanged): discharged by design (D1/D2 above are additive,
  opt-in extensions) and verified directly — the pre-existing
  `TestTasksMultipleMilestonesOneBad` (plain, untracked Steps) and every
  pre-existing `archive_test.go`/txtar fixture that never writes a
  `tasks.md` at all both still pass unmodified.
- Path-set safety (a milestone's `paths` declaration must be usable as a
  real diff-confinement boundary): discharged by rejecting absolute paths
  and `..` traversal at validation time (`parseContractBlock`), so a
  malformed declaration is caught here, not by the execution engine that
  trusts it later.

## ADR proposals

(none) — this change adds capability inside spec-lifecycle's existing
architecture; it does not touch any of the three standing ADRs
(pure-Go engine, conformance-corpus provenance, ledger-seq ordering).

## Risks / Trade-offs

- [Risk] A milestone author forgets the `[ ]`/`[x]` checkbox is opt-in and
  assumes ALL Steps are gated → [Mitigation] the updated
  `openspec/schemas/kentra-spec-lifecycle/templates/tasks.md` template and
  both stage skills (`lifecycle-plan`, `lifecycle-archive`) now say so
  explicitly, and the gate's own refusal message names the exact
  unchecked step.
- [Risk] A `paths` glob that is syntactically valid but too broad (e.g.
  `**`) would let an execution engine's diff-confinement be a no-op →
  [Mitigation] out of scope here — this primitive validates well-formedness,
  not glob tightness; that judgment call belongs to plan-stage human
  review, same as everything else in a Validation contract.
