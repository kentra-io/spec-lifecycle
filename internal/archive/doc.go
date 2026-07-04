// Package archive implements `lifecycle archive <change>` (spec-lifecycle.md
// §6.2, implementation-plan.md §2.5): the 5-step pipeline that turns an
// approved change folder into a permanent projection update — gate-check,
// conflict-check, pre-image digests, native fold + relocate, post-image
// digests + monotonic-seq ledger append — plus a cheap post-write
// self-check. This is the terminal transition in the lifecycle (spec-lifecycle.md
// §3.1): once archived, the change folder moves under changes/archive/ and
// is thereafter append-only (guard-checked in M5).
//
// # Gate-check reuse (not a second stage-set implementation)
//
// Step 1 does NOT re-derive "which stages does this change type require" —
// it calls internal/status.Change, the SAME code internal/status already
// uses to report gate state (spec-lifecycle.md §8/§2.11: bug-vs-feature
// base stage set, unioned with whatever stage names a promoted bug's
// gates[] actually carries). A gate is satisfied if status reports it
// StateApproved OR StateSkipped (the design-skip case, spec-lifecycle.md
// §3.2); StatePending or StateRejected are violations. This is the single
// mechanism that implements "feature: refine[+design unless
// designSkipped]+plan; bug: repro[+fix]; promoted bug: mixed set keyed off
// intake change type" (implementation-plan.md's task description) without
// duplicating internal/status's stage-set logic.
//
// # "Soft in Phase 1" — this package's reading
//
// spec-lifecycle.md §3.3 item 5 says stage-gate enforcement is "external" in
// Phase 1 ("this primitive never blocks") — that describes the per-STAGE
// approval conversation (refine/design/plan), where `lifecycle approve`
// only ever records what a human already decided; Conductor is the future
// hard-blocker. `lifecycle archive` is a different kind of verb: like
// `validate`/`approve`, it is a deterministic, mutating CLI command with
// its own exit contract, and folding an unapproved delta into the living
// spec is exactly the kind of consequential, hard-to-cleanly-undo action
// this primitive's OTHER mutating verbs already refuse by default
// (`approve`'s consentPolicy: strict). This package's decision: **archive's
// gate-check defaults to a hard local refusal** (exit 1, ErrGatesNotApproved)
// — mirroring `approve`'s own default posture — with an explicit escape
// hatch, `--force-gates`, for the human operator who has a real reason to
// archive anyway (e.g. recovering a stuck change, or Phase-1 conversational
// approval that never got recorded through `lifecycle approve`). The
// override is never silent: Result.GatesOverridden is set, a warning is
// printed, and (deliberately, beyond what the pinned ledger record shape
// requires) `gatesOverridden: true` is stamped onto every ledger record
// this archive appends — the append-only ledger is this primitive's
// permanent audit trail, so an override belongs there, not just on
// stderr. The SAME reasoning and mechanism (a dedicated `--force-conflicts`
// flag, `ConflictsOverridden`/`conflictsOverridden`) applies to step 2's
// conflict-check.
//
// # Conflict-check ("detection by construction")
//
// Step 2 parses every OTHER live change folder's capability deltas
// (internal/spec.ParseDelta — the same parser as everywhere else) and
// compares each capability's set of MODIFIED/REMOVED/RENAMED(from)
// requirement names, case-insensitively, against this change's own set for
// that capability (spec-lifecycle.md §6.2/§6.5). ADDED names are not
// compared here — an ADDED/ADDED name collision on the SAME capability
// would already be refused by Fold itself (spec.KindFoldAddExists) once
// one of the two changes archives first; this check exists to catch the
// case Fold cannot see on its own: two STILL-in-flight changes that would
// silently race to overwrite each other's edit once both eventually
// archive (the #1246 failure class, implementation-plan.md §0.5). A parse
// failure in an unrelated change's delta is not fatal to THIS archive — it
// is surfaced as a warning and that change's contribution to the
// conflict-check is skipped, since a malformed sibling folder is that
// folder's `lifecycle validate` problem, not a reason to block an
// otherwise-clean archive.
//
// # The ledger: location, format, record shape
//
// Location: <root>/openspec/ledger.jsonl — inside the openspec/ tree
// lifecycle exclusively owns (implementation-plan.md §2.9's ownership
// line: "lifecycle owns the entire openspec/ tree... and the archive
// ledger"), a sibling of changes/ and specs/, not inside any one change
// folder (the ledger spans every change, past and present — it is not
// change-scoped like approval-state.json).
//
// Format: JSON Lines (one compact JSON object per line, LF-terminated) —
// append-only by construction (AppendRecords only ever grows the file),
// trivially diffable in git, and exactly the shape a future from-empty
// replay (M5) streams through in order. seq is assigned by this package at
// append time as (max seq already in the file) + 1, 1-based, monotonic and
// wholly independent of change-folder names or dates (spec-lifecycle.md
// §6.3/errata 3).
//
// Record shape matches implementation-plan.md §2.4 exactly (seq, change,
// issue, capability, preImageSha, postImageSha, deltaOps, archiveManifestSha)
// PLUS two additive, omitempty fields this package introduces for the
// override posture above (gatesOverridden, conflictsOverridden) — additive
// only: a record from an un-overridden archive round-trips through any
// consumer that only knows the pinned shape.
//
// deltaOps lists each op in the fold's own fixed order (RENAMED -> REMOVED
// -> MODIFIED -> ADDED); a RENAMED entry's Requirement is "<from> -> <to>"
// (the pinned shape's {op, requirement} does not have room for two names,
// and recording only one of them would lose information a human reading
// the ledger needs). deltaOps is informational/audit only — the
// from-empty replay guard (M5) recomputes the fold from the archived
// delta FILES (preserved verbatim under changes/archive/<name>/), never
// from this summary.
//
// # Pre-image sentinel for a brand-new capability
//
// spec-lifecycle.md §6.1's Fold already has an explicit "capability does
// not exist yet" branch (base == nil, synthesizing the oracle's new-capability
// skeleton). This package's reading of "documented sentinel" for that
// case's pre-image (implementation-plan.md's task description, step c):
// the sha256 of the EMPTY byte string ("sha256:e3b0c...855", computed
// once via hashBytes(nil), never hand-typed) — the same value already
// visible in testdata/conformance/manifest.json for a 0-byte fixture file,
// so it is a value this codebase already treats as "the hash of nothing"
// elsewhere, not a new invented constant.
//
// # Bug (delta-less) archive: one record, no capability
//
// A bug with no specs/ delta (validate.HasSpecsDeltas == false) skips the
// fold entirely (spec-lifecycle.md §8/errata 9) and this package appends
// exactly ONE ledger record — not one per capability, since a delta-less
// bug fix touches no capability at all — with Capability: "" (empty
// string, the sentinel for "no capability" a JSON consumer can test for
// falsiness on), PreImageSha == PostImageSha == the same empty-byte-string
// sentinel above (nothing changed), and DeltaOps as an explicit empty JSON
// array `[]` (never `null` — this package always allocates a non-nil
// slice for it) so a machine reader never has to special-case null vs.
// empty. A PROMOTED bug that DOES carry a specs/ delta (spec-lifecycle.md
// §8's promotion hatch) is archived exactly like a feature: full fold, one
// record per affected capability, real deltaOps.
//
// # Writing the folded specs: prepare, then commit, as one group
//
// A multi-capability change folds N capabilities, and every one of their
// openspec/specs/<cap>/spec.md writes happens before the change-folder
// rename (the commit point below). Doing this as N independent
// atomicwrite.WriteFile calls would mean a failure partway through (e.g.
// capability 2 of 3 fails to flush) leaves capability 1's live spec
// already folded while the change folder is still under changes/ and no
// ledger record exists — a half-folded, un-recorded change that a
// straight re-run of archive cannot recover from (Fold refuses to
// re-apply an ADDED requirement that already exists). Archive instead
// uses atomicwrite.Prepare/Commit: every capability's write is first
// staged as a temp file (Prepare — the part that can fail: allocating,
// writing, flushing), and ALL of them are committed (renamed into place)
// only once every one of them has staged cleanly. A Prepare failure for
// capability N therefore leaves every earlier capability's live spec
// untouched as well, not just N's — the common failure case (a write/flush
// error) never leaves a half-folded change behind. The much smaller
// residual window is a failure DURING the commit loop itself (an atomic
// rename per file, not a full write+flush): if commit N of the group
// fails, commits 1..N-1 already landed and N..end did not, which is the
// same category of partial state as above, just far less likely to occur
// (a rename is far cheaper and less failure-prone than a write+fsync) and
// bounded to the commit step only.
//
// # The post-write self-check
//
// After the ledger append, Archive re-reads (from disk, not from the
// in-memory values already computed) the archived folder's manifest hash
// and, when a fold happened, every folded openspec/specs/<cap>/spec.md,
// and compares them against the record(s) just written. A mismatch
// returns ErrSelfCheckFailed (exit 2 — an internal-invariant failure, not
// a user-actionable refusal) rather than silently trusting the in-memory
// values that produced the record. This is deliberately cheap (a handful
// of re-reads + re-hashes, no ledger-wide replay) — the full guard verb
// (immutability manifest + digest chain + from-empty replay across the
// WHOLE ledger) is `lifecycle guard`, M5 (spec-lifecycle.md §6.3); this is
// only the archive-time spot-check that what Archive just wrote to disk
// is what it just recorded.
package archive
