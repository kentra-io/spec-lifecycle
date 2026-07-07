## Milestone 1: <name>
**Goal** — <one sentence>.
**Deliverables** — <files/components/behaviors produced>.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - <command/test to run> — <expected outcome>
  - <which spec scenario(s) this milestone makes pass>

  <!-- OPTIONAL — an execution engine (e.g. the orchestration module's
       read_plan step, harness orchestration.md §5.5) grades against this
       machine-readable contract instead of parsing the bullets above.
       Omit the whole ```contract block for a milestone with no automated
       grading; the bullets above are still enough on their own
       (backward compatible — `lifecycle validate` never requires it). -->
  ```contract
  check: <a single executable acceptance-check command, run from the project root>
  criteria: <plain-language acceptance criteria, for the advisory/human reviewer>
  paths:
    - <repo-relative glob this milestone's diff is confined to>
    - <another glob, if needed>
  ```
**Steps** — ordered breakdown, sized per `planGranularity` (lifecycle.yml, spec-lifecycle.md §10):
  1. [ ] <step>
  2. [ ] <step>

<!-- The "[ ]"/"[x]" checkbox on each Steps line is OPTIONAL and opt-in:
     a plain "<n>. <step>" line is still valid and untracked, exactly as
     before. Once a milestone tracks at least one step this way,
     `lifecycle archive` refuses to archive the change until every
     tracked step in it is checked "[x]" (the tasks-completion gate,
     harness orchestration.md §5.5) — an escape hatch,
     --force-incomplete-tasks, is always available and is recorded on the
     ledger record when used. -->
