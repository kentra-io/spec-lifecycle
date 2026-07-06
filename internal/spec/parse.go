package spec

import (
	"fmt"
	"strings"
)

// ParseRequirementSet parses a living capability spec —
// openspec/specs/<capability>/spec.md — into a *RequirementSet. It mirrors
// OpenSpec's extractRequirementsSection (requirement-blocks.ts): it looks
// for a single "## Requirements" H2 header and, if present, splits its body
// into ordered "### Requirement:" blocks; everything else in the document
// (title, Purpose section, any other top-level content) is preserved
// verbatim in RequirementSet.Before/After.
//
// A missing "## Requirements" section is NOT an error: it mirrors the
// oracle's own "spec doesn't have one yet" fold path (buildSpecSkeleton),
// producing a RequirementSet with no requirements and
// HasRequirementsSection=false, that Render still reproduces byte-for-byte
// and a future fold can still target (by setting HasRequirementsSection
// and appending Requirements). This package does not enforce that a
// Purpose section exists either — that is deliberate: the
// grammar this package pins itself to is the byte-stable fold grammar
// (extractRequirementsSection), not OpenSpec's separate Zod-schema
// validation layer, which is a policy concern the plan assigns to M2's
// internal/validate (implementation-plan.md §2.3, §8 M1 vs M2).
//
// What IS a structural error: a "### Requirement:" header appearing
// outside the "## Requirements" section, and a delta section header
// ("## ADDED Requirements" etc.) appearing anywhere in a living spec —
// both mirror spec-structure.ts's findMainSpecStructureIssues.
func ParseRequirementSet(content []byte) (*RequirementSet, error) {
	text := normalizeContent(string(content))
	lines := strings.Split(text, "\n")
	mask := buildFenceMask(lines)

	reqIdx := findFirst(lines, mask, requirementsHeaderRe, 0)
	if reqIdx == -1 {
		if err := checkNoMisplacedHeaders(lines, mask, 0, len(lines)); err != nil {
			return nil, err
		}
		return &RequirementSet{Before: text, After: "\n"}, nil // HasRequirementsSection: false
	}

	if err := checkNoMisplacedHeaders(lines, mask, 0, reqIdx); err != nil {
		return nil, err
	}

	end := nextH2(lines, mask, reqIdx+1)
	preamble, reqs, err := parseRequirementBlocks(lines, mask, reqIdx+1, end)
	if err != nil {
		return nil, err
	}
	if err := checkNoMisplacedHeaders(lines, mask, end, len(lines)); err != nil {
		return nil, err
	}

	before := strings.Join(lines[:reqIdx], "\n")
	after := strings.Join(lines[end:], "\n")
	if !strings.HasPrefix(after, "\n") {
		after = "\n" + after
	}

	return &RequirementSet{Before: before, Preamble: preamble, Requirements: reqs, After: after, HasRequirementsSection: true}, nil
}

// checkNoMisplacedHeaders scans lines[start:end) for headers that are
// invalid outside a "## Requirements" section: a delta section header (only
// valid in a change's delta spec.md) or a bare requirement header (only
// valid inside "## Requirements").
func checkNoMisplacedHeaders(lines []string, mask []bool, start, end int) error {
	for i := start; i < end; i++ {
		if mask[i] {
			continue
		}
		if deltaHeaderRe.MatchString(lines[i]) {
			header := strings.TrimSpace(lines[i])
			return &Error{
				Kind: KindDeltaHeaderInLivingSpec, Line: i + 1, Header: header,
				Msg: fmt.Sprintf("delta header %q is only valid inside a change's changes/<name>/specs/<capability>/spec.md delta, not a living spec", header),
			}
		}
		if requirementHeaderRe.MatchString(lines[i]) {
			header := strings.TrimSpace(lines[i])
			return &Error{
				Kind: KindRequirementOutsideSection, Line: i + 1, Header: header,
				Msg: fmt.Sprintf("requirement header %q appears outside the \"## Requirements\" section", header),
			}
		}
	}
	return nil
}

// parseRequirementBlocks parses a run of "### Requirement: <name>" blocks
// (each with its nested "#### Scenario:" children) from lines[start:end),
// mirroring OpenSpec's extractRequirementsSection/parseRequirementBlocksFromSection.
// It returns any content preceding the first requirement header as
// preamble (rare; preserved verbatim, trimmed of trailing blank lines) and
// the ordered requirement blocks.
//
// Shared by ParseRequirementSet (the living spec's "## Requirements"
// section) and ParseDelta (each ADDED/MODIFIED delta section) — both use
// the identical block grammar; only their section boundaries differ.
func parseRequirementBlocks(lines []string, mask []bool, start, end int) (preamble string, reqs []Requirement, err error) {
	i := start
	preStart := start
	for i < end && !isHeaderAt(lines, mask, i, requirementHeaderRe) {
		i++
	}
	preamble = trimTrailingWS(strings.Join(lines[preStart:i], "\n"))

	seen := map[string]int{}
	for i < end {
		lineNo := i + 1
		m := requirementHeaderRe.FindStringSubmatch(lines[i])
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", nil, &Error{
				Kind: KindMissingRequirementName, Line: lineNo,
				Msg: `"### Requirement:" header is missing a name`,
			}
		}
		key := strings.ToLower(name)
		if prev, dup := seen[key]; dup {
			return "", nil, &Error{
				Kind: KindDuplicateRequirement, Line: lineNo, Header: name,
				Msg: fmt.Sprintf("duplicate requirement %q (first defined at line %d)", name, prev),
			}
		}
		seen[key] = lineNo

		blockStart := i
		i++
		for i < end && !isHeaderAt(lines, mask, i, requirementHeaderRe) {
			i++
		}
		raw := trimTrailingWS(strings.Join(lines[blockStart:i], "\n"))
		body, scenarios, serr := parseScenarios(lines, mask, blockStart+1, i, name)
		if serr != nil {
			return "", nil, serr
		}
		reqs = append(reqs, Requirement{Name: name, Raw: raw, Body: body, Scenarios: scenarios})
	}
	return preamble, reqs, nil
}

// parseScenarios splits a requirement block's content (lines[start:end),
// i.e. everything after its own header line) into its direct body prose and
// its ordered "#### Scenario:" children, mirroring OpenSpec's
// parseScenarios (markdown-parser.ts).
func parseScenarios(lines []string, mask []bool, start, end int, reqName string) (body string, scenarios []Scenario, err error) {
	i := start
	for i < end && !isHeaderAt(lines, mask, i, scenarioHeaderRe) {
		i++
	}
	body = strings.TrimSpace(strings.Join(lines[start:i], "\n"))

	seen := map[string]int{}
	for i < end {
		lineNo := i + 1
		m := scenarioHeaderRe.FindStringSubmatch(lines[i])
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", nil, &Error{
				Kind: KindMissingScenarioName, Line: lineNo, Header: reqName,
				Msg: fmt.Sprintf("\"#### Scenario:\" header in requirement %q is missing a name", reqName),
			}
		}
		key := strings.ToLower(name)
		if prev, dup := seen[key]; dup {
			return "", nil, &Error{
				Kind: KindDuplicateScenario, Line: lineNo, Header: name,
				Msg: fmt.Sprintf("duplicate scenario %q in requirement %q (first defined at line %d)", name, reqName, prev),
			}
		}
		seen[key] = lineNo

		i++
		scStart := i
		for i < end && !isHeaderAt(lines, mask, i, scenarioHeaderRe) {
			i++
		}
		scBody := strings.TrimSpace(strings.Join(lines[scStart:i], "\n"))
		scenarios = append(scenarios, Scenario{Name: name, Body: scBody})
	}
	return body, scenarios, nil
}
