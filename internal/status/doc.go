// Package status implements `lifecycle status` (spec-lifecycle.md §5/§9.1,
// implementation-plan.md §2.6): the read-only gate-state reporter over the
// records internal/approve writes. For one change folder, Change derives:
//
//   - pending: a recognized stage with no recorded gate entry at all
//     (spec-lifecycle.md §5: "pending is DERIVED, never persisted").
//   - skipped: design has no entry, but the LATEST refine entry carries
//     designSkipped:true (spec-lifecycle.md §3.2/§5 — "absent + upstream
//     designSkipped" is legitimately-absent, not pending).
//   - approved / rejected: the latest entry per stage-name
//     (internal/approve.LatestPerStage — highest approvedAt, ties broken
//     by array order).
//   - post-gate artifact drift: for every APPROVED gate, its recorded
//     artifact hashes are re-checked against the change folder's current
//     content (internal/approve.HashDrift) — spec-lifecycle.md §5/§6.2.
//
// # Bug-vs-feature stage set (spec-lifecycle.md §8/§2.11)
//
// The relevant stage set is derived from the change TYPE recorded at
// intake (internal/validate.ReadProposalMeta's Type field — see that
// package's doc comment for where/why): feature ⇒ refine/design/plan;
// bug ⇒ repro/fix. A promoted bug's gates[] may ALSO carry design/plan
// entries in the same folder (spec-lifecycle.md §8's promotion hatch) —
// Change reports the type's base stage set UNIONED with whatever stage
// names actually appear in approval-state.json, so a promoted bug's mixed
// gates are never silently dropped from the report.
package status
