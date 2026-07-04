package spec

import (
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "update golden files in testdata/")

func goldenBytes(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch for %s (run `go test -run <this test> -update` to refresh if the change is intentional):\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}

func TestRequirementSet_Render_Golden(t *testing.T) {
	rs := &RequirementSet{
		Before: "# auth Specification\n\n## Purpose\nProvide user authentication for the product.",
		Requirements: []Requirement{
			NewRequirement(
				"Password Login",
				"The system SHALL allow a user to authenticate with a username and password.",
				[]Scenario{{
					Name: "Valid credentials",
					Body: "- **WHEN** a user submits a correct username and password\n- **THEN** the system SHALL grant access",
				}},
			),
			NewRequirement(
				"Session Expiry",
				"Sessions SHALL expire after 30 minutes of inactivity.",
				nil,
			),
		},
		After: "\n",
	}

	goldenBytes(t, "testdata/render_golden.md", rs.Render())
}

func TestRequirementSet_Render_ParseRenderRoundTrip(t *testing.T) {
	// The golden file itself must be a fixed point: parsing it and
	// rendering again must reproduce it byte-for-byte (render(parse(b))
	// stability, the property the M1 DoD demands of the renderer).
	golden, err := os.ReadFile("testdata/render_golden.md")
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}
	rs, err := ParseRequirementSet(golden)
	if err != nil {
		t.Fatalf("ParseRequirementSet(golden): %v", err)
	}
	if got := rs.Render(); string(got) != string(golden) {
		t.Fatalf("Render(Parse(golden)) != golden:\n--- golden ---\n%s\n--- got ---\n%s", golden, got)
	}
}

func TestNewRequirement_EmptyBodyAndScenarios(t *testing.T) {
	r := NewRequirement("Bare", "", nil)
	if r.Raw != "### Requirement: Bare" {
		t.Errorf("Raw = %q", r.Raw)
	}
	if r.Body != "" || len(r.Scenarios) != 0 {
		t.Errorf("Body/Scenarios = %q/%v, want empty", r.Body, r.Scenarios)
	}
}
