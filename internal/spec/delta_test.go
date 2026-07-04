package spec

import (
	"errors"
	"testing"
)

const validDelta = `# auth

## ADDED Requirements

### Requirement: Password Login
The system SHALL allow a user to authenticate with a username and password.

#### Scenario: Valid credentials
- **WHEN** a user submits a correct username and password
- **THEN** the system SHALL grant access

## MODIFIED Requirements

### Requirement: Session Expiry
Sessions SHALL expire after 15 minutes of inactivity.

#### Scenario: Session times out
- **WHEN** a session is idle for over 15 minutes
- **THEN** the system SHALL require re-authentication

## REMOVED Requirements

### Requirement: Legacy Cookie Auth

**Reason**: superseded by the session-token flow.

## RENAMED Requirements
- FROM: ` + "`### Requirement: Basic Authentication`" + `
- TO: ` + "`### Requirement: Email Authentication`" + `
`

func TestParseDelta_Valid(t *testing.T) {
	d, err := ParseDelta([]byte(validDelta))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !d.Present.Added || !d.Present.Modified || !d.Present.Removed || !d.Present.Renamed {
		t.Fatalf("Present = %+v, want all four true", d.Present)
	}

	if len(d.Added) != 1 || d.Added[0].Name != "Password Login" {
		t.Errorf("Added = %+v", d.Added)
	}
	if len(d.Added[0].Scenarios) != 1 {
		t.Errorf("Added[0].Scenarios = %+v", d.Added[0].Scenarios)
	}

	if len(d.Modified) != 1 || d.Modified[0].Name != "Session Expiry" {
		t.Errorf("Modified = %+v", d.Modified)
	}

	if len(d.Removed) != 1 || d.Removed[0] != "Legacy Cookie Auth" {
		t.Errorf("Removed = %+v", d.Removed)
	}

	if len(d.Renamed) != 1 || d.Renamed[0] != (Rename{From: "Basic Authentication", To: "Email Authentication"}) {
		t.Errorf("Renamed = %+v", d.Renamed)
	}
}

func TestParseDelta_RemovedBulletForm(t *testing.T) {
	src := "## REMOVED Requirements\n- `### Requirement: Old Thing`\n- `### Requirement: Other Thing`\n"
	d, err := ParseDelta([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Removed) != 2 || d.Removed[0] != "Old Thing" || d.Removed[1] != "Other Thing" {
		t.Errorf("Removed = %+v", d.Removed)
	}
}

func TestParseDelta_Malformed(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantKind Kind
	}{
		{
			name:     "no delta sections at all",
			src:      "# Just a title\n\nSome prose, no delta headers.\n",
			wantKind: KindNoDeltaSections,
		},
		{
			name: "ADDED missing SHALL/MUST",
			src: "## ADDED Requirements\n### Requirement: X\nThe system does the thing.\n\n" +
				"#### Scenario: it happens\n- **WHEN** x\n- **THEN** y\n",
			wantKind: KindMissingRFC2119,
		},
		{
			name:     "ADDED missing scenario",
			src:      "## ADDED Requirements\n### Requirement: X\nThe system SHALL do the thing.\n",
			wantKind: KindMissingScenarioBlock,
		},
		{
			name: "duplicate requirement within ADDED",
			src: "## ADDED Requirements\n### Requirement: X\nSHALL.\n\n#### Scenario: s\nb\n\n" +
				"### Requirement: X\nSHALL again.\n\n#### Scenario: s2\nb\n",
			wantKind: KindDuplicateRequirement,
		},
		{
			name:     "empty ADDED section",
			src:      "## ADDED Requirements\n\nJust prose, no requirement header.\n",
			wantKind: KindEmptyDeltaSection,
		},
		{
			name:     "duplicate ADDED section header",
			src:      "## ADDED Requirements\n### Requirement: X\nSHALL.\n\n#### Scenario: s\nb\n\n## ADDED Requirements\n### Requirement: Y\nSHALL.\n\n#### Scenario: s2\nb\n",
			wantKind: KindDuplicateDeltaSection,
		},
		{
			name:     "dangling RENAMED FROM with no TO",
			src:      "## RENAMED Requirements\n- FROM: `### Requirement: A`\n",
			wantKind: KindDanglingRename,
		},
		{
			name: "RENAMED TO collides with ADDED",
			src: "## ADDED Requirements\n### Requirement: New Name\nSHALL.\n\n#### Scenario: s\nb\n\n" +
				"## RENAMED Requirements\n- FROM: `### Requirement: Old Name`\n- TO: `### Requirement: New Name`\n",
			wantKind: KindConflictingOps,
		},
		{
			name: "name present in both ADDED and REMOVED",
			src: "## ADDED Requirements\n### Requirement: X\nSHALL.\n\n#### Scenario: s\nb\n\n" +
				"## REMOVED Requirements\n### Requirement: X\n",
			wantKind: KindConflictingOps,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDelta([]byte(tc.src))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			var perr *Error
			if !errors.As(err, &perr) {
				t.Fatalf("error is not *spec.Error: %T: %v", err, err)
			}
			if perr.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q (msg: %s)", perr.Kind, tc.wantKind, perr.Msg)
			}
		})
	}
}
