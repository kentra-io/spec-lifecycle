<!-- Bootstrap note (design.md "Bootstrap asymmetry"): this change's OWN
     proposal/specs deltas are authored in the markdown OpenSpec format
     being replaced, and this tasks.md is graded by the CURRENT
     `lifecycle validate --stage plan` (four-label markdown milestones),
     not by milestoned-plan-dag — the plan-integration gate this change
     builds only governs FUTURE changes. Re-authoring the two live specs
     AND the archived deltas (001, 003) to YAML lands in Milestone 8 so
     `lifecycle guard`'s from-empty replay recomputes over a fully-YAML
     tree. Executing/archiving this change is out of the plan stage's
     scope (spec-lifecycle.md §3.1). -->

## Milestone 1: YAML data model, published JSON Schemas, embedded validator
**Goal** — Establish the owned YAML data model for the living spec and spec deltas, publish the two JSON Schemas that describe it, and validate that YAML in-process with an embedded pure-Go validator (design D1, D2, D4; constitution ADR-0004).
**Deliverables** —
- New `internal/spec` YAML types: a living-spec model (`capability`, `purpose`, ordered `requirements[]` each with ordered `scenarios[]` carrying `given`/`when`/`then` clause sequences — D1) and a delta model (op-tagged entries `ADDED`/`MODIFIED`/`REMOVED`/`RENAMED`, requirement-keyed, with `from`/`to` for renames — D2). Requirements/scenarios are sequences (author order preserved), slugs never stored.
- Two draft-2020-12 JSON Schemas under `openspec/schemas/kentra-spec-lifecycle/`: `living-spec.schema.json` and `spec-delta.schema.json`, sharing the requirement/scenario sub-shape via `$ref` (D4), embedded into the binary via `//go:embed` and installed by `lifecycle init`.
- An embedded pure-Go JSON-Schema validator (recommended dep: `github.com/santhosh-tekuri/jsonschema/v6`, no cgo, no external process) that validates a decoded YAML document against the schema and reports the offending JSON path on violation.
- `internal/config` specFormat vocabulary updated so the tool operates in YAML mode: the `Validate` hard-refusal of non-markdown conventions no longer blocks the YAML format (the `openspec/` on-disk layout name is deliberately kept — issue #6); `lifecycle.yml`/`config.Default()` seed the YAML format vocabulary.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `CGO_ENABLED=0 go build ./...` — succeeds; the result is a single static binary with no external language runtime (ADR-0004).
  - `go test ./internal/spec/... ./internal/schema/... ./internal/config/...` — passes, including new tests where a conforming spec-delta YAML validates clean and a schema-violating one is rejected with the offending path named.
  - `grep -R "santhosh-tekuri\|jsonschema" go.mod` — the JSON-Schema validator dependency is present and pure-Go (no `// indirect` cgo dep introduced).
  - Makes pass: spec-format scenario "YAML is validated against the published schema".
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Define the living-spec and delta YAML structs in `internal/spec/types.go` (ordered sequences; drop the byte-fidelity `Raw` field in favour of the structured model).
  2. Author `living-spec.schema.json` + `spec-delta.schema.json` (shared `$ref` sub-shape) and embed + install them.
  3. Add the pure-Go JSON-Schema validator dependency; wire a `spec.ValidateYAML(doc, schema)` that returns offending-path errors.
  4. Update `internal/config` specFormat validation to admit the YAML format while keeping the `openspec/` layout name.
  5. Unit tests: struct decode preserves order; conform vs. violate schema fixtures.

## Milestone 2: YAML parse + fold engine, refine-stage delta reading
**Goal** — Retarget the parse and fold engine to operate on the structured YAML — reading the living spec and each change delta from YAML and applying deltas keyed by requirement name in the fixed op order (design D1, D2).
**Deliverables** —
- `internal/spec` parse retargeted to `yaml.Unmarshal` into the M1 model (replacing the markdown header/regex scanners in `parse.go`/`delta.go`/`lines.go`), with duplicate-requirement-name detection at parse time (D1 alternative-rejected rationale).
- `Fold` retargeted to apply delta entries over the structured model, keyed by requirement name, in the fixed `RENAMED → REMOVED → MODIFIED → ADDED` order the engine already uses (D2) — the from-empty (`base == nil`) case synthesizes a structured empty living spec, not a markdown skeleton.
- `internal/validate/delta.go` refine-stage validation reads `specs/<capability>/spec.yaml` as the authoritative delta (via the M1 schema validate + parse), not any markdown file.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/spec/...` — fold tests pass: applying a delta exercising all four ops to a base living spec yields the expected folded model; a delta naming a duplicate requirement is rejected.
  - A refine-stage validation run over a change whose `specs/<cap>/spec.yaml` is well-formed passes, and one whose YAML is malformed fails naming the offending path — asserted by `go test ./internal/validate/...`.
  - Makes pass: spec-format scenario "a spec delta is authored and read as YAML" (the tool reads the YAML, not any markdown file, as the authoritative delta).
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Replace `ParseRequirementSet`/`ParseDelta` internals with YAML decode into the M1 structs; delete the now-dead markdown-scanning helpers in `lines.go`.
  2. Retarget `Fold` to the structured model, preserving the fixed op order and requirement-name key; structured empty base for the `nil` case.
  3. Port RFC-2119 (SHALL/MUST) and "≥1 scenario per added/modified requirement" content checks onto the YAML model.
  4. Point `internal/validate/delta.go` at `spec.yaml` (schema-validate then parse); update `ArtifactsForStage` refine globs to `specs/**/spec.yaml`.
  5. Update the spec-package unit tests (`parse_test`, `delta_test`, `fold_test`) to the YAML fixtures.

## Milestone 3: Deterministic markdown projection with derived slugs + golden fixtures
**Goal** — Render the living-spec YAML to a byte-stable, read-only markdown projection carrying a derived kebab-slug for every requirement and scenario, proven by checked-in golden fixtures (design D3, D5; constitution ADR-0005).
**Deliverables** —
- `internal/spec` render retargeted to emit markdown from the structured model with: a managed "generated / read-only, do not hand-edit" header marker (D3); the D5 slug algorithm (lowercase; each maximal run of non-`[a-z0-9]` → single `-`; trim leading/trailing `-`) emitted as a stable anchor/marker per requirement and per scenario; and the D5 byte-stability rules (fixed section/key order: purpose → requirements in source order → scenarios in source order; LF endings; no trailing whitespace; single trailing newline).
- Golden projection fixtures under `internal/spec/testdata/` (the OpenSpec `render_golden.md` replaced by source-YAML ↔ expected-markdown pairs) verified byte-identical, with a `-update` regen flag.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/spec/...` — golden projection test passes: a fixed living-spec YAML renders byte-identical to its checked-in golden markdown, and rendering the same YAML twice is byte-identical.
  - The golden fixture for a requirement `New feature intake` with a scenario `Human confirms the drafted issue` shows derived slugs `new-feature-intake` and `human-confirms-the-drafted-issue` in the projection — asserted by that test.
  - Makes pass: spec-format scenario "the projection is regenerated and carries slugs".
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Implement the slug function + unit table test (edge cases: punctuation runs, leading/trailing separators, digits).
  2. Rewrite `render.go` to emit the projection from the model with the managed header and D5 byte-stability rules; emit slug markers per requirement/scenario.
  3. Add source-YAML ↔ golden-markdown fixture pairs and the golden harness (`-update` flag); assert byte-identity and render idempotence.
  4. Delete the OpenSpec round-trip/fixed-point tests made obsolete by the projection model (the byte-fidelity `parse(render(x))==x` contract no longer holds — projection is one-way).

## Milestone 4: From-empty replay guard on the spec YAML
**Goal** — Retarget `lifecycle guard`'s from-empty replay to recompute the fold over the archived YAML deltas and compare it against the live YAML's markdown projection, naming any divergence (design D9; constitution ADR-0003, retargeted-but-unchanged).
**Deliverables** —
- `internal/guard/replay.go` retargeted: fold from empty through each archived change's `specs/<cap>/spec.yaml` in ledger `seq` order, render the projection, and byte-compare against the live `openspec/specs/<cap>/spec.md` projection; report agreement or name the diverging capability (preserving the brownfield PreImage exemption).
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/guard/...` — from-empty replay tests pass over YAML fixtures: an archive ledger of YAML deltas folds to a model whose projection equals the live projection (agreement), and a tampered live projection is reported as divergence naming the capability.
  - `lifecycle guard` returns exit 0 on a fixture project whose live YAML matches the folded archived YAML deltas.
  - Makes pass: spec-format scenario "from-empty replay confirms the live spec". (Full-repo `lifecycle guard` green is gated on Milestone 8's re-authoring of this repo's own archived deltas.)
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Point `checkReplay` at `spec.yaml` archived deltas and the YAML parse/fold path.
  2. Compare the recomputed projection against the live `spec.md` projection; keep the brownfield first-record exemption.
  3. Update guard tests/fixtures to the YAML archive shape.

## Milestone 5: plan-integration — plan gate + archive gate delegate to milestoned-plan-dag
**Goal** — Delegate plan-stage validation and the archive step-completion gate to `milestoned-plan-dag` over the change's `plan.yaml`, a CLI/YAML process boundary with no Go import (design D6).
**Deliverables** —
- `internal/validate` plan-stage validation shells out to `milestoned-plan-dag validate openspec/changes/<change>/plan.yaml`, passing the gate only on a valid report and surfacing the plan primitive's error verbatim on an invalid one (replacing the `tasks.md`-parsing `validatePlan`).
- `internal/archive` step-completion gate reads milestone done-states from `milestoned-plan-dag resolve` (replacing `checkTasksComplete`'s `validate.ParseMilestones` call), refusing to archive while any milestone is not `done` and naming it; the `--force-incomplete-tasks` escape hatch is preserved.
- A `milestoned-plan-dag` binary locate/version helper + preflight in `lifecycle init` mirroring the `constitution` preflight (warn-not-fail; overridable via a `--plan-dag-bin` flag / env, testable via an injectable path), and the required binary documented alongside `constitution`.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/validate/... ./internal/archive/...` — with a stub `milestoned-plan-dag` on the injected path: a plan the stub reports invalid fails the plan gate surfacing the stub's message; a plan it reports valid passes; a `resolve` reporting an undone milestone makes `archive` refuse and name that milestone.
  - `go test ./internal/scaffold/...` — `lifecycle init` with an absent `milestoned-plan-dag` emits a warning (never an error), matching the constitution preflight.
  - Makes pass: plan-integration scenarios "an invalid plan fails the plan-stage gate", "a valid plan passes the plan-stage gate", and "archive is blocked by an outstanding milestone".
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Add `internal/plandag` (or equivalent) locate/version helper + injectable binary path, paralleling `internal/constitution`.
  2. Rewrite plan-stage validation to shell out to `milestoned-plan-dag validate <plan.yaml>` and surface its report.
  3. Rewrite the archive step-completion gate to shell out to `milestoned-plan-dag resolve` and refuse on undone milestones; keep `--force-incomplete-tasks`.
  4. Add `preflightPlanDAG` to `RunInit`; wire the override flag/env.
  5. Tests with a stub binary for both gates + the init preflight warning.

## Milestone 6: Retire `lifecycle apply`, the `tasks` artifact/template, and the `apply:` block
**Goal** — Remove the surfaces the plan-schema extraction relocates to `milestoned-plan-dag`: the `lifecycle apply` verb, the `tasks` artifact + `tasks.md` template, and the schema descriptor's `apply:` block, and retarget the descriptor's spec-delta documentation to YAML (design D7; supersedes change #4).
**Deliverables** —
- `cmd/lifecycle/apply.go` deleted and its registration removed from `cmd/lifecycle/main.go`; the now-unused `tasks.md`-parsing plan machinery (`internal/validate/tasks.go`, and `ParseMilestones`/`Milestone`/`Step`/`Contract` if no longer referenced after Milestone 5) removed.
- `internal/schema/schema.yaml` drops the `tasks` artifact entry, the top-level `apply:` block, and the `templates/tasks.md` file; the `design` artifact's `requires:` chain terminates at `design`; the `specs` artifact instruction + `templates/spec.md` are rewritten to document the YAML delta grammar (not the markdown `## ADDED/### Requirement:` grammar).
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `lifecycle apply anything` — fails as an unknown command (the verb is gone); `go build ./...` succeeds with no dangling references.
  - `grep -E "id: tasks|^apply:|template: tasks.md" internal/schema/schema.yaml` — returns nothing; `test ! -f internal/schema/templates/tasks.md`.
  - `go test ./internal/schema/...` — descriptor loads and declares no `tasks` artifact.
  - Makes pass: plan-integration scenario "no lifecycle apply verb or tasks artifact".
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Delete `apply.go` + its registration in `main.go`; delete `internal/validate/tasks.go` and any milestone-parsing types left unused after Milestone 5.
  2. Edit `schema.yaml`: remove the `tasks` artifact + `apply:` block; terminate the `design` requires-chain; rewrite the `specs` instruction to the YAML grammar.
  3. Remove `internal/schema/templates/tasks.md`; rewrite `templates/spec.md` as a YAML delta template.
  4. Update the CLI e2e testscripts that referenced `apply`/`tasks.md`.

## Milestone 7: `lifecycle status --format yaml` with capabilityWarnings; drop `--format json`
**Goal** — Make `lifecycle status`'s machine output YAML, surfacing oversized-capability warnings as a `capabilityWarnings` sequence, and remove the `--format json` option (design D8; folds in issue #2).
**Deliverables** —
- `cmd/lifecycle/status.go` accepts `--format yaml` and rejects `--format json` (`--format must be text|yaml`); the YAML machine output carries a `capabilityWarnings` sequence, each entry naming the capability and its line count, matching the same data as `--format text`.
- `internal/status` gains the oversized-capability warning derivation (it does not exist today — it must be implemented, reading the configured threshold and each live `openspec/specs/<cap>/spec.md` projection's line count).
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/status/... ./cmd/lifecycle/...` — passes, including: with a live spec exceeding the threshold, `lifecycle status --format yaml` emits a `capabilityWarnings` entry naming that capability and its current line count; `lifecycle status --format json` exits non-zero with a `must be text|yaml` message.
  - Makes pass: status-reporting scenario "YAML status output with an oversized capability".
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Implement oversized-capability detection in `internal/status` (threshold from config; per-capability line count).
  2. Add a YAML writer emitting the status document with `capabilityWarnings`; remove the JSON writer.
  3. Update `runStatus` format validation to `text|yaml`; update the status e2e testscripts.

## Milestone 8: Bootstrap cleanup — re-author to YAML, retire the OpenSpec corpus, thin the skill, docs
**Goal** — Complete the clean break: re-author this repo's own living specs and archived deltas to YAML, remove the retired OpenSpec conformance corpus, thin the `lifecycle-plan` skill toward plan-dag, and reconcile the docs — leaving the whole tree YAML and `lifecycle guard` green (design Non-Goals "clean break, no migrator"; constitution ADR-0005).
**Deliverables** —
- The two live specs (`openspec/specs/feature-intake`, `openspec/specs/status-reporting`) re-authored as `spec.yaml` source + regenerated read-only `spec.md` projection; the archived change deltas that fold into them (`openspec/changes/archive/001-*`, `003-*`) re-authored as `spec.yaml` so the from-empty replay recomputes over a fully-YAML tree.
- The OpenSpec conformance corpus removed: `testdata/conformance/` (cases + `manifest.json` + `regen.sh` + `README.md`) and the corpus tests in `internal/spec/conformance_test.go` deleted (ADR-0005 forbids reintroducing it).
- The `lifecycle-plan` skill thinned/retired in favour of `milestoned-plan-dag`'s `plan-author` skill (and re-fanned to `.claude`/`.cursor`/`.agents`); repo docs (README / any `openspec/`-format references) reconciled to the YAML-source model, with the `openspec/` directory rename left to issue #6.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `test ! -d testdata/conformance` and `go test ./...` — the corpus is gone and the whole suite passes.
  - `lifecycle guard` — exit 0: the from-empty replay over the re-authored YAML archived deltas agrees with the live YAML projections (Milestone 4's engine, now over this repo's real tree).
  - `lifecycle status --format yaml` runs clean over the re-authored `feature-intake` + `status-reporting` specs.
  - Re-affirms: spec-format scenarios "the projection is regenerated and carries slugs" and "from-empty replay confirms the live spec" on this repo's own specs.
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Re-author `feature-intake` + `status-reporting` living specs to `spec.yaml`; regenerate `spec.md`.
  2. Re-author the archived `001`/`003` deltas to `spec.yaml`; run `lifecycle guard` to confirm from-empty agreement.
  3. Delete `testdata/conformance/` and `internal/spec/conformance_test.go`.
  4. Thin/retire the `lifecycle-plan` skill toward `plan-author`; re-fan skills; reconcile README/docs to the YAML model.
