package spec

import (
	"fmt"
	"strings"
)

// Fold applies one change's capability delta to that capability's current
// requirement set — the Go reimplementation of OpenSpec's
// specs-apply.ts buildUpdatedSpec (implementation-plan.md §0.5/§2.3 spike 3;
// spec-lifecycle.md §6.1) — and returns the folded *RequirementSet ready for
// Render.
//
// base is the capability's living spec.md, already parsed by
// ParseRequirementSet, or nil if the capability does not exist yet: Fold
// then synthesizes the oracle's exact new-capability skeleton
// (buildSpecSkeleton) — "# <capability> Specification" / "## Purpose" /
// "TBD - created by archiving change <changeName>. Update Purpose after
// archive." / an empty "## Requirements" section — before folding into it.
// capability is the bare capability name (e.g. "auth", used verbatim in the
// skeleton title) and changeName is the change folder name (e.g.
// "001-add-password-login", baked into the skeleton Purpose sentence).
//
// Ops apply in the fixed order the oracle uses, RENAMED -> REMOVED ->
// MODIFIED -> ADDED (spec-lifecycle.md §6.1), against a single ordered
// working set keyed by lower-cased requirement name:
//
//   - RENAMED deletes the FROM entry and inserts it under the TO key,
//     regenerating its "### Requirement:" header (via NewRequirement) but
//     preserving its Body/Scenarios verbatim — this is why a rename-only
//     delta moves the requirement to the END of the file (delete+insert on
//     a Go map-like ordered set never rewrites in place), while a
//     RENAMED+MODIFIED of the same requirement keeps the ORIGINAL position
//     (RENAMED's insert already created the key; MODIFIED's subsequent
//     update-in-place does not move it). Locked by fold_test.go against
//     conformance cases 05/06.
//   - REMOVED deletes the named entry.
//   - MODIFIED replaces the named entry's value in place (position
//     unchanged) with the delta's authored requirement block.
//   - ADDED inserts a brand-new entry at the end.
//
// Divergences from the oracle (implementation-plan.md §0.5's "conflicts
// detected, never silently dropped" posture — see also §12 spike 3 and
// testdata/conformance/README.md's oracle-quirks section):
//
//	| Case                                             | Oracle (as probed/inferred)                          | This engine                                  |
//	|---------------------------------------------------|-------------------------------------------------------|------------------------------------------------|
//	| MODIFIED of a nonexistent requirement              | hard error ("... not found"), exit 1, matches ours     | KindFoldModifyMissing — MATCHES oracle          |
//	| ADDED of an already-existing name                  | hard error ("... already exists"), exit 1, matches ours| KindFoldAddExists — MATCHES oracle              |
//	| REMOVED of a nonexistent requirement                | not probed; a JS Map.delete() of a missing key is a silent no-op, not a throw — likely silent | KindFoldRemoveMissing — DELIBERATE DIVERGENCE (error, not silent no-op) |
//	| RENAMED FROM naming a nonexistent requirement       | not probed; likely a silent no-op insert of a fresh TO entry, losing the intended rename | KindFoldRenameSourceMissing — DELIBERATE DIVERGENCE (error) |
//	| RENAMED TO colliding with an untouched existing name | not probed; a JS Map.set() on an existing key silently overwrites its value — the #1246 silent-loss class | KindFoldRenameTargetExists — DELIBERATE DIVERGENCE (error) |
//
// None of the ten conformance-corpus cases exercises a divergent path (all
// are well-formed deltas against consistent base specs), so no corpus case
// trips one of these — verified by conformance_test.go.
func Fold(capability, changeName string, base *RequirementSet, d *Delta) (*RequirementSet, error) {
	var before, preamble, after string
	if base == nil {
		before = fmt.Sprintf(
			"# %s Specification\n\n## Purpose\nTBD - created by archiving change %s. Update Purpose after archive.",
			capability, changeName,
		)
		after = "\n"
	} else {
		before = base.Before
		preamble = base.Preamble
		after = base.After
		if after == "" {
			after = "\n"
		}
	}

	set := newFoldSet(base)

	for _, rn := range d.Renamed {
		fromKey, toKey := foldKey(rn.From), foldKey(rn.To)
		req, ok := set.get(fromKey)
		if !ok {
			return nil, &Error{
				Kind: KindFoldRenameSourceMissing, Header: rn.From,
				Msg: fmt.Sprintf("capability %q: RENAMED FROM %q does not match any existing requirement", capability, rn.From),
			}
		}
		if toKey != fromKey && set.has(toKey) {
			return nil, &Error{
				Kind: KindFoldRenameTargetExists, Header: rn.To,
				Msg: fmt.Sprintf("capability %q: RENAMED TO %q collides with an existing requirement of the same name", capability, rn.To),
			}
		}
		set.delete(fromKey)
		set.insertNew(toKey, NewRequirement(rn.To, req.Body, req.Scenarios))
	}

	for _, name := range d.Removed {
		if !set.delete(foldKey(name)) {
			return nil, &Error{
				Kind: KindFoldRemoveMissing, Header: name,
				Msg: fmt.Sprintf("capability %q: REMOVED %q does not match any existing requirement", capability, name),
			}
		}
	}

	for _, req := range d.Modified {
		if !set.setExisting(foldKey(req.Name), req) {
			return nil, &Error{
				Kind: KindFoldModifyMissing, Header: req.Name,
				Msg: fmt.Sprintf("capability %q: MODIFIED %q does not match any existing requirement", capability, req.Name),
			}
		}
	}

	for _, req := range d.Added {
		if !set.insertNew(foldKey(req.Name), req) {
			return nil, &Error{
				Kind: KindFoldAddExists, Header: req.Name,
				Msg: fmt.Sprintf("capability %q: ADDED %q already exists", capability, req.Name),
			}
		}
	}

	return &RequirementSet{
		Before:                 before,
		Preamble:               preamble,
		Requirements:           set.list(),
		After:                  after,
		HasRequirementsSection: true,
	}, nil
}

// foldKey is the same case-insensitive keying every duplicate/conflict
// check elsewhere in this package uses (parse.go, delta.go).
func foldKey(name string) string {
	return strings.ToLower(name)
}

// foldSet is an insertion-ordered map of requirement name (lower-cased) ->
// Requirement, mirroring the JS Map semantics buildUpdatedSpec relies on:
// delete-then-insert moves an entry to the end; an in-place update (set on
// an already-present key) never changes its position.
type foldSet struct {
	order []string
	byKey map[string]Requirement
}

func newFoldSet(base *RequirementSet) *foldSet {
	s := &foldSet{byKey: map[string]Requirement{}}
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

func (s *foldSet) has(key string) bool {
	_, ok := s.byKey[key]
	return ok
}

func (s *foldSet) get(key string) (Requirement, bool) {
	r, ok := s.byKey[key]
	return r, ok
}

// delete removes key, reporting whether it was present.
func (s *foldSet) delete(key string) bool {
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

// setExisting replaces the value for an already-present key without
// changing its position, reporting whether the key was present.
func (s *foldSet) setExisting(key string, req Requirement) bool {
	if _, ok := s.byKey[key]; !ok {
		return false
	}
	s.byKey[key] = req
	return true
}

// insertNew appends a brand-new key at the end, reporting whether the
// insert happened (false if the key was already present).
func (s *foldSet) insertNew(key string, req Requirement) bool {
	if _, ok := s.byKey[key]; ok {
		return false
	}
	s.order = append(s.order, key)
	s.byKey[key] = req
	return true
}

func (s *foldSet) list() []Requirement {
	if len(s.order) == 0 {
		return nil
	}
	out := make([]Requirement, len(s.order))
	for i, k := range s.order {
		out[i] = s.byKey[k]
	}
	return out
}
