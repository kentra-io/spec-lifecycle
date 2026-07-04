// Package approve is the gate writer: `lifecycle approve --stage <s>`
// (spec-lifecycle.md §5, implementation-plan.md §2.6). It resolves a
// stage's generated artifact(s) via the embedded kentra-spec-lifecycle
// schema (internal/schema), hashes them, recomputes the constitutionHash
// (internal/constitution), runs the SAME validation code path
// `lifecycle validate` uses (internal/validate) before ever writing an
// entry, and — at gates 2/3 — shells out to
// `constitution deviation validate` before recording a design/plan gate.
// Every write is append-only into <change>/approval-state.json
// (spec-lifecycle.md §5).
//
// # Stage vocabulary: five names, two flows
//
// internal/validate's Stage type (M2) stays feature-flow-only
// (StageRefine/StageDesign/StagePlan) by its own doc.go's explicit
// decision, deferring the bug flow's "repro"/"fix" stage names
// (spec-lifecycle.md §0's Stage glossary) to "whichever milestone wires
// the bug flow's gate records" — this one. Stage here is a superset:
// the three feature names plus StageRepro/StageFix.
//
// # Bug-flow artifact reuse (a decision this package makes explicit)
//
// spec-lifecycle.md §8 states the bug flow's artifacts reuse the SAME
// filenames the feature flow already generates: "proposal.md = the repro
// description...", "tasks.md optional (usually a single milestone...)".
// There is no separate schema.yaml entry for "repro"/"fix" — this package
// resolves their generates: glob to the schema's existing "proposal"
// (+ "specs", if a delta is present — a promoted bug, spec-lifecycle.md
// §8's promotion hatch) and "tasks" artifacts respectively, and validates
// them with internal/validate's exported Proposal/Plan/SpecsDeltas
// primitives directly (not the stage-dispatching Change, which would
// wrongly force a specs/ delta requirement onto every bug). See
// artifacts.go and validate.go for the exact composition.
//
// # Change type — where "recorded at intake" lives
//
// spec-lifecycle.md §8/§2.11 says gate-checks "key off the change type
// recorded at intake" without pinning where that's recorded. This build's
// decision (documented in internal/validate's ProposalMeta, whose Type
// field this package reads via validate.ReadProposalMeta): proposal.md's
// frontmatter carries a `type: feature|bug` field, mirroring the
// already-load-bearing `issue`/`designSkip` fields there. Approve itself
// does not gate WHICH stage names are legal for a change's type (a
// promoted bug legitimately mixes repro/fix with design/plan in the same
// gates[] array, spec-lifecycle.md §8) — that type-aware "what's the
// required/pending stage set" reasoning is internal/status's job, over
// records this package already wrote.
//
// # Consent
//
// Approve refuses to write — for either an approval OR a --reject — under
// consentPolicy: strict without an explicit confirmation: see consent.go's
// ConsentGate, a direct mirror of the sibling adr-sourced-constitution
// primitive's own cmd/constitution/consent.go (copied into this
// unit-testable package rather than cmd/lifecycle, since Approve's own
// contract, not just the CLI's, is "refuse to write without an explicit
// --approve flag" — implementation-plan.md §2.6/§2.10).
package approve
