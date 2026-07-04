package schema

import (
	"fmt"

	yaml "go.yaml.in/yaml/v3"
)

// Definition is the parsed shape of the embedded schema.yaml — the
// SINGLE runtime consumer of its structured content. This is distinct
// from Install/Verify (schema.go), which treat every embedded asset,
// including schema.yaml itself, as an opaque byte blob written to a
// project for format-compatibility/documentation only (schema.go's
// package doc: "nothing at runtime reads this descriptor BACK" — meaning
// the on-disk, project-local copy Install writes). Load instead reads the
// original asset the embed directive baked into the binary at build
// time — it is not the "[experimental] custom-schema loader" risk class
// that package doc disclaims; that phrase is about re-reading a
// project's on-disk copy as a configurable input, which lifecycle never
// does.
//
// implementation-plan.md §2.6 directs `lifecycle approve` to "resolve the
// stage's artifact set via the schema's generates: globs" — Definition
// and Generates are what let internal/approve do that without hand-typing
// a second copy of "proposal.md" / "specs/**/spec.md" / "design.md" /
// "tasks.md" (schema.yaml already IS that source of truth).
type Definition struct {
	Name        string     `yaml:"name"`
	Version     int        `yaml:"version"`
	Description string     `yaml:"description"`
	Artifacts   []Artifact `yaml:"artifacts"`
}

// Artifact is one schema.yaml `artifacts[]` entry.
type Artifact struct {
	ID          string   `yaml:"id"`
	Generates   string   `yaml:"generates"`
	Description string   `yaml:"description"`
	Template    string   `yaml:"template"`
	Instruction string   `yaml:"instruction"`
	Requires    []string `yaml:"requires"`
}

// Load parses the embedded schema.yaml into a Definition.
func Load() (*Definition, error) {
	data, err := assets.ReadFile("schema.yaml")
	if err != nil {
		return nil, fmt.Errorf("schema: reading embedded schema.yaml: %w", err)
	}
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("schema: embedded schema.yaml: not valid YAML: %w", err)
	}
	return &def, nil
}

// Generates returns the generates: glob for the artifact with the given
// id (e.g. "proposal", "specs", "design", "tasks"), or "" if id is
// unknown to this schema.
func (d *Definition) Generates(id string) string {
	for _, a := range d.Artifacts {
		if a.ID == id {
			return a.Generates
		}
	}
	return ""
}
