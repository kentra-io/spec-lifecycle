package approve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// resolveArtifactFiles returns every file matching pattern relative to
// dir, as slash-separated paths relative to dir, sorted. It supports
// exactly the two glob shapes the embedded kentra-spec-lifecycle schema's
// generates: field uses (schema.yaml): a literal path with no wildcard
// ("proposal.md", "design.md", "tasks.md" — a single-file existence
// check), and a "<prefix>/**/<name>" recursive glob with exactly one "**"
// path segment ("specs/**/spec.md" — every file named <name> anywhere
// under <prefix>). A pattern matching nothing (file/dir absent) is not an
// error: it returns a nil slice — approve's own upstream validation step
// is what turns a legitimately-missing MANDATORY artifact into a refusal
// (validate.go); this function only resolves what generates: says a stage
// COULD produce.
func resolveArtifactFiles(dir, pattern string) ([]string, error) {
	if !strings.Contains(pattern, "*") {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(pattern))); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return []string{pattern}, nil
	}

	parts := strings.SplitN(pattern, "/**/", 2)
	if len(parts) != 2 || strings.Contains(parts[1], "/") {
		return nil, fmt.Errorf("approve: unsupported generates glob %q (only a literal path or \"<prefix>/**/<name>\" is supported)", pattern)
	}
	prefix, name := parts[0], parts[1]
	root := filepath.Join(dir, filepath.FromSlash(prefix))

	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != name {
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		matches = append(matches, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}
