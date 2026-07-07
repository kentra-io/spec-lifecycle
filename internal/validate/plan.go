package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// This file implements the two additive tasks.md capabilities harness
// orchestration.md §5.5 depends on (neither changes anything about the
// four fixed milestone labels or the free-text Validation-contract bullets
// tasks.go already enforces — both are opt-in and backward compatible: a
// milestone that uses neither still validates exactly as it did before):
//
//   - A machine-readable, structured validation contract: an OPTIONAL
//     ```contract fenced YAML block inside a milestone's Validation
//     contract section, carrying an executable acceptance-check command,
//     plain-language criteria, and the allowed path-set (the
//     diff-confined-paths declaration an execution engine grades a diff
//     against). validatePlan enforces it is well-formed WHEN PRESENT;
//     its total absence is not an error (existing plans without a
//     contract still validate, per this addendum's own requirement).
//   - Opt-in checkbox tracking on Steps-list lines ("<n>. [ ] <step>" /
//     "<n>. [x] <step>", GFM task-list convention grafted onto the
//     existing ordered list) — used by internal/archive's
//     tasks-completion gate and surfaced (with the contract above) by
//     `lifecycle apply --format json` (ParseMilestones below). A bare
//     "<n>. <step>" line (no brackets) remains perfectly valid — it is
//     simply untracked, exactly as every Steps line was before this
//     addendum.

// contractFenceOpenRe / contractFenceCloseRe delimit the optional
// structured contract block. The info string is deliberately plain
// "contract" (not e.g. "yaml") so a milestone's contract block is visibly
// distinct from an ordinary fenced code sample elsewhere in the document.
var contractFenceOpenRe = regexp.MustCompile("^\\s*```contract\\s*$")
var contractFenceCloseRe = regexp.MustCompile("^\\s*```\\s*$")

// stepLineRe recognizes one Steps-list line: "<n>. <text>"
// (spec-lifecycle.md §4.2, unchanged), optionally carrying a "[ ]"/"[x]"
// checkbox right after the number. Group 1 is the checkbox mark (" ",
// "x", "X") or "" when the line carries no checkbox at all; group 2 is
// the step text.
var stepLineRe = regexp.MustCompile(`^\s*\d+\.\s+(?:\[([ xX])\]\s+)?(.+?)\s*$`)

// Contract is one milestone's optional, structured validation contract —
// the parsed, trusted shape (see contractYAML for the raw wire shape
// before field validation).
type Contract struct {
	// Check is a single executable acceptance-check command (run from the
	// project root) — e.g. "go test ./internal/foo/...".
	Check string `json:"check"`
	// Criteria is the plain-language acceptance description a
	// human/advisory-judge reviewer grades against alongside Check's exit
	// code (harness orchestration.md §5.5's 3-layer verification).
	Criteria string `json:"criteria"`
	// Paths is the allowed path-set: repo-relative glob patterns an
	// execution engine confines this milestone's diff to (harness
	// orchestration.md §5.5's diff-confined-paths declaration). Never
	// absolute, never containing "..".
	Paths []string `json:"paths"`
}

// contractYAML is the raw shape decoded out of a ```contract fenced
// block, kept distinct from Contract so a YAML-level mistake (wrong
// type, empty required field) is caught by validateContractBlock before
// ever being trusted as a real Contract.
type contractYAML struct {
	Check    string   `yaml:"check"`
	Criteria string   `yaml:"criteria"`
	Paths    []string `yaml:"paths"`
}

// Step is one Steps-list line, with its opt-in checkbox state.
type Step struct {
	Text string `json:"text"`
	// Tracked is true when the line carried a "[ ]"/"[x]" checkbox at
	// all — Checked is only meaningful when Tracked is true.
	Tracked bool `json:"tracked"`
	Checked bool `json:"checked"`
}

// Milestone is one tasks.md "## Milestone <n>: <name>" block, extracted
// beyond lifecycle validate's structural checks for two consumers: the
// archive tasks-completion gate (internal/archive) and the `lifecycle
// apply --format json` machine-readable plan surface (cmd/lifecycle) —
// both read this shape instead of re-parsing tasks.md themselves.
type Milestone struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Steps []Step `json:"steps"`
	// Contract is nil when this milestone carries no ```contract block
	// (backward compatible — see this file's package-level doc comment).
	Contract *Contract `json:"contract,omitempty"`
}

// ParseMilestones reads dir/tasks.md and extracts every milestone's Steps
// (with checkbox tracking, if any) and optional structured Contract.
// ok is false, with a nil error, when tasks.md does not exist at all —
// callers that gate on completeness (internal/archive) or completeness
// reporting (cmd/lifecycle's `apply`) treat a missing tasks.md the same
// way they treat a milestone with no tracked steps: nothing to check,
// never a new hard requirement this addendum invents.
//
// ParseMilestones is deliberately lenient about a malformed ```contract
// block (it is simply omitted, Contract stays nil) — by the time a
// change reaches archive/apply it should already have passed `lifecycle
// validate --stage plan`, which is what actually enforces a *present*
// contract block is well-formed (validateContractBlock, called from
// tasks.go's validatePlan).
func ParseMilestones(dir string) (milestones []Milestone, ok bool, err error) {
	path := filepath.Join(dir, tasksFile)
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("validate: reading %s: %w", path, readErr)
	}

	blocks, hasHeadings := splitMilestoneBlocks(data)
	if !hasHeadings {
		return nil, true, nil
	}

	for _, b := range blocks {
		ms := Milestone{ID: b.id, Title: b.title}
		if section, ok := labelSection(b.bodyLines, stepsLabel); ok {
			ms.Steps = parseSteps(section)
		}
		if section, ok := labelSection(b.bodyLines, validationContractLabel); ok {
			if c, findings := parseContractBlock(section); len(findings) == 0 {
				ms.Contract = c
			}
		}
		milestones = append(milestones, ms)
	}
	return milestones, true, nil
}

// parseSteps extracts every recognized "<n>. <text>" Steps-list line from
// section (already bounded to the Steps label's own section by
// labelSection). Lines that don't match the ordered-list shape at all
// (blank lines, prose continuation) are simply skipped — Steps' free-form
// wording is otherwise unconstrained, exactly as before this addendum.
func parseSteps(section []string) []Step {
	var steps []Step
	for _, line := range section {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := stepLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mark, text := m[1], m[2]
		steps = append(steps, Step{
			Text:    text,
			Tracked: mark != "",
			Checked: strings.EqualFold(mark, "x"),
		})
	}
	return steps
}

// parseContractBlock looks for a ```contract fenced block within section
// (a milestone's Validation-contract section) and, if found, decodes and
// field-validates it. Returns (nil, nil) when no block is present at all
// (the backward-compatible case). Any Finding returned means the block
// IS present but malformed; the caller (validateContractBlock, for
// `lifecycle validate`) reports these; ParseMilestones treats any finding
// as "no trustworthy contract" and leaves Milestone.Contract nil.
func parseContractBlock(section []string) (*Contract, []string) {
	var openIdx, closeIdx = -1, -1
	var opens int
	for i, line := range section {
		if contractFenceOpenRe.MatchString(line) {
			opens++
			if openIdx == -1 {
				openIdx = i
			}
			continue
		}
		if openIdx != -1 && closeIdx == -1 && contractFenceCloseRe.MatchString(line) {
			closeIdx = i
		}
	}
	if openIdx == -1 {
		return nil, nil // no contract block at all — backward compatible
	}
	if opens > 1 {
		return nil, []string{"more than one ```contract block is present; a milestone may declare at most one"}
	}
	if closeIdx == -1 {
		return nil, []string{"```contract block is never closed with a matching ``` fence"}
	}

	yamlText := strings.Join(dedent(section[openIdx+1:closeIdx]), "\n")
	var raw contractYAML
	if err := yaml.Unmarshal([]byte(yamlText), &raw); err != nil {
		return nil, []string{fmt.Sprintf("```contract block is not valid YAML: %s", err)}
	}

	var problems []string
	if strings.TrimSpace(raw.Check) == "" {
		problems = append(problems, `"check" is required and must be a non-empty executable acceptance-check command`)
	}
	if strings.TrimSpace(raw.Criteria) == "" {
		problems = append(problems, `"criteria" is required and must be non-empty plain-language acceptance criteria`)
	}
	if len(raw.Paths) == 0 {
		problems = append(problems, `"paths" is required and must declare at least one allowed-path glob (the diff-confined-paths declaration)`)
	}
	for _, p := range raw.Paths {
		trimmed := strings.TrimSpace(p)
		switch {
		case trimmed == "":
			problems = append(problems, `"paths" must not contain an empty entry`)
		case strings.HasPrefix(trimmed, "/"):
			problems = append(problems, fmt.Sprintf("paths entry %q must be repo-relative, not absolute", p))
		case strings.Contains(trimmed, ".."):
			problems = append(problems, fmt.Sprintf("paths entry %q must not contain parent-directory traversal (..)", p))
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return &Contract{Check: raw.Check, Criteria: raw.Criteria, Paths: raw.Paths}, nil
}

// dedent strips the minimum common leading-whitespace prefix from every
// non-blank line in lines — a ```contract block is typically indented to
// sit visually under its milestone's bullet list, but YAML itself is
// indentation-sensitive, so that cosmetic indent must not become part of
// the parsed document.
func dedent(lines []string) []string {
	minIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent <= 0 {
		return lines
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= minIndent {
			out[i] = l[minIndent:]
		} else {
			out[i] = strings.TrimLeft(l, " \t")
		}
	}
	return out
}

// validateContractBlock adapts parseContractBlock's plain-string problems
// into this package's Finding shape for validatePlan (tasks.go), one
// Finding per problem, all tagged "malformed_contract" (except the
// duplicate-block case, "duplicate_contract") and pointed at the
// milestone's heading line.
func validateContractBlock(path, milestoneName string, headingLine int, section []string) []Finding {
	_, problems := parseContractBlock(section)
	if len(problems) == 0 {
		return nil
	}
	findings := make([]Finding, 0, len(problems))
	for _, p := range problems {
		kind := "malformed_contract"
		if strings.Contains(p, "more than one") {
			kind = "duplicate_contract"
		}
		findings = append(findings, Finding{
			File: path, Line: headingLine, Kind: kind,
			Message:  fmt.Sprintf("%s's ```contract block is malformed: %s (spec-lifecycle.md §4.2 addendum)", milestoneName, p),
			Severity: SeverityError,
		})
	}
	return findings
}
