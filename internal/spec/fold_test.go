package spec

import (
	"errors"
	"strings"
	"testing"
)

func mustParseBaseTwoReqs(t *testing.T) *RequirementSet {
	t.Helper()
	rs, err := ParseRequirementSet([]byte(baseTwoReqs))
	if err != nil {
		t.Fatalf("ParseRequirementSet: %v", err)
	}
	return rs
}

func mustParseDelta(t *testing.T, src string) *Delta {
	t.Helper()
	d, err := ParseDelta([]byte(src))
	if err != nil {
		t.Fatalf("ParseDelta: %v", err)
	}
	return d
}

const baseTwoReqs = `# auth Specification

## Purpose
Authentication.

## Requirements
### Requirement: Password login
The system SHALL allow a user to log in with a password.

#### Scenario: ok
- **WHEN** valid credentials
- **THEN** the system SHALL grant a session

### Requirement: Session expiry
The system SHALL expire sessions after 24 hours.

#### Scenario: expires
- **WHEN** idle 24h
- **THEN** the system SHALL require re-auth
`

func TestFold_NewCapabilitySkeleton(t *testing.T) {
	d := mustParseDelta(t, `## ADDED Requirements
### Requirement: Password login
The system SHALL allow a user to log in with a password.

#### Scenario: ok
- **WHEN** valid credentials
- **THEN** the system SHALL grant a session
`)
	rs, err := Fold("auth", "001-add-password-login", nil, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	if got, want := rs.Title(), "auth Specification"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	wantPurpose := "TBD - created by archiving change 001-add-password-login. Update Purpose after archive."
	if got := rs.Purpose(); got != wantPurpose {
		t.Errorf("Purpose = %q, want %q", got, wantPurpose)
	}
	if len(rs.Requirements) != 1 || rs.Requirements[0].Name != "Password login" {
		t.Errorf("Requirements = %+v", rs.Requirements)
	}
}

func TestFold_AddedAppendsInOrder(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, `## ADDED Requirements
### Requirement: MFA
The system SHALL support multi-factor authentication.

#### Scenario: mfa
- **WHEN** mfa enabled
- **THEN** the system SHALL require a second factor
`)
	rs, err := Fold("auth", "002-add-mfa", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	names := reqNames(rs)
	want := []string{"Password login", "Session expiry", "MFA"}
	assertNames(t, names, want)
}

func TestFold_ModifiedReplacesInPlace(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, `## MODIFIED Requirements
### Requirement: Session expiry
The system SHALL expire sessions after 72 hours.

#### Scenario: expires
- **WHEN** idle 72h
- **THEN** the system SHALL require re-auth
`)
	rs, err := Fold("auth", "003-widen-session-expiry", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	assertNames(t, reqNames(rs), []string{"Password login", "Session expiry"})
	if got := rs.Requirements[1].Body; got == "" {
		t.Fatal("modified requirement body empty")
	}
	if !rfc2119Re.MatchString(rs.Requirements[1].Body) {
		t.Errorf("modified body lost RFC2119 content: %q", rs.Requirements[1].Body)
	}
	if want := "72 hours"; !strings.Contains(rs.Requirements[1].Raw, want) {
		t.Errorf("Raw = %q, want to contain %q", rs.Requirements[1].Raw, want)
	}
}

func TestFold_RemovedDeletesFromMiddle(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, "## REMOVED Requirements\n### Requirement: Password login\n")
	rs, err := Fold("auth", "004-remove-password-login", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	assertNames(t, reqNames(rs), []string{"Session expiry"})
}

func TestFold_RenameOnlyMovesToEnd(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, "## RENAMED Requirements\n- FROM: `### Requirement: Password login`\n- TO: `### Requirement: Username and password login`\n")
	rs, err := Fold("auth", "005-rename-password-login", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	// Locks the fold-order quirk from testdata/conformance case 05: a
	// rename-only delta moves the renamed requirement to the END.
	assertNames(t, reqNames(rs), []string{"Session expiry", "Username and password login"})
}

func TestFold_RenameThenModifyKeepsOriginalPosition(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, `## RENAMED Requirements
- FROM: `+"`### Requirement: Session expiry`"+`
- TO: `+"`### Requirement: Session inactivity timeout`"+`

## MODIFIED Requirements
### Requirement: Session inactivity timeout
The system SHALL expire sessions after 72 hours.

#### Scenario: expires
- **WHEN** idle 72h
- **THEN** the system SHALL require re-auth
`)
	rs, err := Fold("auth", "006-rename-and-widen-session-expiry", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	// Locks case 06: RENAMED+MODIFIED of the same requirement in one delta
	// keeps its ORIGINAL position — contrast with the rename-only case
	// above, where it moves to the end.
	assertNames(t, reqNames(rs), []string{"Password login", "Session inactivity timeout"})
	if !strings.Contains(rs.Requirements[1].Raw, "72 hours") {
		t.Errorf("modified rename target missing new body: %q", rs.Requirements[1].Raw)
	}
}

func TestFold_MultiOpNonOverlapping(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, `## ADDED Requirements
### Requirement: Lockout
The system SHALL lock an account after 5 failed attempts.

#### Scenario: locked
- **WHEN** 5 failures
- **THEN** the system SHALL lock the account

## MODIFIED Requirements
### Requirement: Session expiry
The system SHALL expire sessions after 1 hour.

#### Scenario: expires
- **WHEN** idle 1h
- **THEN** the system SHALL require re-auth

## REMOVED Requirements
### Requirement: Password login
`)
	rs, err := Fold("auth", "007-auth-cleanup-and-lockout", base, d)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	assertNames(t, reqNames(rs), []string{"Session expiry", "Lockout"})
}

// --- Fold edge cases the plan pins as spike-3 (lock, don't silently drop) ---

func TestFold_EdgeCases(t *testing.T) {
	base := mustParseBaseTwoReqs(t)

	tests := []struct {
		name     string
		delta    string
		wantKind Kind
	}{
		{
			name:     "MODIFIED of nonexistent requirement",
			delta:    "## MODIFIED Requirements\n### Requirement: Does Not Exist\nThe system SHALL do a thing.\n\n#### Scenario: s\n- **WHEN** x\n- **THEN** the system SHALL y\n",
			wantKind: KindFoldModifyMissing,
		},
		{
			name:     "REMOVED of nonexistent requirement",
			delta:    "## REMOVED Requirements\n### Requirement: Does Not Exist\n",
			wantKind: KindFoldRemoveMissing,
		},
		{
			name:     "RENAMED FROM of nonexistent requirement",
			delta:    "## RENAMED Requirements\n- FROM: `### Requirement: Does Not Exist`\n- TO: `### Requirement: New Name`\n",
			wantKind: KindFoldRenameSourceMissing,
		},
		{
			name:     "ADDED of an existing name",
			delta:    "## ADDED Requirements\n### Requirement: Password login\nThe system SHALL do a new thing.\n\n#### Scenario: s\n- **WHEN** x\n- **THEN** the system SHALL y\n",
			wantKind: KindFoldAddExists,
		},
		{
			name:     "RENAMED TO collides with an untouched existing requirement",
			delta:    "## RENAMED Requirements\n- FROM: `### Requirement: Password login`\n- TO: `### Requirement: Session expiry`\n",
			wantKind: KindFoldRenameTargetExists,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustParseDelta(t, tc.delta)
			_, err := Fold("auth", "999-edge-case", base, d)
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

func TestFold_Determinism(t *testing.T) {
	base := mustParseBaseTwoReqs(t)
	d := mustParseDelta(t, `## ADDED Requirements
### Requirement: MFA
The system SHALL support multi-factor authentication.

#### Scenario: mfa
- **WHEN** mfa enabled
- **THEN** the system SHALL require a second factor

## MODIFIED Requirements
### Requirement: Session expiry
The system SHALL expire sessions after 72 hours.

#### Scenario: expires
- **WHEN** idle 72h
- **THEN** the system SHALL require re-auth

## REMOVED Requirements
### Requirement: Password login
`)

	var first []byte
	for i := 0; i < 100; i++ {
		rs, err := Fold("auth", "det-check", base, d)
		if err != nil {
			t.Fatalf("iteration %d: Fold: %v", i, err)
		}
		out := rs.Render()
		if i == 0 {
			first = out
			continue
		}
		if string(out) != string(first) {
			t.Fatalf("iteration %d: fold output diverged:\n--- first ---\n%s\n--- this ---\n%s", i, first, out)
		}
	}
}

func reqNames(rs *RequirementSet) []string {
	names := make([]string, len(rs.Requirements))
	for i, r := range rs.Requirements {
		names[i] = r.Name
	}
	return names
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
}
