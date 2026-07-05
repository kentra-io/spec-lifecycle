// Package config loads and validates lifecycle.yml (spec-lifecycle.md
// §10, implementation-plan.md §2.10): the project config `lifecycle init`
// writes and every command reads, at repo root, sibling of openspec/.
// Mirrors the sibling adr-sourced-constitution primitive's own
// internal/config in shape and validation style deliberately (plain
// yaml.Unmarshal into a versioned struct, no Viper, unknown schemaVersion
// refuses outright, no migration machinery) — implementation-plan.md §2.10
// pins this explicitly: "mirror the constitution's config loader".
package config

import (
	"fmt"
	"os"

	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
)

// SchemaVersion is the only schemaVersion this build understands.
// implementation-plan.md §2.10: "unknown schemaVersion ⇒ refuse; no
// migration machinery in v1."
const SchemaVersion = 1

// Consent policy vocabulary (spec-lifecycle.md §10, parity with
// constitution.yml's consent.policy): "strict" | "off".
const (
	ConsentStrict = "strict"
	ConsentOff    = "off"
)

// ConventionOpenSpec is the only SpecFormat.Convention value v1 defines
// (spec-lifecycle.md §10); the field is a documentation/conformance
// anchor, never an installed dependency (implementation-plan.md §0.5).
const ConventionOpenSpec = "openspec"

// SourceTracking.Type values this build recognizes (spec-lifecycle.md §10
// shows "github-issue"; kept open beyond that single value since
// lifecycle.yml's sourceTracking is documentation/join-key metadata, not
// something this build branches its own behavior on the way the
// constitution's sourceTracking.type does).
const (
	SourceTrackingGitHubIssue = "github-issue"
	SourceTrackingGeneric     = "generic"
	SourceTrackingJira        = "jira"
	SourceTrackingNone        = "none"
)

// Runtimes vocabulary (spec-lifecycle.md §9.2/§10): the skill fan-out
// targets `lifecycle init` maps to trees (internal/scaffold.BuildSkillItems)
// — claude-code → .claude/skills/, cursor → .cursor/skills/, codex →
// .agents/skills/ (the cross-agent AGENTS.md convention). Gemini CLI was
// dropped from the default list (Google EOL'd it 2026-06-18, §10).
const (
	RuntimeClaudeCode = "claude-code"
	RuntimeCursor     = "cursor"
	RuntimeCodex      = "codex"
)

// DefaultRuntimes is the fan-out target list `lifecycle init` seeds a fresh
// lifecycle.yml with (spec-lifecycle.md §10 example).
var DefaultRuntimes = []string{RuntimeClaudeCode, RuntimeCursor, RuntimeCodex}

var validRuntimes = map[string]bool{
	RuntimeClaudeCode: true,
	RuntimeCursor:     true,
	RuntimeCodex:      true,
}

// Config is the lifecycle.yml schema (spec-lifecycle.md §10).
type Config struct {
	SchemaVersion int          `yaml:"schemaVersion"`
	SpecFormat    SpecFormat   `yaml:"specFormat"`
	Constitution  Constitution `yaml:"constitution"`
	// ConsentPolicy gates every mutating verb (`lifecycle approve`
	// today): "strict" refuses to write without an explicit --approve
	// (or an interactive confirmation); "off" removes the gate. Defaults
	// to "strict" when absent (fail-closed, matching constitution.yml's
	// own default).
	ConsentPolicy string `yaml:"consentPolicy"`
	// PlanGranularity sizes tasks.md's Steps (spec-lifecycle.md §4.2,
	// §10): "coarse" | "medium" | "fine". Not enforced by any M3 code
	// path — advisory content the plan-stage skill reads — but the
	// value is still validated here so a typo is caught at config-load
	// time, not silently ignored.
	PlanGranularity string         `yaml:"planGranularity,omitempty"`
	SourceTracking  SourceTracking `yaml:"sourceTracking"`
	// ChangeNaming documents the change-folder naming convention (spec
	// §10, e.g. "<issue-number>-<slug>"). Advisory only — no code path
	// in this build enforces or derives folder names from it.
	ChangeNaming string   `yaml:"changeNaming,omitempty"`
	Runtimes     []string `yaml:"runtimes,omitempty"`
}

// SpecFormat records the on-disk format convention lifecycle conforms to
// (implementation-plan.md §2.10): NOT an installed runtime dependency —
// see implementation-plan.md §0.5/"Option B".
type SpecFormat struct {
	Convention string `yaml:"convention"`
	Grammar    string `yaml:"grammar"`
}

// Constitution pins the companion adr-sourced-constitution primitive's
// version (spec-lifecycle.md §7 item 5, §10) so the deviation.json
// contract can't silently drift across independent release cadences.
// Version may carry an "x" wildcard component (e.g. "0.1.x") —
// internal/constitution.CheckVersion interprets that shape.
type Constitution struct {
	Version string `yaml:"version"`
}

// SourceTracking keeps the single join key (issue number) across change
// folders, ADR source fields, and telemetry (spec-lifecycle.md §10).
type SourceTracking struct {
	Type string `yaml:"type"`
	Repo string `yaml:"repo"`
}

var validPlanGranularities = map[string]bool{"coarse": true, "medium": true, "fine": true}

var validSourceTrackingTypes = map[string]bool{
	SourceTrackingGitHubIssue: true,
	SourceTrackingGeneric:     true,
	SourceTrackingJira:        true,
	SourceTrackingNone:        true,
}

// Load reads and validates lifecycle.yml at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %w", path, err)
	}

	if err := cfg.validate(path); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks c against this build's schema (the same rules Load
// applies to a just-read file), applying the same fill-in-the-default
// normalization for empty optional fields. path is used only to word
// error messages consistently with Load's. Exported so `lifecycle init`
// (internal/scaffold) can validate an in-memory config it composed from
// flags/defaults BEFORE writing it to disk (implementation-plan.md §2.9
// step e), mirroring the constitution's own init.go pre-flight-before-
// persist discipline.
func (c *Config) Validate(path string) error {
	return c.validate(path)
}

func (c *Config) validate(path string) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf(
			"%s: unsupported schemaVersion %d (this build only supports schemaVersion %d); refusing to run against an unrecognized config schema",
			path, c.SchemaVersion, SchemaVersion,
		)
	}

	if c.ConsentPolicy == "" {
		c.ConsentPolicy = ConsentStrict
	} else if c.ConsentPolicy != ConsentStrict && c.ConsentPolicy != ConsentOff {
		return fmt.Errorf(
			"%s: field %q: must be %q or %q (got %q)",
			path, "consentPolicy", ConsentStrict, ConsentOff, c.ConsentPolicy,
		)
	}

	if c.SpecFormat.Convention == "" {
		c.SpecFormat.Convention = ConventionOpenSpec
	} else if c.SpecFormat.Convention != ConventionOpenSpec {
		return fmt.Errorf(
			"%s: field %q: must be %q (got %q) — this build only implements the OpenSpec on-disk format",
			path, "specFormat.convention", ConventionOpenSpec, c.SpecFormat.Convention,
		)
	}

	if c.PlanGranularity != "" && !validPlanGranularities[c.PlanGranularity] {
		return fmt.Errorf(
			"%s: field %q: must be one of %q, %q, %q (got %q)",
			path, "planGranularity", "coarse", "medium", "fine", c.PlanGranularity,
		)
	}

	if c.SourceTracking.Type != "" && !validSourceTrackingTypes[c.SourceTracking.Type] {
		return fmt.Errorf(
			"%s: field %q: must be one of %q, %q, %q, %q (got %q)",
			path, "sourceTracking.type",
			SourceTrackingGitHubIssue, SourceTrackingGeneric, SourceTrackingJira, SourceTrackingNone,
			c.SourceTracking.Type,
		)
	}

	for _, r := range c.Runtimes {
		if !validRuntimes[r] {
			return fmt.Errorf(
				"%s: field %q: unknown runtime %q (allowed: %q, %q, %q)",
				path, "runtimes", r, RuntimeClaudeCode, RuntimeCursor, RuntimeCodex,
			)
		}
	}

	return nil
}

// Default returns the lifecycle.yml `lifecycle init` seeds a project with
// when none exists yet (spec-lifecycle.md §10's example, minus the
// project-specific sourceTracking.repo and constitution.version, which the
// caller fills in — the detected constitution version pin, resp. any
// --source-repo/--source-type the human supplied).
func Default() *Config {
	return &Config{
		SchemaVersion:   SchemaVersion,
		SpecFormat:      SpecFormat{Convention: ConventionOpenSpec, Grammar: "1.5.0"},
		ConsentPolicy:   ConsentStrict,
		PlanGranularity: "medium",
		SourceTracking:  SourceTracking{Type: SourceTrackingNone},
		ChangeNaming:    "<issue-number>-<slug>",
		Runtimes:        append([]string(nil), DefaultRuntimes...),
	}
}

// Save atomically writes cfg to path as lifecycle.yml. Used only to seed a
// brand-new file (`lifecycle init`, implementation-plan.md §2.9 step e) —
// once written, lifecycle.yml is a human-owned file no other verb
// mutates, so there is no surgical-edit concern here the way there is for
// openspec/config.yaml (internal/scaffold.EnsureProjectConfig).
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return atomicwrite.WriteFile(path, data, 0o644)
}
