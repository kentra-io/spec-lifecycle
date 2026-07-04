# Format-conformance corpus

A static corpus of real OpenSpec-format change folders and their expected
post-archive `openspec/specs/` output, captured **once** from the real
reference oracle — `@fission-ai/openspec` — so `spec-lifecycle`'s pure-Go
parser/fold/renderer can be proven byte-identical to it **without depending
on it at runtime**. See `spec-lifecycle.md` §12.1/§6.1 and
`implementation-plan.md` §0.5/§2.8/§9/M1 for why this corpus exists instead
of a shelled-out `openspec` binary.

## Oracle provenance

- Package: `@fission-ai/openspec@1.5.0` (installed via `npm install` into a
  throwaway `/tmp` directory — never vendored into this repo).
- Source cross-referenced at tag `v1.5.0`, commit
  `546224e00db26bd1be69874be465d5d6f5e4a851`
  (`git clone --depth 1 --branch v1.5.0 https://github.com/Fission-AI/OpenSpec`).
  Both the installed npm package version and the cloned tag's HEAD commit
  were verified to match these exact values at capture time (2026-07-04).
- Key source files read to understand exact fold/render/validate behavior:
  `src/core/archive.ts` (`ArchiveCommand.run`, the archive pipeline),
  `src/core/specs-apply.ts` (`buildUpdatedSpec` — the fold itself),
  `src/core/parsers/requirement-blocks.ts` (delta + spec-section parsing),
  `src/core/parsers/spec-structure.ts` (`findMainSpecStructureIssues`),
  `src/core/validation/validator.ts` (`validateChangeDeltaSpecs`),
  `src/utils/task-progress.ts` (`tasks.md` completion-gate parsing).

## Exact generation steps

For each case:
1. Hand-build a scratch `openspec/{specs,changes/<name>}` tree (no
   `openspec init` — the case only needs the two subtrees the archive
   command reads: `specs/` and `changes/<name>/`).
2. Seed `before/specs/` = the pre-archive `openspec/specs/` tree (may be
   empty — the fold-from-empty cases).
3. Author the change folder — `proposal.md`, `tasks.md` (all steps
   pre-checked `- [x]`, since `archive`'s incomplete-task warning would
   otherwise require an extra `--yes` justification path we don't want to
   exercise here), and `specs/<capability>/spec.md` (the delta). Snapshot
   this as `change/<name>/` **before** archiving (archive relocates the
   folder to `changes/archive/<dated-name>/`, so it must be captured first).
4. Run the oracle non-interactively:
   ```
   openspec archive <name> --json --yes
   ```
   `--json` makes output machine-parseable and non-interactive by
   construction (any point that would prompt in human mode throws
   instead). `--yes` auto-confirms the "proceed with spec updates?" and
   "incomplete tasks" confirmations (moot here since all tasks are
   pre-checked, but required for `--json` mode regardless — see quirk
   below). Grammar/flags confirmed via `openspec archive --help` and by
   reading `ArchiveCommand.run` directly (`archive.ts`).
5. Snapshot the resulting `openspec/specs/` tree as `expected/specs/`.

This is exactly what `regen.sh` automates (given already-authored
`before/`+`change/` fixtures) — see that script for the reproducible,
scriptable version of steps 1–5.

**Quirk found while doing this:** in `--json` mode, `proposal.md` is
**never validated** (`archive.ts`: `if (!json) { ...validateChange(proposal.md)... }`
is guarded by `!json`) — only the change's delta specs
(`validateChangeDeltaSpecs`) are validated in both modes, and that
validation *does* block the archive (non-zero exit, no files written) on
failure. `--json` also skips prose entirely, which is why we chose it as
the "right flags" for scripting.

## Directory shape (per case)

```
cases/<NN-slug>/
  before/specs/                 the openspec/specs/ tree BEFORE archiving
                                 (empty + .gitkeep for fold-from-empty cases)
  change/<change-name>/         the change folder AS AUTHORED, pre-archive
                                 (proposal.md, tasks.md, specs/<cap>/spec.md)
  expected/specs/                the openspec/specs/ tree AFTER archiving,
                                 byte-exact as the oracle wrote it
```

We deliberately do **not** keep the oracle's raw JSON stdout or the
relocated `changes/archive/<dated-name>/` folder: both embed non-reproducible
data (an absolute tmp path, and today's date in the archive folder name —
see "Renderer/oracle quirks" below, item on archive-name dating). The
per-case totals (`added`/`modified`/`removed`/`renamed`) are recorded in the
case table below instead.

## Case inventory

| # | Slug | Change folder | Oracle totals | What it proves |
|---|---|---|---|---|
| 01 | `01-add-empty-capability` | `001-add-password-login` | +1 | ADDED into a capability that doesn't exist yet (fold-from-empty); confirms the `buildSpecSkeleton` skeleton shape (`# <cap> Specification` / `## Purpose` / TBD line / `## Requirements`) and its exact blank-line canonicalization |
| 02 | `02-add-to-existing` | `002-add-mfa` | +1 | ADDED appended after 2 pre-existing requirements, preserving their order and content verbatim |
| 03 | `03-modified-requirement` | `003-widen-session-expiry` | ~1 | MODIFIED replaces a requirement's body + scenarios **in place** (position preserved) |
| 04 | `04-removed-requirement` | `004-remove-legacy-token-login` | -1 | REMOVED deletes a requirement from the middle of the list; surrounding requirements keep their order |
| 05 | `05-renamed-requirement` | `005-rename-password-login` | →1 | RENAMED-only: **the renamed requirement moves to the end of the list** (see "Fold-order quirk" below) |
| 06 | `06-renamed-and-modified` | `006-rename-and-widen-session-expiry` | ~1 →1 | RENAMED + MODIFIED of the **same** requirement in one delta (spike-12.3 edge, MODIFIED must cite the new/TO name): the requirement **keeps its original position** — different from case 05, see quirk below |
| 07 | `07-multi-op-delta` | `007-auth-cleanup-and-lockout` | +1 ~1 -1 | REMOVED + MODIFIED + ADDED together in one capability; confirms fixed apply order RENAMED→REMOVED→MODIFIED→ADDED doesn't corrupt ordering when ops don't overlap |
| 08 | `08-multi-capability` | `008-trial-signup` | +2 (1 per cap) | One change with deltas for two different capabilities (`auth` + `billing`); both fold independently and correctly |
| 09 | `09-scenario-rich` | `009-rate-limit-api` | +1 (4 scenarios) | One requirement, 4 scenarios, a YAML code fence, a JSON code fence, a numbered list, a nested bullet list, and deliberately irregular scenario-line whitespace (`-    **GIVEN**   ...`) — proves the fold/render is a **byte-verbatim passthrough** of block bodies; only structural joins between sections are canonicalized |
| 10 | `10-new-capability-alongside-existing` | `010-add-webhooks` | +2 (1 new cap + 1 existing cap) | One change creates a brand-new capability (`webhooks`) and simultaneously adds a requirement to an existing one (`auth`) |

## Renderer-canonicalization notes (feed the Go renderer)

These are the byte-level rules the real oracle enforces on **every** fold,
independent of which case — confirmed by diffing `cat -A` output across all
10 cases:

1. **Exactly one newline, never zero, never a blank line,** between the
   spec's pre-`## Requirements` content (title + Purpose section, or
   anything else that happened to precede it) and the `## Requirements`
   header itself — **regardless of how many blank lines separated them in
   the source file.** `extractRequirementsSection` computes `before =
   <everything before the header>.trimEnd()`, so any original blank-line
   spacing there is destroyed on every fold. (Visible in every case: e.g.
   case 02's `before/specs/auth/spec.md` has a blank line between the
   Purpose paragraph and `## Requirements`; `expected/specs/auth/spec.md`
   has none.)
2. Likewise **exactly one newline, never a blank line,** between the
   `## Requirements` header and the first `### Requirement:` block.
3. Requirement/scenario **block bodies are copied byte-for-byte** — no
   whitespace normalization, no re-indentation, no re-wrapping. Case 09
   proves this: irregular leading whitespace on scenario bullets
   (`-    **GIVEN**   a client...`), a YAML fence, a JSON fence, a numbered
   list, and a nested bullet list all survive the fold unchanged.
4. Blocks are joined with a **blank line (`\n\n`) between consecutive
   requirement blocks** (visible in every multi-requirement case).
5. The file **always ends with exactly one trailing blank line** (content
   ends `...\n\n`, i.e. one blank line before EOF) — true whether the spec
   is brand new (case 01) or pre-existing (all others).
6. Any accidental run of 3+ consecutive newlines produced by the above
   joins is collapsed to exactly 2 (`rebuilt.replace(/\n{3,}/g, '\n\n')` in
   `buildUpdatedSpec`) — a final safety net, not something we saw trigger
   in these 10 cases (our joins never produced 3 in a row), but the Go
   renderer must replicate it since malformed input could.
7. **New-capability skeleton** (`buildSpecSkeleton`): title is the
   capability folder name verbatim (`# <cap> Specification`), Purpose is
   always `TBD - created by archiving change <change-name>. Update Purpose
   after archive.` — the change name is baked into the generated prose,
   which is why the Go implementation must reproduce this exact sentence
   (case 01 and case 10's `webhooks` spec).

## Fold-order quirk: RENAMED moves a block to the end — unless MODIFIED also targets it

This is the single most important, least obvious behavior in the corpus
(cases 05 vs. 06), directly relevant to implementation-plan.md §12
spike 3 ("fold determinism edge cases") and spec.md §6.1's fixed apply
order RENAMED→REMOVED→MODIFIED→ADDED:

- **Case 05 (RENAMED only):** renaming "Password login" → "Username and
  password login" moves it from **first** position (before "Session
  expiry") to **last** position in the rebuilt file.
- **Case 06 (RENAMED + MODIFIED of the same requirement):** renaming
  "Session expiry" → "Session inactivity timeout" **and** modifying it in
  the same delta leaves it in its **original** position (still second,
  right after "Password login") — it does **not** move to the end.

Why (traced in `buildUpdatedSpec`, `specs-apply.ts`): the requirement map
is keyed by name and preserves **insertion order** (a JS `Map`).
`RENAMED` always executes as `map.delete(oldKey); map.set(newKey, ...)` —
since `newKey` is a fresh key, it is inserted at the **end** of the map's
iteration order, regardless of where the old entry lived. Recomposition
walks the **original** file order first (keeping only blocks whose *original*
name is still a live key — i.e., a renamed-away name is now absent, so its
old slot is skipped), then appends whatever's left in the map in insertion
order. A bare RENAME's new key was inserted after every other original
entry, so it lands at the end. But `MODIFIED` calls `map.set(existingKey,
...)` — and `Map.set` on an **already-present** key does **not** change
its position. Because RENAMED runs first (fixed op order) and creates the
new key, a same-requirement MODIFIED right after it is just a same-key
`set()` — a no-op for ordering — so the block stays wherever RENAMED just
put it. The net rule for the Go fold to replicate:

> Final order = [requirements that kept their original name and weren't
> removed, in original file order] followed by [requirements renamed in
> this delta, in the order their RENAMED pairs appear in the delta]
> followed by [requirements newly ADDED in this delta, in delta order].
> MODIFIED never changes position — it only ever touches a key that
> already exists at the moment it runs (either an original name, or the
> post-rename name if paired with a RENAMED in the same delta).

## Oracle edge-case behavior (no corpus fixtures — probed, not captured)

Per the task brief, these three scenarios were run against the oracle and
observed directly; no fixtures are checked in for them (they're
`ArchiveBlockedError`/no-op probes, not fold outputs worth pinning
byte-for-byte). All three were run with `--json --yes` exactly like the
corpus cases.

### 1. MODIFIED of a nonexistent requirement

Delta: `## MODIFIED Requirements` naming a requirement that was never
`ADDED` and doesn't exist in the target spec.

**Result: hard error, archive aborted, exit code 1, zero files written.**
```json
{
  "archive": null,
  "status": [{
    "severity": "error",
    "code": "archive_spec_update_failed",
    "message": "auth MODIFIED failed for header \"### Requirement: Ghost requirement that was never added\" - not found",
    "fix": "Fix the change delta specs and rerun. No files were changed."
  }]
}
```
The change folder is **not** relocated/archived — it's left exactly as
authored, free to be fixed and re-run. (Traced to `buildUpdatedSpec`'s
MODIFIED loop: `if (!nameToBlock.has(key)) throw ...`; all `prepared`
updates for **every** capability in the change are computed — and any
single one throwing aborts the whole archive — before any file is written,
so a multi-capability change with one bad capability writes nothing at
all, not a partial fold.)

### 2. ADDED of an already-existing name

Delta: `## ADDED Requirements` naming a requirement that already exists
in the target spec (with different body text).

**Result: hard error, archive aborted, exit code 1, zero files written** —
same shape as above:
```json
{
  "archive": null,
  "status": [{
    "severity": "error",
    "code": "archive_spec_update_failed",
    "message": "auth ADDED failed for header \"### Requirement: Password login\" - already exists",
    "fix": "Fix the change delta specs and rerun. No files were changed."
  }]
}
```

### 3. Two in-flight changes touching the same requirement

Two separate change folders (A and B), both authored against the **same**
pre-change spec, each with a `MODIFIED` block for the identical requirement
("Password login") but with different replacement text. Archived
sequentially: `archive A` (succeeds, exit 0, spec now has A's text), then
`archive B` (**also succeeds, exit 0**, spec now has B's text — A's edit is
gone with no trace, no warning, no error).

**Result: the real oracle has NO cross-change conflict detection at
all.** `archive.ts`'s pipeline only ever reads the ONE named change's delta
against whatever the live `openspec/specs/` currently contains — it has no
concept of "another change is also in flight against this requirement."
The second archive's `MODIFIED` finds the key present (because it's a
`MODIFIED`, not an `ADDED`) and unconditionally overwrites the whole block
with **B**'s content; **A**'s edit is silently and completely lost. This is
exactly the class of failure spec-lifecycle.md's Option-B rationale cites
(OpenSpec issue #1246, "silently drops overlapping scenarios") and is the
concrete justification for `lifecycle archive`'s own §6.2 conflict-check —
which has **zero** equivalent in the reference oracle. Anything our Go
`archive` does to detect this (surface it loudly instead of silently
overwriting) is net-new behavior with no oracle fixture to conform to;
there is nothing to be "byte-compatible" with here because the oracle's
behavior in this case is the exact bug we're deliberately not
reproducing.

## What we did not keep, and why

- **The oracle's raw `--json` stdout.** It embeds an absolute host tmp path
  (`root.path`) and a date-stamped archive folder name (`archivedAs:
  "<today>-<change-name>"`) — neither reproducible across regen runs on a
  different day/machine. The meaningful part (`totals`) is folded into the
  case table above instead.
- **The relocated `changes/archive/<dated-name>/` folder.** Same
  date-non-reproducibility; also redundant with `change/<name>/`, which is
  byte-identical content just pre-relocation.
- **A running `openspec init`-scaffolded tree.** We hand-build the two
  subtrees `archive.ts` actually reads (`specs/`, `changes/<name>/`)
  instead of running `openspec init` first — `init`'s output for the
  default (`spec-driven`) schema doesn't even create `specs/`/`changes/`
  eagerly (confirmed by running it once), so there was nothing to gain
  from it here, and every case needs full control over the "before" state
  anyway.

## Reproducing this corpus

See `regen.sh`. It pins the oracle version/tag above, re-installs it fresh,
and re-runs archive against the **existing** `before/`+`change/` fixtures
checked into each case directory, overwriting `expected/` and
`manifest.json`. It does **not** invent new fixture content — case
authorship (steps 1–3 above) is a one-time, human-reviewed decision;
`regen.sh` only re-proves an existing case against a (possibly newer, if
you edit `ORACLE_VERSION`) oracle release. Verified working end-to-end
2026-07-04 (single-case regen reproduced byte-identical `expected/` and
`manifest.json` output).
