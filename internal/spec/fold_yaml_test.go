package spec

import (
	"errors"
	"testing"
)

// baseYAML is a two-requirement living spec used as the fold base.
const baseYAML = `capability: auth
purpose: Authentication.
requirements:
  - name: Password login
    text: The system SHALL allow a user to log in with a password.
    scenarios:
      - name: ok
        when:
          - valid credentials
        then:
          - the system grants a session
  - name: Session expiry
    text: The system SHALL expire sessions after 24 hours.
    scenarios:
      - name: expires
        when:
          - idle 24h
        then:
          - the system requires re-auth
`

func mustParseLivingYAML(t *testing.T, src string) *LivingSpec {
	t.Helper()
	ls, err := ParseLivingSpecYAML([]byte(src))
	if err != nil {
		t.Fatalf("ParseLivingSpecYAML: %v", err)
	}
	return ls
}

func mustParseDeltaYAML(t *testing.T, src string) *SpecDelta {
	t.Helper()
	d, err := ParseDeltaYAML([]byte(src))
	if err != nil {
		t.Fatalf("ParseDeltaYAML: %v", err)
	}
	return d
}

func yamlReqNames(ls *LivingSpec) []string {
	names := make([]string, len(ls.Requirements))
	for i, r := range ls.Requirements {
		names[i] = r.Name
	}
	return names
}

// TestFoldYAML_FourOps exercises all four ops (ADDED/MODIFIED/REMOVED/RENAMED)
// in one delta over the two-requirement base and asserts the folded model.
// Milestone 2 validation contract: "applying a delta exercising all four ops
// to a base living spec yields the expected folded model".
func TestFoldYAML_FourOps(t *testing.T) {
	base := mustParseLivingYAML(t, baseYAML)
	d := mustParseDeltaYAML(t, `capability: auth
deltas:
  - op: RENAMED
    from: Session expiry
    to: Session timeout
  - op: MODIFIED
    requirement:
      name: Password login
      text: The system SHALL allow a user to log in with a username and password.
      scenarios:
        - name: ok
          then:
            - the system MUST grant a session
  - op: REMOVED
    requirement:
      name: Session timeout
  - op: ADDED
    requirement:
      name: Password reset
      text: The system SHALL allow a user to reset a forgotten password.
      scenarios:
        - name: reset
          then:
            - the system sends a reset link
`)

	folded, err := FoldYAML("auth", base, d)
	if err != nil {
		t.Fatalf("FoldYAML: %v", err)
	}

	// RENAMED moved "Session expiry" -> "Session timeout", then REMOVED deleted
	// it; MODIFIED updated "Password login" in place (position preserved);
	// ADDED appended "Password reset".
	got := yamlReqNames(folded)
	want := []string{"Password login", "Password reset"}
	if len(got) != len(want) {
		t.Fatalf("folded requirement names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("folded requirement names = %v, want %v", got, want)
		}
	}

	// MODIFIED replaced the body in place.
	if folded.Requirements[0].Text != "The system SHALL allow a user to log in with a username and password." {
		t.Errorf("MODIFIED body = %q, not replaced", folded.Requirements[0].Text)
	}
	if folded.Capability != "auth" {
		t.Errorf("folded capability = %q, want auth", folded.Capability)
	}
}

// TestFoldYAML_FromEmptyBase folds an ADDED delta over a nil base and asserts a
// structured empty living spec is synthesized (not a markdown skeleton).
func TestFoldYAML_FromEmptyBase(t *testing.T) {
	d := mustParseDeltaYAML(t, `capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system SHALL allow a user to log in with a password.
      scenarios:
        - name: ok
          then:
            - grant a session
`)
	folded, err := FoldYAML("auth", nil, d)
	if err != nil {
		t.Fatalf("FoldYAML(nil base): %v", err)
	}
	if got := yamlReqNames(folded); len(got) != 1 || got[0] != "Password login" {
		t.Fatalf("from-empty fold names = %v, want [Password login]", got)
	}
	if folded.Capability != "auth" {
		t.Errorf("capability = %q, want auth", folded.Capability)
	}
}

// TestParseDeltaYAML_DuplicateRequirementRejected asserts the M2 duplicate-
// requirement rejection: a delta naming the same requirement twice is rejected.
// Milestone 2 validation contract: "a delta naming a duplicate requirement is
// rejected".
func TestParseDeltaYAML_DuplicateRequirementRejected(t *testing.T) {
	_, err := ParseDeltaYAML([]byte(`capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system SHALL allow a user to log in.
      scenarios:
        - name: ok
          then:
            - grant
  - op: MODIFIED
    requirement:
      name: Password login
      text: The system MUST allow a user to log in with 2FA.
      scenarios:
        - name: ok
          then:
            - grant
`))
	if err == nil {
		t.Fatal("ParseDeltaYAML with a duplicate requirement name: want error, got nil")
	}
	if !errors.Is(err, &Error{Kind: KindDuplicateRequirement}) {
		t.Errorf("error = %v, want KindDuplicateRequirement", err)
	}
}

// TestParseLivingSpecYAML_DuplicateRequirementRejected asserts a living spec
// naming the same requirement twice is rejected at parse time (D1).
func TestParseLivingSpecYAML_DuplicateRequirementRejected(t *testing.T) {
	_, err := ParseLivingSpecYAML([]byte(`capability: auth
requirements:
  - name: Password login
    text: The system SHALL allow a user to log in.
  - name: password login
    text: The system MUST allow a user to log in with 2FA.
`))
	if err == nil {
		t.Fatal("ParseLivingSpecYAML with a duplicate requirement name: want error, got nil")
	}
	if !errors.Is(err, &Error{Kind: KindDuplicateRequirement}) {
		t.Errorf("error = %v, want KindDuplicateRequirement", err)
	}
}

// TestParseDeltaYAML_ContentChecks asserts the ported RFC-2119 and
// >=1-scenario checks on the YAML model.
func TestParseDeltaYAML_ContentChecks(t *testing.T) {
	missingKeyword := `capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system lets a user log in.
      scenarios:
        - name: ok
          then:
            - grant
`
	if _, err := ParseDeltaYAML([]byte(missingKeyword)); !errors.Is(err, &Error{Kind: KindMissingRFC2119}) {
		t.Errorf("missing SHALL/MUST: error = %v, want KindMissingRFC2119", err)
	}

	missingScenario := `capability: auth
deltas:
  - op: ADDED
    requirement:
      name: Password login
      text: The system SHALL allow a user to log in.
`
	if _, err := ParseDeltaYAML([]byte(missingScenario)); !errors.Is(err, &Error{Kind: KindMissingScenarioBlock}) {
		t.Errorf("missing scenario: error = %v, want KindMissingScenarioBlock", err)
	}
}

// TestParseDeltaYAML_MalformedYAML asserts a non-YAML byte stream is a decode
// error, not a structural rejection.
func TestParseDeltaYAML_MalformedYAML(t *testing.T) {
	if _, err := ParseDeltaYAML([]byte("capability: auth\ndeltas: [unterminated\n")); err == nil {
		t.Fatal("ParseDeltaYAML on malformed YAML: want error, got nil")
	}
}
