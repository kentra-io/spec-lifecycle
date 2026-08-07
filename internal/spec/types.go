package spec

// Op is one of the four delta operations a change's capability delta can
// carry.
type Op string

// The four delta operations, in the fixed fold order (RENAMED -> REMOVED ->
// MODIFIED -> ADDED) that implementation-plan.md §0.5/§2.4 pins and FoldYAML
// applies.
const (
	OpRenamed  Op = "RENAMED"
	OpRemoved  Op = "REMOVED"
	OpModified Op = "MODIFIED"
	OpAdded    Op = "ADDED"
)

// ---------------------------------------------------------------------------
// Native YAML spec model (change 007 — YAML source of truth; design D1/D2).
//
// This is the owned, hand-authored structured model — the sole source of
// truth, described by the two published JSON Schemas and validated in
// process (ValidateYAML). The markdown OpenSpec grammar it replaced, and the
// parse/fold/render engine built on it, were deleted once milestones 2–4
// retargeted everything onto this model.
//
// This model carries NO byte-fidelity Raw field:
// markdown is a one-way projection derived from these structs (design D3/D5),
// never re-parsed back into them, so there is nothing to round-trip.
// ---------------------------------------------------------------------------

// LivingSpec is the structured YAML source of a capability's living spec —
// openspec/specs/<capability>/spec.yaml (design D1). Markdown (spec.md) is a
// deterministic read-only projection of this document.
//
// Requirements is an ordered SEQUENCE, not a map: author order is the
// projection order and must be preserved (YAML maps are unordered, and a map
// keyed by name could not detect duplicate names at parse time — D1
// alternatives-rejected). Kebab-slugs are derived at render time from each
// name and are never stored in the source (a stored slug would be a second
// source of truth that can drift — D1).
type LivingSpec struct {
	Capability   string        `yaml:"capability"`
	Purpose      string        `yaml:"purpose,omitempty"`
	Requirements []Requirement `yaml:"requirements"`
}

// Requirement is one requirement in the structured model — the shared
// requirement/scenario sub-shape that both a LivingSpec's requirements[] and
// a delta's ADDED/MODIFIED (and name-only REMOVED) entries carry. The two
// published JSON Schemas share it via $ref (design D4). Scenarios is an
// ordered sequence (author order preserved).
type Requirement struct {
	Name      string     `yaml:"name"`
	Text      string     `yaml:"text,omitempty"`
	Scenarios []Scenario `yaml:"scenarios,omitempty"`
}

// Scenario is one scenario under a Requirement. Given/When/Then are
// SEQUENCES of clauses — a single scenario may carry several of each (design
// D1) — and their author order is preserved.
type Scenario struct {
	Name  string   `yaml:"name"`
	Given []string `yaml:"given,omitempty"`
	When  []string `yaml:"when,omitempty"`
	Then  []string `yaml:"then,omitempty"`
}

// Delta is the structured YAML source of one change's per-capability
// spec delta — openspec/changes/<change>/specs/<capability>/spec.yaml
// (design D2). Deltas is an ordered sequence of op-tagged entries; the fold
// applies them keyed by requirement name in the fixed op order the engine
// uses (RENAMED → REMOVED → MODIFIED → ADDED, see the Op constants above).
// The fold retarget onto this model lands in milestone 2.
type Delta struct {
	Capability string       `yaml:"capability"`
	Deltas     []DeltaEntry `yaml:"deltas"`
}

// DeltaEntry is one op-tagged delta operation (design D2):
//
//   - ADDED / MODIFIED — Requirement carries the full requirement (name,
//     text, scenarios); From/To are empty.
//   - REMOVED — Requirement carries the name only (the fold keys off the
//     name); text/scenarios are absent.
//   - RENAMED — From and To name the rename; Requirement is nil.
type DeltaEntry struct {
	Op          Op           `yaml:"op"`
	Requirement *Requirement `yaml:"requirement,omitempty"`
	From        string       `yaml:"from,omitempty"`
	To          string       `yaml:"to,omitempty"`
}
