---
name: lifecycle-bug
description: Conduct the minimal bug-flow profile of a spec-lifecycle change — repro-first, no spec delta by default, design skipped, promotable into the full feature flow. Invoke explicitly with /lifecycle-bug.
disable-model-invocation: true
---

# lifecycle-bug

**Stub (M6).** This bundle ships with correct frontmatter and a pointer to
the right verbs and docs so `lifecycle init` fans out a real, loadable
skill; the full prompt body lands in M7 (implementation-plan.md §8's
milestone map). Do not treat this file as the finished skill.

## The compressed profile (spec-lifecycle.md §8)

- `proposal.md` (`type: bug` in frontmatter) = the repro description plus a
  pointer to the **failing test** — both the requirement artifact and the
  validation contract. Repro-first is mandatory: not reproducible ⇒ back to
  the human (`Needs Input`).
- **No spec delta by default** — a bug is a failure to meet already-specced
  behavior. **If the repro reveals mis-specced behavior**, the fix is
  spec-affecting: add a `specs/<capability>/spec.md` delta and gate it like
  a feature refine.
- `design` is skipped by default; `tasks.md` is optional (usually one
  milestone whose contract is the repro test).

## Promotion hatch

If the fix spans architecture or would require a constitutional deviation,
promote into the full feature flow (insert `design`+`plan` stages in the
same folder) — see `lifecycle-design`/`lifecycle-plan`. A promoted bug's
gates may mix `repro`/`fix` and `design`/`plan`; gate-checks key off the
change type recorded at intake, not a fixed stage list.

## Gate mechanics

1. Reproduce the bug; write the failing test.
2. Run `lifecycle validate --stage repro` (and `--stage fix` once fixed)
   and fix any findings.
3. Surface the repro + fix to the human; on approval run
   `lifecycle approve --stage repro` / `--stage fix`. Mutating — never
   pre-grant in `allowed-tools`.
4. Archiving a delta-less bug is pure archival — `lifecycle-archive` still
   applies; the ledger record simply carries an empty `deltaOps`.

## Never

- Never skip the repro step — an unreproduced bug goes back to the human.
- Never add a spec delta for behavior that was already correctly specced.
