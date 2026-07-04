package validate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kentra-io/spec-lifecycle/internal/spec"
)

// specsDir is the refine-stage delta directory, relative to a change
// folder: specs/<capability>/spec.md per capability touched
// (spec-lifecycle.md §4).
const specsDir = "specs"

// validateSpecsDeltas walks dir/specs/**/spec.md — the refine-stage delta
// artifact — and delegates ALL grammar checking to internal/spec.ParseDelta,
// the single delta-grammar code path this package never duplicates
// (implementation-plan.md §2.3: "the parser is shared by validate, fold,
// and guard — one code path, one grammar definition"). This function only
// adds the WARNING-level unrecognized-section heuristic described in the
// package doc.
func validateSpecsDeltas(dir string) ([]Finding, error) {
	root := filepath.Join(dir, specsDir)
	paths, err := findSpecFiles(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: root, Kind: "missing_artifact",
				Message:  "no specs/ delta found (expected at least one specs/<capability>/spec.md, spec-lifecycle.md §4)",
				Severity: SeverityError,
			}}, nil
		}
		return nil, err
	}
	if len(paths) == 0 {
		return []Finding{{
			File: root, Kind: "missing_artifact",
			Message:  "specs/ is present but contains no spec.md delta files (spec-lifecycle.md §4)",
			Severity: SeverityError,
		}}, nil
	}

	var findings []Finding
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("validate: reading %s: %w", path, err)
		}

		if _, perr := spec.ParseDelta(data); perr != nil {
			var serr *spec.Error
			if errors.As(perr, &serr) {
				findings = append(findings, Finding{
					File: path, Line: serr.Line, Kind: string(serr.Kind),
					Message: serr.Msg, Severity: SeverityError,
				})
			} else {
				findings = append(findings, Finding{
					File: path, Kind: "delta_parse_error",
					Message: perr.Error(), Severity: SeverityError,
				})
			}
			continue // a hard grammar error already flags this file; skip the warning scan
		}

		findings = append(findings, unrecognizedSectionWarnings(path, data)...)
	}
	return findings, nil
}

// findSpecFiles returns every "spec.md" file under root, sorted.
func findSpecFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "spec.md" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// h2LineRe/deltaHeaderLineRe/requirementLineRe are a deliberately simple,
// non-fence-mask-aware mirror of internal/spec's own header regexes
// (lines.go's h2Re/deltaHeaderRe/requirementHeaderRe), used ONLY by the
// warning-level heuristic below — see the package doc's "Warning-level
// polish" section for why this one heuristic gets its own small scan
// instead of extending internal/spec.
var (
	h2LineRe          = regexp.MustCompile(`^##\s+`)
	deltaHeaderLineRe = regexp.MustCompile(`(?i)^##\s+(ADDED|MODIFIED|REMOVED|RENAMED)\s+Requirements\s*$`)
	requirementLineRe = regexp.MustCompile(`(?i)^###\s+Requirement:\s*(.+)$`)
)

// unrecognizedSectionWarnings flags a "### Requirement:" heading that sits
// under an H2 section internal/spec.ParseDelta does not recognize as one
// of the four delta sections — the "silent drop, no diagnostic" case the
// M1 verifier found (internal/spec/doc.go's divergence table; matches the
// oracle's own silent-ignore behavior, per ParseDelta's doc comment on
// unrecognized H2 sections). Only called when ParseDelta already succeeded
// on this file, so these are always advisories layered on top of an
// otherwise-valid delta, never a substitute for a real grammar error.
func unrecognizedSectionWarnings(path string, data []byte) []Finding {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var findings []Finding
	haveSection := false
	recognized := false
	for i, line := range lines {
		if h2LineRe.MatchString(line) {
			haveSection = true
			recognized = deltaHeaderLineRe.MatchString(line)
			continue
		}
		if !haveSection || recognized {
			continue
		}
		if m := requirementLineRe.FindStringSubmatch(line); m != nil {
			findings = append(findings, Finding{
				File: path, Line: i + 1, Kind: "requirement_under_unrecognized_section",
				Message: fmt.Sprintf(
					`"### Requirement: %s" appears under an unrecognized section heading and will be silently ignored by the fold — move it under an ADDED/MODIFIED/REMOVED/RENAMED Requirements section`,
					strings.TrimSpace(m[1]),
				),
				Severity: SeverityWarning,
			})
		}
	}
	return findings
}
