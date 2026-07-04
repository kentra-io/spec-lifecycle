# `spec-lifecycle` — Implementation Plan (v1)

*Generated: 2026-07-03. Status: **PLAN — pending user review.** Companion to [spec-lifecycle.md](./spec-lifecycle.md) (the design spec) and, in the harness repo, [mvp-plan.md](../mvp-plan.md) (Phase 1), [planning.md](../planning.md), and [kentra-sdlc.md](../kentra-sdlc.md). Produced from the spec plus a 4-agent parallel research run + empirical verification (2026-07-03; all OpenSpec-format claims verified against live source at tag `v1.5.0` = commit `546224e`, the npm registry, and by building the claudebox and running `@fission-ai/openspec@1.5.0` end-to-end as a **reference oracle** — see §15). Mirrors the shape of [`adr-sourced-constitution/implementation-plan.md`](../adr-sourced-constitution/implementation-plan.md).*

> **What this document is.** The sequenced, buildable plan for **v1** of the primitive. The spec decides *what it is*; this decides *what gets built, in what order, with which concrete stack*, pins every TBD the spec left open (§2), and records the errata the research surfaced (§1). Milestones carry validation contracts (Definition of Done); nothing is complete without proving it. Every decision here is an assumption the user can veto at review.

> **Option B (pure Go) — the decision this plan implements.** An earlier draft treated **OpenSpec (`@fission-ai/openspec`, a Node CLI) as a required runtime** that `lifecycle` shells out to for parse/validate/archive/fold. That is **reversed**. We build `spec-lifecycle` as a **single static Go binary that reimplements the OpenSpec on-disk *format*** (directory layout, delta grammar, fold semantics) — **no Node, no `openspec` install, no shell-out.** OpenSpec-the-tool is a **reference oracle** for a static conformance corpus, not a dependency we run. §0.5 scopes exactly what we own, what it costs, and what we drop.

---

## 0. v1 scope (proposed — vetoable at review)

| # | Question | Decision |
|---|---|---|
| **P1** | Integration surface | **Core + default only**: the `lifecycle` CLI (6 verbs — §4), the `kentra-spec-lifecycle` schema (natively owned, §2.2), Layer-2 skills, and the zero-extra-framework integration (`openspec/config.yaml` + managed AGENTS.md/CLAUDE.md pointer blocks + skills fan-out to claude-code/cursor/codex). **OpenSpec is a FORMAT we implement, not a runtime we depend on.** Constitution integration = folder + pointer + CLI shell-out (matches the constitution's own v1 default integration; a process seam, not a Go import). Superpowers = referenced-by-suggestion, never required. |
| **P2** | Distribution | **Full pipeline in v1**: GoReleaser → GitHub Releases + a second `lifecycle` cask in the existing `kentra-io/homebrew-tap`, `go install`, predictable linux artifact URL for claudebox `COPY`. **No claudebox OpenSpec/Node stanza** (dropped with the runtime) — the box gets a static Go binary, exactly like the constitution. |
| **P3** | Module / repo / binary | Repo **`github.com/kentra-io/spec-lifecycle`**; binary **`lifecycle`**. **No shared library** — the three small frozen helpers (atomicwrite, managed-block engine, skill fan-out) are **copied** into this repo's `internal/` so both primitives stay standalone and `adr-sourced-constitution` gains no dependency (§2.12). `kentra-io` org exists; the repo does not yet — creation is Milestone 0. |
| **P4** | The format engine (parse · validate · fold · render) | **Reimplement OpenSpec's parser/merge/render/archive in Go** as the authoritative implementation of the format. `lifecycle` owns the delta grammar, the deterministic fold (`buildUpdatedSpec`), spec rendering, and archive — plus the thing OpenSpec never had: an explicit total-order ledger with per-fold pre/post digests (§2.4/§2.5). Owning the fold in-process makes the replay guard a **true from-empty recompute** (§2.4), not a digest-only workaround. |
| **P5** | `tasks.md` filename | **Keep `tasks.md`** (load-bearing in the format), customize only its template/content (§1.1, §2.1). |

**Non-goals for v1** (spec §11 + P1): framework adapters, brownfield living-spec extraction, drift sweep, prose synthesis over the fold, the async-validation scenario tag (harness `roadmap-ideas.md`), deterministic cross-change auto-merge beyond the one-change-per-capability policy, capability-**folder** restructuring fold, Conductor-MCP surface, `chore`/`question` profiles.

---

## 0.5 Option B scope — what we own, what it costs, what we drop

**What "OpenSpec-format-compatible" commits us to** (three things, all reimplemented in Go):

| Surface | What we reimplement | Reference oracle | Est. LOC |
|---|---|---|---|
| **Delta/spec parser** | Read `## ADDED\|MODIFIED\|REMOVED\|RENAMED Requirements` (H2), `### Requirement:` (H3), `#### Scenario:` (H4), GIVEN/WHEN/THEN prose, RFC-2119 (`SHALL`/`MUST` load-bearing). Parse a living `spec.md` into a requirement set; parse a change delta into an op set. | OpenSpec `src/core/parser/*`, `src/core/validation/*` | ~250–350 |
| **Validator** | Delta-grammar structural rules + our custom-artifact structure (proposal frontmatter/issue-ref, design NFR-discharge section, `tasks.md` milestone+contract format). Precise error messages. | OpenSpec `openspec validate` behavior (captured, then bettered — we validate the custom artifacts it never did) | ~150–250 |
| **Fold (`buildUpdatedSpec`)** | Apply a change's ops to a capability requirement set, keyed by requirement name, fixed order RENAMED→REMOVED→MODIFIED→ADDED. Deterministic; conflicts detected, not silently dropped. | OpenSpec `src/core/archive.ts` `buildUpdatedSpec` | ~120–200 |
| **Renderer** | Serialize a folded requirement set back to `spec.md` markdown, byte-stable (inverse of the parser). | round-trip of OpenSpec output | ~100–150 |
| **Scaffolder (`init`)** | Create the `openspec/` tree, `config.yaml`, schema descriptor — natively (no `openspec init` delegation). | OpenSpec `openspec init` output shape | ~150 |
| **Archive** | Gate-check, conflict-check, fold + relocate, ledger append (§2.5). | OpenSpec `openspec archive` semantics (incl. `--skip-specs` behavior) | ~150 |

**Net new Go we own vs the runtime design: ~900–1400 LOC** of parser/fold/render/validate/scaffold. That is the whole cost. It is bounded, testable against a static oracle corpus, and — unlike the runtime — fully under our control.

**What we DROP** (was required under Option A):
- Node ≥20.19.0 + `@fission-ai/openspec@1.5.0` install (dev, CI, and the claudebox `Dockerfile` stanza + `OPENSPEC_TELEMETRY=0`).
- The `internal/openspec` exec wrapper, `openspec --version` exact-match enforcement, `openspec doctor`.
- The two-tier test strategy (fake-`openspec`-shim unit tier + real-openspec integration tier) → **one hermetic Go tier** + a static conformance corpus (§9).
- OpenSpec's `[experimental]` custom-schema loader (we own the schema natively — §2.2).
- The `openspec: { package, version }` runtime pin in `lifecycle.yml` (→ `specFormat: { convention, grammar }`, a documentation anchor — §2.10).

**What we GAIN:**
- **Pure static binary, no language runtime** — parity with the constitution; the "single static binary" story survives.
- **In-process deterministic fold** → the replay guard becomes a *true from-empty recompute* (§2.4), the gold-standard fidelity check that was un-buildable when the fold lived behind a Node CLI with no safe programmatic entrypoint.
- **Conflict handling by construction** — #1246's silent scenario-loss is impossible because our fold surfaces conflicts (§2.5).
- **No external-runtime risk class** — the young/`[experimental]` subsystem, archive-order ambiguity (#409/#1192), and version-bump breakage all disappear as *external* risks.

**What we LOSE (accepted):** automatic parity if OpenSpec's grammar evolves upstream. Mitigation: we pin to the v1.5.0 grammar, format drift becomes an explicit choice, and the conformance corpus (§9) proves byte-compatibility. The research explicitly sanctioned this path ("Ejection stays cheap because files are canonical — port the archive algorithm into a bespoke `lifecycle` CLI" — `references/sdd-framework-research-2026-07.md`).

---

## 1. Spec errata — corrections folded into the spec

Research (incl. building and running OpenSpec 1.5.0 as an oracle) found these. Under Option B most of them stop being *risks we manage around an external tool* and become *behavior we simply own correctly.* All are already applied to `spec-lifecycle.md`.

1. **`plan.md` rename is unsafe — keep `tasks.md`.** OpenSpec hardcodes `tasks.md`/`proposal.md` (`src/utils/task-progress.ts`, `src/core/archive.ts`); a renamed terminal artifact silently no-ops the archive completion-gate in the real tool. Keeping the filename preserves format-compatibility for zero cost; divergent content lives in the template, and — because we own the parser now — `lifecycle validate` *enforces* the milestone/contract structure OpenSpec never checked. (Closes old spec §12.3 — negative result.)
2. **"30-tool command generation" was stale and is now moot.** v1.5.0 had 11 workflow IDs, schema-agnostic, driven by profile config — never by our schema, and never something we ran. Reference removed from the spec; we do our own skill fan-out.
3. **No native archive total-order — we own it.** The `YYYY-MM-DD-<name>` folder prefix is cosmetic (never read back; same-day archives sort alphabetically — #409/#1192). Our ledger's monotonic `seq` is the authoritative order (§2.4).
4. **guard vs archive ordering split cleanly.** A **pre-archive gate/conflict check** lives inside `lifecycle archive` (§2.5); a **post-archive fold-fidelity guard** is `lifecycle guard` (§2.4). Both reworded in the spec.
5. **Constitution validation verb wired in.** `constitution deviation validate <path>` (exit 0/1/2; ADR-0009) is called before writing a gate-2/3 entry (§2.7).
6. **`lifecycle.yml` gains `constitution: { version }` + `consentPolicy`.** Independent release cadences ⇒ pin the companion so the `deviation.json` contract can't drift (§2.7/§2.10). (There is **no** `openspec` runtime pin anymore — §2.10.)
7. **`approval-state.json` write path pinned** (files hashed via `generates:` glob; two constitution hashes — `constitutionHash` + `deviationConstitutionHash`; `pending` derived not persisted; `designSkipped` sentinel; latest-entry-per-stage) — §2.6.
8. **Post-gate artifact-drift check.** Change-folder artifacts stay freely editable after a gate; `lifecycle status`/`guard` re-hash recorded gate artifacts to flag drift (§2.6).
9. **Bug flow mechanism.** Delta-less archival is native (`lifecycle archive` writes a ledger record with empty `deltaOps`, no fold) — no `openspec archive --skip-specs` shell-out needed. A promoted bug's `gates[]` mixes `repro`/`fix` + `design`/`plan`; gate-checks key off the intake change-type (§2.11).
10. **6 verbs** — `lifecycle archive` is a first-class verb (§2.5), and `lifecycle validate` is exposed as a read-only checkpoint verb (§2.3, review 2026-07-04).
11. **Capability-**folder** restructuring is bespoke fold logic** — deferred past v1 (§13); requirement-grain RENAMED/REMOVED/ADDED covers v1.
12. **Kebab-case capability naming is convention-only** (no enforcing verb) — stated explicitly (contrast the constitution's hard-errored category vocabulary).

---

## 2. Pinned decisions (spec TBDs + research)

### 2.1 Terminal artifact: `tasks.md`, custom template (spec §4)
Keep the format filenames `proposal.md` / `tasks.md`. Our `tasks.md` **template** carries the milestone + validation-contract format (spec §4.2) verbatim. `lifecycle validate` (§2.3) enforces the milestone/contract structure — enforceable now because the parser is ours.

### 2.2 The `kentra-spec-lifecycle` schema — natively owned (spec §4)
The schema (artifact set, `requires:` DAG `proposal → specs → design → tasks`, `apply.tracks: tasks`, templates) is **compiled into `lifecycle`** (`go:embed`) and is the authority. `lifecycle init` writes an `openspec/schemas/kentra-spec-lifecycle/{schema.yaml, templates/*.md}` descriptor for **format-compatibility + documentation** (keeps the tree re-adoptable and human-readable), but **no runtime reads it back** — we do not use OpenSpec's `[experimental]` schema loader. Stage ordering + the DAG are enforced by **our gate records and `lifecycle validate`**, never by a schema interpreter. The DAG is evaluated against approved gate records and is **skip-aware**: a `designSkipped:true` refine entry satisfies the `design` edge (spec §4/§3.2). (This removes the entire "young experimental subsystem" risk class.)

### 2.3 `lifecycle validate` — we own the parser now (spec §3.3; **reverses the Option-A "do not reimplement the parser"**)
Under the runtime design we deliberately did *not* reimplement the delta parser (it lived in OpenSpec, drift-prone to track). Under Option B **we own it** — it is the point. `lifecycle validate` implements, in Go:
- **Delta grammar:** `## ADDED|MODIFIED|REMOVED|RENAMED Requirements`, `### Requirement:`, `#### Scenario:` structure, RFC-2119 (`SHALL`/`MUST` load-bearing). Pinned to the v1.5.0 grammar; proven byte-compatible by the conformance corpus (§9).
- **Custom-artifact structure:** proposal frontmatter/issue-ref; design NFR-discharge section present; `tasks.md` milestone headings + validation-contract block present.
Single pure-Go call — no `openspec validate` shell-out. The parser is shared by validate, fold, and guard (one code path, one grammar definition). `lifecycle validate --stage <s>` is **exposed as a read-only verb** (review 2026-07-04): the stage skills invoke it as the §3.3 gate pre-check, and `approve` calls the same code path before writing a gate entry — validation and approval can never diverge.

### 2.4 The replay guard — digest chain **plus** true from-empty replay (spec §6.3; errata 3/4)
`lifecycle` keeps an **append-only ledger** (the total order OpenSpec lacks). On each archive (§2.5), append one record per affected capability:
```json
{ "seq": 7, "change": "042-user-auth", "issue": "kentra-io/kafka-dq#42",
  "capability": "auth", "preImageSha": "sha256-…", "postImageSha": "sha256-…",
  "deltaOps": [{"op":"ADDED","requirement":"Password login"}],
  "archiveManifestSha": "sha256-…" }
```
`seq` is the authoritative total order (folder dates cosmetic, errata 3). `lifecycle guard` is deterministic, no LLM, and now runs **three** checks:
1. **Immutability:** content-hash `changes/archive/**` and match each record's `archiveManifestSha`. Drift ⇒ `archive_mutated`.
2. **Projection fidelity (digest chain, fast path):** live `openspec/specs/<cap>/spec.md` hash == that capability's latest `postImageSha`; each record's `preImageSha` == prior record's `postImageSha` (`chain_break`).
3. **From-empty replay (deep check — buildable under Option B):** recompute `fold(all archived deltas, in `seq` order, from empty)` **in-process** and diff against live `openspec/specs/`. Mismatch ⇒ `projection_drift`. *(This was §13-deferred in the runtime design because OpenSpec's fold had no safe callable entrypoint; owning the Go fold makes it cheap and it becomes the gold-standard check.)*
Exit `0`/`1`/`2`; `--format json` with enum `archive_mutated | projection_drift | chain_break | ledger_missing`. Runs in CI and inside `lifecycle archive` (post-fold self-check).

### 2.5 `lifecycle archive` — the 5th verb, native fold (errata 4/9/10)
1. **Gate check:** refuse unless `approval-state.json` shows every required stage `approved` (feature: refine[+design unless skipped]+plan; bug: repro[+fix]). Soft in Phase 1 → hard under Conductor/CI.
2. **Conflict check:** if the change's `MODIFIED`/`REMOVED`/`RENAMED` targets a requirement also touched by another in-flight change, surface it loudly. Our fold **detects** the conflict by construction (no silent drop — the #1246 failure mode is structurally absent).
3. **Record pre-image** digests of affected capability specs.
4. **Fold** the delta into `openspec/specs/<cap>/spec.md` (our engine, §0.5) and **relocate** the folder to `changes/archive/<name>/`. Delta-less (bug) change: skip the fold, empty `deltaOps`.
5. **Record post-image** digests + append the ledger record(s) with the next `seq`; run guard's self-check.
The `lifecycle-archive` skill calls **`lifecycle archive`** — there is no `openspec archive` to call.

### 2.6 `approval-state.json` write path (spec §5; errata 7/8)
- **Files hashed:** `lifecycle approve --stage <s>` resolves the artifact's `generates:` glob for `<s>` from the (native) schema and hashes every matching file in the change folder (handles multi-capability refine). Deterministic, no LLM.
- **`status` enum:** persist only `approved | rejected`; `pending` is derived by `lifecycle status`.
- **Two constitution hashes:** persist `constitutionHash` (`lifecycle`'s recompute at approval — authoritative) and `deviationConstitutionHash` (the value stamped into `deviation.json`, gates 2/3 only, else `null`). A mismatch ⇒ the constitution moved via an accepted ADR at gate 2 — both kept, warn (§2.7; spec §5/§7.5).
- **Skips:** a skipped `design` records no `design` entry; the skip is `designSkipped:true` on the *refine* entry; consumers treat "absent + upstream `designSkipped`" as legitimately-absent.
- **Multiple entries per stage:** `--reject` appends; consumers take the latest entry per stage-name (highest `approvedAt`, tie-break array order).
- **Post-gate drift:** `status`/`guard` re-hash each recorded gate's artifacts vs the stored hash and flag drift (errata 8) — the tamper-evidence the event-sourcing spine implies.

### 2.7 Constitution seam (spec §7; errata 5/6) — unchanged by Option B
The seam was always a **runtime CLI process boundary**, not a Go import, and stays one.
- The gate's plan-gate step emits `deviation.json` **into the change folder** — skills pass `--out <changefolder>/deviation.json` (constitution defaults to `./`).
- Before writing a gate-2/3 entry, `lifecycle approve` shells out to **`constitution deviation validate <path>`** and surfaces the result. `lifecycle` recomputes `constitutionHash` at approval time; if it differs from the `deviation.json` value (constitution changed via an accepted ADR at gate 2), record both (`constitutionHash` + `deviationConstitutionHash`, §2.6) and warn.
- `lifecycle.yml` gains `constitution: { version }`; `lifecycle init` preflights the `constitution` binary's presence **and** version. *(Open: `constitution deviation validate` is "hidden" — confirm stable for cross-primitive use, or promote/duplicate. Spike §12.)*

### 2.8 Format conformance corpus — replaces runtime version-enforcement (spec §12.1)
There is **no `openspec` runtime to pin or enforce** (the old §2.8 in-box install + `openspec --version` exact-match + `OPENSPEC_TELEMETRY=0` + `openspec doctor` are all **deleted**). In their place: a **static conformance corpus** — a checked-in set of real OpenSpec-format change folders + their expected fold/render outputs, generated **once** from the reference tool at v1.5.0. Tests assert our Go parser/fold/render is byte-identical to the corpus. This is how we hold format-compatibility without a dependency; regenerate only if we ever choose to track a newer grammar (an explicit decision, not an upgrade treadmill).

### 2.9 `lifecycle init` — native scaffold (no `openspec init`)
Ordered, idempotent, all native:
1. If `openspec/` absent → create `openspec/{changes,specs}` + a seed `config.yaml` (we own the layout — no `openspec init` delegation).
2. If `openspec/schemas/kentra-spec-lifecycle/` absent → write it from embedded assets.
3. Set `config.yaml`'s top-level `schema:` key to `kentra-spec-lifecycle` (check/set — don't clobber user edits below it).
4. Seed `config.yaml`'s `context:` block with the constitution injection **for format-compat**; actual context injection is driven by our stage skills reading `constitution/constitution.md` directly (spec §7.1).
5. Preflight the `constitution` binary (presence + version, §2.7); fan out skills; write managed AGENTS.md/CLAUDE.md blocks.
**Ownership line (in the spec):** `lifecycle` owns the entire `openspec/` tree, `lifecycle.yml`, every `approval-state.json`, and the archive ledger. Nothing else writes there.

### 2.10 `lifecycle.yml` (spec §10)
Replace the runtime pin with a format anchor: `specFormat: { convention: openspec, grammar: "1.5.0" }` (documentation/conformance, **not** an installed dependency). Add `consentPolicy: strict | off` (parity with `constitution.yml`) and `constitution: { version }` (§2.7). Versioned struct (`schemaVersion: 1`; unknown ⇒ refuse; no migration machinery; no Viper) — mirror the constitution's config loader. `sourceTracking` keeps its `repo:` field; `init` warns if it disagrees with `constitution.yml`.

### 2.11 Bug flow (spec §8; errata 9)
Native delta-less archival: `lifecycle archive` skips the fold and writes a ledger record with empty `deltaOps` for a bug with no delta — unless the repro revealed mis-specced behavior (promotion → full feature flow, same folder). A promoted bug's `gates[]` may mix `repro`/`fix` + `design`/`plan`; `status`/`guard`/`archive` gate-checks key off the *change type* recorded at intake, not a fixed stage list.

### 2.12 Shared helpers — **copy, don't couple** (spec §9.1; user decision 2026-07-03)
Both primitives stay **fully standalone**: `adr-sourced-constitution` gains **no new dependency** and is not modified; there is **no shared library to maintain**. Options weighed and rejected: (A) extract a `cli-kit` module both import — makes the constitution depend on a maintained lib (vetoed); (B) import a promoted-public constitution package — forces a public Go API + version-locks lifecycle's build (vetoed); (D) adopt an OSS lib — the one obvious atomic-write lib (`google/renameio`) exports nothing on Windows (a build target; constitution already rejected it), and managed-block/skill-fanout is bespoke marker logic with no library equivalent.

**Decision: (C) copy the three frozen helpers into `spec-lifecycle`'s `internal/`.** Small (~200–300 LOC total), already fuzz/crash-tested, *done*:
- **`atomicwrite`** — verbatim from `adr-sourced-constitution/internal/atomicwrite` (temp-in-same-dir + `os.Rename`, `MoveFileEx` on Windows), tests included.
- **managed-block + drift-state engine** — from `internal/scaffold/{block.go,state.go}`, marker text → `BEGIN/END spec-lifecycle v1`, `.state` path repointed.
- **skill fan-out** — from the constitution's `go:embed` fan-out to `.claude/`/`.agents/`/`.cursor/`.
Guard/manifest, config, and the whole **format engine** are hand-written fresh (they share no logic with the constitution). **The only cross-repo dependency is the loose runtime CLI seam:** `lifecycle` shells to the installed `constitution` binary (§2.7).

### 2.13 Dogfood
`spec-lifecycle` **plans itself**: once the CLI works, run `lifecycle init` on this repo and drive the remaining milestones (or a representative feature) as a real change through refine→design→plan→archive. First real exercise + living example — mirrors the constitution's §2.11.

---

## 3. Stack & repo layout (mirrors the constitution; pure Go, no external runtime)

| Concern | Choice | Notes |
|---|---|---|
| CLI framework | **urfave/cli v3** (v3.10.1) | exact mirror of constitution |
| YAML | **`go.yaml.in/yaml/v3`** (v3.0.4) | mirror; maintained successor to archived yaml.v3 |
| Atomic writes | own `internal/atomicwrite` — **copied** from constitution (§2.12) | frozen, tests included |
| Managed block / skill fan-out | own `internal/scaffold` — **copied** from constitution (§2.12) | marker text → `spec-lifecycle` |
| **Format engine** | own `internal/spec` — **parse · validate · fold · render**, pure Go (§0.5) | the heart of Option B; ~900–1400 LOC; pinned to v1.5.0 grammar |
| Delta/spec validation | **native** (`internal/spec` parser) | no `openspec validate`; one grammar code path |
| Fold / archive | **native fold** + own ledger (§2.4/2.5) | in-process, deterministic; no shell-out |
| Guard | fresh (immutability manifest + digest chain + **from-empty replay**) | shares the fold with the engine; no runtime |
| **External runtime dep** | **none** — single static Go binary | **no Node, no `openspec`**; the constitution's posture, restored |
| Go / CI | floor **1.25.0**; matrix {1.25,1.26}×{ubuntu,macos,windows}; golangci-lint **v2** (v2.12.2) via `golangci-lint-action@v9`; `actions/checkout@v6`+`setup-go@v6` | exact mirror of constitution — **Go-only CI, no npm step** |
| Testing | stdlib golden (`testdata/`, `-update`) · `rogpeppe/go-internal/testscript` black-box e2e · a **static format-conformance corpus** (§9) generated once from the reference tool | one hermetic tier; fuzz target = the parser/fold/render round-trip + the ledger digest comparator |
| Distribution | GoReleaser v2 (`homebrew_casks`), `project_name: lifecycle`, linux/darwin/windows×amd64/arm64, `-trimpath -s -w`+version ldflags, second cask in existing `kentra-io/homebrew-tap` | mirror; binary name `lifecycle` collides with nothing it creates |
| Version reporting | ldflags vars + `runtime/debug.ReadBuildInfo()` fallback | mirror |

```
spec-lifecycle/
  cmd/lifecycle/main.go            ← binary `lifecycle` (go install …/cmd/lifecycle@latest)
    version.go
  internal/
    spec/        THE FORMAT ENGINE — parser, validator, fold, renderer (pure Go)
    schema/      embedded kentra-spec-lifecycle schema.yaml + templates; install/verify
    approve/     stage resolution (generates: glob), hashing, approval-state.json writer
    archive/     gate-check, conflict-check, native fold, relocate, ledger append
    guard/       immutability manifest + digest chain + from-empty replay; JSON output
    validate/    thin wrapper: delta grammar (via internal/spec) + custom-artifact checks
    constitution/ thin exec wrapper: presence/version, deviation validate
    config/      lifecycle.yml load/validate (schemaVersion)
    status/      gate-state reporter (reads records)
    atomicwrite/ copied from constitution (frozen)
    scaffold/    init: create openspec/ tree, config.yaml wiring, managed blocks + skills fan-out (copied engine)
  skills/                          ← go:embed'ed Layer-2 bundles
    lifecycle-refine/SKILL.md  lifecycle-design/SKILL.md  lifecycle-plan/SKILL.md
    lifecycle-bug/SKILL.md     lifecycle-archive/SKILL.md
  templates/                       schema templates mirror + pointer-block texts
  testdata/
    conformance/                   ← static OpenSpec-format corpus + expected fold/render (§9)
  docs/                            CI example, claudebox COPY snippet (static binary, no runtime)
  .goreleaser.yaml  .golangci.yml  .github/workflows/{ci,release}.yml
  spec-lifecycle.md  implementation-plan.md  README.md  LICENSE
  openspec/                        ← the repo's own (dogfood, M7)
```

---

## 4. CLI surface (v1 — 6 verbs)

| Command | Behavior |
|---|---|
| `lifecycle init` | §2.9 native compose (create `openspec/` tree → schema descriptor → `config.yaml` `schema:`+`context:` → constitution preflight → skills + managed pointers). Idempotent; managed-block drift needs `--force`. |
| `lifecycle validate --stage <s> [--format json]` | §2.3: delta grammar (via `internal/spec`) + custom-artifact structure. Read-only, deterministic. Stage skills run it as the §3.3 gate pre-check; `approve` re-runs the same code path internally. |
| `lifecycle approve --stage <s> [--reject] [--design-skip] [--notes …] [--approve]` | §2.6: resolve `generates:` glob → hash artifacts + `constitutionHash`; at gates 2/3 run `constitution deviation validate`; append gate entry. Consent = permission boundary (§2.10). |
| `lifecycle status [--change <n>] [--format json]` | Gate state across change folders; derives `pending`; flags post-gate artifact drift. Read-only. |
| `lifecycle archive <change>` | §2.5: gate-check → conflict-check → pre-image → **native fold** + relocate → post-image + ledger append. |
| `lifecycle guard [--format json]` | §2.4: archive-immutability manifest + digest chain + **from-empty replay**. Exit 0/1/2. CI + inside `lifecycle archive`. |

`lifecycle validate` (§2.3, delta grammar + custom-artifact structure, via `internal/spec`) is **exposed as a read-only checkpoint verb** (review 2026-07-04): the stage skills run it before surfacing artifacts, and `approve` re-runs the same code path so no gate entry is written over an invalid artifact. **Nothing is shelled out to OpenSpec** — there is no OpenSpec runtime.

---

## 5. The `kentra-spec-lifecycle` schema & 6. Default integration & 7. Skills

- **Schema (§2.2):** natively owned, `go:embed`'ed, written by `init` as a format-compat descriptor. Artifacts `proposal/specs/design/tasks` with the DAG and tuned templates (tasks.md = milestone+validation-contract format). Enforced by `lifecycle validate` + gate records, not a schema interpreter.
- **Integration (§2.9):** `config.yaml` `schema:`+`context:`; managed AGENTS.md/CLAUDE.md pointer blocks (copied `internal/scaffold`, `BEGIN spec-lifecycle v1` markers); skills fan-out to `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`.
- **Skills (spec §9.2):** `lifecycle-refine`, `lifecycle-design`, `lifecycle-plan`, `lifecycle-bug`, `lifecycle-archive`. Fresh-session discipline; run `lifecycle validate` + (gates 2/3) the constitution plan-gate & `constitution deviation validate`; call `lifecycle approve`/`archive`. Mutating verbs never pre-granted in `allowed-tools`. Authoring the prompt bodies is real design work — its own milestone (M7).

---

## 8. Milestones — each with a validation contract

### M0 — Repo bootstrap + copy the frozen helpers
Create `kentra-io/spec-lifecycle`. **Copy** `atomicwrite` + `scaffold` (block/state) + skill-fanout from the constitution into this repo's `internal/`, swap marker text to `spec-lifecycle`, keep their tests (**`adr-sourced-constitution` is not modified**). `go.mod` (floor 1.25), `cmd/lifecycle` skeleton `--version`, **Go-only CI matrix** (no npm step), `.golangci.yml` v2, testscript harness, apply spec errata §1.
**DoD:** CI green on all legs; copied `atomicwrite`/`scaffold` tests pass unchanged; `go install …/cmd/lifecycle@<sha>` yields a `lifecycle` binary reporting build info.

### M1 — The format engine (`internal/spec`) + conformance corpus  ★ the Option-B heart
Pure-Go **parser · validator (delta grammar) · fold (`buildUpdatedSpec`) · renderer**. Capture the static conformance corpus (§2.8/§9) from the reference tool once; assert byte-identical parse→fold→render.
**DoD:** the corpus round-trips byte-identically (parse a real OpenSpec delta, fold, render → matches the oracle's `spec.md`); malformed deltas fail validation with precise messages; fold op-order (RENAMED→REMOVED→MODIFIED→ADDED) and conflict-detection are unit-proven; fuzz the parser/round-trip with no panics.

### M2 — Schema + custom-artifact validation
`internal/schema` (embed, install descriptor, verify), `internal/validate` (custom-artifact structure over the M1 parser: proposal frontmatter/issue-ref, design NFR-discharge, `tasks.md` milestone+contract format).
**DoD:** `lifecycle init` writes a schema descriptor; a well-formed change passes `lifecycle validate`; malformed custom artifacts fail with precise messages; the `requires:` DAG is documented and enforced via gate records (not a schema interpreter).

### M3 — Records: `lifecycle approve` + `status` + constitution seam
`internal/approve` (glob resolution, hashing, `constitutionHash`, consent gate `strict|off`, append), `internal/status` (derive `pending`, skip handling, drift detection), `internal/config`, `internal/constitution` wrapper (presence/version + `deviation validate`).
**DoD (testscript):** full gate sequence on a fixture change (refine→design→plan) writes correct hash-anchored entries on 3 OSes; `--design-skip` records the flag and omits a design entry; `strict` refuses without `--approve`; a post-gate edit to an approved artifact is flagged by `status`; a planted invalid `deviation.json` is caught via `constitution deviation validate`.

### M4 — `lifecycle archive` + the baseline ledger (native fold)
`internal/archive`: gate-check, conflict-check, pre/post digest capture, **native fold + relocate** (via M1), monotonic `seq` ledger append.
**DoD (testscript):** archiving with an un-approved gate is refused; a feature change folds (byte-matches the oracle for the same delta) and appends a correct ledger record; a bug change archives delta-less; two changes touching the same requirement trigger the conflict warning (no silent drop); ledger `seq` is monotonic and folder-date-independent.

### M5 — `lifecycle guard` (digest chain + from-empty replay)
`internal/guard`: immutability manifest over `changes/archive/**`, digest chain, **from-empty replay** (recompute fold via M1, diff vs live specs), exit 0/1/2, `--format json`.
**DoD (testscript):** clean repo passes; a hand-edit to an archived delta → `archive_mutated`; a hand-edit to `specs/<cap>/spec.md` → `projection_drift` (caught by both digest-chain and replay); a tampered ledger link → `chain_break`; from-empty replay of a multi-change history reproduces the live projection exactly; JSON validates; guard runs inside `lifecycle archive` and in CI.

### M6 — `lifecycle init` + integration wiring
`internal/scaffold`: the §2.9 native compose, `config.yaml` `schema:`/`context:` editing (idempotent), skills fan-out (`go:embed` → three trees), managed pointer blocks, constitution preflight.
**DoD:** on an empty scratch repo, one `init` yields a working setup (`lifecycle validate` clean, schema descriptor installed, `config.yaml` points at it, pointers + skills present, constitution preflight passes); re-run is byte-identical; managed-block hand-edit needs `--force`.

### M7 — Skill content + dogfood + live-agent spike
Author the five SKILL.md bodies; wire the constitution plan-gate + `deviation.json --out`; **dogfood:** `lifecycle init` on this repo, drive a real change end-to-end (§2.13). Run the live spike (§12).
**DoD (live sessions):** an agent conducts refine→design→plan, each gate approved **only** at the human permission prompt; a planted constitution deviation surfaces and blocks approval until conform/amend; a change archives and `lifecycle guard` passes; this repo's own `openspec/` is live with `guard` in its CI.

### M8 — Distribution
`.goreleaser.yaml` (second `lifecycle` cask into `kentra-io/homebrew-tap`; linux/darwin/windows×amd64/arm64; ldflags; checksums), promote `HOMEBREW_TAP_TOKEN` to an org secret, tag-triggered release workflow, `docs/` claudebox `COPY` snippet (**a static binary — no Node/OpenSpec install**).
**DoD:** `v0.1.0` cut by CI; `brew install kentra-io/tap/lifecycle` and `go install …/cmd/lifecycle@v0.1.0` both work; in a scratch container with **only the static binary**, `lifecycle init`→`approve`→`archive`→`guard` runs green.

### M9 — Harness acceptance (feeds mvp-plan Phase 1)
On the `kafka-dq` greenfield testbed: full adoption — `lifecycle init` → a feature through all three gates with the constitution seam live → archive + guard → a bug through the minimal profile. Capture friction as issues; sync companion notes back to the harness docs.
**DoD:** mvp-plan Phase-1 lifecycle DoD items are demonstrably satisfied by this primitive standalone (staged gates recorded, deviation surfaced at gates 2/3, living-spec fold + guard fidelity, bug repro-first).

*Sequencing:* **M1 (format engine) is the critical path** — M4/M5 fold and replay depend on it, M2 validation builds on its parser. M0→M1→M2→M3→M4→M5 are strictly ordered. M6 depends on M2/M3. M8 can start after M0 but the `v0.1.0` cut waits for M7. Riskiest: **M1** (faithful reimplementation + byte-compat) and M4/M5 (the ledger + fold-fidelity spine) and M7 (prompt design + live validation). Everything else is routine Go.

---

## 9. Testing strategy
**One hermetic Go tier** — no npm, no network, no fake shim (all consequences of dropping the runtime):
- **Unit/e2e (every PR):** golden files (`-update`), `testscript` black-box, exact error-message assertions. Covers the format engine, record logic, gate resolution, guard hashing+replay, config, scaffold, status.
- **Format-conformance corpus (every PR):** the static set of real OpenSpec-format change folders + expected fold/render outputs, captured once from the reference tool at v1.5.0 (§2.8). Our parser/fold/render must reproduce them **byte-identically** — the tripwire that we stayed format-compatible. No runtime installed; the corpus *is* the oracle.
- **Property/determinism:** parser round-trips (parse→render→parse stable); fold op-order + conflict detection; guard idempotent; ledger `seq` order stable and folder-date-independent; from-empty replay == live projection across 100 histories and 3 OSes.
- **Fuzz:** the parser and the parse→fold→render round-trip (no panics, no data loss); the ledger digest comparator.
- **Live-agent (M7 only):** scripted acceptance per the M7 DoD.
**Coverage gate:** `go test -coverprofile` in CI, start **85% on `internal/...`**, ratchet up; core packages (`spec`, `approve`, `archive`, `guard`) ≥95%. Coverage is a floor — the DoD contracts (conformance corpus, fold-fidelity, drift detection) are the real bar.

---

## 10. Owner prerequisites (only what differs from the constitution's already-established setup)
The `kentra-gh-bot` machine account, `kentra-io/homebrew-tap`, and `KENTRA_BOT_GH_TOKEN` sandbox wiring **already exist** (constitution §9). New/changed:
1. **Create `kentra-io/spec-lifecycle`** (public); wire the submodule's remote. (No second/shared repo — helpers are copied, §2.12.)
2. **Extend the bot's write access** to the new repo (org-member + team/direct).
3. **Re-scope (or reissue) the fine-grained PAT** to include `spec-lifecycle` — keep `Contents: r/w`, `Pull requests: r/w`, **`Workflows: r/w`** (it pushes `.github/workflows/*`).
4. **Promote `HOMEBREW_TAP_TOKEN` to an org-level secret** (scoped to primitive repos) rather than duplicating per-repo — more primitives are coming (harness AGENTS.md).
5. **No new bot/tap, no registry credential, and — new vs Option A — no claudebox Node/OpenSpec stanza.** The box gets a static Go binary exactly like the constitution; there is nothing runtime-specific to provision.

## 11. Risks
| Risk | Mitigation |
|---|---|
| **Faithful reimplementation of the grammar/fold** (the new core burden) | Static conformance corpus proving byte-compat (M1/§9); the grammar is small and pinned; fuzz the round-trip |
| Format drift if OpenSpec evolves upstream | We pin the v1.5.0 grammar; drift is an *explicit* choice, not an upgrade treadmill; corpus regenerated only on deliberate decision |
| Fold correctness (conflicts, op-order, edge cases) | We own it → conflicts detected by construction (no #1246 silent drop); op-order unit-locked; from-empty replay in guard is a continuous cross-check |
| Cross-primitive `deviation.json` drift across constitution versions | `constitution: { version }` pin + `constitution deviation validate` at the gate (§2.7) |
| `constitution deviation validate` is a "hidden" verb — contract stability | Spike §12; fallback = re-implement the light JSON schema-check in `lifecycle` |
| ~~Young/`[experimental]` OpenSpec subsystem, silent data-loss, no archive order~~ | **Eliminated as external risks** — we own the engine; the ledger owns order; the fold owns conflict handling |
| ~~Hard Node/OpenSpec runtime dep breaks the static-binary story~~ | **Eliminated** — pure Go, single static binary |

## 12. Remaining spikes (scheduled, not blockers)
1. **Live-agent gate eval (M7):** fresh-session gate discipline + permission-prompt consent, across Claude Code + ≥1 other runtime.
2. **`constitution deviation validate` contract (M3):** confirm the hidden verb is safe for cross-primitive dependence, or promote/duplicate.
3. **Fold edge cases (M1):** same-name requirements across capabilities; RENAMED-then-MODIFIED in one delta; MODIFIED of a nonexistent requirement — enumerate and lock behavior in the engine (our decision now).
4. **Conformance-corpus capture (M1):** decide the corpus breadth (how many real change shapes) that gives confident byte-compat without over-fitting.

## 13. Deferred (spec §11 + P1)
Framework adapters · brownfield extraction · drift sweep · prose synthesis over the fold · deterministic cross-change auto-merge · capability-**folder** restructuring fold · async-validation tag (roadmap-ideas) · Conductor-MCP surface · `chore`/`question` profiles.

## 14. Component → milestone map
| Spec component | Milestone |
|---|---|
| Copied helpers (atomicwrite, scaffold, skill fan-out) | M0 |
| **Format engine (parse/validate/fold/render) + conformance corpus** | **M1** |
| `kentra-spec-lifecycle` schema + templates + custom-artifact `lifecycle validate` | M2 |
| `approval-state.json` + `approve`/`status` + constitution seam | M3 |
| `lifecycle archive` + baseline ledger (native fold) | M4 |
| `lifecycle guard` (immutability + digest chain + from-empty replay) | M5 |
| `lifecycle init` + integration wiring | M6 |
| Skill bodies + dogfood | M7 |
| Distribution (GoReleaser/tap; no box runtime) | M8 |
| Harness Phase-1 acceptance | M9 |

## 15. Provenance
- v1 scope P1–P5 + the Option-B pivot: proposed 2026-07-03 (this plan), pending user lock. The pivot is the user's decision (2026-07-03): "follow the OpenSpec convention but not its runtime — rebuild everything in Go."
- Research: 4 parallel agents (2026-07-03) — (A) OpenSpec custom-schema subsystem [source-read], (B) archive/fold + CLI [source-read + adversarial re-verify], (C) constitution repo reuse [local-read], (D) box/distribution + spec completeness [**empirical — built the claudebox, ran `@fission-ai/openspec@1.5.0`**]. Under Option B the empirical run is a **reference oracle** for the conformance corpus, not a runtime we ship. All format claims verified against `Fission-AI/OpenSpec` at tag `v1.5.0` (commit `546224e`).
- Load-bearing negatives that *motivated* Option B: the deterministic fold engine (`buildUpdatedSpec`) has **zero upstream callers** (no safe programmatic entrypoint — the real path is agent-driven `/opsx:sync`); the custom-schema subsystem is `[experimental]` with open bugs; no native archive total-order (#409/#1192); #1246 silently drops overlapping scenarios; `openspec validate` is schema-blind. Owning the engine in Go turns each from an external liability into controlled behavior. The research also *sanctioned* this path: "Ejection stays cheap because files are canonical — port the archive algorithm into a bespoke `lifecycle` CLI."
- Cross-primitive facts: constitution stack + `internal/` layout (helpers **copied**, not shared/imported — both stay standalone, user decision 2026-07-03); `constitution deviation validate` verb (ADR-0009); `kentra-gh-bot`/tap/token already established (constitution §9).
