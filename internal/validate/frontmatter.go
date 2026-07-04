package validate

import "strings"

// splitFrontmatter splits a "---\n...\n---\n" delimited YAML frontmatter
// block from the rest of a document, mirroring the common Jekyll/Hugo-style
// convention. ok is false if the content doesn't open with a "---"
// delimiter line, or that delimiter is never closed — in either case body
// is the whole (CRLF-normalized) input, unchanged, for a caller that wants
// to report on it anyway.
func splitFrontmatter(data []byte) (frontmatter string, body string, ok bool) {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", normalized, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fm := strings.Join(lines[1:i], "\n")
			rest := strings.Join(lines[i+1:], "\n")
			return fm, rest, true
		}
	}
	return "", normalized, false
}
