package validate

import "strings"

// splitFrontmatter extracts a "---\n...\n---\n" delimited YAML frontmatter
// block from the start of a document, mirroring the common Jekyll/Hugo-style
// convention. ok is false if the content doesn't open with a "---"
// delimiter line, or that delimiter is never closed. (An earlier version
// also returned the remaining body; dropped when no caller used it —
// unparam.)
func splitFrontmatter(data []byte) (frontmatter string, ok bool) {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}
