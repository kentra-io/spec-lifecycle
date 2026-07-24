# Flip spec-lifecycle to a YAML source of truth — Design

## Context

`spec-lifecycle` today conforms to the OpenSpec on-disk *format*: living
specs and per-change deltas are hand-authored markdown in the OpenSpec delta
grammar, and correctness is proven against a checked-in OpenSpec conformance
corpus (constitution ADR-0001, ADR-0002). The refine-approved proposal
(`proposal.md`, issue #7) drops OpenSpec conformance entirely in favour of
**primitive-family consistency**: the same YAML-source / published-JSON-Schema
/ rendered-markdown-projection shape the sibling primitives
(`milestoned-plan-dag`, `adr-sourced-constitution`) already use.

Three gate-1-approved spec deltas define the behaviour this design must
realise:

- **spec-format** (new) — YAML is the authoritative source for the living
  spec and every delta; a JSON Schema describes that YAML; markdown is a
  deterministic read-only projection carrying derived kebab-slugs (folds
  in #5); the fold + from-empty-replay engine is retargeted onto the YAML.
- **plan-integration** (new) — the plan-stage gate delegates to
  `milestoned-plan-dag validate`; the archive step-completion gate reads
  done-states from `milestoned-plan-dag resolve`; there is no `lifecycle
  apply` verb and the schema descriptor drops the `tasks.md` template and
  the `tasks` artifact.
- **status-reporting** (modified) — `lifecycle status`'s machine output is
  YAML (`--format yaml`); `--format json` is dropped (folds in #2).

Constraints carried in: the tool MUST stay a single static Go binary with no
external language runtime (ADR-0001's surviving half); the archive ledger's
monotonic `seq` + true from-empty replay differentiator MUST be preserved
(ADR-0003, retargeted to YAML, unchanged in principle). This change also
**supersedes pending change #4** (`execution-handoff`) — the very `apply`
verb / contract block / checkbox archive gate it would have added are
relocated to `milestoned-plan-dag` here.

Bootstrap note: this change's *own* spec deltas remain authored in the
markdown OpenSpec format being replaced — the change builds the YAML format;
it is not authored in it.

## Goals / Non-Goals

**Goals:**

- A single owned YAML data model for the living spec and for spec deltas,
  described by a published JSON Schema, validated in-process (no runtime).
- A deterministic, byte-stable markdown projection with derived
  requirement/scenario kebab-slugs.
- The delta/fold/from-empty-replay engine retargeted onto the structured
  YAML, preserving the ADR-0003 replay-guard differentiator.
- A CLI (process-boundary) integration with `milestoned-plan-dag` for the
  plan gate and the archive completion gate — no Go import.
- Removal of the `lifecycle apply` verb, the `tasks.md` template, and the
  `tasks` artifact from the schema descriptor.
- `lifecycle status --format yaml`, `--format json` removed.
- Two constitution amendments (supersede ADR-0001, ADR-0002); ADR-0003
  retargeted-but-unchanged.

**Non-Goals:**

- The downstream `agent-orchestration` repoint (its `plan.py` ingestion +
  `apply`/`archive`/`status` shell-outs moving to plan-dag) — a separate
  change in that repo (a noted dependency, not this work).
- A markdown→YAML migrator. Clean break, dev mode: existing markdown living
  specs are re-authored, not migrated.
- Renaming the `openspec/` directory or the reference cleanup (tracked in
  #6).
- Any change to gate mechanics, the constitution seam, or `approval-state`
  schema.

## Decisions

### D1 — Living-spec YAML: an ordered sequence of requirements, slugs derived not stored

A capability's living spec is `openspec/specs/<capability>/spec.yaml` (source)
projecting to a co-located read-only `spec.md`. Shape:

```yaml
capability: status-reporting
purpose: <one-paragraph purpose>
requirements:
  - name: Machine-readable capability warnings
    text: |
      The system SHALL ...
    scenarios:
      - name: YAML status output with an oversized capability
        given: [ "openspec/specs/auth/spec.md exceeds the threshold" ]
        when:  [ "lifecycle status --format yaml runs" ]
        then:  [ "the capabilityWarnings sequence contains an entry for auth" ]
```

`requirements` and `scenarios` are **sequences, not maps** — author order is
the projection order and must be preserved; YAML maps are unordered.
`given`/`when`/`then` are sequences of clauses (a scenario can have several).
Kebab-slugs are **derived at render time** from `name`, never stored in the
source — a stored slug is a second source of truth that can drift.

*Alternatives considered.* (a) Requirement `name` as a YAML map key —
rejected: loses author order and forbids duplicate-name detection at parse
time. (b) Store the slug in source — rejected: duplicates the truth; the slug
is a pure function of the name.

### D2 — Delta YAML: op-tagged entries, requirement-keyed, fixed op order

A per-change delta is `specs/<capability>/spec.yaml` in the change folder:

```yaml
capability: status-reporting
deltas:
  - op: MODIFIED
    requirement: { name: ..., text: ..., scenarios: [ ... ] }
  - op: ADDED
    requirement: { name: ..., text: ..., scenarios: [ ... ] }
  - op: REMOVED
    requirement: { name: ... }
  - op: RENAMED
    from: <old name>
    to:   <new name>
```

The four ops mirror the OpenSpec ADDED/MODIFIED/REMOVED/RENAMED verbs the
current grammar already carries, so the fold semantics are a straight
retarget rather than a redesign. The fold applies deltas **keyed by
requirement name, in the fixed op order** the engine already uses, so the
requirement-keyed replay guard (ADR-0003) carries over unchanged.

*Alternative considered.* RFC-6902 / JSON-Patch pointer deltas — rejected:
not human-authorable, and it discards the requirement-name key the fold and
replay guard are built on.

### D3 — On-disk layout: check in the markdown projection as the review surface

`spec.yaml` is source; `spec.md` is a generated, read-only projection checked
in beside it and regenerated by the renderer. The projection carries a
managed header marker (constitution-style) declaring it generated and
read-only. Rationale: the markdown diff is the human review surface in PRs
and the git history; regenerating-and-committing matches `milestoned-plan-dag`
(which also renders markdown) and keeps the projection guard meaningful
(replay recomputes the projection and compares).

*Alternative considered.* Generate markdown on demand, never commit —
rejected: loses the reviewable diff and the guard's compare target.

### D4 — JSON Schema: two schemas, embedded Go validator, no runtime

Publish two JSON Schema (draft 2020-12) documents under
`openspec/schemas/kentra-spec-lifecycle/`: `living-spec.schema.json` and
`spec-delta.schema.json` (the shapes differ — the delta carries op tags).
`lifecycle validate` validates the YAML against them with an **embedded Go
JSON-Schema validator** compiled into the binary — no external process, no
Node, preserving the surviving half of ADR-0001. The schema is *published*
(so external tools can validate the YAML) but validation at gate time is
in-process and owned.

*Alternative considered.* Shell out to a JSON-Schema CLI — rejected: violates
the single-static-binary / no-language-runtime constraint.

### D5 — Projection determinism + slug derivation

Slug = lowercase; each maximal run of non-`[a-z0-9]` → single `-`; trim
leading/trailing `-`. Emitted in the markdown as a stable anchor/marker for
every requirement and scenario. Byte-stability rules: fixed key/section
order (purpose → requirements in source order → scenarios in source order),
LF line endings, no trailing whitespace, single trailing newline. This is
what makes "the same YAML always renders byte-identical markdown" (spec-format
scenario) and the from-empty replay compare exact. Determinism is thus a
**spec-observable behaviour** (D5 realises a spec scenario), not a design-only
NFR — see NFR Discharge.

### D6 — plan-integration is a CLI process boundary to milestoned-plan-dag

`validate --stage plan` shells out to `milestoned-plan-dag validate <plan>`
over the change's plan YAML (`openspec/changes/<change>/plan.yaml`) and passes
the gate only on a valid report; the archive step-completion gate shells out
to `milestoned-plan-dag resolve` and refuses to archive while any milestone
is not done, naming it. **No Go import** — a CLI/YAML boundary, matching the
constitution-primitive seam posture (both primitives stay standalone). `lifecycle
init` preflights the `milestoned-plan-dag` binary the same way it preflights
`constitution`.

*Alternative considered.* Vendor `milestoned-plan-dag` as a Go module import —
rejected: couples the two binaries' build/versioning and breaks the
"standalone primitives" constitution posture (ADR-0003 of the *harness*
family); the CLI boundary keeps them independently releasable.

### D7 — Drop `lifecycle apply`, the `tasks.md` template, and the `tasks` artifact

The machine plan surface is `milestoned-plan-dag resolve`, so `lifecycle
apply` is removed from the CLI, and the schema descriptor
(`schema.yaml`) drops both the `tasks` artifact entry and the top-level
`apply:` block. The `design` artifact's `requires:` chain terminates at
`design` (plan is now plan-dag's concern, gated but not a lifecycle
artifact). This is the concrete relocation that supersedes change #4.

### D8 — `lifecycle status --format yaml`; drop `--format json`

`status` emits a YAML document; oversized-capability warnings surface as a
`capabilityWarnings` sequence (each entry: capability + line count), matching
`--format text`. `--format json` is removed. Consistency with the
YAML-everywhere family posture (folds in #2).

### D9 — Constitution: supersede ADR-0001 and ADR-0002; ADR-0003 survives

- **ADR-0001** (reimplement the OpenSpec format natively; single static Go
  binary, no Node) is *bundled*: the single-static-binary / no-language-runtime
  half survives, the OpenSpec-on-disk-format-conformance half does not. Since
  an accepted ADR body is frozen, the only way to change it is a superseding
  ADR that restates the surviving constraint and drops the OpenSpec
  conformance — `adr-proposals/ADR-own-yaml-format.md`.
- **ADR-0002** (prove format compat with a static OpenSpec conformance corpus)
  no longer has a corpus to prove: correctness is proven by checked-in
  YAML→markdown golden projection fixtures verified byte-identical, plus the
  from-empty replay for fold correctness — `adr-proposals/ADR-projection-golden-tests.md`.
- **ADR-0003** (archive `seq` is the sole order; from-empty replay) is
  format-agnostic in wording and survives **unchanged** — the replay now
  recomputes the folded YAML and compares against the live YAML's projection;
  the principle is intact, so **no amendment is proposed** for it.

## NFR Discharge

Per spec-lifecycle.md §4.1's routing rule, a design-homed NFR is an
internal-quality concern with **no externally observable behaviour**. This
change declares none of that kind:

- **Determinism of the projection** (byte-identical render; exact replay
  compare) is **behaviour-observable** — it is stated as a spec scenario in
  `specs/spec-format/spec.md` ("the same YAML always renders byte-identical
  markdown") and discharged by D5. Home: spec delta, not design.
- **Single static Go binary, no external language runtime** is a
  **cross-cutting project-wide invariant** — its home is a constitution ADR
  (the surviving half of ADR-0001, carried into
  `adr-proposals/ADR-own-yaml-format.md`), and it constrains D4's embedded
  validator choice. Home: constitution ADR, not design.
- **No `milestoned-plan-dag` Go import (CLI boundary)** is likewise a
  cross-cutting standalone-primitive invariant, satisfied by D6.

**Design-homed NFRs: (none declared.)**

## ADR proposals

Each is reviewed and consented to **individually** at gate 2 — never bundled
into blanket design approval (spec-lifecycle.md §7 item 3). Both are
supersessions written via `constitution supersede`:

- `adr-proposals/ADR-own-yaml-format.md` — **supersedes ADR-0001.** Own a
  native YAML spec format described by a published JSON Schema; drop OpenSpec
  on-disk conformance; keep the single-static-Go-binary / no-external-runtime
  constraint.
- `adr-proposals/ADR-projection-golden-tests.md` — **supersedes ADR-0002.**
  Prove correctness with checked-in YAML→markdown golden projection fixtures
  (byte-identical on every PR) plus from-empty replay, not an OpenSpec
  conformance corpus.

(No amendment is proposed for ADR-0003 — it survives unchanged, D9.)

## Risks / Trade-offs

- **One-way loss of OpenSpec interop** → accepted deliberately by the refine
  proposal; the family-consistency win (owned format, one shape across
  primitives) is the chosen trade. Mitigation: the `openspec/` layout name is
  kept for now (#6), so re-adoption isn't foreclosed.
- **CLI-boundary coupling to `milestoned-plan-dag`** (D6) → a version/flag
  drift between the two binaries can break the plan gate at runtime, not build
  time. Mitigation: `lifecycle init` preflights the binary; the gate surfaces
  plan-dag's own error verbatim; both are pinned in the family's submodule
  wiring.
- **Two-schema maintenance** (D4) → living-spec and delta schemas can drift
  apart. Mitigation: they share the requirement/scenario sub-shape by
  `$ref`; golden fixtures exercise both.
- **Bootstrap asymmetry** → this change's own deltas are markdown while it
  builds YAML. Mitigation: it is the last change authored in the old format;
  the archive fold of *this* change is the last markdown fold.
- **Constitution moves mid-gate** → accepting the two supersessions at gate 2
  changes the constitution before `approve`; handled by the seam's two-hash
  record + warn (spec-lifecycle.md §7.5) and by regenerating a clean
  `deviation.json` against the amended constitution before approval.
