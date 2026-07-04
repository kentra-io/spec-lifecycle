// Package spec is the format engine: the pure-Go, parse/render half of
// spec-lifecycle's reimplementation of the OpenSpec on-disk format
// (implementation-plan.md §0.5, §2.3; spec-lifecycle.md §6.1), pinned to the
// grammar of `@fission-ai/openspec` v1.5.0 (commit 546224e).
//
// Two document shapes share one grammar:
//
//   - A living capability spec — openspec/specs/<capability>/spec.md — an
//     ordered set of "### Requirement:" blocks inside a single
//     "## Requirements" section. Parsed by ParseRequirementSet into a
//     *RequirementSet.
//   - A change's capability delta — openspec/changes/<change>/specs/<capability>/spec.md
//     — the same requirement-block grammar, but grouped under up to four
//     "## ADDED|MODIFIED|REMOVED|RENAMED Requirements" sections. Parsed by
//     ParseDelta into a *Delta.
//
// Byte fidelity. This package never reformats a requirement's interior: a
// Requirement's Raw field is the exact source bytes of its block (header
// line through its last scenario), and RequirementSet.Render re-emits those
// Raw blocks verbatim, joined by the same fixed separators OpenSpec's own
// fold uses (single blank line between blocks; a single newline between a
// section header and its body). That is deliberate — it mirrors how
// OpenSpec's `buildUpdatedSpec` achieves byte-stable folding (it relocates
// untouched raw blocks rather than re-serializing parsed fields), and it is
// what makes this package's round-trip properties hold:
//
//   - parse(render(x)) == x for any *RequirementSet x built by this package
//     (by the parser, or by NewRequirement + direct struct construction of
//     RequirementSet) — rendering and re-parsing recovers the same value.
//   - render(parse(b)) converges to a stable canonical form for arbitrary
//     well-formed input bytes b — re-rendering that canonical form is a
//     fixed point (parsing it again and rendering again yields identical
//     bytes), even though b's own incidental whitespace may not survive the
//     first pass unchanged.
//
// Grammar decisions made where the v1.5.0 source was internally ambiguous
// or inconsistent are called out on the relevant regexp var docs in
// parse.go and delta.go.
package spec
