// Package constitution is a thin exec wrapper around the sibling
// adr-sourced-constitution primitive's `constitution` binary
// (spec-lifecycle.md §7, implementation-plan.md §2.7). The seam is a
// runtime CLI process boundary, never a Go import: this package never
// imports github.com/kentra-io/adr-sourced-constitution.
//
// # Spike §12.2 findings (recorded here, not just in the build report)
//
// `constitution deviation validate <path>` EXISTS exactly as
// implementation-plan.md §2.7/spec-lifecycle.md §7 item 5 describe it —
// confirmed by reading adr-sourced-constitution's cmd/constitution/deviation.go,
// internal/deviation/deviation.go, and its own
// cmd/constitution/testdata/script/deviation_validate.txtar (tag/commit at
// the time of this read). No fallback JSON re-implementation
// (implementation-plan.md §12 item 3's contingency) is needed. The
// contract, precisely:
//
//   - It is a HIDDEN subcommand (`deviation validate`, not top-level) —
//     stable enough to depend on (it is exercised by adr-sourced-
//     constitution's own e2e suite) but intentionally not advertised in
//     `constitution --help`.
//   - Argument: exactly one positional <path> to a deviation.json file.
//     Absolute or relative — relative is resolved against the process's
//     CURRENT WORKING DIRECTORY, not the path's own location.
//   - Working-directory precondition: the process must be run with its
//     cwd set to "a constitution project root" — a directory containing
//     both constitution.yml and constitution/adr/. This package always
//     sets exec.Cmd.Dir explicitly (DeviationValidate's root parameter) and
//     always passes an ABSOLUTE path for <path>, so callers never have to
//     reason about relative-path resolution against that directory.
//   - Exit codes: 0 valid (deviation.json is schema-valid, every adrId
//     cites an ACTIVE, rule-bearing ADR, deviation ids are unique, and the
//     summary tallies) · 1 invalid (schema/citation/tally errors, printed
//     one per line on stderr) · 2 could not run (path unreadable, cwd is
//     not a constitution project root, or the ADR log itself can't be
//     read).
//   - Output: on success, "deviation validate: <path> is valid" on
//     stdout. A constitutionHash that no longer matches the live
//     constitution/constitution.md is an ADVISORY, not a failure — printed
//     to stderr ("constitutionHash mismatch [HIGH — stale gate]...")
//     regardless of the exit code, including on the valid (0) path. This
//     package folds that advisory into Result.Advisories rather than
//     discarding it.
//
// implementation-plan.md §2.7's spec-lifecycle.md §7.5 constitutionHash
// recompute does NOT need to shell out at all: Hash reimplements the
// identical sha256("sha256:"+hex) algorithm adr-sourced-constitution's own
// internal/deviation.ConstitutionHash uses, over the same
// constitution/constitution.md path, so lifecycle's own recompute and the
// value a deviation.json stamps are directly, byte-for-byte comparable
// without a process round-trip.
package constitution
