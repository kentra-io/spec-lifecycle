package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lifecycle.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := write(t, `
schemaVersion: 1
specFormat: { convention: openspec, grammar: "1.5.0" }
constitution: { version: "0.1.x" }
consentPolicy: strict
planGranularity: medium
sourceTracking: { type: github-issue, repo: kentra-io/kafka-dq }
changeNaming: "<issue-number>-<slug>"
runtimes: [claude-code, cursor, codex]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SpecFormat.Convention != ConventionOpenSpec || cfg.SpecFormat.Grammar != "1.5.0" {
		t.Errorf("SpecFormat = %+v, want convention=%q grammar=1.5.0", cfg.SpecFormat, ConventionOpenSpec)
	}
	if cfg.Constitution.Version != "0.1.x" {
		t.Errorf("Constitution.Version = %q, want 0.1.x", cfg.Constitution.Version)
	}
	if cfg.ConsentPolicy != ConsentStrict {
		t.Errorf("ConsentPolicy = %q, want %q", cfg.ConsentPolicy, ConsentStrict)
	}
	if cfg.SourceTracking.Repo != "kentra-io/kafka-dq" {
		t.Errorf("SourceTracking.Repo = %q, want kentra-io/kafka-dq", cfg.SourceTracking.Repo)
	}
	if len(cfg.Runtimes) != 3 {
		t.Errorf("Runtimes = %v, want 3 entries", cfg.Runtimes)
	}
}

func TestLoadDefaults(t *testing.T) {
	path := write(t, "schemaVersion: 1\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ConsentPolicy != ConsentStrict {
		t.Errorf("default ConsentPolicy = %q, want %q", cfg.ConsentPolicy, ConsentStrict)
	}
	if cfg.SpecFormat.Convention != ConventionOpenSpec {
		t.Errorf("default SpecFormat.Convention = %q, want %q", cfg.SpecFormat.Convention, ConventionOpenSpec)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yml"))
	if err == nil {
		t.Fatal("Load() error = nil, want an error for a missing file")
	}
}

func TestLoadBadYAML(t *testing.T) {
	path := write(t, "schemaVersion: [1\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want a YAML parse error")
	}
	if !strings.Contains(err.Error(), "not valid YAML") {
		t.Errorf("Load() error = %q, want it to mention \"not valid YAML\"", err.Error())
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "unsupported schemaVersion",
			content: "schemaVersion: 2\n",
			wantErr: `unsupported schemaVersion 2 (this build only supports schemaVersion 1); refusing to run against an unrecognized config schema`,
		},
		{
			name:    "bad consentPolicy",
			content: "schemaVersion: 1\nconsentPolicy: advisory\n",
			wantErr: `field "consentPolicy": must be "strict" or "off" (got "advisory")`,
		},
		{
			name:    "bad specFormat convention",
			content: "schemaVersion: 1\nspecFormat: { convention: openspec2 }\n",
			wantErr: `field "specFormat.convention": must be "openspec" (got "openspec2")`,
		},
		{
			name:    "bad planGranularity",
			content: "schemaVersion: 1\nplanGranularity: chunky\n",
			wantErr: `field "planGranularity": must be one of "coarse", "medium", "fine" (got "chunky")`,
		},
		{
			name:    "bad sourceTracking type",
			content: "schemaVersion: 1\nsourceTracking: { type: trello }\n",
			wantErr: `field "sourceTracking.type": must be one of "github-issue", "generic", "jira", "none" (got "trello")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := write(t, tt.content)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
