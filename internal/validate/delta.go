package validate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/spec"
)

// specsDir is the refine-stage delta directory, relative to a change
// folder: specs/<capability>/spec.yaml per capability touched
// (spec-lifecycle.md §4; change 007 — YAML is the authoritative delta, not
// any markdown file).
const specsDir = "specs"

// validateSpecsDeltas walks dir/specs/**/spec.yaml — the refine-stage delta
// artifact — and reads each YAML delta as the AUTHORITATIVE source of truth
// (change 007, design D2; spec-format scenario "a spec delta is authored and
// read as YAML"): each file is schema-validated against the published
// spec-delta JSON Schema (via internal/spec's M1 validator) and then parsed by
// internal/spec.ParseDeltaYAML, the single delta-grammar code path this
// package never duplicates. A file that is not valid YAML, or that violates
// the schema, is reported as an error naming the offending file path.
func validateSpecsDeltas(dir string) ([]Finding, error) {
	root := filepath.Join(dir, specsDir)
	paths, err := findSpecYAMLFiles(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: root, Kind: "missing_artifact",
				Message:  "no specs/ delta found (expected at least one specs/<capability>/spec.yaml, spec-lifecycle.md §4)",
				Severity: SeverityError,
			}}, nil
		}
		return nil, err
	}
	if len(paths) == 0 {
		return []Finding{{
			File: root, Kind: "missing_artifact",
			Message:  "specs/ is present but contains no spec.yaml delta files (spec-lifecycle.md §4)",
			Severity: SeverityError,
		}}, nil
	}

	sch, err := spec.SpecDeltaSchema()
	if err != nil {
		return nil, fmt.Errorf("validate: compiling spec-delta schema: %w", err)
	}

	var findings []Finding
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("validate: reading %s: %w", path, err)
		}

		// Malformed YAML — the file does not decode at all. Surface it naming
		// the offending file path (change 007 refine-read contract).
		var doc any
		if derr := yaml.Unmarshal(data, &doc); derr != nil {
			findings = append(findings, Finding{
				File: path, Kind: "malformed_yaml",
				Message:  fmt.Sprintf("%s: not valid YAML: %v", path, derr),
				Severity: SeverityError,
			})
			continue
		}

		// Schema violation — decodes as YAML but does not match the published
		// spec-delta schema; the error names the offending JSON path.
		if verr := spec.ValidateYAML(doc, sch); verr != nil {
			findings = append(findings, Finding{
				File: path, Kind: "delta_schema_error",
				Message:  fmt.Sprintf("%s: %v", path, verr),
				Severity: SeverityError,
			})
			continue
		}

		// Grammar/content checks (RFC-2119, >=1 scenario, duplicate names) —
		// delegated to the single internal/spec delta code path.
		if _, perr := spec.ParseDeltaYAML(data); perr != nil {
			var serr *spec.Error
			if errors.As(perr, &serr) {
				findings = append(findings, Finding{
					File: path, Line: serr.Line, Kind: string(serr.Kind),
					Message: serr.Msg, Severity: SeverityError,
				})
			} else {
				findings = append(findings, Finding{
					File: path, Kind: "delta_parse_error",
					Message: fmt.Sprintf("%s: %v", path, perr), Severity: SeverityError,
				})
			}
		}
	}
	return findings, nil
}

// findSpecYAMLFiles returns every "spec.yaml" file under root, sorted — the
// refine-stage delta artifact (change 007). It is distinct from findSpecFiles
// (spec.md) which still backs HasSpecsDeltas for the not-yet-retargeted
// archive/approve callers.
func findSpecYAMLFiles(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "spec.yaml" {
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

// findSpecFiles returns every "spec.md" file under root, sorted. Retained for
// HasSpecsDeltas (internal/approve, internal/archive), whose retarget to YAML
// is out of this milestone's scope.
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
