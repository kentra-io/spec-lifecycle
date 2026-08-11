// Package validate implements the custom-artifact structural checks
// spec-lifecycle.md §3.3 wires into each gate's pre-check
// ("lifecycle validate --stage <s>") and implementation-plan.md §2.3
// assigns to M2. On top of internal/spec's delta-grammar parser (which
// this package delegates to for every specs/**/spec.yaml delta file — one
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
//   - plan.yaml: delegated wholesale to `milestoned-plan-dag validate`,
//     which owns the plan schema (change 007, design D6); this package
//     surfaces that report and adds no plan grammar of its own.
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
//	refine  -> proposal.md + every changes/<change>/specs/**/spec.yaml delta
//	design  -> design.md
//	plan    -> plan.yaml (delegated to `milestoned-plan-dag validate`)
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
// Every Finding this package produces is SeverityError: there is no
// warning-level, advisory-only class of Finding anywhere in this package.
package validate
