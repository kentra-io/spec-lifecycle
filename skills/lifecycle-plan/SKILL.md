---
name: lifecycle-plan
description: Conduct the plan stage of a spec-lifecycle change — tasks.md milestones with pre-committed validation contracts, gated at gate 3 alongside a re-run of the constitution plan-gate. Invoke explicitly with /lifecycle-plan.
disable-model-invocation: true
---

# lifecycle-plan

**Stub (M6).** This bundle ships with correct frontmatter and a pointer to
the right verbs and docs so `lifecycle init` fans out a real, loadable
skill; the full prompt body lands in M7 (implementation-plan.md §8's
milestone map). Do not treat this file as the finished skill.

## What this stage produces

- `tasks.md` — milestones in the fixed shape (spec-lifecycle.md §4.2):
  **Goal**, **Deliverables**, **Validation contract** (checkable,
  pre-committed acceptance criteria — commands/tests and expected outcomes,
  which spec scenarios the milestone makes pass), **Steps** (sized per
  `planGranularity` in `lifecycle.yml`). Planning owns the contract;
  execution consumes it — never re-authors it.

## Gate mechanics (spec-lifecycle.md §3.3, §7)

1. Draft `tasks.md` in the change folder.
2. Re-run the constitution primitive's **plan-gate** skill against the
   (possibly design-gate-amended) constitution, emitting a fresh
   `deviation.json`.
3. Run `lifecycle validate --stage plan` and fix any findings.
4. Surface tasks.md and the deviation report to the human.
5. On approval, run `lifecycle approve --stage plan`. This verb is
   mutating — never pre-grant it in `allowed-tools`.

Execution (the implement→verify loop) is out of this primitive's scope
(spec-lifecycle.md §3.1) — this skill's job ends at an approved plan.

## Never

- Never grade a milestone against anything but its own pre-committed
  Validation contract.
- Never run `lifecycle approve` without the human's explicit approval.
