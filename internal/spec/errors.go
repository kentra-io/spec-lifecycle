package spec

import "fmt"

// Kind identifies the class of a parse error, independent of its message —
// callers (a future validator, tests) can switch on Kind without matching
// prose.
type Kind string

// Living-spec structural error kinds.
const (
	// KindMissingRequirementName is a "### Requirement:" header with no
	// name after the colon.
	KindMissingRequirementName Kind = "missing_requirement_name"
	// KindMissingScenarioName is a "#### Scenario:" header with no name
	// after the colon.
	KindMissingScenarioName Kind = "missing_scenario_name"
	// KindDuplicateRequirement is two "### Requirement:" headers with the
	// same name (case-insensitive) in the same section.
	KindDuplicateRequirement Kind = "duplicate_requirement"
	// KindDuplicateScenario is two "#### Scenario:" headers with the same
	// name (case-insensitive) under the same requirement.
	KindDuplicateScenario Kind = "duplicate_scenario"
	// KindDeltaHeaderInLivingSpec is a "## ADDED|MODIFIED|REMOVED|RENAMED
	// Requirements" header found in a living spec, where only a change's
	// delta spec.md may have one.
	KindDeltaHeaderInLivingSpec Kind = "delta_header_in_living_spec"
	// KindRequirementOutsideSection is a "### Requirement:" header found
	// outside the "## Requirements" section of a living spec.
	KindRequirementOutsideSection Kind = "requirement_outside_requirements_section"
)

// Delta-grammar error kinds.
const (
	// KindNoDeltaSections is a delta spec.md with none of the four
	// recognized H2 sections present at all.
	KindNoDeltaSections Kind = "no_delta_sections"
	// KindEmptyDeltaSection is a recognized H2 section present with zero
	// requirement/rename entries parsed from its body.
	KindEmptyDeltaSection Kind = "empty_delta_section"
	// KindDuplicateDeltaSection is the same H2 section title appearing
	// more than once in one delta spec.md.
	KindDuplicateDeltaSection Kind = "duplicate_delta_section"
	// KindMissingRequirementBody is an ADDED/MODIFIED requirement whose
	// header is followed by no body text at all.
	KindMissingRequirementBody Kind = "missing_requirement_body"
	// KindMissingRFC2119 is an ADDED/MODIFIED requirement whose body lacks
	// the load-bearing SHALL/MUST keyword.
	KindMissingRFC2119 Kind = "missing_rfc2119_keyword"
	// KindMissingScenarioBlock is an ADDED/MODIFIED requirement with zero
	// "#### Scenario:" children.
	KindMissingScenarioBlock Kind = "missing_scenario_block"
	// KindDanglingRename is a RENAMED FROM with no matching TO (or vice
	// versa).
	KindDanglingRename Kind = "dangling_rename"
	// KindDuplicateRenameFrom is two RENAMED pairs with the same FROM
	// name.
	KindDuplicateRenameFrom Kind = "duplicate_rename_from"
	// KindDuplicateRenameTo is two RENAMED pairs with the same TO name.
	KindDuplicateRenameTo Kind = "duplicate_rename_to"
	// KindConflictingOps is a requirement name (or RENAMED pair) claimed
	// by two conflicting operations in the same delta.
	KindConflictingOps Kind = "conflicting_delta_ops"
)

// Fold error kinds — base-spec-aware checks that ParseDelta cannot make on
// its own (it never sees the living spec it will be applied to). See
// fold.go's package doc for the divergence-from-oracle table these
// correspond to.
const (
	// KindFoldRenameSourceMissing is a RENAMED FROM naming a requirement
	// that does not exist in the capability's current requirement set at
	// the point RENAMED is applied.
	KindFoldRenameSourceMissing Kind = "fold_rename_source_missing"
	// KindFoldRenameTargetExists is a RENAMED TO naming a requirement that
	// already exists in the capability's current requirement set (other
	// than the FROM entry being renamed) — folding it would silently
	// overwrite that requirement.
	KindFoldRenameTargetExists Kind = "fold_rename_target_exists"
	// KindFoldRemoveMissing is a REMOVED entry naming a requirement that
	// does not exist in the capability's current requirement set at the
	// point REMOVED is applied.
	KindFoldRemoveMissing Kind = "fold_remove_missing"
	// KindFoldModifyMissing is a MODIFIED entry naming a requirement that
	// does not exist in the capability's current requirement set at the
	// point MODIFIED is applied (after RENAMED/REMOVED have already run —
	// matches the oracle's own "not found" archive failure).
	KindFoldModifyMissing Kind = "fold_modify_missing"
	// KindFoldAddExists is an ADDED entry naming a requirement that
	// already exists in the capability's current requirement set at the
	// point ADDED is applied (after RENAMED/REMOVED/MODIFIED have already
	// run — matches the oracle's own "already exists" archive failure).
	KindFoldAddExists Kind = "fold_add_exists"
)

// Error is a precise, position-anchored parse error: every failure this
// package raises names the 1-based source Line (0 if not tied to a single
// line) and, where one exists, the offending Header text, in addition to a
// human-readable Msg.
type Error struct {
	Kind   Kind
	Line   int
	Header string
	Msg    string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
	}
	return e.Msg
}

// Is makes errors.Is(err, &spec.Error{Kind: spec.KindNoDeltaSections})
// match on Kind alone, so callers don't need to compare Line/Header/Msg.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok || t.Kind == "" {
		return false
	}
	return e.Kind == t.Kind
}
