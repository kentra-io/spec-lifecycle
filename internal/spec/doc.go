package spec

// This file collects, in one place, every point where this package's fold
// (fold.go's Fold, the Go reimplementation of OpenSpec's specs-apply.ts
// buildUpdatedSpec) deliberately behaves differently from the reference
// oracle (@fission-ai/openspec v1.5.0), per implementation-plan.md §0.5's
// "conflicts detected, never silently dropped" posture and §12 spike 3
// ("fold determinism edge cases... enumerate and lock behavior in the
// engine — our decision now, not OpenSpec's").
//
// Divergence table:
//
//	| # | Case                                                | Oracle behavior                                                                              | This engine                              | Status                        |
//	|---|------------------------------------------------------|-----------------------------------------------------------------------------------------------|-------------------------------------------|--------------------------------|
//	| 1 | MODIFIED of a nonexistent requirement                 | Hard error (archive_spec_update_failed, "... not found"), exit 1, zero files written (probed)  | KindFoldModifyMissing (hard error)        | Matches oracle                |
//	| 2 | ADDED of an already-existing name                     | Hard error ("... already exists"), exit 1, zero files written (probed)                         | KindFoldAddExists (hard error)            | Matches oracle                |
//	| 3 | REMOVED of a nonexistent requirement                  | Not probed; buildUpdatedSpec's Map.delete() of a missing key is a silent JS no-op, not a throw | KindFoldRemoveMissing (hard error)        | Deliberate divergence: stricter |
//	| 4 | RENAMED FROM naming a nonexistent requirement         | Not probed; inferred silent no-op-as-insert (a fresh TO entry with no source), losing intent  | KindFoldRenameSourceMissing (hard error)  | Deliberate divergence: stricter |
//	| 5 | RENAMED TO colliding with an untouched existing name  | Not probed; Map.set() on an existing key silently overwrites — the #1246 silent-loss class     | KindFoldRenameTargetExists (hard error)   | Deliberate divergence: stricter |
//	| 6 | Duplicate delta section header (e.g. two "## ADDED")  | splitTopLevelSections silently lets the second occurrence overwrite the first (confirmed read) | KindDuplicateDeltaSection (hard error)    | Deliberate divergence: stricter (ParseDelta, delta.go) |
//	| 7 | RENAMED FROM with no matching TO                      | Silently dropped (confirmed read)                                                              | KindDanglingRename (hard error)           | Deliberate divergence: stricter (ParseDelta, delta.go) |
//	| 8 | Two in-flight changes archiving overlapping edits      | No conflict detection at all: the second archive silently and completely overwrites the first  | N/A — this is an archive-time (M4) concern, out of scope for the in-process Fold this package implements; the ledger's per-capability conflict-check (implementation-plan.md §2.5) is the intended guard, not this package | Out of M1 scope, tracked for M4 |
//
// Rows 1-2 were directly probed against the live oracle (testdata/
// conformance/README.md's "Oracle edge-case findings") and this engine
// matches it exactly. Rows 3-5 were not probed (the corpus's 10 cases are
// all well-formed deltas against consistent base specs — verified by
// conformance_test.go, which would fail if any case tripped one of these
// checks) — the oracle's JS Map-based fold semantics make a silent no-op or
// silent overwrite the likely behavior, and this package deliberately
// errors instead, consistent with the "conflicts detected, not silently
// dropped" project posture. Row 6-7 are ParseDelta-level (not Fold-level)
// divergences, included here for completeness since they are the same
// posture applied one layer up (see delta.go's doc comments for detail).
// Row 8 is explicitly out of scope for this package: it is a cross-change,
// archive-time concern (implementation-plan.md §2.5's conflict-check),
// not something a single Fold call over one already-parsed Delta can see.
