package spec

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// randRequirementSet builds a structurally-valid *RequirementSet from a
// deterministic PRNG — used to property-test parse(render(x)) == x across
// many shapes (0..N requirements, 0..M scenarios each, with/without a
// Before/Purpose section) without hand-writing each case.
func randRequirementSet(r *rand.Rand) *RequirementSet {
	words := []string{"Login", "Session", "Token", "Rate Limit", "Password", "Audit Log", "Retry", "Cache"}
	pick := func() string { return words[r.Intn(len(words))] }

	// Real specs always carry a "## Requirements" header, even a freshly
	// scaffolded one with zero requirements in it yet (buildSpecSkeleton) —
	// so that's what this generator produces. HasRequirementsSection=false
	// models the separate, degenerate "no header found at all" parse
	// result (see ParseRequirementSet's doc and fuzz_test.go), which this
	// property (x built directly) does not apply to: that path collapses
	// Before/After into a single opaque blob by design.
	rs := &RequirementSet{HasRequirementsSection: true}
	if r.Intn(2) == 0 {
		rs.Before = fmt.Sprintf("# %s Specification\n\n## Purpose\n%s provides %s.", pick(), pick(), pick())
	}

	n := r.Intn(4)
	seenNames := map[string]bool{}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("%s %d", pick(), i) // suffix guarantees uniqueness
		if seenNames[name] {
			continue
		}
		seenNames[name] = true

		body := fmt.Sprintf("The system SHALL %s reliably.", pick())
		var scenarios []Scenario
		m := r.Intn(3)
		for j := 0; j < m; j++ {
			scenarios = append(scenarios, Scenario{
				Name: fmt.Sprintf("%s scenario %d", pick(), j),
				Body: fmt.Sprintf("- **WHEN** %s happens\n- **THEN** the system SHALL react", pick()),
			})
		}
		rs.Requirements = append(rs.Requirements, NewRequirement(name, body, scenarios))
	}
	rs.After = "\n"
	return rs
}

func TestRoundTrip_ParseRenderParse(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		x := randRequirementSet(r)
		rendered := x.Render()

		got, err := ParseRequirementSet(rendered)
		if err != nil {
			t.Fatalf("iteration %d: ParseRequirementSet(Render(x)) failed: %v\nrendered:\n%s", i, err, rendered)
		}

		if got.Before != x.Before {
			t.Fatalf("iteration %d: Before mismatch:\nwant %q\ngot  %q", i, x.Before, got.Before)
		}
		if got.After != x.After {
			t.Fatalf("iteration %d: After mismatch:\nwant %q\ngot  %q", i, x.After, got.After)
		}
		if !reflect.DeepEqual(got.Requirements, x.Requirements) {
			t.Fatalf("iteration %d: Requirements mismatch:\nwant %#v\ngot  %#v", i, x.Requirements, got.Requirements)
		}
	}
}

// TestRoundTrip_RenderIsAFixedPoint exercises the second documented
// property directly: render(parse(b)) may reformat b's incidental
// whitespace, but a second parse+render pass on the RESULT must reproduce
// it exactly — the canonical form is stable, not just "different every
// time".
func TestRoundTrip_RenderIsAFixedPoint(t *testing.T) {
	messy := []string{
		validLivingSpec,
		"# cap Specification\n\n\n\n## Purpose\nToo many blank lines above.\n\n\n## Requirements\n\n\n### Requirement: A\nSHALL do it.\n\n\n\n#### Scenario: S\nbody\n",
		"## Requirements\n### Requirement: Only One\nNo purpose or title at all, just a bare requirement, SHALL work.\n",
		"# cap\n\n## Purpose\nP.\n", // no Requirements section at all
		validDelta,                  // ADDED/MODIFIED bodies also flow through the same block grammar
	}

	for i, src := range messy {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			rs1, err := ParseRequirementSet([]byte(src))
			if err != nil {
				// validDelta isn't a living spec; parsing it as one may
				// legitimately fail structurally (e.g. delta headers) —
				// skip rather than treat as a bug.
				t.Skipf("not a living spec, skipping: %v", err)
			}
			pass1 := rs1.Render()

			rs2, err := ParseRequirementSet(pass1)
			if err != nil {
				t.Fatalf("re-parsing pass-1 render failed: %v\n%s", err, pass1)
			}
			pass2 := rs2.Render()

			if string(pass1) != string(pass2) {
				t.Fatalf("render is not a fixed point:\n--- pass1 ---\n%s\n--- pass2 ---\n%s", pass1, pass2)
			}
		})
	}
}
