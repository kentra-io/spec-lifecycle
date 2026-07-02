# spec-lifecycle

A standalone SDD primitive: a **staged, human-gated, spec-driven issue lifecycle** built on [OpenSpec](https://github.com/Fission-AI/OpenSpec) as the artifact runtime.

One GitHub issue ↔ one change folder, moving through **refine → design → plan**, each stage a fresh agent session emitting a human-approved artifact. Gates are **durable file records** (`approval-state.json`, `deviation.json`) — this primitive writes them; any enforcement engine (an orchestrator, CI) reads and blocks. On completion, the change's structured spec delta folds deterministically into the **living spec** (`openspec/specs/`), with a replay guard verifying the projection never drifts from its event log.

Companion primitive to [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution) — the same event-sourcing invariants (append-only events, derived projections, tool-only writes, verifiable fidelity), applied to the functional *what* instead of the architectural *how*.

**Status: design — pending review.** See [spec-lifecycle.md](./spec-lifecycle.md) for the full specification.

## Shape

- **Layer 1** — `lifecycle` CLI (Go, deterministic, no LLM): `init` · `approve` · `status` · `guard`
- **Layer 2** — agent-agnostic skills (SKILL.md standard): stage conduct, bug repro-first flow, archive discipline
- **Layer 3** — integrations: a custom OpenSpec schema (`kentra`), the constitution seam, engine/CI record consumers

MIT.
