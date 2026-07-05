---
name: lifecycle-design
description: Conduct the design stage of a spec-lifecycle change — design.md with an NFR-discharge section and any ADR proposals, gated at gate 2 alongside the constitution plan-gate. Invoke explicitly with /lifecycle-design.
disable-model-invocation: true
---

# lifecycle-design

**Stub (M6).** This bundle ships with correct frontmatter and a pointer to
the right verbs and docs so `lifecycle init` fans out a real, loadable
skill; the full prompt body lands in M7 (implementation-plan.md §8's
milestone map). Do not treat this file as the finished skill.

## What this stage produces

- `design.md` — Context, Goals/Non-Goals, Decisions, an explicit
  **NFR Discharge** section accounting for every NFR the refine-stage delta
  declared that belongs here (spec-lifecycle.md §4.1), and any ADR
  proposals (`adr-proposals/*.md`, MADR body shape) this design requires.

Skip this artifact entirely only when refine proposed — and gate 1
approved — a design-skip (spec-lifecycle.md §3.2); the plan-gate still runs
at gate 3 regardless.

## Gate mechanics (spec-lifecycle.md §3.3, §7)

1. Draft `design.md` (and any ADR proposals) in the change folder.
2. Load `constitution/constitution.md` and run the companion
   `adr-sourced-constitution` primitive's **plan-gate** skill, emitting
   `deviation.json` into this change folder; every deviation resolves
   conform-or-amend before approval. Each proposed ADR is reviewed and
   consented to **individually** — never bundled into blanket design
   approval (spec-lifecycle.md §7 item 3, HARD RULE).
3. Run `lifecycle validate --stage design` and fix any findings.
4. Surface design.md, the deviation report, and any ADR proposals to the
   human.
5. On approval, run `lifecycle approve --stage design`. This verb is
   mutating — never pre-grant it in `allowed-tools`.

## Never

- Never bundle multiple ADR proposals into one approval decision.
- Never run `lifecycle approve` without the human's explicit approval.
