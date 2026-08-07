package spec

// This file collects, in one place, the fold's strictness rules — every case
// where FoldYAML refuses rather than silently reconciling a delta that does
// not match the living spec it is applied to. The governing posture is
// implementation-plan.md §0.5's "conflicts detected, never silently dropped"
// and §12 spike 3's "enumerate and lock behavior in the engine — our
// decision now".
//
// Refusal table:
//
//	| # | Case                                                 | This engine                              |
//	|---|------------------------------------------------------|------------------------------------------|
//	| 1 | MODIFIED of a nonexistent requirement                 | KindFoldModifyMissing (hard error)       |
//	| 2 | ADDED of an already-existing name                     | KindFoldAddExists (hard error)           |
//	| 3 | REMOVED of a nonexistent requirement                  | KindFoldRemoveMissing (hard error)       |
//	| 4 | RENAMED FROM naming a nonexistent requirement         | KindFoldRenameSourceMissing (hard error) |
//	| 5 | RENAMED TO colliding with an untouched existing name  | KindFoldRenameTargetExists (hard error)  |
//	| 6 | Duplicate requirement name within one document        | KindDuplicateRequirement (hard error)    |
//	| 7 | RENAMED entry missing its from/to pair                | KindDanglingRename (hard error)          |
//
// Every row is a hard error: nothing is written, and the caller is told which
// requirement and which op collided. Rows 3-5 in particular are the
// silent-data-loss class this engine exists to eliminate — a map-based fold
// would treat a delete of a missing key as a no-op and a rename onto an
// occupied key as an overwrite, losing the author's intent without a word.
//
// Cross-change conflicts — two in-flight changes whose deltas touch the same
// requirement — are deliberately NOT this package's concern: a single fold
// call over one already-parsed delta cannot see them. That is an
// archive-time check (implementation-plan.md §2.5's per-capability
// conflict-check, internal/archive/conflict.go).
