package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
const stepsLabel = "**Steps**"

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

	blocks, hasHeadings := splitMilestoneBlocks(data)
	if !hasHeadings {
		return []Finding{{
			File: path, Line: 1, Kind: "no_milestone_headings",
			Message:  `tasks.md has no "## Milestone <n>: <name>" headings (spec-lifecycle.md §4.2)`,
			Severity: SeverityError,
		}}, nil
	}

	var findings []Finding
	for _, b := range blocks {
		body := strings.Join(b.bodyLines, "\n")
		for _, label := range milestoneLabels {
			if !strings.Contains(body, label) {
				findings = append(findings, Finding{
					File: path, Line: b.headingLine, Kind: "missing_milestone_label",
					Message:  fmt.Sprintf("%s is missing the %s label (spec-lifecycle.md §4.2)", b.name, label),
					Severity: SeverityError,
				})
			}
		}
		if section, ok := labelSection(b.bodyLines, validationContractLabel); ok {
			if !hasNonBlankLine(section) {
				findings = append(findings, Finding{
					File: path, Line: b.headingLine, Kind: "empty_validation_contract",
					Message:  fmt.Sprintf("%s's Validation contract has no checkable lines under it (spec-lifecycle.md §4.2)", b.name),
					Severity: SeverityError,
				})
			}
			findings = append(findings, validateContractBlock(path, b.name, b.headingLine, section)...)
		}
	}
	return findings, nil
}

// milestoneBlock is one "## Milestone <n>: <name>" section of tasks.md,
// split out for both validatePlan's structural checks (above) and
// ParseMilestones' extraction (plan.go) — one parse, two consumers, so
// the two never drift on what counts as a milestone's boundaries.
type milestoneBlock struct {
	id          int
	name        string   // the full heading text, e.g. "## Milestone 1: Password login"
	title       string   // just "<name>", e.g. "Password login"
	bodyLines   []string // lines strictly after the heading, up to (not including) the next heading
	headingLine int      // 1-based heading line number, for Finding.Line
}

// splitMilestoneBlocks splits data's lines into one milestoneBlock per
// "## Milestone <n>: <name>" heading found (spec-lifecycle.md §4.2).
// hasHeadings is false when tasks.md has no such heading at all — the
// caller decides what that means (validatePlan: a hard "no_milestone_headings"
// Finding; ParseMilestones: simply zero milestones, not an error).
func splitMilestoneBlocks(data []byte) (blocks []milestoneBlock, hasHeadings bool) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	var headingLines []int // 0-based indices of milestone headings
	for i, line := range lines {
		if milestoneHeadingRe.MatchString(line) {
			headingLines = append(headingLines, i)
		}
	}
	if len(headingLines) == 0 {
		return nil, false
	}
	for i, hi := range headingLines {
		hi2 := len(lines)
		if i+1 < len(headingLines) {
			hi2 = headingLines[i+1]
		}
		m := milestoneHeadingRe.FindStringSubmatch(lines[hi])
		id, _ := strconv.Atoi(m[1])
		blocks = append(blocks, milestoneBlock{
			id:          id,
			name:        strings.TrimSpace(lines[hi]),
			title:       strings.TrimSpace(m[2]),
			bodyLines:   lines[hi+1 : hi2],
			headingLine: hi + 1,
		})
	}
	return blocks, true
}

// labelSection returns the body lines strictly between label's own line
// and the next fixed milestone label (or the block's end, when label is
// the last one — "**Steps**") — the "the label's own line may carry
// inline teaser text; only lines strictly after it count" rule
// (spec-lifecycle.md §4.2) applied generically, not just for Validation
// contract. ok is false when label isn't found in bodyLines at all.
func labelSection(bodyLines []string, label string) (section []string, ok bool) {
	labelLine := indexOfLineContaining(bodyLines, label)
	if labelLine == -1 {
		return nil, false
	}
	next := len(bodyLines)
	for i := labelLine + 1; i < len(bodyLines); i++ {
		if containsAnyLabel(bodyLines[i]) {
			next = i
			break
		}
	}
	return bodyLines[labelLine+1 : next], true
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
