# `spec-lifecycle` — Design Specification

*Version: v1 draft. Generated: 2026-07-02. Revised: 2026-07-03 (approval-authority framing; artifact-tree clarification; schema → `kentra-spec-lifecycle`; naming resolved) then **2026-07-03 (Option B: pure-Go rebuild — OpenSpec is now a format we conform to, not a runtime we run; research errata folded in)** then **2026-07-07 (§4.2/§6.2/§9.1 addendum — the "execution handoff" the harness `orchestration` module's §5.5 needs: an optional structured validation contract on a milestone, a Steps checkbox-tracking convention + `archive` tasks-completion gate, and a new `lifecycle apply` verb surfacing milestones + contracts as JSON — all three additive, backward compatible, shipped in this same revision).** Status: **DESIGN — pending user review.***

*A standalone SDD primitive: a staged, gated, spec-driven issue lifecycle. It adopts the **OpenSpec on-disk convention** — directory layout, delta grammar, fold semantics — but **reimplements the whole mechanism in pure Go**; there is no OpenSpec runtime, no Node, no shell-out. OpenSpec-the-tool is a **format we stay byte-compatible with** (so a kentra repo's `openspec/` tree still reads as an OpenSpec repo, and we could re-adopt their tooling later), not a dependency we execute. **File-based gate records** are the canonical interface to any external enforcement engine. Companion primitive to [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution) (the governance substrate). Consumed by the kentra harness, but — like the constitution primitive — framework-portable and not harness-bound.*

*Decision provenance: the harness's `references/sdd-framework-research-2026-07.md` (four-agent framework evaluation, 2026-07-02) and `tasks/planning-module-handoff.md` (decision ledger P0–P6). Naming settled (§12.4): repo/CLI stay framework-neutral (`spec-lifecycle`/`lifecycle`); the format-compatible schema is kentra-branded (`kentra-spec-lifecycle`); the opinionated methodology composing this primitive + the constitution is `kentra-sdlc` (harness doc). The **pure-Go decision (Option B)** reverses the earlier "OpenSpec as artifact runtime" call (P0) after research showed the runtime we'd depend on is young, `[experimental]` in the exact subsystem we need, carries live silent-data-loss bugs (#1246), and exposes no stable programmatic fold — see [implementation-plan.md](./implementation-plan.md) §0–§2.*

---

## 0. Terminology (locked)

- **Change** — one unit of work moving through the lifecycle: one GitHub issue ↔ one change folder (`openspec/changes/<name>/`, OpenSpec-format layout). "Issue's spec-folder" in older harness docs = the change folder.
- **Stage** — a lifecycle phase producing one approved artifact set: `refine`, `design`, `plan` (features); `repro`, `fix` (bugs).
- **Gate** — an approval boundary between stages, recorded durably in `approval-state.json`. Gates are **records, not enforcement**: this primitive writes them; an engine (Conductor) or CI reads and blocks. Same capability line as the constitution primitive.
- **Approval authority** — whoever holds the `approve` permission at a gate. **Human by default**, but the gate is a *pluggable consent boundary* — a tool-written record behind a permission, not a hardcoded human prompt — so progressive autonomy narrows human involvement without changing the mechanism. The trajectory: human everywhere → human only where it matters most (constitution amendments, requirements) → trusted policy for low-risk changes. "Human"/"you" below means "the approval authority, human by default."
- **Living spec** — `openspec/specs/<capability>/spec.md`: the current-state behavior contract of the whole system, produced by folding archived change deltas. The functional twin of `constitution.md`.
- **Delta** — a structured spec change (`## ADDED/MODIFIED/REMOVED/RENAMED Requirements` with `### Requirement:` / `#### Scenario:` blocks) inside a change folder. Deltas are the **event log** of the living spec.
- **The OpenSpec format** — the on-disk convention we conform to: the `openspec/` directory layout, the delta grammar (H2 op headers, `### Requirement:`, `#### Scenario:`, RFC-2119 `SHALL`/`MUST`), and the fold semantics (keyed-by-requirement-name apply, fixed op order). **We own a pure-Go implementation of all of it (§2, §6.1); we do not run OpenSpec.**
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
| Fidelity check | `constitution guard` | `lifecycle guard` (replay + digest chain, §6.3) |
| Event grain rationale | Decisions are sparse, discrete, citable (`ADR-id`) | Requirements are a dense corpus; block-level ops are the right grain |

Both are **single static Go binaries with no language-runtime dependency** — a property Option B restores for this primitive (the earlier OpenSpec-runtime design would have broken it).

## 2. Position in the stack

```
  Obra Superpowers (optional companion plugin)        ← execution disciplines: TDD,
      no dependency from this primitive                  systematic-debugging, subagents
  ─────────────────────────────────────────────
  spec-lifecycle (THIS) — single static Go binary     ← stages, gates, records, schema,
      owns the whole OpenSpec-format engine:              bug flow, validation contracts,
      parse · validate · fold · render · archive         AND the format mechanism itself
  ─────────────────────────────────────────────
  OpenSpec on-disk FORMAT (convention, NOT a runtime)  ← directory layout, delta grammar,
      byte-compatible; no Node, no shell-out             fold semantics — reimplemented in Go
  ─────────────────────────────────────────────
  adr-sourced-constitution (sibling primitive)         ← ADR log, constitution.md, plan-gate
      runtime CLI seam: `lifecycle` shells to it           (a process seam, not a Go import)
  Conductor / CI (external)                            ← reads approval-state.json /
                                                          deviation.json; ENFORCES
```

**What "OpenSpec-format-compatible" means and costs.** We commit to three things and reimplement each in Go (implementation-plan.md §2):

1. **Directory layout** — `openspec/{specs/<cap>/spec.md, changes/<name>/{proposal.md, design.md, tasks.md, specs/<cap>/spec.md}, changes/archive/<name>/, config.yaml, schemas/kentra-spec-lifecycle/}`. We keep the directory named `openspec/` deliberately: it *is* the convention, and preserving it keeps the tree re-adoptable by the real tool later. Filenames `proposal.md`/`tasks.md` are load-bearing in the format (§4).
2. **Delta grammar** — `## ADDED|MODIFIED|REMOVED|RENAMED Requirements` (H2), `### Requirement: <name>` (H3), `#### Scenario: <name>` (H4), GIVEN/WHEN/THEN scenario prose (convention), RFC-2119 with `SHALL`/`MUST` load-bearing. Our Go parser accepts exactly this grammar.
3. **Fold semantics** — apply a change's delta ops to the living spec keyed by requirement name, in the fixed op order RENAMED→REMOVED→MODIFIED→ADDED. Our Go fold is deterministic and byte-stable.

**What we drop by not running OpenSpec:** the Node ≥20.19 runtime, the `@fission-ai/openspec` npm install (and the claudebox stanza for it), OpenSpec's `[experimental]` custom-schema loader, its command-generation (which our schema never drove), and the class of failure where an *external* young runtime silently changes fold behavior under us (#1246 silent scenario-loss, #409/#1192 no archive total-order). **What we gain:** a pure static binary (parity with the constitution), an in-process deterministic fold — which makes the replay guard a *true* from-empty recompute (§6.3), not just a digest workaround — and full control over conflict handling. **What we lose:** automatic parity if OpenSpec's grammar evolves upstream. We accept this: we pin to the v1.5.0 grammar, format drift becomes our explicit choice, and a static conformance corpus (implementation-plan.md §9) proves byte-compatibility without a runtime.

Everything novel — gates, records, contracts, bug flow, replay guard, constitution seam — was always this primitive's. Option B simply moves the *format plumbing* (folders, grammar, fold) from "someone else's runtime we call" to "our Go code." **Ejection is now trivial by construction:** all canonical state is plain files in the consuming repo, and we already own every operation over them.

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
 PLAN     → tasks.md (milestones +            ══ GATE 3: plan approved
   │         validation contracts;                (constitution plan-gate re-runs)
   ▼         constitution plan-gate re-runs)
 [execution — out of scope]
   │
   ▼
 ARCHIVE  → lifecycle archive folds the delta into openspec/specs/;
            change folder moves to changes/archive/<name>/; ledger record appended
```

**Three stages, three gates. No distinct `tasks` stage** — step breakdown lives inside `tasks.md` milestones, sized by `planGranularity` (§10). Each stage runs in a fresh agent session; the previous stage's approved artifacts are the entire input ("the artifact is the interface").

### 3.2 Collapse rule (small work)

The refine stage MAY propose skipping `design` (small, local, architecturally inert work). The skip is approved by the human **at gate 1** and recorded in that gate's approval entry (`"designSkipped": true`). A skipped design does not skip the constitution gate — the plan-gate still runs at gate 3. Mirrors the bug flow's built-in collapse (§8).

### 3.3 Gate mechanics (per gate)

1. Agent produces/revises the stage artifacts in the change folder.
2. Deterministic checks, **all in-process (`lifecycle`), no external runtime:**
   - `lifecycle validate --stage <s>` — delta-grammar validation (our Go parser, §6.1) **and** custom-artifact structure (proposal frontmatter/issue-ref; design NFR-discharge section; `tasks.md` milestone + validation-contract format).
   - at gates 2 and 3 — the constitution primitive's **plan-gate** skill, emitting `deviation.json` into the change folder (each finding cites an `ADR-id`); then `constitution deviation validate <path>` (§7).
3. Agent surfaces artifacts + any deviations conversationally. Every deviation resolves **conform or amend** before approval (amend = the constitution primitive's consent flow).
4. The **approval authority** approves conversationally → `lifecycle approve --stage <s>` (§9.1) writes the gate entry: artifact hashes, `constitutionHash`, deviation reference, approver, timestamp. (Human by default; whatever holds the `approve` permission is the authority — this is the seam for progressive autonomy, §0.)
5. **Enforcement is external:** in Phase 1 (no engine) the approval authority honors the records; Conductor later hard-blocks on them. This primitive never blocks.

**Constitution gate placement — twice, by design:** at design (where architecture violations bite) and at plan (final pre-execution re-check against the *current* constitution — which may have changed at gate 2 via accepted ADRs). Cheap (Haiku-class), so redundancy is free insurance.

## 4. Artifacts & the `kentra-spec-lifecycle` schema

The schema is **owned by `lifecycle` natively** — the artifact set, its `requires:` DAG, and its templates are compiled into the Go binary (embedded via `go:embed`) and are the authority. On `init`, `lifecycle` also writes an `openspec/schemas/kentra-spec-lifecycle/{schema.yaml, templates/*.md}` descriptor for **format-compatibility and documentation** (so the tree stays re-adoptable and human-readable), but nothing at runtime reads that descriptor back — we do **not** depend on OpenSpec's `[experimental]` schema loader. Stage ordering and the DAG are enforced by **our gate records and `lifecycle validate`**, never assumed from a schema interpreter. The DAG is evaluated against *approved gate records*, so it is **skip-aware**: a refine entry carrying `designSkipped:true` satisfies the `design` node's downstream `requires:` edge, and `plan` validates directly against the approved `specs` delta (§3.2, §5).

**Where artifacts live — two trees, two axes.** The change folder is the *transaction* (one issue); `openspec/specs/` is the *projection* (all capabilities). `specs/` appears in both — the one thing worth internalizing:

```
openspec/
  specs/                        ← PROJECT-level living spec, grouped by CAPABILITY
    <capability>/spec.md            (permanent; the fold target; the projection)
  changes/
    042-user-auth/              ← ISSUE-level: everything here belongs to ONE issue
      proposal.md  design.md  tasks.md   ← issue-level singletons
      specs/                    ← the DELTA — grouped by capability, scoped to THIS issue
        auth/spec.md                (ADDED / MODIFIED / REMOVED blocks)
    archive/042-user-auth/      ← same shape, frozen (the event log)
```

`proposal.md` / `design.md` / `tasks.md` are **issue-level singletons**. The change-folder `specs/` delta is also issue-scoped, but organized by capability because one issue may touch several. On archive, each delta folds into the matching project-level `openspec/specs/<capability>/spec.md`. **Capability = the axis of the product; issue = the axis of the work; the delta's capability headers are the join.**

*(Archive folders keep whatever OpenSpec-style name the change had; the on-disk `YYYY-MM-DD-` date prefix, if present, is **cosmetic** — the authoritative total order is the ledger's monotonic `seq`, §6.3. This is a deliberate departure forced by errata: no date-derived order is reliable.)*

Artifact DAG:

| Artifact | Stage | Content | Divergence from stock OpenSpec |
|---|---|---|---|
| `proposal.md` | refine | Why / What changes / Impact; issue ref in frontmatter (§10); may propose design-skip or a new capability | none (template tuned) |
| `specs/<capability>/spec.md` | refine | **The requirements artifact**: delta blocks, RFC 2119 requirements + GIVEN/WHEN/THEN scenarios. **Functional and non-functional merged** (§4.1) | none — same delta grammar |
| `design.md` | design | Technical design; explicit **NFR-discharge section** (how each declared NFR is met); **ADR proposals** (§7) | content conventions only |
| `tasks.md` | plan | Milestones + validation contracts (§4.2) | **filename kept** (load-bearing in the format); only the *template content* diverges (structured contracts vs unstructured checkboxes) |

**Filename note (errata, was §12.3).** The earlier draft renamed `tasks.md` → `plan.md`. That is dropped: `tasks.md` (and `proposal.md`) are load-bearing filenames in the OpenSpec format — a renamed terminal artifact makes the archive completion-gate silently no-op in the real tool, and keeping the name preserves format-compatibility for zero cost. The design intent ("structured validation contracts, not checkboxes") is filename-independent and lives entirely in the template. Because we own the parser now, our `lifecycle validate` *enforces* the milestone/contract structure inside `tasks.md` — something the real `openspec validate` never did.

### 4.1 NFR routing rule (format-conformant)

NFRs are requirements (harness decision P2). Placement follows OpenSpec's own line — *the spec is a behavior contract*:

| NFR kind | Home |
|---|---|
| Measurable, behavior-observable ("SHALL sustain 10k msg/s") | ordinary requirement block in the **spec delta**, scenarios included — merged with functional |
| Cross-cutting / project-wide invariant (stack, security posture, whether continuous perf-testing exists) | **constitution ADR** |
| Internal quality with no observable behavior | **`design.md`** |

Design MUST account for every declared NFR (the discharge section). Perf/benchmark validation is asynchronous by policy (P2) — codebase tests run alongside development, not per-milestone. *(A scenario-level async-validation tag was considered and deferred: harness `roadmap-ideas.md`.)*

### 4.2 `tasks.md` — milestone format

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

Planning owns contracts; execution consumes them (harness planning.md §9). The verifier grades against the contract, never a moving target. `lifecycle validate` (§6.1) checks these headings are present and structured — the parser is ours, so this is enforceable, not advisory.

**Addendum (2026-07-07, harness `orchestration.md` §5.5's "execution handoff") — two additive, opt-in extensions, both backward compatible: a plan authored before this addendum still validates unchanged.**

- **Steps checkbox tracking.** A Steps line may carry a `[ ]`/`[x]` checkbox right after the number — `1. [ ] <step>` / `1. [x] <step>` — GFM task-list convention grafted onto the existing ordered list. A bare `1. <step>` (no brackets) is still perfectly valid and simply untracked, exactly as every Steps line behaved before this addendum. The moment any step in a milestone opts into tracking, `lifecycle archive` (§6.2) refuses to archive the change until every tracked step in it is checked — the **tasks-completion gate**.
- **Structured validation contract.** A milestone's Validation contract MAY additionally carry a fenced ` ```contract ` block (YAML) alongside its free-text bullets:

  ```yaml
  check: <a single executable acceptance-check command, run from the project root>
  criteria: <plain-language acceptance criteria, for the advisory/human reviewer>
  paths:
    - <repo-relative glob this milestone's diff is confined to>
  ```

  All three fields (`check`, `criteria`, at least one `paths` entry) are required when the block is present at all; `paths` entries must be repo-relative (no leading `/`, no `..`) — the **diff-confined-paths declaration** an execution engine grades a milestone's diff against. `lifecycle validate --stage plan` enforces the block is well-formed WHEN PRESENT; its total absence is not an error. `lifecycle apply <change> --format json` (§9.1) is how an execution engine reads every milestone's Steps + contract back out as data, never by parsing markdown itself.

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
      "status": "approved",            // approved | rejected (pending is DERIVED, never persisted)
      "designSkipped": false,
      "artifacts": { "proposal.md": "sha256-…", "specs/auth/spec.md": "sha256-…" },
      "constitutionHash": "sha256-…",  // lifecycle's recompute at approval time (authoritative)
      "deviationConstitutionHash": null,// hash stamped into deviation.json; set at gates 2/3, null otherwise; differs ⇒ constitution moved mid-gate → both kept + warn (§7.5)
      "deviationRef": null,             // path of deviation.json when a gate ran the plan-gate
      "approvedBy": "jan",
      "approvedAt": "2026-07-02T14:00:00Z",
      "notes": ""
    }
  ]
}
```

Write path (errata-pinned, implementation-plan.md §2.6):
- **Hashed files** are resolved from the schema's `generates:` glob for the stage (handles multi-capability refine); hashes are computed, never hand-typed (tool-only writes).
- **Two constitution hashes:** `constitutionHash` is `lifecycle`'s own recompute at approval time (authoritative); `deviationConstitutionHash` is the value the plan-gate stamped into `deviation.json` (gates 2/3 only, else `null`). A mismatch means the constitution changed via an accepted ADR at gate 2 — both are persisted and a warning surfaces (§7.5).
- **`status`** persists only `approved | rejected`; `lifecycle status` *derives* `pending` for a stage with no entry.
- **Skips** record no `design` entry — the skip is `designSkipped:true` on the *refine* entry; consumers treat "absent + upstream `designSkipped`" as legitimately-absent.
- **`--reject` appends**; consumers take the **latest entry per stage-name**.
- **Post-gate drift:** because change-folder artifacts stay freely editable after approval, `lifecycle status`/`guard` re-hash each recorded gate's artifacts against the stored hash and flag drift — the tamper-evidence the event-sourcing spine implies.

Only `lifecycle approve` writes it. Consent model mirrors the constitution primitive: skills do **not** pre-grant the mutating verb — the agent-harness permission boundary is the v1 consent checkpoint (`consentPolicy: strict`, §10).

## 6. Living spec, archiving & the replay guard

### 6.1 The format engine — our Go implementation of parse · validate · fold · render

This is the heart of Option B. `lifecycle` owns, in-process:

- **Parse** — read a delta (`specs/<cap>/spec.md` in a change folder) into a structured op set (ADDED/MODIFIED/REMOVED/RENAMED × requirements × scenarios), and read a living `spec.md` into a requirement set.
- **Validate** — the delta-grammar rules (header shape, `### Requirement:`/`#### Scenario:` structure, RFC-2119 `SHALL`/`MUST`) plus our custom-artifact structure (§3.3, §4.2).
- **Fold (`buildUpdatedSpec`)** — apply a change's ops to a capability's requirement set, keyed by requirement name, in the fixed order RENAMED→REMOVED→MODIFIED→ADDED. Deterministic, order-independent of folder names.
- **Render** — serialize a folded requirement set back to `spec.md` markdown, byte-stable.

All four are pinned to the OpenSpec v1.5.0 grammar and proven against a static conformance corpus (real OpenSpec-format fixtures + expected fold outputs, captured once — implementation-plan.md §9). No runtime, no shell-out, no network.

### 6.2 `lifecycle archive` — fold + record

On completion, `lifecycle archive <change>`:
1. **Gate-check** — refuse unless `approval-state.json` shows every required stage `approved` (soft in Phase 1 → hard under Conductor/CI).
2. **Tasks-completion gate** (addendum, §4.2) — refuse if `tasks.md` declares any checkbox-tracked Steps item that is not checked. A `tasks.md` with no tracked steps at all, or no `tasks.md`, is never gated by this. Overridable with `--force-incomplete-tasks` (recorded on every ledger record, same posture as the two overrides below).
3. **Conflict-check** — if the change's `MODIFIED`/`REMOVED`/`RENAMED` targets a requirement also touched by another in-flight change, surface it loudly. Because we own the fold, a genuine conflict is **detected by construction**, not silently dropped (contrast OpenSpec #1246).
4. **Record pre-image** digests of affected capability specs.
5. **Fold** the delta into `openspec/specs/<cap>/spec.md` (our engine, §6.1) and relocate the folder to `changes/archive/<name>/`.
6. **Record post-image** digests + append the monotonic-`seq` ledger record(s).

The archive directory is thereafter **append-only** — it is the event log; archived folders are never edited (guard-checked, §6.3).

### 6.3 Replay guard (`lifecycle guard`) — now a *true* replay, plus a digest chain

Owning the fold in-process makes the fidelity check strictly stronger than the earlier digest-only design. `lifecycle guard` runs, deterministically, no LLM:

1. **Immutability** — content-hash `changes/archive/**` and match each ledger record's `archiveManifestSha`. Drift ⇒ `archive_mutated`.
2. **Projection fidelity (digest chain, fast path)** — for each capability, the live `openspec/specs/<cap>/spec.md` hash must equal that capability's **latest** `postImageSha` in the ledger; each record's `preImageSha` must equal the prior record's `postImageSha` (`chain_break`).
3. **From-empty replay (deep check, now buildable)** — recompute `fold(all archived deltas, in ledger `seq` order, from empty)` **in-process** and diff against live `openspec/specs/`. This was deferred in the runtime design (OpenSpec's fold had no callable, safe entrypoint); in pure Go it is cheap, deterministic, and the gold-standard fidelity proof. Mismatch ⇒ `projection_drift`.

The ledger's monotonic `seq` is the **authoritative total order** (on-disk folder dates are cosmetic — errata). Exit contract mirrors the constitution's guard: `0` clean / `1` violation / `2` could-not-run; `--format json` with a typed violation enum (`archive_mutated | projection_drift | chain_break | ledger_missing`). Run in CI and inside `lifecycle archive` (post-fold self-check).

Ledger record shape:
```json
{ "seq": 7, "change": "042-user-auth", "issue": "kentra-io/kafka-dq#42",
  "capability": "auth", "preImageSha": "sha256-…", "postImageSha": "sha256-…",
  "deltaOps": [{"op":"ADDED","requirement":"Password login"}],
  "archiveManifestSha": "sha256-…" }
```

### 6.4 Capability taxonomy

- Capabilities are **created on demand by refine**: a delta targeting a new capability flags it in `proposal.md`; the human approves it at gate 1. Greenfield starts empty and accumulates (same philosophy as the ADR log).
- Kebab-case nouns (convention; not hard-errored — contrast the constitution's fixed category vocabulary). Split/merge/rename of capabilities MUST be expressed **as a change** (RENAMED/REMOVED/ADDED delta ops) — never by hand-editing `openspec/specs/` — so replayability survives restructuring. *(Capability-**folder** restructuring at folder grain — as opposed to requirement grain — needs bespoke fold logic and is deferred past v1, §11.)*
- Size discipline: warn when a capability's `spec.md` exceeds ~200 lines (mirror of the constitution's projection warning) — a split candidate.

### 6.5 Cross-change conflicts

Policy first: **prefer one in-flight change per capability** (the harness's GitHub claim-mutex makes this cheap to honor; refine SHOULD flag capability overlap with in-flight changes in the proposal). When conflicts still occur at archive time, the §6.2 conflict-check surfaces them and resolution is agent-assisted, but MUST produce a revised, re-validated delta in the *unarchived* change — the archive log stays untouched, and `lifecycle guard` (incl. from-empty replay) must pass afterward.

## 7. Constitution seam (design ↔ ADR)

Unchanged by Option B — the constitution seam was always a **runtime CLI process boundary**, not a Go import, and stays one.

1. **Context in:** the design (and plan) stages load `constitution/constitution.md` — via the constitution primitive's skills. *(The `openspec/config.yaml` `context:` block is written for format-compat, but injection is driven by our stage skills reading the constitution directly, not by an OpenSpec runtime.)*
2. **Proposals out:** design drafts **ADR proposals as files in the change folder** (`adr-proposals/*.md`, MADR body shape). Proposals are ephemeral — they never enter `constitution/adr/` (the primitive's rule: the accept IS the write).
3. **Consent at gate 2:** the approval authority reviews the design *and each proposed ADR individually* (HARD RULE — per-ADR explicit consent, never bundled into blanket design approval). Constitution amendments are the **last thing to automate** (§0 trajectory): this per-ADR consent boundary is where human judgment persists longest. Each accepted proposal is written via `constitution adr new --body-file …` (source-ref = the issue), which auto-`regen`s `constitution.md`.
4. **Timing consequence (deliberate):** the constitution updates **eagerly at design-approval**; the living spec updates **at completion (archive)**. Plan and execution therefore always validate against an already-current constitution. The decisions-lead-behavior asymmetry is correct: a decision governs the work; the behavior contract reflects *delivered* work.
5. **Validation:** the gate's plan-gate step emits `deviation.json` **into the change folder** (skills pass `--out <changefolder>/deviation.json`); before writing a gate-2/3 entry, `lifecycle approve` shells out to **`constitution deviation validate <path>`** (exit 0/1/2; checks schema, every `adrId` cites an *active* ADR, `constitutionHash` freshness — constitution ADR-0009) and surfaces the result. Deviations resolve conform-or-amend before approval (§3.3). If `lifecycle`'s recomputed `constitutionHash` differs from the `deviation.json` value (constitution changed via an accepted ADR at gate 2), record both (`constitutionHash` + `deviationConstitutionHash`, §5) and warn.

## 8. Bug flow (minimal schema)

Bugs get a change folder too — uniform folder-per-issue; records need a home — but a compressed profile:

- `proposal.md` = the repro description + pointer to the **failing test**, which is *both* the requirement artifact *and* the validation contract (done = test passes + no regressions). Repro-first is mandatory; not reproducible ⇒ back to the human (`Needs Input`).
- **No spec delta by default** — a bug is by definition a failure to meet already-specced behavior; fixing it changes no contract. **If the repro reveals mis-specced behavior**, the fix is spec-affecting: add a delta, and gate it like a feature refine.
- `design` skipped by default; `tasks.md` optional (usually a single milestone whose contract is the repro test).
- **Promotion hatch** (unchanged from harness planning.md §11): if the fix spans architecture or would require a constitutional deviation, promote the change into the full feature flow (insert design + plan stages; same folder). A promoted bug's `gates[]` may mix `repro`/`fix` and `design`/`plan` names — `status`/`guard`/`archive` gate-checks key off the *change type* recorded at intake, not a fixed stage list.
- Archiving a delta-less change is pure archival — `lifecycle archive` records the ledger entry with an empty `deltaOps` and no fold; guard's replay is unaffected (no delta to replay).

## 9. The three layers

Mirrors the constitution primitive's shape.

### 9.1 Layer 1 — CORE: the `lifecycle` CLI

Go, single static binary, sibling of `constitution`, **no external language runtime**. Shares three small frozen internals with the constitution — `atomicwrite`, the managed-block/scaffold engine, and the skill fan-out — which are **copied** into this repo's `internal/` (not extracted into a shared library: both primitives stay standalone and the constitution gains no dependency — implementation-plan.md §2.12). Deterministic, no LLM. v1 verbs (6), plus one addendum verb (2026-07-07, §4.2/§6.2's "execution handoff"):

| Verb | Does |
|---|---|
| `lifecycle init` | Scaffold: create the `openspec/` tree, write the `kentra-spec-lifecycle` schema descriptor into `openspec/schemas/`, write `lifecycle.yml`, seed `openspec/config.yaml`, fan out skills, write managed AGENTS.md/CLAUDE.md pointer blocks (constitution-style markers). Preflights the `constitution` binary. |
| `lifecycle validate --stage <s> [--format json]` | Delta-grammar validation (our Go parser, §6.1) + custom-artifact structure (§4.2, incl. the optional structured contract block). **Read-only, deterministic.** Stage skills run it as the §3.3 gate pre-check; `approve` re-runs the same code path before writing a gate entry. |
| `lifecycle approve --stage <s> [--reject] [--notes …] [--design-skip] [--approve]` | Compute artifact hashes + `constitutionHash`; at gates 2/3 run `constitution deviation validate`; append the gate entry to `approval-state.json` |
| `lifecycle status [--change <n>] [--format json]` | Report gate state across change folders (reads records only); derive `pending`; flag post-gate artifact drift |
| `lifecycle archive <change> [--force-incomplete-tasks]` | §6.2: gate-check → tasks-completion gate → conflict-check → pre-image digests → fold + relocate → post-image digests + ledger append |
| `lifecycle guard [--format json]` | §6.3: archive-immutability manifest + digest chain + from-empty replay. Exit 0/1/2 |
| `lifecycle apply <change> [--format json]` | Addendum: read-only projection of `tasks.md`'s milestones — Steps (with checkbox state) + the optional structured contract — as text or JSON. Refuses (exit 1) if the same plan-stage validation `validate --stage plan` runs finds an error, so the data it surfaces is always already trustworthy. The machine-readable surface the harness `orchestration` module's read_plan step consumes. |

`lifecycle validate` (delta-grammar + custom-artifact structure, §6.1) is **exposed as a read-only checkpoint verb** (review decision 2026-07-04): the stage skills run it before surfacing artifacts (§3.3), and `approve` re-runs the same code path so a gate entry can never be written over an invalid artifact. **Nothing is delegated to an external tool** — parse, validate, fold, archive are all ours.

### 9.2 Layer 2 — AGENT SURFACE: skills

Agent-agnostic (SKILL.md standard), fanned out like the constitution's: `lifecycle-refine`, `lifecycle-design`, `lifecycle-plan` (stage conduct: fresh-session discipline, artifact templates, gate mechanics incl. running the constitution plan-gate at 2/3), `lifecycle-bug` (repro-first flow), `lifecycle-archive` (conflict-check → archive → guard). Skills call `lifecycle validate`/`approve`/`archive` — never a raw OpenSpec command (there is no OpenSpec runtime to call). Mutating verbs (`approve`, `archive`, `constitution adr new`) are never pre-granted in `allowed-tools`. Fan-out maps `runtimes:` (§10) to trees: `claude-code → .claude/skills/`, `cursor → .cursor/skills/`, `codex → .agents/skills/` (the cross-agent AGENTS.md convention). Whether `.agents/` subsumes `.cursor/` is open (§12.5).

### 9.3 Layer 3 — INTEGRATIONS

- **OpenSpec format** (conformed-to, not run): the `openspec/` layout + delta grammar + schema descriptor. No dependency, no version pin on a runtime — we own the engine. The format is pinned to the v1.5.0 grammar and proven by the conformance corpus (implementation-plan.md §9).
- **Constitution primitive** (required companion): §7 seam; `constitution` CLI presence + version checked by `lifecycle init`; `deviation validate` called at gates 2/3.
- **Enforcement engines** (optional consumers): Conductor reads `approval-state.json`/`deviation.json` (harness mvp-plan Phase 2); any CI can do the same (`lifecycle status`/`guard` exit codes).
- **Superpowers** (optional companion, no dependency): execution-discipline skills; our stage skills reference it only by suggestion, never requirement.

## 10. Configuration — `lifecycle.yml` (repo root)

```yaml
schemaVersion: 1
specFormat: { convention: openspec, grammar: "1.5.0" }   # the on-disk format we conform to (NOT a runtime)
constitution: { version: "0.1.x" }                       # companion primitive pin (deviation.json contract)
consentPolicy: strict                                    # strict | off — parity with constitution.yml
planGranularity: medium                                  # coarse | medium | fine — sizes tasks.md Steps
sourceTracking: { type: github-issue, repo: kentra-io/kafka-dq }  # matches constitution.yml vocabulary
changeNaming: "<issue-number>-<slug>"                             # 042-user-auth
runtimes: [claude-code, cursor, codex]                            # skill fan-out targets
```

Versioned like `constitution.yml` (unknown `schemaVersion` ⇒ refuse; no migration machinery, no Viper). Note the shift from the runtime design: there is **no `openspec: { package, version }` runtime pin** — `specFormat.grammar` records which grammar version our engine targets, a documentation/conformance anchor, not an installed dependency. `constitution: { version }` pins the companion so the `deviation.json` contract can't silently drift across constitution releases. `changeNaming` + `sourceTracking` give the single join key (issue number) across change folders, ADR `source` fields, engine run-state, and telemetry. `lifecycle init` warns if `lifecycle.yml` and `constitution.yml` `sourceTracking` disagree. *(Gemini CLI dropped from default runtimes — Google EOL 2026-06-18.)*

## 11. Deferred — explicitly not in v1

| Item | Why |
|---|---|
| Async-validation scenario tag | Conform to the base grammar first — harness `roadmap-ideas.md` |
| Living-spec prose synthesis (readable narrative over the folded specs) | The fold is deterministic and sufficient; narrative is a human-docs concern (harness-deferred) |
| Deterministic cross-change merge (beyond §6.5 policy) | Rare under one-change-per-capability; conflict-check + guard suffices |
| Capability-**folder** restructuring fold (split/merge/rename at folder grain) | Requirement-grain RENAMED/REMOVED/ADDED covers v1; folder-grain needs bespoke fold logic |
| `chore`/`question` profiles; per-project stage overrides | Feature + bug cover v1 (harness scope) |
| Brownfield living-spec extraction | Greenfield-first, mirroring the constitution primitive |
| Conductor-MCP tool surface | Harness Phase 2; this primitive only owns the records it reads |

## 12. Open items — build-time spikes (not blockers)

1. **Format conformance corpus** — capture a static set of real OpenSpec-format change folders + their expected fold outputs (generated once from the reference tool), and prove our Go parser/fold/render is byte-identical. This is how we hold format-compat without a runtime; re-verify if we ever choose to track a newer grammar. *(Replaces the old "pin the runtime + fixtures against the young schema subsystem" spike — the subsystem risk is gone with the runtime.)*
2. **Fold determinism edge cases** — same-name requirements across capabilities, RENAMED-then-MODIFIED in one delta, MODIFIED of a nonexistent requirement: enumerate and lock behavior in the engine (our decision now, not OpenSpec's). The `seq` ledger makes archive order authoritative; confirm no code path re-derives order from folder names.
3. **`constitution deviation validate` contract** — the verb is marked "hidden" in the constitution CLI; confirm it's a stable contract for cross-primitive use, or promote it (or re-implement the light JSON check in `lifecycle`).
4. **Naming — RESOLVED 2026-07-03.** Repo/CLI stay framework-neutral (`spec-lifecycle` / `lifecycle`). The format-compatible schema is kentra-branded (**`kentra-spec-lifecycle`**). The opinionated methodology composing this primitive + the constitution is **`kentra-sdlc`** (harness `kentra-sdlc.md`). No naming question remains for this primitive.
5. **Skill fan-out targets** — check whether Cursor's `.agents/skills/` support makes the `.cursor/skills/` target redundant (also flagged for the constitution primitive's copied helpers).

## 13. Research provenance

- Harness `references/sdd-framework-research-2026-07.md` (2026-07-02): four-agent evaluation (Superpowers v6.1.0, OpenSpec v1.5.0, Spec-Kit v0.12.4 re-eval, bespoke baseline + field scan), consolidated R1–R9 scorecard. Note: that doc's P0 conclusion ("OpenSpec as artifact runtime") is **superseded by Option B** — see this spec's header and implementation-plan.md §0–§2. What survives intact from the research: the *format* is the right convention to adopt (delta grammar, fold model, two-tree layout); Spec-Kit eliminated; Superpowers = execution disciplines with no artifact spine; no framework supplies gates/contracts/records.
- Second research pass (2026-07-03, 4-agent + empirical build/run of `@fission-ai/openspec@1.5.0` at tag `v1.5.0`/commit `546224e`): surfaced the load-bearing negatives that motivated Option B — the deterministic fold engine has zero upstream callers (no safe programmatic entrypoint), the custom-schema subsystem is `[experimental]` with open bugs, archive has no native total-order (#409/#1192), and #1246 silently drops overlapping scenarios. Owning the engine in Go removes all four as *external* risks.
- Harness `tasks/planning-module-handoff.md`: decision ledger P0–P6 and the session trail.
- [`adr-sourced-constitution`](https://github.com/kentra-io/adr-sourced-constitution): the sibling primitive whose conventions (three layers, tool-only writes, guard, consent boundary, managed pointer blocks, `*.yml` config shape, pure-static-binary/no-runtime posture) this spec deliberately mirrors.
