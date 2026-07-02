# `spec-lifecycle` — Design Specification

*Version: v1 draft. Generated: 2026-07-02. Status: **DESIGN — pending user review.***

*A standalone SDD primitive: a staged, human-gated, spec-driven issue lifecycle built on **OpenSpec** as the artifact runtime, with **file-based gate records** as the canonical interface to any external enforcement engine. Companion primitive to [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution) (the governance substrate). Consumed by the kentra harness, but — like the constitution primitive — framework-portable and not harness-bound.*

*Decision provenance: the harness's `references/sdd-framework-research-2026-07.md` (four-agent framework evaluation, 2026-07-02) and `tasks/planning-module-handoff.md` (decision ledger P0–P6). Repo name `spec-lifecycle` is a working name — revisit before publishing.*

---

## 0. Terminology (locked)

- **Change** — one unit of work moving through the lifecycle: one GitHub issue ↔ one OpenSpec change folder. "Issue's spec-folder" in older harness docs = the change folder.
- **Stage** — a lifecycle phase producing one approved artifact set: `refine`, `design`, `plan` (features); `repro`, `fix` (bugs).
- **Gate** — a human-approval boundary between stages, recorded durably in `approval-state.json`. Gates are **records, not enforcement**: this primitive writes them; an engine (Conductor) or CI reads and blocks. Same capability line as the constitution primitive.
- **Living spec** — `openspec/specs/<capability>/spec.md`: the current-state behavior contract of the whole system, produced by folding archived change deltas. The functional twin of `constitution.md`.
- **Delta** — a structured spec change (`## ADDED/MODIFIED/REMOVED/RENAMED Requirements` with `### Requirement:` / `#### Scenario:` blocks) inside a change folder. Deltas are the **event log** of the living spec.
- **Validation contract** — the pre-committed, checkable acceptance criteria attached to each plan milestone. Authored at plan time, consumed (never re-authored) by execution.
- **SDD** — spec-driven development. (Obra Superpowers uses "SDD" for *subagent*-driven development — unrelated.)

## 1. Purpose & scope

**What it is.** The staged planning lifecycle for a repository: intake → refine → design → plan, each stage emitting a human-approved artifact into a per-issue change folder; plus the living-spec fold (archive) on completion, and the seams to the constitution primitive and to an external enforcement engine.

**What it is NOT (owned elsewhere):**
- **Not orchestration.** No scheduler, no run-state, no blocking. Conductor (or CI) enforces gates by reading this primitive's records. (planning.md two-mode model: this primitive is the artifact/record layer under both modes.)
- **Not governance.** ADR log, `constitution.md` projection, plan-time deviation gate: all `adr-sourced-constitution`. This spec only defines *when* those are invoked and *where* their records land.
- **Not execution.** The implement→verify loop, code-time constitution check, retries/escalation: execution domain (harness).
- **Not the drift detector, not human-facing reference docs** — deferred harness-level concerns.

**Event-sourcing symmetry (the design's spine).** Both companion primitives obey one invariant set: **append-only events · derived projections · tool-only writes · verifiable fidelity · issue-linked provenance.**

| | `adr-sourced-constitution` | `spec-lifecycle` |
|---|---|---|
| Event log | ADRs (whole decisions, supersede) | Archived change deltas (requirement ops) |
| Projection | `constitution.md` — pure re-render of active set | `openspec/specs/` — ordered fold of deltas |
| Fidelity check | `constitution guard` | `lifecycle guard` (replay check, §6.3) |
| Event grain rationale | Decisions are sparse, discrete, citable (`ADR-id`) | Requirements are a dense corpus; block-level ops are the right grain |

## 2. Position in the stack

```
  Obra Superpowers (pinned, co-installed plugin)      ← execution disciplines: TDD,
      no dependency from this primitive                  systematic-debugging, subagents
  ─────────────────────────────────────────────
  spec-lifecycle (THIS)                                ← stages, gates, records, schema
      │ runs on                                          bug flow, validation contracts
      ▼
  OpenSpec (@fission-ai/openspec, PINNED — v1.5.0     ← artifact runtime: change folders,
      at spec time; LiteLLM pinning precedent)           delta format, validate, archive/fold,
      via custom schema openspec/schemas/kentra/         30-tool command generation
  ─────────────────────────────────────────────
  adr-sourced-constitution (sibling primitive)         ← ADR log, constitution.md, plan-gate
  Conductor / CI (external)                            ← reads approval-state.json /
                                                          deviation.json; ENFORCES
```

**Non-negotiable dependencies' roles:** OpenSpec supplies plumbing that is commodity (folders, delta grammar, deterministic single-change fold, cross-agent command generation). Everything novel — gates, records, contracts, bug flow, replay guard, constitution seam — is this primitive's, implemented *alongside* OpenSpec's files, never by patching OpenSpec. **Ejection stays cheap by construction:** all canonical state is files in the consuming repo; if OpenSpec's (young, ~5-month-old) schema subsystem disappoints, we port the fold algorithm into `lifecycle` and keep every folder and record unchanged.

## 3. The lifecycle model — stages & gates

### 3.1 Feature flow (default)

```
 GitHub issue (type: feature, human-initiated)
   │
   ▼
 REFINE   → proposal.md + specs/ delta        ══ GATE 1: requirements approved
   │         (functional + NFR requirements)      (may also approve a design-skip)
   ▼
 DESIGN   → design.md + ADR proposals         ══ GATE 2: design approved
   │         (constitution plan-gate runs)        (+ per-ADR consent, HARD RULE)
   ▼
 PLAN     → plan.md (milestones +             ══ GATE 3: plan approved
   │         validation contracts;                (constitution plan-gate re-runs)
   ▼         constitution plan-gate re-runs)
 [execution — out of scope]
   │
   ▼
 ARCHIVE  → openspec archive folds the delta into openspec/specs/;
            change folder moves to changes/archive/YYYY-MM-DD-<name>/
```

**Three stages, three gates. No distinct `tasks` stage** — step breakdown lives inside `plan.md` milestones, sized by `planGranularity` (§10). Each stage runs in a fresh agent session; the previous stage's approved artifacts are the entire input ("the artifact is the interface").

### 3.2 Collapse rule (small work)

The refine stage MAY propose skipping `design` (small, local, architecturally inert work). The skip is approved by the human **at gate 1** and recorded in that gate's approval entry (`"designSkipped": true`). A skipped design does not skip the constitution gate — the plan-gate still runs at gate 3. Mirrors the bug flow's built-in collapse (§8).

### 3.3 Gate mechanics (per gate)

1. Agent produces/revises the stage artifacts in the change folder.
2. Deterministic checks: `openspec validate` (structure) and — at gates 2 and 3 — the constitution primitive's **plan-gate** skill, emitting `deviation.json` into the change folder (each finding cites an `ADR-id`).
3. Agent surfaces artifacts + any deviations conversationally. Every deviation resolves **conform or amend** before approval (amend = the constitution primitive's consent flow).
4. Human approves conversationally → `lifecycle approve --stage <s>` (§9.1) writes the gate entry: artifact hashes, `constitutionHash`, deviation reference, approver, timestamp.
5. **Enforcement is external:** in Phase 1 (no engine) the human honors the records; Conductor later hard-blocks on them. This primitive never blocks.

**Constitution gate placement — twice, by design:** at design (where architecture violations bite) and at plan (final pre-execution re-check against the *current* constitution — which may have changed at gate 2 via accepted ADRs). Cheap (Haiku-class), so redundancy is free insurance.

## 4. Artifacts & the `kentra` schema

Shipped as a custom OpenSpec schema (`openspec/schemas/kentra/{schema.yaml, templates/*.md}`), committed in the consuming repo — the maintainer-endorsed extension path (OpenSpec #557/#536). Artifact DAG:

| Artifact | Stage | Content | Divergence from stock OpenSpec |
|---|---|---|---|
| `proposal.md` | refine | Why / What changes / Impact; issue ref in frontmatter (§10); may propose design-skip or a new capability | none (template tuned) |
| `specs/<capability>/spec.md` | refine | **The requirements artifact**: delta blocks, RFC 2119 requirements + GIVEN/WHEN/THEN scenarios. **Functional and non-functional merged** (§4.1) | none — stock delta grammar |
| `design.md` | design | Technical design; explicit **NFR-discharge section** (how each declared NFR is met); **ADR proposals** (§7) | content conventions only |
| `plan.md` | plan | Milestones + validation contracts (§4.2) | **replaces stock `tasks.md`** — the one renamed artifact, because the content genuinely differs (structured contracts vs unstructured checkboxes) |

### 4.1 NFR routing rule (stock-OpenSpec-conformant)

NFRs are requirements (harness decision P2). Placement follows OpenSpec's own line — *the spec is a behavior contract*:

| NFR kind | Home |
|---|---|
| Measurable, behavior-observable ("SHALL sustain 10k msg/s") | ordinary requirement block in the **spec delta**, scenarios included — merged with functional |
| Cross-cutting / project-wide invariant (stack, security posture, whether continuous perf-testing exists) | **constitution ADR** |
| Internal quality with no observable behavior | **`design.md`** |

Design MUST account for every declared NFR (the discharge section). Perf/benchmark validation is asynchronous by policy (P2) — codebase tests run alongside development, not per-milestone. *(A scenario-level async-validation tag was considered and deferred: harness `roadmap-ideas.md`.)*

### 4.2 `plan.md` — milestone format

Per milestone, fixed headings (machine-parseable by a verifier agent, human-readable first):

```markdown
## Milestone <n>: <name>
**Goal** — one sentence.
**Deliverables** — files/components/behaviors produced.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - commands/tests to run and their expected outcomes
  - which spec scenarios this milestone makes pass
**Steps** — ordered breakdown, sized per `planGranularity`.
```

Planning owns contracts; execution consumes them (harness planning.md §9). The verifier grades against the contract, never a moving target.

## 5. Gate records — the canonical interface

Two JSON records per change folder; both committed; an engine reads both. `deviation.json` is defined by the constitution primitive (SARIF-shaped, `adrId` required per finding) — this spec only fixes its location: the change folder.

**`approval-state.json`** (this primitive's, one file per change, append-per-gate):

```json
{
  "schemaVersion": 1,
  "change": "042-user-auth",
  "issue": "kentra-io/kafka-dq#42",
  "gates": [
    {
      "stage": "refine",
      "status": "approved",            // approved | rejected | pending
      "designSkipped": false,
      "artifacts": { "proposal.md": "sha256-…", "specs/auth/spec.md": "sha256-…" },
      "constitutionHash": "sha256-…",  // constitution.md at approval time
      "deviationRef": null,             // path of deviation.json when a gate ran the plan-gate
      "approvedBy": "jan",
      "approvedAt": "2026-07-02T14:00:00Z",
      "notes": ""
    }
  ]
}
```

Only `lifecycle approve` writes it (tool-only writes; hashes are computed, never hand-typed). Consent model mirrors the constitution primitive: skills do **not** pre-grant the mutating verb — the agent-harness permission boundary is the v1 consent checkpoint.

## 6. Living spec, archiving & the replay guard

### 6.1 Fold (stock OpenSpec)

On completion, `openspec archive` deterministically applies the change's delta ops to `openspec/specs/` and relocates the folder to `changes/archive/YYYY-MM-DD-<name>/`. The archive directory is thereafter **append-only** — it is the event log; archived folders are never edited (guard-checked, §6.3).

### 6.2 Capability taxonomy

- Capabilities are **created on demand by refine**: a delta targeting a new capability flags it in `proposal.md`; the human approves it at gate 1. Greenfield starts empty and accumulates (same philosophy as the ADR log).
- Kebab-case nouns. Split/merge/rename of capabilities MUST be expressed **as a change** (RENAMED/REMOVED/ADDED delta ops) — never by hand-editing `openspec/specs/` — so replayability survives restructuring.
- Size discipline: warn when a capability's `spec.md` exceeds ~200 lines (mirror of the constitution's projection warning) — a split candidate.

### 6.3 Replay guard (`lifecycle guard`)

The constitution-grade fidelity check OpenSpec lacks: recompute `fold(archived deltas, in archive-date order, from empty)` and diff against `openspec/specs/`. Also verify archive immutability (content-hash manifest over `changes/archive/`, mirroring `constitution guard`'s mechanism). Exit contract: `0` clean / `1` violation / `2` could-not-run; `--format json`. Run in CI and before any archive.

### 6.4 Cross-change conflicts

Policy first: **prefer one in-flight change per capability** (the harness's GitHub claim-mutex makes this cheap to honor; refine SHOULD flag capability overlap with in-flight changes in the proposal). When conflicts still occur at archive time, resolution is agent-assisted (OpenSpec's model) but MUST produce a revised, re-validated delta in the *unarchived* change — the archive log stays untouched, and `lifecycle guard` must pass afterward.

## 7. Constitution seam (design ↔ ADR)

1. **Context in:** the design (and plan) stages load `constitution/constitution.md` — via the constitution primitive's skills and an entry in OpenSpec's `config.yaml` context block (its endorsed injection point).
2. **Proposals out:** design drafts **ADR proposals as files in the change folder** (`adr-proposals/*.md`, MADR body shape). Proposals are ephemeral — they never enter `constitution/adr/` (the primitive's rule: the accept IS the write).
3. **Consent at gate 2:** the human reviews the design *and each proposed ADR individually* (HARD RULE — per-ADR explicit consent, never bundled into blanket design approval). Each accepted proposal is written via `constitution adr new --body-file …` (source-ref = the issue), which auto-`regen`s `constitution.md`.
4. **Timing consequence (deliberate):** the constitution updates **eagerly at design-approval**; the living spec updates **at completion (archive)**. Plan and execution therefore always validate against an already-current constitution. The decisions-lead-behavior asymmetry is correct: a decision governs the work; the behavior contract reflects *delivered* work.
5. Deviations found by the plan-gate resolve conform-or-amend before gate approval (§3.3); `deviation.json` lands in the change folder.

## 8. Bug flow (minimal schema)

Bugs get a change folder too — uniform folder-per-issue; records need a home — but a compressed profile:

- `proposal.md` = the repro description + pointer to the **failing test**, which is *both* the requirement artifact *and* the validation contract (done = test passes + no regressions). Repro-first is mandatory; not reproducible ⇒ back to the human (`Needs Input`).
- **No spec delta by default** — a bug is by definition a failure to meet already-specced behavior; fixing it changes no contract. **If the repro reveals mis-specced behavior**, the fix is spec-affecting: add a delta, and gate it like a feature refine.
- `design` skipped by default; `plan.md` optional (usually a single milestone whose contract is the repro test).
- **Promotion hatch** (unchanged from harness planning.md §11): if the fix spans architecture or would require a constitutional deviation, promote the change into the full feature flow (insert design + plan stages; same folder).
- Archiving a delta-less change is pure archival — no fold, guard unaffected.

## 9. The three layers

Mirrors the constitution primitive's shape.

### 9.1 Layer 1 — CORE: the `lifecycle` CLI

Go, single static binary, sibling of `constitution` (shared `skillfanout` + atomic-write + guard-manifest internals — extract the shared package when this repo bootstraps). Deterministic, no LLM. v1 verbs:

| Verb | Does |
|---|---|
| `lifecycle init` | Scaffold: install the `kentra` schema into `openspec/schemas/`, write `lifecycle.yml`, wire OpenSpec `config.yaml` context entries, fan out skills, write managed AGENTS.md/CLAUDE.md pointer blocks (constitution-style markers) |
| `lifecycle approve --stage <s> [--reject] [--notes …] [--design-skip]` | Compute artifact hashes + `constitutionHash`, append the gate entry to `approval-state.json` |
| `lifecycle status [--change <n>]` | Report gate state across change folders (reads records only) |
| `lifecycle guard [--format json]` | §6.3 replay + archive-immutability check |

Not in the CLI: anything OpenSpec already does (`validate`, `archive`, command generation) — we call theirs, pinned.

### 9.2 Layer 2 — AGENT SURFACE: skills

Agent-agnostic (SKILL.md standard), fanned out like the constitution's: `lifecycle-refine`, `lifecycle-design`, `lifecycle-plan` (stage conduct: fresh-session discipline, artifact templates, gate mechanics incl. running the constitution plan-gate at 2/3), `lifecycle-bug` (repro-first flow), `lifecycle-archive` (conflict-check → archive → guard). Mutating verbs (`approve`, `constitution adr new`) are never pre-granted in `allowed-tools`.

### 9.3 Layer 3 — INTEGRATIONS

- **OpenSpec** (required runtime): the schema + config wiring, version pinned in `lifecycle.yml`.
- **Constitution primitive** (required companion): §7 seam; `constitution` CLI presence checked by `lifecycle init`.
- **Enforcement engines** (optional consumers): Conductor reads `approval-state.json`/`deviation.json` (harness mvp-plan Phase 2); any CI can do the same (`lifecycle status`/`guard` exit codes).
- **Superpowers** (optional companion, no dependency): execution-discipline skills; our stage skills reference it only by suggestion, never requirement.

## 10. Configuration — `lifecycle.yml` (repo root)

```yaml
schemaVersion: 1
openspec: { package: "@fission-ai/openspec", version: "1.5.0" }   # pinned
planGranularity: medium          # coarse | medium | fine — sizes plan.md Steps
sourceTracking: { type: github-issue, repo: kentra-io/kafka-dq }  # matches constitution.yml vocabulary
changeNaming: "<issue-number>-<slug>"                             # 042-user-auth
runtimes: [claude-code, cursor, codex]                            # skill fan-out targets
```

Versioned like `constitution.yml` (unknown `schemaVersion` ⇒ refuse). `changeNaming` + `sourceTracking` give the single join key (issue number) across change folders, ADR `source` fields, engine run-state, and telemetry. *(Gemini CLI dropped from default runtimes — Google EOL 2026-06-18.)*

## 11. Deferred — explicitly not in v1

| Item | Why |
|---|---|
| Async-validation scenario tag | Conform to stock OpenSpec first — harness `roadmap-ideas.md` |
| Living-spec prose synthesis (readable narrative over the folded specs) | The fold is deterministic and sufficient; narrative is a human-docs concern (harness-deferred) |
| Deterministic cross-change merge (beyond §6.4 policy) | Rare under one-change-per-capability; OpenSpec's judgment model + guard suffices |
| `chore`/`question` profiles; per-project stage overrides | Feature + bug cover v1 (harness scope) |
| Brownfield living-spec extraction | Greenfield-first, mirroring the constitution primitive |
| Conductor-MCP tool surface | Harness Phase 2; this primitive only owns the records it reads |

## 12. Open items — build-time spikes (not blockers)

1. **OpenSpec schema-subsystem hardening** — the custom-schema layer is ~5 months old with open edge-case bugs (#731, #1212, #1222): pin, write conformance fixtures for our schema against the pinned version, re-verify on every bump.
2. **Archive-order determinism** — confirm `openspec archive` fold order is fully specified (same-day archives, tie-breaks) or fix ordering in `lifecycle guard`'s replay definition (archive-folder name = date-prefix + name is the presumed total order).
3. **`plan.md` in the schema DAG** — verify a renamed terminal artifact (replacing `tasks.md`) is fully supported by `openspec validate`/`archive` in a custom schema, or keep the filename `tasks.md` with our template (cosmetic fallback).
4. **Repo/CLI naming** — `spec-lifecycle`/`lifecycle` are working names; decide before publishing to the `kentra-io` org.
5. **Skill fan-out targets** — check whether Cursor's `.agents/skills/` support makes the `.cursor/skills/` target redundant (also flagged for the constitution primitive's shared package).

## 13. Research provenance

- Harness `references/sdd-framework-research-2026-07.md` (2026-07-02): four-agent evaluation (Superpowers v6.1.0, OpenSpec v1.5.0, Spec-Kit v0.12.4 re-eval, bespoke baseline + field scan), consolidated R1–R9 scorecard, and the composite decision this spec implements. Key verified facts: OpenSpec's deterministic single-change fold + maintainer-endorsed custom-schema path (#557/#536); Spec-Kit eliminated (core workflow engine = orchestration collision; no safe customize-and-upgrade path); Superpowers = execution disciplines + companion-plugin shape, no artifact spine; no framework supplies gates/contracts/records.
- Harness `tasks/planning-module-handoff.md`: decision ledger P0–P6 and the session trail (2026-07-02).
- [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution): the sibling primitive whose conventions (three layers, tool-only writes, guard, consent boundary, managed pointer blocks, `*.yml` config shape) this spec deliberately mirrors.
