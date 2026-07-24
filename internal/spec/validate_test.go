package spec

import (
	"strings"
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

// A spec-delta YAML exercising all four ops, conforming to the published
// schema (the ADDED/MODIFIED requirement carries text + a scenario; REMOVED
// is name-only; RENAMED carries from/to).
const conformingDeltaYAML = `
capability: status-reporting
deltas:
  - op: RENAMED
    from: Old capability warning
    to: Machine-readable capability warnings
  - op: REMOVED
    requirement:
      name: Retired requirement
  - op: MODIFIED
    requirement:
      name: Machine-readable capability warnings
      text: The system SHALL surface oversized capabilities as warnings.
      scenarios:
        - name: YAML status output with an oversized capability
          given: ["openspec/specs/auth/spec.md exceeds the threshold"]
          when: ["lifecycle status --format yaml runs"]
          then: ["the capabilityWarnings sequence contains an entry for auth"]
  - op: ADDED
    requirement:
      name: New feature intake
      text: The system SHALL draft an issue from an intake request.
      scenarios:
        - name: Human confirms the drafted issue
          given: ["a drafted issue exists"]
          when: ["the human confirms it"]
          then: ["the change folder is created"]
`

const conformingLivingSpecYAML = `
capability: status-reporting
purpose: Report the lifecycle status of a project's changes.
requirements:
  - name: New feature intake
    text: The system SHALL draft an issue from an intake request.
    scenarios:
      - name: Human confirms the drafted issue
        given: ["a drafted issue exists"]
        when: ["the human confirms it"]
        then: ["the change folder is created"]
`

func TestValidateYAML_ConformingSpecDelta(t *testing.T) {
	sch, err := SpecDeltaSchema()
	if err != nil {
		t.Fatalf("SpecDeltaSchema: %v", err)
	}
	if err := ValidateYAMLBytes([]byte(conformingDeltaYAML), sch); err != nil {
		t.Fatalf("conforming spec-delta was rejected: %v", err)
	}
}

func TestValidateYAML_ConformingLivingSpec(t *testing.T) {
	sch, err := LivingSpecSchema()
	if err != nil {
		t.Fatalf("LivingSpecSchema: %v", err)
	}
	if err := ValidateYAMLBytes([]byte(conformingLivingSpecYAML), sch); err != nil {
		t.Fatalf("conforming living spec was rejected: %v", err)
	}
}

// TestValidateYAML_SchemaViolationNamesPath realises the spec-format scenario
// "YAML is validated against the published schema": a document that violates
// the schema is rejected with the offending path named.
func TestValidateYAML_SchemaViolationNamesPath(t *testing.T) {
	sch, err := SpecDeltaSchema()
	if err != nil {
		t.Fatalf("SpecDeltaSchema: %v", err)
	}

	// op "DELETED" is not one of ADDED/MODIFIED/REMOVED/RENAMED — the
	// enum on /deltas/0/op is the offending node.
	const violating = `
capability: status-reporting
deltas:
  - op: DELETED
    requirement:
      name: X
`
	err = ValidateYAMLBytes([]byte(violating), sch)
	if err == nil {
		t.Fatal("schema-violating spec-delta was accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "/deltas/0/op") {
		t.Errorf("error does not name the offending path /deltas/0/op: %v", err)
	}
}

// TestValidateYAML_UnknownFieldRejected confirms additionalProperties:false
// is enforced — a stray key is a schema violation naming its path.
func TestValidateYAML_UnknownFieldRejected(t *testing.T) {
	sch, err := LivingSpecSchema()
	if err != nil {
		t.Fatalf("LivingSpecSchema: %v", err)
	}
	const violating = `
capability: status-reporting
requirements:
  - name: A requirement
    text: The system SHALL do a thing.
    bogus: nope
`
	err = ValidateYAMLBytes([]byte(violating), sch)
	if err == nil {
		t.Fatal("living spec with an unknown requirement field was accepted, want rejection")
	}
	if !strings.Contains(err.Error(), "/requirements/0") {
		t.Errorf("error does not name the offending path /requirements/0: %v", err)
	}
}

// TestDecodePreservesOrder confirms requirements and scenarios decode into
// ordered sequences (design D1): author order is preserved, not reordered as
// a map would.
func TestDecodePreservesOrder(t *testing.T) {
	const src = `
capability: ordering
requirements:
  - name: Zeta
    scenarios:
      - name: third
      - name: first
      - name: second
  - name: Alpha
  - name: Mu
`
	var ls LivingSpec
	if err := yaml.Unmarshal([]byte(src), &ls); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	wantReqs := []string{"Zeta", "Alpha", "Mu"}
	if len(ls.Requirements) != len(wantReqs) {
		t.Fatalf("got %d requirements, want %d", len(ls.Requirements), len(wantReqs))
	}
	for i, want := range wantReqs {
		if ls.Requirements[i].Name != want {
			t.Errorf("requirement[%d] = %q, want %q (order not preserved)", i, ls.Requirements[i].Name, want)
		}
	}
	wantScenarios := []string{"third", "first", "second"}
	for i, want := range wantScenarios {
		if ls.Requirements[0].Scenarios[i].Name != want {
			t.Errorf("scenario[%d] = %q, want %q (order not preserved)", i, ls.Requirements[0].Scenarios[i].Name, want)
		}
	}
}

// TestDeltaEntryDecode confirms the op-tagged delta entries decode into the
// structured model with from/to for renames and a nil requirement there.
func TestDeltaEntryDecode(t *testing.T) {
	var sd SpecDelta
	if err := yaml.Unmarshal([]byte(conformingDeltaYAML), &sd); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(sd.Deltas) != 4 {
		t.Fatalf("got %d delta entries, want 4", len(sd.Deltas))
	}
	if sd.Deltas[0].Op != OpRenamed || sd.Deltas[0].From != "Old capability warning" || sd.Deltas[0].Requirement != nil {
		t.Errorf("RENAMED entry decoded wrong: %+v", sd.Deltas[0])
	}
	if sd.Deltas[1].Op != OpRemoved || sd.Deltas[1].Requirement == nil || sd.Deltas[1].Requirement.Name != "Retired requirement" {
		t.Errorf("REMOVED entry decoded wrong: %+v", sd.Deltas[1])
	}
	if sd.Deltas[3].Op != OpAdded || sd.Deltas[3].Requirement == nil || len(sd.Deltas[3].Requirement.Scenarios) != 1 {
		t.Errorf("ADDED entry decoded wrong: %+v", sd.Deltas[3])
	}
}
