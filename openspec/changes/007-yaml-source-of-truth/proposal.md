---
issue: "kentra-io/spec-lifecycle#7"
designSkip: false
type: feature
---

# Flip spec-lifecycle to a YAML source of truth (drop OpenSpec conformance)

## Why

The plan schema (the `tasks.md` milestone/contract grammar) is a reusable
primitive, now extracted to
[`milestoned-plan-dag`](https://github.com/kentra-io/milestoned-plan-dag);
spec-lifecycle should stop owning it. At the same time we are dropping
OpenSpec conformance entirely and choosing **primitive-family
consistency** instead: **YAML becomes the source of truth for the living
spec and spec deltas, a published JSON Schema describes that YAML, and
markdown becomes a read-only projection.** This is a deliberate, one-way
trade of OpenSpec interop for a format we own end to end — the same
YAML-source / schema / rendered-projection shape the sibling primitives
already use.

## What Changes

- **spec-format** (NEW capability) — the living spec
  (`openspec/specs/<capability>/spec.md` today) and per-change spec deltas
  are authored as **YAML**, not the markdown OpenSpec delta grammar; a
  **JSON Schema** describing that YAML is published with the tool; markdown
  becomes a **deterministic, read-only projection** of the YAML; the
  projection surfaces **derived kebab-slug fields** for every requirement
  and scenario (folds in #5). The delta / fold / from-empty-replay engine
  is retargeted to operate on the structured YAML — the gated-fold + replay
  guard differentiator is kept (constitution ADR-0003 survives, retargeted
  to YAML; raised at design).
- **plan-integration** (NEW capability) — the **plan-stage gate** validates
  a change's plan by delegating to `milestoned-plan-dag validate` (a
  CLI/YAML boundary, no Go import); the **archive step-completion gate**
  reads milestone done-states from `milestoned-plan-dag resolve` instead of
  parsing `tasks.md`; the machine plan surface is plan-dag's `resolve` (so
  there is **no `lifecycle apply` verb**, and the `tasks.md` template + the
  `tasks` schema artifact are dropped from the schema descriptor).
- **status-reporting** (MODIFIED capability) — `lifecycle status`'s machine
  output is **YAML**; the `--format json` option is dropped (folds in #2).

## Impact

- **Scope: spec-lifecycle only.** The downstream `agent-orchestration`
  repoint (its `plan.py` ingestion + `apply`/`archive`/`status` shell-outs
  moving to plan-dag) is a **separate change in that repo** — a noted
  dependency, not part of this work.
- **Supersedes pending change #4 (`execution-handoff`).** #4 (unapproved,
  unfolded) would have *added* `lifecycle apply`, the milestone `contract`
  block, and the checkbox archive gate to spec-lifecycle — the very
  surfaces this change relocates to `milestoned-plan-dag`. #4 is obsoleted
  by the plan-schema extraction; its issue is closed and its change folder
  (`openspec/changes/4-execution-handoff/`) removed as part of this work.
- **Constitution (design stage).** A new ADR **supersedes ADR-0001** (own
  YAML format, not the OpenSpec on-disk convention) and **ADR-0002** (prove
  correctness with projection golden tests, not a runtime-pin conformance
  corpus); **ADR-0003 survives**, retargeted to YAML. This is why the
  change is **not** design-skippable — the plan-gate / constitution gate
  runs at design.
- **Clean break, no migrator** (dev mode). Existing markdown living specs
  are re-authored, not migrated.
- **`openspec/` directory name kept for now**; the reference cleanup +
  eventual rename is tracked in #6.
- **Bootstrap note.** This change's own spec deltas are authored in the
  current markdown OpenSpec format — the very format being replaced. The
  change *builds* the YAML format; it is not authored in it.
- Affected surfaces: the Go format engine (parse/validate/fold/render), the
  published JSON Schema + schema descriptor (drop `tasks.md`/`tasks`), the
  `status`/`archive`/`validate --stage plan` verbs, the dropped OpenSpec
  conformance corpus, and the `lifecycle-plan` skill (retires/thins in
  favor of plan-dag's `plan-author` skill).
