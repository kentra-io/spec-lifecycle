package spec

import (
	"fmt"
	"regexp"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// ---------------------------------------------------------------------------
// YAML parse + fold engine (change 007, Milestone 2 — design D1/D2).
//
// The sole parse/fold path: reads the owned YAML source of truth (spec.yaml)
// into the M1 model (types.go's LivingSpec/Delta) and folds a change delta
// over the living spec keyed by requirement name in the fixed op order
// (RENAMED -> REMOVED -> MODIFIED -> ADDED).
//
// The markdown engine these replaced (parse.go/delta.go/fold.go/render.go and
// their byte-fidelity Raw model) was deleted once every caller had been
// retargeted — it carried a pin to an external reference tool's grammar,
// which constitution ADR-0005 forbids as a compatibility mechanism.
// ---------------------------------------------------------------------------

// rfc2119Re is the load-bearing keyword check: an added or modified
// requirement's text must assert a normative SHALL/MUST.
var rfc2119Re = regexp.MustCompile(`\b(SHALL|MUST)\b`)

// foldKey is the case-insensitive keying every duplicate/conflict check in
// this package uses.
func foldKey(name string) string {
	return strings.ToLower(name)
}

// missingRFC2119Msg explains a missing SHALL/MUST, calling out the common
// mistake of putting the keyword in the requirement's name instead of its
// text.
func missingRFC2119Msg(op Op, name string) string {
	base := fmt.Sprintf("%s %q must contain SHALL or MUST", op, name)
	if rfc2119Re.MatchString(name) {
		return base + " in the requirement text, not only in the name; move the SHALL/MUST statement into the requirement's `text:` field"
	}
	return base
}

// ParseLivingSpecYAML decodes a living-spec YAML document
// (openspec/specs/<capability>/spec.yaml) into the structured LivingSpec model
// (design D1). Duplicate requirement names (case-insensitive) are rejected at
// parse time — a stored map keyed by name could not surface a collision, so
// the source is a sequence and this parse detects the duplicate explicitly
// (D1 alternatives-rejected rationale). A document that is not valid YAML is
// reported as a decode error naming the failure, distinct from a structural
// rejection.
func ParseLivingSpecYAML(data []byte) (*LivingSpec, error) {
	var ls LivingSpec
	if err := yaml.Unmarshal(data, &ls); err != nil {
		return nil, fmt.Errorf("spec: not valid living-spec YAML: %w", err)
	}
	if err := checkDuplicateRequirementNames(ls.Requirements); err != nil {
		return nil, err
	}
	return &ls, nil
}

// ParseDeltaYAML decodes a change's per-capability spec delta
// (openspec/changes/<change>/specs/<capability>/spec.yaml) into the structured
// Delta model (design D2) and enforces the delta content rules the markdown
// ParseDelta enforced, ported onto the YAML model:
//
//   - Each op is one of ADDED/MODIFIED/REMOVED/RENAMED.
//   - ADDED/MODIFIED requirements carry body text containing SHALL or MUST and
//     at least one scenario.
//   - RENAMED entries name both a FROM and a TO.
//   - No requirement name is named twice across the delta's op entries
//     (case-insensitive) — the duplicate-requirement rejection M2 pins.
//
// A document that is not valid YAML is reported as a decode error.
func ParseDeltaYAML(data []byte) (*Delta, error) {
	var d Delta
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("spec: not valid spec-delta YAML: %w", err)
	}
	if err := validateDeltaEntries(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

// checkDuplicateRequirementNames rejects two requirements sharing a name
// (case-insensitive) in an ordered requirement sequence.
func checkDuplicateRequirementNames(reqs []Requirement) error {
	seen := map[string]bool{}
	for _, r := range reqs {
		key := foldKey(r.Name)
		if seen[key] {
			return &Error{
				Kind:   KindDuplicateRequirement,
				Header: r.Name,
				Msg:    fmt.Sprintf("duplicate requirement %q", r.Name),
			}
		}
		seen[key] = true
	}
	return nil
}

// validateDeltaEntries enforces the ported delta content rules on the
// structured model (see ParseDeltaYAML's doc).
func validateDeltaEntries(d *Delta) error {
	seen := map[string]bool{}
	noteName := func(name string) error {
		key := foldKey(name)
		if seen[key] {
			return &Error{
				Kind:   KindDuplicateRequirement,
				Header: name,
				Msg:    fmt.Sprintf("duplicate requirement %q across delta entries", name),
			}
		}
		seen[key] = true
		return nil
	}

	for _, e := range d.Deltas {
		switch e.Op {
		case OpAdded, OpModified:
			if e.Requirement == nil || strings.TrimSpace(e.Requirement.Name) == "" {
				return &Error{Kind: KindMissingRequirementName, Msg: fmt.Sprintf("%s entry is missing a requirement name", e.Op)}
			}
			r := e.Requirement
			if strings.TrimSpace(r.Text) == "" {
				return &Error{Kind: KindMissingRequirementBody, Header: r.Name, Msg: fmt.Sprintf("%s %q is missing requirement text", e.Op, r.Name)}
			}
			if !rfc2119Re.MatchString(r.Text) {
				return &Error{Kind: KindMissingRFC2119, Header: r.Name, Msg: missingRFC2119Msg(e.Op, r.Name)}
			}
			if len(r.Scenarios) == 0 {
				return &Error{Kind: KindMissingScenarioBlock, Header: r.Name, Msg: fmt.Sprintf("%s %q must include at least one scenario", e.Op, r.Name)}
			}
			if err := noteName(r.Name); err != nil {
				return err
			}
		case OpRemoved:
			if e.Requirement == nil || strings.TrimSpace(e.Requirement.Name) == "" {
				return &Error{Kind: KindMissingRequirementName, Msg: "REMOVED entry is missing a requirement name"}
			}
			if err := noteName(e.Requirement.Name); err != nil {
				return err
			}
		case OpRenamed:
			if strings.TrimSpace(e.From) == "" || strings.TrimSpace(e.To) == "" {
				return &Error{Kind: KindDanglingRename, Header: e.From, Msg: "RENAMED entry must name both a FROM and a TO"}
			}
		default:
			return &Error{Kind: KindNoDeltaSections, Header: string(e.Op), Msg: fmt.Sprintf("unrecognized delta op %q (want ADDED/MODIFIED/REMOVED/RENAMED)", e.Op)}
		}
	}

	if len(d.Deltas) == 0 {
		return &Error{Kind: KindNoDeltaSections, Msg: "no delta entries found; a spec delta needs at least one ADDED/MODIFIED/REMOVED/RENAMED entry"}
	}
	return nil
}

// FoldYAML applies one change's structured delta to a capability's living spec
// over the YAML model — the structured-model counterpart to Fold — and returns
// the folded *LivingSpec. base is the capability's current living spec, or nil
// if the capability does not exist yet: unlike the markdown Fold (which
// synthesizes a markdown skeleton), FoldYAML synthesizes a structured empty
// living spec (no requirements) and folds into it (design D1/D2).
//
// Ops apply in the fixed order the engine uses — RENAMED -> REMOVED ->
// MODIFIED -> ADDED — against a single insertion-ordered set keyed by
// lower-cased requirement name, with the same conflict divergences the
// markdown Fold surfaces (see fold.go's divergence table and errors.go's
// Fold error kinds).
func FoldYAML(capability string, base *LivingSpec, d *Delta) (*LivingSpec, error) {
	set := newYAMLFoldSet(base)

	var renamed, removed, modified, added []DeltaEntry
	for _, e := range d.Deltas {
		switch e.Op {
		case OpRenamed:
			renamed = append(renamed, e)
		case OpRemoved:
			removed = append(removed, e)
		case OpModified:
			modified = append(modified, e)
		case OpAdded:
			added = append(added, e)
		}
	}

	for _, e := range renamed {
		fromKey, toKey := foldKey(e.From), foldKey(e.To)
		req, ok := set.get(fromKey)
		if !ok {
			return nil, &Error{
				Kind: KindFoldRenameSourceMissing, Header: e.From,
				Msg: fmt.Sprintf("capability %q: RENAMED FROM %q does not match any existing requirement", capability, e.From),
			}
		}
		if toKey != fromKey && set.has(toKey) {
			return nil, &Error{
				Kind: KindFoldRenameTargetExists, Header: e.To,
				Msg: fmt.Sprintf("capability %q: RENAMED TO %q collides with an existing requirement of the same name", capability, e.To),
			}
		}
		set.delete(fromKey)
		set.insertNew(toKey, Requirement{Name: e.To, Text: req.Text, Scenarios: req.Scenarios})
	}

	for _, e := range removed {
		name := e.Requirement.Name
		if !set.delete(foldKey(name)) {
			return nil, &Error{
				Kind: KindFoldRemoveMissing, Header: name,
				Msg: fmt.Sprintf("capability %q: REMOVED %q does not match any existing requirement", capability, name),
			}
		}
	}

	for _, e := range modified {
		req := *e.Requirement
		if !set.setExisting(foldKey(req.Name), req) {
			return nil, &Error{
				Kind: KindFoldModifyMissing, Header: req.Name,
				Msg: fmt.Sprintf("capability %q: MODIFIED %q does not match any existing requirement", capability, req.Name),
			}
		}
	}

	for _, e := range added {
		req := *e.Requirement
		if !set.insertNew(foldKey(req.Name), req) {
			return nil, &Error{
				Kind: KindFoldAddExists, Header: req.Name,
				Msg: fmt.Sprintf("capability %q: ADDED %q already exists", capability, req.Name),
			}
		}
	}

	purpose := ""
	if base != nil {
		purpose = base.Purpose
	}
	return &LivingSpec{
		Capability:   capability,
		Purpose:      purpose,
		Requirements: set.list(),
	}, nil
}

// RenderSource serializes the living spec back to its owned YAML source form
// (openspec/specs/<capability>/spec.yaml) — the inverse of ParseLivingSpecYAML.
// internal/archive persists the folded LivingSpec through this so a subsequent
// archive of the same capability can read it back as the fold base (markdown
// spec.md is a one-way projection and can never be re-parsed into the model —
// design D3). The bytes are not held to a byte-stability contract (only the
// markdown projection is, design D5): they need only round-trip cleanly through
// ParseLivingSpecYAML, which yaml.Marshal/Unmarshal guarantees.
func (ls *LivingSpec) RenderSource() ([]byte, error) {
	data, err := yaml.Marshal(ls)
	if err != nil {
		return nil, fmt.Errorf("spec: rendering living-spec YAML source: %w", err)
	}
	return data, nil
}

// yamlFoldSet is the insertion-ordered map of requirement name (lower-cased) ->
// Requirement used by FoldYAML — the structured-model counterpart to
// foldSet, with the same JS-Map semantics (delete-then-insert moves an entry
// to the end; an in-place update never changes its position).
type yamlFoldSet struct {
	order []string
	byKey map[string]Requirement
}

func newYAMLFoldSet(base *LivingSpec) *yamlFoldSet {
	s := &yamlFoldSet{byKey: map[string]Requirement{}}
	if base == nil {
		return s
	}
	for _, r := range base.Requirements {
		key := foldKey(r.Name)
		s.order = append(s.order, key)
		s.byKey[key] = r
	}
	return s
}

func (s *yamlFoldSet) has(key string) bool {
	_, ok := s.byKey[key]
	return ok
}

func (s *yamlFoldSet) get(key string) (Requirement, bool) {
	r, ok := s.byKey[key]
	return r, ok
}

func (s *yamlFoldSet) delete(key string) bool {
	if _, ok := s.byKey[key]; !ok {
		return false
	}
	delete(s.byKey, key)
	for i, k := range s.order {
		if k == key {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return true
}

func (s *yamlFoldSet) setExisting(key string, req Requirement) bool {
	if _, ok := s.byKey[key]; !ok {
		return false
	}
	s.byKey[key] = req
	return true
}

func (s *yamlFoldSet) insertNew(key string, req Requirement) bool {
	if _, ok := s.byKey[key]; ok {
		return false
	}
	s.order = append(s.order, key)
	s.byKey[key] = req
	return true
}

func (s *yamlFoldSet) list() []Requirement {
	if len(s.order) == 0 {
		return nil
	}
	out := make([]Requirement, len(s.order))
	for i, k := range s.order {
		out[i] = s.byKey[k]
	}
	return out
}
