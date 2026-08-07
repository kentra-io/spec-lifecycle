// Package spec is the format engine: the pure-Go parse / validate / fold /
// render half of spec-lifecycle (implementation-plan.md §0.5, §2.3;
// spec-lifecycle.md §6.1). The format is owned here — structured YAML
// described by two JSON Schemas published with the tool — not a
// compatibility layer over any external tool's grammar (constitution
// ADR-0004, ADR-0005).
//
// Two document shapes share one requirement/scenario sub-shape, which the
// schemas share by $ref (design D4):
//
//   - A living capability spec — openspec/specs/<capability>/spec.yaml — a
//     capability name, an optional purpose, and an ordered sequence of
//     requirements, each with an ordered sequence of scenarios carrying
//     given/when/then clause sequences. Parsed by ParseLivingSpecYAML into a
//     *LivingSpec.
//   - A change's capability delta —
//     openspec/changes/<change>/specs/<capability>/spec.yaml — the same
//     requirement shape wrapped in op-tagged entries
//     (ADDED/MODIFIED/REMOVED/RENAMED). Parsed by ParseDeltaYAML into a
//     *Delta.
//
// One-way projection. Markdown is never parsed and never hand-authored: it
// is a deterministic, read-only projection rendered from the YAML source by
// LivingSpec.RenderProjection, carrying a derived kebab-slug per requirement
// and per scenario (design D3/D5). Because the projection is one-way, there
// is no round-trip contract to preserve and the model carries no
// byte-fidelity Raw field — rendering is byte-stable instead: fixed section
// and key order, LF endings, no trailing whitespace, a single trailing
// newline. That byte-stability is what lets checked-in golden fixtures and
// `lifecycle guard`'s from-empty replay prove fold and projection
// correctness (constitution ADR-0003, ADR-0005).
//
// Ordering. Requirements and scenarios are sequences, not maps: author order
// is projection order and must be preserved, and a map keyed by name could
// not surface a duplicate name at parse time (design D1). Slugs are derived
// at render time and never stored — a stored slug would be a second source
// of truth that can drift.
package spec
