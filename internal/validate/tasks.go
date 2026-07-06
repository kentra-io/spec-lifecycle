package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// tasksFile is the plan-stage singleton artifact's filename
// (spec-lifecycle.md §4 — load-bearing, kept identical to stock OpenSpec).
const tasksFile = "tasks.md"

// milestoneHeadingRe recognizes a "## Milestone <n>: <name>" heading
// (spec-lifecycle.md §4.2).
var milestoneHeadingRe = regexp.MustCompile(`(?i)^##\s+Milestone\s+(\d+)\s*:\s*(.+)$`)

// milestoneLabels are the four fixed bold labels spec-lifecycle.md §4.2
// pins, in their required order.
var milestoneLabels = []string{"**Goal**", "**Deliverables**", "**Validation contract**", "**Steps**"}

const validationContractLabel = "**Validation contract**"

// validatePlan checks tasks.md's custom-artifact structure: every
// milestone block carries all four fixed labels, and its Validation
// contract has at least one checkable line under it
// (spec-lifecycle.md §4.2, verbatim). It does not grade whether the
// contract's content is actually checkable, or whether Steps are properly
// sized — those are human/agent review concerns, not machine-checkable
// ones.
func validatePlan(dir string) ([]Finding, error) {
	path := filepath.Join(dir, tasksFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: path, Kind: "missing_artifact",
				Message:  tasksFile + " not found",
				Severity: SeverityError,
			}}, nil
		}
		return nil, fmt.Errorf("validate: reading %s: %w", path, err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	type block struct {
		name    string
		bodyLo  int // first line index (0-based, inclusive) of the milestone's body, right after its heading
		bodyHi  int // last line index (0-based, exclusive)
		heading int // 1-based heading line number, for Finding.Line
	}
	var blocks []block
	var headingLines []int // 0-based indices of milestone headings
	for i, line := range lines {
		if milestoneHeadingRe.MatchString(line) {
			headingLines = append(headingLines, i)
		}
	}
	if len(headingLines) == 0 {
		return []Finding{{
			File: path, Line: 1, Kind: "no_milestone_headings",
			Message:  `tasks.md has no "## Milestone <n>: <name>" headings (spec-lifecycle.md §4.2)`,
			Severity: SeverityError,
		}}, nil
	}
	for i, hi := range headingLines {
		hi2 := len(lines)
		if i+1 < len(headingLines) {
			hi2 = headingLines[i+1]
		}
		blocks = append(blocks, block{
			name:    strings.TrimSpace(lines[hi]),
			bodyLo:  hi + 1,
			bodyHi:  hi2,
			heading: hi + 1,
		})
	}

	var findings []Finding
	for _, b := range blocks {
		bodyLines := lines[b.bodyLo:b.bodyHi]
		body := strings.Join(bodyLines, "\n")
		for _, label := range milestoneLabels {
			if !strings.Contains(body, label) {
				findings = append(findings, Finding{
					File: path, Line: b.heading, Kind: "missing_milestone_label",
					Message:  fmt.Sprintf("%s is missing the %s label (spec-lifecycle.md §4.2)", b.name, label),
					Severity: SeverityError,
				})
			}
		}
		if labelLine := indexOfLineContaining(bodyLines, validationContractLabel); labelLine != -1 {
			// The label's OWN line may carry inline teaser text (e.g. "—
			// checkable acceptance criteria, pre-committed:", per the
			// template) — that is not itself a checkable line. Only the
			// lines strictly BETWEEN the label's line and the next
			// label/heading count.
			next := len(bodyLines)
			for i := labelLine + 1; i < len(bodyLines); i++ {
				if containsAnyLabel(bodyLines[i]) {
					next = i
					break
				}
			}
			if !hasNonBlankLine(bodyLines[labelLine+1 : next]) {
				findings = append(findings, Finding{
					File: path, Line: b.heading, Kind: "empty_validation_contract",
					Message:  fmt.Sprintf("%s's Validation contract has no checkable lines under it (spec-lifecycle.md §4.2)", b.name),
					Severity: SeverityError,
				})
			}
		}
	}
	return findings, nil
}

func indexOfLineContaining(lines []string, substr string) int {
	for i, l := range lines {
		if strings.Contains(l, substr) {
			return i
		}
	}
	return -1
}

func containsAnyLabel(line string) bool {
	for _, l := range milestoneLabels {
		if strings.Contains(line, l) {
			return true
		}
	}
	return false
}

func hasNonBlankLine(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}
