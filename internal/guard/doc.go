// Package guard implements `lifecycle guard` (spec-lifecycle.md §6.3,
// implementation-plan.md §2.4): the deterministic, no-LLM fidelity check
// that the archive ledger (internal/archive's ledger.jsonl), the archived
// changes it describes (openspec/changes/archive/**), and the live
// projection (openspec/specs/**) all still agree with each other.
//
// Run performs, in order, one precondition and three checks, pooling every
// problem found into one Result rather than stopping at the first (this
// package's Findings model, below):
//
//   - precondition: is there a ledger to check at all? If
//     openspec/ledger.jsonl is absent while at least one folder exists
//     under openspec/changes/archive/, that alone is reported
//     (KindLedgerMissing) and checks 1-3 do not run — there is nothing for
//     them to check against. If the ledger file exists but a line in it
//     fails to parse, that is NOT a finding: it is an environment/usage
//     failure (a precise wrapped error), matching internal/archive's own
//     ReadAll behavior, which this package reuses as-is.
//   - check 1, immutability (immutability.go): content-hash every archived
//     change folder and compare against its ledger record(s)'
//     archiveManifestSha, using internal/archive's own ManifestSHA — the
//     SAME manifest algorithm Archive used to produce that hash in the
//     first place (task instruction: reuse, never re-derive).
//   - check 2, digest chain (chain.go): the fast path — per capability, a
//     live spec.md's hash must equal that capability's latest
//     postImageSha, and every record AFTER a capability's first must have
//     a preImageSha equal to the prior record's postImageSha.
//   - check 3, from-empty replay (replay.go): the deep path — recompute
//     fold(all archived deltas, in ledger seq order, from empty) via
//     internal/spec, in-process, and diff the rendered bytes against the
//     live projection.
//
// # Deviation from a literal "every capability originates from empty"
//
// Neither check 2 nor check 3 requires a capability's FIRST-EVER ledger
// record to have preImageSha == archive.EmptyImageSHA. A capability can
// legitimately enter lifecycle's tracked history already populated — a
// brownfield spec.md present before the first `lifecycle archive` ever
// ran (exactly what cmd/lifecycle's own archive_conformance_case02 and
// archive_conflict fixtures set up) — and internal/archive's own Archive
// correctly records a real, non-empty preImageSha for such a capability's
// first record. Requiring an empty origin would misreport every ordinary
// brownfield adoption as a violation. See chain.go's and replay.go's own
// doc comments for exactly what each check verifies (and, for replay,
// deliberately does NOT attempt) in that case.
//
// # Findings model
//
// Every Finding names which Check found it, which Kind it is (the four
// pinned enum members, types.go), and whatever subset of
// Capability/Change/Seq applies. Run never stops at the first Finding: all
// three checks always run to completion (given a parseable ledger) and
// every problem any of them finds is returned together. This is
// deliberate — an operator debugging a corrupted history wants the whole
// picture in one invocation, not one problem revealed per re-run. A
// checkscoped helper returning a non-nil error (rather than appending a
// Finding) always means "guard itself could not complete its work" — an
// I/O failure reading a file guard needs, not evidence about the
// project's state — and always maps to the exit-2 "could not run"
// contract, never exit 1.
//
// # The missing/extra-capability decision (task's open decision, resolved)
//
// The pinned Kind enum has exactly four members (spec-lifecycle.md §6.3);
// there is no dedicated "capability set mismatch" kind. Two structural
// cases only the from-empty replay can see are deliberately folded into
// KindProjectionDrift rather than inventing a fifth member:
//
//   - A live openspec/specs/<cap>/spec.md exists, but no ledger record
//     ever folds that capability (replay.go's "extra live capability"
//     pass) — e.g. a capability created by hand outside `lifecycle
//     archive`, or whose entire ledger history was deleted.
//   - A capability's replayed history produces content, but
//     openspec/specs/<cap>/spec.md does not exist on disk (replay.go's
//     "replayed capability missing live spec.md") — e.g. someone `rm`'d a
//     live spec file the ledger still accounts for.
//
// Both are, semantically, exactly what KindProjectionDrift already means:
// "the live projection does not match what the archived history says it
// should be" — these are just the coarsest possible grain of that
// mismatch (a whole capability present/absent) rather than a byte
// difference within one. Reusing the existing kind keeps the enum closed
// (spec-lifecycle.md §6.3's exact four members) while the Message text (and
// which Capability is set, with no Change/Seq — there is no single
// ledger record to blame for an ENTIRE missing/extra capability) says
// precisely which of the two structural cases fired.
//
// # Exit contract (cmd/lifecycle/guard.go)
//
// 0 clean (no findings), 1 findings (Result.Summary.Clean == false, Run's
// error is nil), 2 could-not-run (Run returns a non-nil error: a malformed
// ledger, or an I/O failure reading a file guard needs). `--format json`
// emits Result verbatim.
//
// # Archive integration (implementation-plan.md §2.5 step 5)
//
// internal/archive cannot import this package (this package imports
// internal/archive for its Record type, ledger reader, and manifest/hash
// helpers — the reverse import would cycle). The wrap therefore happens
// one layer up, in cmd/lifecycle/archive.go: after `lifecycle archive`
// successfully writes its ledger record(s) and internal/archive's OWN
// narrower in-process selfCheck (archive/doc.go's "post-write self-check":
// a spot check of only the just-written manifest and just-folded specs)
// passes, the CLI command additionally runs a FULL guard.Run over the
// whole project — immutability across every archived change ever, the
// whole digest chain, and a full from-empty replay, not just this one
// archive's own effect. This is strictly stronger than (a superset of)
// the narrower in-package check, so nothing is lost by keeping both: the
// in-package selfCheck remains Archive()'s own immediate invariant
// (useful to any caller of the archive package as a library, independent
// of the CLI), and the CLI-level guard run is the comprehensive
// implementation-plan.md §2.5 step 5 self-check. A guard problem found
// here is surfaced loudly (a non-zero process exit) even though the
// archive itself already committed successfully — the point is detection,
// not rollback (task instruction).
package guard
