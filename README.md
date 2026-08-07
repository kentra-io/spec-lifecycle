# spec-lifecycle

A standalone SDD primitive: a **staged, human-gated, spec-driven issue lifecycle** with an **owned, native YAML spec format** — living specs and per-change spec deltas are structured YAML described by JSON Schemas published with the tool, and markdown is a deterministic, read-only projection (never hand-authored).

One GitHub issue ↔ one change folder, moving through **refine → design → plan**, each stage a fresh agent session emitting a human-approved artifact. Gates are **durable file records** (`approval-state.json`, `deviation.json`) — this primitive writes them; any enforcement engine (an orchestrator, CI) reads and blocks. On completion, the change's YAML spec delta folds deterministically into the **living spec** (`openspec/specs/*/spec.yaml`), with a from-empty replay guard verifying the projection never drifts from its event log. (The `openspec/` directory name is a legacy of the format's OpenSpec ancestry — a rename is tracked in issue #6; the format itself is owned here, not OpenSpec's.)

Companion primitive to [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution) — the same event-sourcing invariants (append-only events, derived projections, tool-only writes, verifiable fidelity), applied to the functional *what* instead of the architectural *how*. The plan stage delegates to [`milestoned-plan-dag`](https://github.com/kentra-io/milestoned-plan-dag): a change's `plan.yaml` is authored with that primitive's `plan-author` skill and validated by shelling out to its CLI — a process boundary, no Go import.

**Status: live.** See [spec-lifecycle.md](./spec-lifecycle.md) for the original design specification (historical — see its status banner) and `openspec/specs/` for the living spec.

## Shape

- **Layer 1** — `lifecycle` CLI (single static Go binary, deterministic, no LLM, no external language runtime): `init` · `validate` · `approve` · `status` · `archive` · `guard`. YAML parse, JSON-Schema validation, fold, and markdown render are all in-process (constitution ADR-0004).
- **Layer 2** — agent-agnostic skills (SKILL.md standard), fanned out by `lifecycle init` to the configured runtime trees: stage conduct (`lifecycle-refine`, `lifecycle-design`), intake (`lifecycle-new-feature`, `lifecycle-bug`), archive discipline (`lifecycle-archive`), setup (`lifecycle-init`). Plan authoring uses `milestoned-plan-dag`'s `plan-author` skill.
- **Layer 3** — integrations: the `kentra-spec-lifecycle` schema descriptor + published JSON Schemas (`living-spec.schema.json`, `spec-delta.schema.json`), the constitution seam (plan-gate at gates 2/3), the `milestoned-plan-dag` seam (plan gate + archive step-completion gate), engine/CI record consumers.

Correctness is proven by checked-in golden projection fixtures (source YAML ↔ expected markdown, byte-identical) plus `lifecycle guard`'s from-empty replay (constitution ADR-0003/ADR-0005).

MIT.
