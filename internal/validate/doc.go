// Package validate implements the custom-artifact structural checks
// spec-lifecycle.md §3.3 wires into each gate's pre-check
// ("lifecycle validate --stage <s>") and implementation-plan.md §2.3
// assigns to M2. On top of internal/spec's delta-grammar parser (which
// this package delegates to for every specs/**/spec.md delta file — one
// grammar code path, never duplicated: this package never re-implements a
// rule ParseDelta already enforces), it checks the three artifacts the
// OpenSpec format itself has no opinion on:
//
//   - proposal.md: a "---"-delimited YAML frontmatter block is present,
//     and its `issue` field is a non-empty string (spec-lifecycle.md §4's
//     proposal row, §10's sourceTracking join key).
//   - design.md: an explicit NFR-discharge section ("## NFR Discharge",
//     case/hyphen-insensitive) is present (spec-lifecycle.md §4's design
//     row, §4.1's NFR routing rule, §7's ADR-proposal seam).
//   - tasks.md: every "## Milestone <n>: <name>" block carries the four
//     fixed labels **Goal** / **Deliverables** / **Validation contract** /
//     **Steps**, and Validation contract has at least one non-blank line
//     under it (spec-lifecycle.md §4.2, verbatim).
//
// # Stage -> artifact mapping
//
// Derived from spec-lifecycle.md §4's artifact table (its Stage column)
// and §3.3's gate mechanics: each stage's `lifecycle validate --stage <s>`
// checks only the artifact(s) *produced during* that stage. An earlier
// stage's artifacts were already gated approved by the time a later stage
// runs — the schema's requires: DAG (internal/schema) already establishes
// their existence is a precondition, not something a later stage's
// validate call needs to re-check.
//
//	refine  -> proposal.md + every changes/<change>/specs/**/spec.md delta
//	design  -> design.md
//	plan    -> tasks.md
//
// (The bug flow's compressed profile — spec-lifecycle.md §8 — and its
// repro/fix stage names are out of scope for this package; M2 covers only
// the three feature-flow stages. A future bug-flow validate call is a
// straightforward extension of the same shape, deferred to whichever
// milestone wires the bug flow's gate records.)
//
// A stage whose artifact (or, for refine, whose specs/ delta directory)
// does not exist at all is reported as a single "missing_artifact"
// Finding, not a read error — `lifecycle validate` can legitimately run
// before an agent has produced anything yet, as a pre-check sanity probe
// (spec-lifecycle.md §3.3 step 2).
//
// # Warning-level polish
//
// One additional, WARNING-only (never error-, never exit-code-affecting)
// heuristic runs over each specs/**/spec.md delta: a "### Requirement:"
// heading that sits under an H2 section OpenSpec's own delta grammar does
// not recognize (i.e. not one of ADDED/MODIFIED/REMOVED/RENAMED
// Requirements) is silently ignored by both the real oracle and this
// package's own internal/spec.ParseDelta (see internal/spec/doc.go's
// divergence table) — a genuine "requirement-shaped drop with no
// diagnostic" that the M1 verifier flagged. This package's warning scan is
// a plain, non-fence-mask-aware line scan (deliberately simpler than
// internal/spec's own header detection): it may rarely false-positive on
// a "### Requirement:"-shaped line inside a fenced code sample, which is
// acceptable for an advisory that never affects validity or the exit
// code, and it does not change internal/spec's parsing behavior or the
// conformance corpus's results in any way.
package validate
