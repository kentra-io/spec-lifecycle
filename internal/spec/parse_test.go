package spec

import (
	"errors"
	"strings"
	"testing"
)

const validLivingSpec = `# auth Specification

## Purpose
Provide user authentication for the product.
## Requirements
### Requirement: Password Login
The system SHALL allow a user to authenticate with a username and password.

#### Scenario: Valid credentials
- **WHEN** a user submits a correct username and password
- **THEN** the system SHALL grant access

#### Scenario: Invalid credentials
- **WHEN** a user submits an incorrect password
- **THEN** the system SHALL deny access

### Requirement: Session Expiry
Sessions SHALL expire after 30 minutes of inactivity.

#### Scenario: Session times out
- **WHEN** a session is idle for over 30 minutes
- **THEN** the system SHALL require re-authentication
`

func TestParseRequirementSet_Valid(t *testing.T) {
	rs, err := ParseRequirementSet([]byte(validLivingSpec))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := rs.Title(), "auth Specification"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := rs.Purpose(), "Provide user authentication for the product."; got != want {
		t.Errorf("Purpose() = %q, want %q", got, want)
	}
	if len(rs.Requirements) != 2 {
		t.Fatalf("len(Requirements) = %d, want 2", len(rs.Requirements))
	}

	r0 := rs.Requirements[0]
	if r0.Name != "Password Login" {
		t.Errorf("Requirements[0].Name = %q", r0.Name)
	}
	if !strings.Contains(r0.Body, "SHALL allow a user") {
		t.Errorf("Requirements[0].Body = %q", r0.Body)
	}
	if len(r0.Scenarios) != 2 {
		t.Fatalf("len(Requirements[0].Scenarios) = %d, want 2", len(r0.Scenarios))
	}
	if r0.Scenarios[0].Name != "Valid credentials" {
		t.Errorf("Requirements[0].Scenarios[0].Name = %q", r0.Scenarios[0].Name)
	}
	if !strings.Contains(r0.Scenarios[0].Body, "**WHEN** a user submits a correct") {
		t.Errorf("Requirements[0].Scenarios[0].Body = %q", r0.Scenarios[0].Body)
	}
	if !strings.HasPrefix(r0.Raw, "### Requirement: Password Login\n") {
		t.Errorf("Requirements[0].Raw does not start with its header line: %q", r0.Raw)
	}

	r1 := rs.Requirements[1]
	if r1.Name != "Session Expiry" || len(r1.Scenarios) != 1 {
		t.Errorf("Requirements[1] = %+v", r1)
	}
}

func TestParseRequirementSet_CRLF(t *testing.T) {
	crlf := strings.ReplaceAll(validLivingSpec, "\n", "\r\n")
	rs, err := ParseRequirementSet([]byte(crlf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rs.Requirements) != 2 {
		t.Fatalf("len(Requirements) = %d, want 2", len(rs.Requirements))
	}
	if strings.Contains(rs.Requirements[0].Raw, "\r") {
		t.Errorf("Raw retains CR: %q", rs.Requirements[0].Raw)
	}
}

func TestParseRequirementSet_NoRequirementsSection(t *testing.T) {
	rs, err := ParseRequirementSet([]byte("# auth Specification\n\n## Purpose\nTBD.\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rs.Requirements) != 0 {
		t.Errorf("len(Requirements) = %d, want 0", len(rs.Requirements))
	}
	if rs.After != "\n" {
		t.Errorf("After = %q, want %q", rs.After, "\n")
	}
	// Must still round-trip and be extendable by a future fold.
	out := rs.Render()
	rs2, err := ParseRequirementSet(out)
	if err != nil {
		t.Fatalf("re-parse of skeleton render failed: %v", err)
	}
	if len(rs2.Requirements) != 0 {
		t.Errorf("re-parsed skeleton has %d requirements, want 0", len(rs2.Requirements))
	}
}

func TestParseRequirementSet_CodeFenceHidesHeaderLookingText(t *testing.T) {
	src := "# cap Specification\n\n## Purpose\nP.\n## Requirements\n### Requirement: Real One\nThe system SHALL do a thing.\n\n#### Scenario: it happens\nExample:\n```\n### Requirement: Fake One\n#### Scenario: also fake\n```\nEnd of body.\n"
	rs, err := ParseRequirementSet([]byte(src))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rs.Requirements) != 1 {
		t.Fatalf("len(Requirements) = %d, want 1 (fenced fake headers must not be parsed)", len(rs.Requirements))
	}
	if len(rs.Requirements[0].Scenarios) != 1 {
		t.Fatalf("len(Scenarios) = %d, want 1", len(rs.Requirements[0].Scenarios))
	}
	if !strings.Contains(rs.Requirements[0].Scenarios[0].Body, "### Requirement: Fake One") {
		t.Errorf("fenced content lost from scenario body: %q", rs.Requirements[0].Scenarios[0].Body)
	}
}

func TestParseRequirementSet_Malformed(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantKind Kind
		wantLine int
	}{
		{
			name: "requirement header outside Requirements section",
			src: "# cap Specification\n\n### Requirement: Stray\nBody SHALL do it.\n\n" +
				"## Requirements\n",
			wantKind: KindRequirementOutsideSection,
			wantLine: 3,
		},
		{
			name:     "delta header inside living spec",
			src:      "# cap Specification\n\n## ADDED Requirements\n### Requirement: X\nSHALL.\n",
			wantKind: KindDeltaHeaderInLivingSpec,
			wantLine: 3,
		},
		{
			name:     "empty requirement name",
			src:      "## Requirements\n### Requirement:   \nBody.\n",
			wantKind: KindMissingRequirementName,
			wantLine: 2,
		},
		{
			name:     "empty scenario name",
			src:      "## Requirements\n### Requirement: A\nBody SHALL.\n\n#### Scenario:   \nstuff\n",
			wantKind: KindMissingScenarioName,
			wantLine: 5,
		},
		{
			name: "duplicate requirement name",
			src: "## Requirements\n### Requirement: A\nBody.\n\n" +
				"### Requirement: A\nBody again.\n",
			wantKind: KindDuplicateRequirement,
			wantLine: 5,
		},
		{
			name: "duplicate requirement name is case-insensitive",
			src: "## Requirements\n### Requirement: A\nBody.\n\n" +
				"### Requirement: a\nBody again.\n",
			wantKind: KindDuplicateRequirement,
			wantLine: 5,
		},
		{
			name: "duplicate scenario name",
			src: "## Requirements\n### Requirement: A\nBody.\n\n" +
				"#### Scenario: S\nx\n\n#### Scenario: S\ny\n",
			wantKind: KindDuplicateScenario,
			wantLine: 8,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRequirementSet([]byte(tc.src))
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
			if perr.Line != tc.wantLine {
				t.Errorf("Line = %d, want %d (msg: %s)", perr.Line, tc.wantLine, perr.Msg)
			}
		})
	}
}

func TestError_Is(t *testing.T) {
	_, err := ParseRequirementSet([]byte("## Requirements\n### Requirement:   \nBody.\n"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, &Error{Kind: KindMissingRequirementName}) {
		t.Errorf("errors.Is with Kind sentinel should match: %v", err)
	}
	if errors.Is(err, &Error{Kind: KindDuplicateRequirement}) {
		t.Errorf("errors.Is should not match a different Kind")
	}
}
