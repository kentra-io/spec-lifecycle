package spec

import (
	"regexp"
	"strings"
)

// normalizeContent mirrors OpenSpec's MarkdownParser.normalizeContent:
// CRLF and lone CR are both folded to LF before any line-based parsing, so
// this package is line-ending-agnostic.
func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

// fence tracks an open code fence: its marker character (backtick or
// tilde) and the length of the run that opened it — a closing fence must
// use the same marker and be at least as long (CommonMark fenced-code-block
// rule, mirrored from OpenSpec's buildCodeFenceMask/isClosingFence).
type fence struct {
	marker byte
	length int
}

var (
	fenceOpenRe  = regexp.MustCompile("^\\s*(`{3,}|~{3,})")
	fenceCloseRe = regexp.MustCompile("^\\s*(`{3,}|~{3,})\\s*$")
)

func fenceMarker(line string) *fence {
	m := fenceOpenRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	return &fence{marker: m[1][0], length: len(m[1])}
}

func isClosingFence(line string, active *fence) bool {
	m := fenceCloseRe.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	return m[1][0] == active.marker && len(m[1]) >= active.length
}

// buildFenceMask marks every line that lies inside a fenced code block, so
// header detection can skip a header-shaped string that only appears
// inside a code sample (mirrors OpenSpec MarkdownParser.buildCodeFenceMask
// exactly, including its "any line at or after an opening fence, up to and
// including its closing fence, is masked" semantics).
func buildFenceMask(lines []string) []bool {
	mask := make([]bool, len(lines))
	var active *fence
	for i, line := range lines {
		if active == nil {
			if f := fenceMarker(line); f != nil {
				active = f
				mask[i] = true
			}
			continue
		}
		mask[i] = true
		if isClosingFence(line, active) {
			active = nil
		}
	}
	return mask
}

// Header regexes. Where the v1.5.0 source used two *different* patterns for
// the same conceptual header across its two parser implementations
// (requirement-blocks.ts's extractRequirementsSection allows zero spaces —
// "###Requirement:" — via `###\s*Requirement:`, while spec-structure.ts's
// structural validator requires at least one — `###\s+Requirement:`), this
// package picks the stricter `\s+` form throughout: it matches ordinary
// Markdown header conventions, matches every real fixture in the reference
// corpus, and is the form the oracle's OWN structural validator enforces.
var (
	titleHeaderRe        = regexp.MustCompile(`^#\s+(.+)$`)
	purposeHeaderRe      = regexp.MustCompile(`(?i)^##\s+Purpose\s*$`)
	requirementsHeaderRe = regexp.MustCompile(`(?i)^##\s+Requirements\s*$`)
	deltaHeaderRe        = regexp.MustCompile(`(?i)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\s+Requirements\s*$`)
	h2Re                 = regexp.MustCompile(`^##\s+`)
	requirementHeaderRe  = regexp.MustCompile(`(?i)^###\s+Requirement:\s*(.+)$`)
	scenarioHeaderRe     = regexp.MustCompile(`(?i)^####\s+Scenario:\s*(.+)$`)

	// blankLinesRe collapses 3+ consecutive newlines to exactly one blank
	// line, mirroring the `.replace(/\n{3,}/g, '\n\n')` cleanup OpenSpec's
	// buildUpdatedSpec applies after joining relocated raw blocks.
	blankLinesRe = regexp.MustCompile(`\n{3,}`)
)

// findFirst returns the index of the first line at or after from that is
// not inside a fenced code block and matches re, or -1 if none does.
func findFirst(lines []string, mask []bool, re *regexp.Regexp, from int) int {
	for i := from; i < len(lines); i++ {
		if isHeaderAt(lines, mask, i, re) {
			return i
		}
	}
	return -1
}

// isHeaderAt reports whether line i is a real (unfenced) match of re — the
// single predicate every block-boundary scan in this package is built from.
func isHeaderAt(lines []string, mask []bool, i int, re *regexp.Regexp) bool {
	return !mask[i] && re.MatchString(lines[i])
}

// nextH2 returns the index of the next top-level ("## ") header at or
// after from that is not inside a fenced code block, or len(lines) if none
// exists — the boundary rule every section in this grammar uses (a section
// runs until the next H2 or EOF).
func nextH2(lines []string, mask []bool, from int) int {
	if i := findFirst(lines, mask, h2Re, from); i != -1 {
		return i
	}
	return len(lines)
}

func trimTrailingWS(s string) string {
	return strings.TrimRight(s, " \t\n\r")
}
