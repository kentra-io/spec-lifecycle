package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// removedBulletRe recognizes the alternate REMOVED-entry form OpenSpec
	// accepts: a bullet naming the requirement instead of a full header
	// line (requirement-blocks.ts parseRemovedNames).
	removedBulletRe = regexp.MustCompile("(?i)^\\s*-\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")

	// renameFromRe/renameToRe recognize a RENAMED pair's FROM/TO lines.
	// These reference a requirement header INLINE in bullet prose (not as
	// a real heading), so — matching the oracle exactly here, both its
	// change-parser.ts and requirement-blocks.ts implementations agree —
	// they allow zero spaces between "###" and "Requirement:".
	renameFromRe = regexp.MustCompile("(?i)^\\s*-?\\s*FROM:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	renameToRe   = regexp.MustCompile("(?i)^\\s*-?\\s*TO:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")

	// rfc2119Re is the load-bearing keyword check the plan pins
	// (implementation-plan.md §2.3): an ADDED/MODIFIED requirement's body
	// must carry SHALL or MUST somewhere (validator.ts containsShallOrMust).
	rfc2119Re = regexp.MustCompile(`\b(SHALL|MUST)\b`)
)

// ParseDelta parses a change's capability delta —
// openspec/changes/<change>/specs/<capability>/spec.md — into a *Delta: an
// op set grouped by its "## ADDED|MODIFIED|REMOVED|RENAMED Requirements"
// H2 sections. It mirrors OpenSpec's requirement-blocks.ts parseDeltaSpec
// for section/body extraction, plus the load-bearing checks
// validator.ts's validateChangeDeltaSpecs and specs-apply.ts's
// buildUpdatedSpec pre-validation enforce before a fold is attempted:
//
//   - ADDED/MODIFIED requirements must have body text containing SHALL or
//     MUST, and at least one scenario.
//   - No duplicate requirement name within a section; no duplicate
//     FROM/TO within RENAMED; every FROM has a matching TO.
//   - No cross-section conflict: a name can't be ADDED and REMOVED, or
//     MODIFIED and REMOVED, or MODIFIED and ADDED; a RENAMED FROM can't
//     also be MODIFIED (MODIFIED must reference the new name); a RENAMED
//     TO can't collide with an ADDED name.
//   - At least one recognized delta section must be present.
//
// A section header appearing twice (e.g. two "## ADDED Requirements")
// is also an error: real OpenSpec's splitTopLevelSections silently lets
// the second occurrence overwrite the first (a plain JS object-key
// assignment) — this package refuses instead, per the project's "conflicts
// detected, not silently dropped" posture (implementation-plan.md §0.5).
//
// REMOVED entries carry only a requirement Name, never a body: real
// REMOVED blocks may carry human-readable "Reason"/"Migration" prose (see
// e.g. OpenSpec's cli-diff spec.md, archived 2025-08-19), but
// specs-apply.ts's buildUpdatedSpec only ever keys REMOVED off the name
// (`plan.removed: string[]`) — that prose has no fold-time meaning, so this
// package does not model it.
func ParseDelta(content []byte) (*Delta, error) {
	text := normalizeContent(string(content))
	lines := strings.Split(text, "\n")
	mask := buildFenceMask(lines)

	var h2 []int
	for i, line := range lines {
		if !mask[i] && h2Re.MatchString(line) {
			h2 = append(h2, i)
		}
	}

	d := &Delta{}
	seenOps := map[Op]int{}

	for k, idx := range h2 {
		m := deltaHeaderRe.FindStringSubmatch(lines[idx])
		if m == nil {
			continue // not a delta section (e.g. a freeform "## Notes") - ignored, matches the oracle
		}
		op := Op(strings.ToUpper(m[1]))
		header := strings.TrimSpace(lines[idx])
		if prev, dup := seenOps[op]; dup {
			return nil, &Error{
				Kind: KindDuplicateDeltaSection, Line: idx + 1, Header: header,
				Msg: fmt.Sprintf("duplicate %q section (first defined at line %d)", header, prev),
			}
		}
		seenOps[op] = idx + 1

		end := len(lines)
		if k+1 < len(h2) {
			end = h2[k+1]
		}
		start := idx + 1

		switch op {
		case OpAdded:
			d.Present.Added = true
			reqs, err := parseDeltaRequirementSection(lines, mask, start, end, op)
			if err != nil {
				return nil, err
			}
			if len(reqs) == 0 {
				return nil, emptyDeltaSectionError(header, idx)
			}
			d.Added = reqs
		case OpModified:
			d.Present.Modified = true
			reqs, err := parseDeltaRequirementSection(lines, mask, start, end, op)
			if err != nil {
				return nil, err
			}
			if len(reqs) == 0 {
				return nil, emptyDeltaSectionError(header, idx)
			}
			d.Modified = reqs
		case OpRemoved:
			d.Present.Removed = true
			names, err := parseRemovedNames(lines, mask, start, end)
			if err != nil {
				return nil, err
			}
			if len(names) == 0 {
				return nil, emptyDeltaSectionError(header, idx)
			}
			d.Removed = names
		case OpRenamed:
			d.Present.Renamed = true
			renames, err := parseRenames(lines, mask, start, end)
			if err != nil {
				return nil, err
			}
			if len(renames) == 0 {
				return nil, emptyDeltaSectionError(header, idx)
			}
			d.Renamed = renames
		}
	}

	if !d.Present.Added && !d.Present.Modified && !d.Present.Removed && !d.Present.Renamed {
		return nil, &Error{
			Kind: KindNoDeltaSections,
			Msg:  `no delta sections found; add a "## ADDED Requirements", "## MODIFIED Requirements", "## REMOVED Requirements", or "## RENAMED Requirements" section`,
		}
	}

	if err := checkDeltaConflicts(d); err != nil {
		return nil, err
	}

	return d, nil
}

func emptyDeltaSectionError(header string, idx int) error {
	return &Error{
		Kind: KindEmptyDeltaSection, Line: idx + 1, Header: header,
		Msg: fmt.Sprintf("%q section found but no requirement entries were parsed; each section needs at least one \"### Requirement:\" block (REMOVED may instead use a bullet list)", header),
	}
}

// parseDeltaRequirementSection parses a run of full requirement blocks
// (shared grammar with the living-spec Requirements section, parse.go) and
// enforces the two delta-only content rules: RFC-2119 body text, and at
// least one scenario.
func parseDeltaRequirementSection(lines []string, mask []bool, start, end int, op Op) ([]Requirement, error) {
	_, reqs, err := parseRequirementBlocks(lines, mask, start, end)
	if err != nil {
		return nil, err
	}
	for _, r := range reqs {
		if strings.TrimSpace(r.Body) == "" {
			return nil, &Error{Kind: KindMissingRequirementBody, Header: r.Name, Msg: fmt.Sprintf("%s %q is missing requirement text", op, r.Name)}
		}
		if !rfc2119Re.MatchString(r.Body) {
			return nil, &Error{Kind: KindMissingRFC2119, Header: r.Name, Msg: missingRFC2119Msg(op, r.Name)}
		}
		if len(r.Scenarios) == 0 {
			return nil, &Error{Kind: KindMissingScenarioBlock, Header: r.Name, Msg: fmt.Sprintf("%s %q must include at least one scenario", op, r.Name)}
		}
	}
	return reqs, nil
}

// missingRFC2119Msg mirrors validator.ts's buildMissingShallOrMustMessage:
// when SHALL/MUST already appears in the requirement's NAME (header) but
// not its body, point the author at the actual fix (the keyword has to be
// on the body line) instead of the generic, confusing message.
func missingRFC2119Msg(op Op, name string) string {
	base := fmt.Sprintf("%s %q must contain SHALL or MUST", op, name)
	if rfc2119Re.MatchString(name) {
		return base + " in the requirement body, not only in the header; move the SHALL/MUST statement to the line immediately after the \"### Requirement: ...\" header"
	}
	return base
}

// parseRemovedNames scans lines[start:end) for REMOVED entries: a full
// "### Requirement: <name>" header, or the bullet form
// "- `### Requirement: <name>`" — mirroring requirement-blocks.ts
// parseRemovedNames exactly, including that any other line (e.g. a
// "Reason"/"Migration" line, or a scenario belonging to the removed
// requirement) is simply not matched and ignored.
func parseRemovedNames(lines []string, mask []bool, start, end int) ([]string, error) {
	var names []string
	seen := map[string]int{}
	for i := start; i < end; i++ {
		if mask[i] {
			continue
		}
		var name string
		if m := requirementHeaderRe.FindStringSubmatch(lines[i]); m != nil {
			name = strings.TrimSpace(m[1])
		} else if m := removedBulletRe.FindStringSubmatch(lines[i]); m != nil {
			name = strings.TrimSpace(m[1])
		} else {
			continue
		}
		if name == "" {
			continue
		}
		lineNo := i + 1
		key := strings.ToLower(name)
		if prev, dup := seen[key]; dup {
			return nil, &Error{Kind: KindDuplicateRequirement, Line: lineNo, Header: name, Msg: fmt.Sprintf("duplicate requirement in REMOVED: %q (first defined at line %d)", name, prev)}
		}
		seen[key] = lineNo
		names = append(names, name)
	}
	return names, nil
}

// parseRenames scans lines[start:end) for FROM/TO pairs, mirroring
// requirement-blocks.ts parseRenamedPairs (a small state machine: a FROM
// opens a pending pair, the next TO closes it). Unlike the oracle — which
// silently drops a FROM that never gets a matching TO — this package
// errors (KindDanglingRename): a lost RENAMED half is exactly the kind of
// silent data loss implementation-plan.md §0.5 commits this engine to
// surfacing instead.
func parseRenames(lines []string, mask []bool, start, end int) ([]Rename, error) {
	var renames []Rename
	fromSeen := map[string]int{}
	toSeen := map[string]int{}
	var pendingFrom string
	var pendingFromLine int

	for i := start; i < end; i++ {
		if mask[i] {
			continue
		}
		lineNo := i + 1
		if m := renameFromRe.FindStringSubmatch(lines[i]); m != nil {
			if pendingFrom != "" {
				return nil, danglingRenameErr(pendingFrom, pendingFromLine, "the next FROM")
			}
			pendingFrom = strings.TrimSpace(m[1])
			pendingFromLine = lineNo
			continue
		}
		if m := renameToRe.FindStringSubmatch(lines[i]); m != nil {
			to := strings.TrimSpace(m[1])
			if pendingFrom == "" {
				return nil, &Error{Kind: KindDanglingRename, Line: lineNo, Header: to, Msg: fmt.Sprintf("RENAMED TO %q has no preceding FROM", to)}
			}
			fromKey, toKey := strings.ToLower(pendingFrom), strings.ToLower(to)
			if prev, dup := fromSeen[fromKey]; dup {
				return nil, &Error{Kind: KindDuplicateRenameFrom, Line: pendingFromLine, Header: pendingFrom, Msg: fmt.Sprintf("duplicate FROM in RENAMED: %q (first defined at line %d)", pendingFrom, prev)}
			}
			if prev, dup := toSeen[toKey]; dup {
				return nil, &Error{Kind: KindDuplicateRenameTo, Line: lineNo, Header: to, Msg: fmt.Sprintf("duplicate TO in RENAMED: %q (first defined at line %d)", to, prev)}
			}
			fromSeen[fromKey], toSeen[toKey] = pendingFromLine, lineNo
			renames = append(renames, Rename{From: pendingFrom, To: to})
			pendingFrom = ""
		}
	}
	if pendingFrom != "" {
		return nil, danglingRenameErr(pendingFrom, pendingFromLine, "the end of the RENAMED section")
	}
	return renames, nil
}

func danglingRenameErr(from string, line int, until string) error {
	return &Error{Kind: KindDanglingRename, Line: line, Header: from, Msg: fmt.Sprintf("RENAMED FROM %q has no matching TO before %s", from, until)}
}

// checkDeltaConflicts enforces the cross-section rules
// specs-apply.ts's buildUpdatedSpec pre-validates before folding: a
// requirement name can't appear in two conflicting sections, and a
// RENAMED pair can't collide with ADDED/MODIFIED.
func checkDeltaConflicts(d *Delta) error {
	added, modified, removed := map[string]string{}, map[string]string{}, map[string]string{}
	for _, r := range d.Added {
		added[strings.ToLower(r.Name)] = r.Name
	}
	for _, r := range d.Modified {
		modified[strings.ToLower(r.Name)] = r.Name
	}
	for _, n := range d.Removed {
		removed[strings.ToLower(n)] = n
	}

	for k, name := range modified {
		if _, ok := removed[k]; ok {
			return conflictErr(name, OpModified, OpRemoved)
		}
		if _, ok := added[k]; ok {
			return conflictErr(name, OpModified, OpAdded)
		}
	}
	for k, name := range added {
		if _, ok := removed[k]; ok {
			return conflictErr(name, OpAdded, OpRemoved)
		}
	}
	for _, rn := range d.Renamed {
		fromKey, toKey := strings.ToLower(rn.From), strings.ToLower(rn.To)
		if _, ok := modified[fromKey]; ok {
			return &Error{Kind: KindConflictingOps, Header: rn.From, Msg: fmt.Sprintf("when a rename exists, MODIFIED must reference the new name %q, not the old name %q", rn.To, rn.From)}
		}
		if _, ok := added[toKey]; ok {
			return &Error{Kind: KindConflictingOps, Header: rn.To, Msg: fmt.Sprintf("RENAMED TO %q collides with an ADDED requirement of the same name", rn.To)}
		}
	}
	return nil
}

func conflictErr(name string, a, b Op) error {
	return &Error{Kind: KindConflictingOps, Header: name, Msg: fmt.Sprintf("requirement %q present in both %s and %s", name, a, b)}
}
