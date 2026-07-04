package spec

import "strings"

// Scenario is a single "#### Scenario: <name>" block nested under a
// Requirement. Body is the GIVEN/WHEN/THEN prose that follows the heading —
// preserved verbatim (whatever bullet style, code fences, or nesting the
// author used), trimmed only of leading/trailing blank lines.
type Scenario struct {
	Name string
	Body string
}

// render reproduces the scenario's block bytes: the header line, then Body
// (if non-empty) on the following line(s). Used by NewRequirement to
// synthesize Raw, and directly comparable to what the parser captures.
func (s Scenario) render() string {
	block := "#### Scenario: " + s.Name
	if body := strings.TrimSpace(s.Body); body != "" {
		block += "\n" + body
	}
	return block
}

// Requirement is a single "### Requirement: <name>" block — the unit both
// a living spec's Requirements section and a change delta's ADDED/MODIFIED
// sections are built from.
type Requirement struct {
	Name string

	// Raw is the block's exact source text, from the "### Requirement:"
	// header line through the end of the block (its own body plus every
	// nested scenario), byte-for-byte as written, trimmed only of trailing
	// blank lines. Render always re-emits Raw verbatim: OpenSpec's own fold
	// relocates untouched raw blocks rather than re-serializing them field
	// by field, and this package mirrors that so round-tripping never
	// reformats a requirement's interior.
	Raw string

	// Body and Scenarios are structured views derived from Raw at parse
	// time (or computed by NewRequirement), for callers that need
	// structured access — a future validator, a fold, a display. They are
	// not independently round-tripped: constructing a Requirement literal
	// by hand with a Body/Scenarios that disagrees with Raw is a caller
	// error (Render always wins with Raw; use NewRequirement to keep them
	// consistent).
	Body      string
	Scenarios []Scenario
}

// NewRequirement builds a Requirement from structured fields, deriving a
// canonical Raw block from them. Use this to synthesize a requirement (e.g.
// in tests, or a future fold/validate caller) rather than assembling a
// Requirement literal whose Raw might drift from its Body/Scenarios.
func NewRequirement(name, body string, scenarios []Scenario) Requirement {
	block := "### Requirement: " + name
	if b := strings.TrimSpace(body); b != "" {
		block += "\n" + b
	}
	for _, sc := range scenarios {
		block += "\n\n" + sc.render()
	}
	return Requirement{Name: name, Raw: block, Body: strings.TrimSpace(body), Scenarios: scenarios}
}

// RequirementSet is the parsed form of a capability's Requirements
// section — either a living spec's openspec/specs/<capability>/spec.md, or
// (via Delta.Added/Delta.Modified, which share the Requirement type) an
// ADDED/MODIFIED delta section. The shape mirrors OpenSpec's own
// before/header/preamble/blocks/after split (requirement-blocks.ts
// extractRequirementsSection) exactly, because that is the split whose join
// rules make fold byte-stable.
type RequirementSet struct {
	// Before is everything in the source before the "## Requirements"
	// header line, verbatim: the title, the Purpose section, anything else
	// a spec.md carries above its Requirements. Use Title/Purpose for
	// structured reads; edit Before directly to change them.
	Before string

	// Preamble is content between the "## Requirements" header line and the
	// first requirement block. Normally empty (no real corpus fixture has
	// one); preserved verbatim when present.
	Preamble string

	// Requirements is the ordered set of requirement blocks.
	Requirements []Requirement

	// After is content following the Requirements section. Normally empty
	// (stored as "\n"); preserved verbatim when present.
	After string

	// HasRequirementsSection reports whether a "## Requirements" header
	// should appear in Render's output even if Requirements is currently
	// empty. ParseRequirementSet sets this true whenever it found a real
	// "## Requirements" header in the source (however many requirements
	// were inside it, including zero) and false when it found none at all.
	// Render trusts this rather than inferring it from Requirements being
	// empty: inventing a header that was never in the source would not be
	// a faithful round-trip of a document this package could not otherwise
	// recognize as spec-shaped (e.g. one entirely swallowed by an
	// unterminated code fence — see render_test.go/fuzz_test.go). A caller
	// synthesizing a brand-new, still-empty capability spec should set
	// this true explicitly.
	HasRequirementsSection bool
}

// Title returns the text of the document's first level-1 ("# ") heading
// found in Before, or "" if none is present. A read view only — OpenSpec's
// own parser never uses this heading for anything but display, and this
// package does not require it to be present.
func (rs *RequirementSet) Title() string {
	lines := strings.Split(rs.Before, "\n")
	mask := buildFenceMask(lines)
	i := findFirst(lines, mask, titleHeaderRe, 0)
	if i == -1 {
		return ""
	}
	m := titleHeaderRe.FindStringSubmatch(lines[i])
	return strings.TrimSpace(m[1])
}

// Purpose returns the trimmed content of the first "## Purpose" section
// found in Before, or "" if none is present. A read view only; edit Before
// to change it.
func (rs *RequirementSet) Purpose() string {
	lines := strings.Split(rs.Before, "\n")
	mask := buildFenceMask(lines)
	i := findFirst(lines, mask, purposeHeaderRe, 0)
	if i == -1 {
		return ""
	}
	end := nextH2(lines, mask, i+1)
	return strings.TrimSpace(strings.Join(lines[i+1:end], "\n"))
}

// Op is one of the four delta operations a change's capability delta can
// carry, matching the H2 section it comes from.
type Op string

// The four delta operations, in the fixed fold order (RENAMED -> REMOVED ->
// MODIFIED -> ADDED) that implementation-plan.md §0.5/§2.4 pins — recorded
// here for callers (e.g. a future fold) that need the canonical order; this
// package's parser and renderer do not themselves depend on it.
const (
	OpRenamed  Op = "RENAMED"
	OpRemoved  Op = "REMOVED"
	OpModified Op = "MODIFIED"
	OpAdded    Op = "ADDED"
)

// Rename is a single FROM/TO pair inside a "## RENAMED Requirements"
// section.
type Rename struct {
	From string
	To   string
}

// DeltaSections reports which of the four delta H2 sections were textually
// present in the source, even one that parsed to zero entries — so a
// caller (e.g. a future M2 validator) can distinguish "section absent" from
// "section present but empty" (mirrors OpenSpec's DeltaPlan.sectionPresence).
type DeltaSections struct {
	Added, Modified, Removed, Renamed bool
}

// Delta is the parsed form of a change's capability delta —
// openspec/changes/<change>/specs/<capability>/spec.md — the op set grouped
// exactly as the four H2 sections group it.
type Delta struct {
	Added    []Requirement
	Modified []Requirement
	// Removed holds requirement names only (no body) — REMOVED entries may
	// carry Reason/Migration prose in real fixtures, but the format only
	// ever keys the fold off the name (see delta.go's ParseDelta doc for
	// the oracle citation this mirrors).
	Removed []string
	Renamed []Rename

	Present DeltaSections
}
