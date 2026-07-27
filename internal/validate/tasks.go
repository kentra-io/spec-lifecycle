package validate

import (
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

// NOTE (change 007, Milestone 5): the plan-stage gate no longer parses
// tasks.md — `validatePlan` now delegates to `milestoned-plan-dag validate`
// over the change's plan.yaml (see plan_gate.go, design D6). The tasks.md
// milestone-parsing helpers below (splitMilestoneBlocks / labelSection /
// milestoneLabels, and ParseMilestones in plan.go) remain only for the
// `lifecycle apply --format json` surface until Milestone 6 retires them
// together with this file.

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
