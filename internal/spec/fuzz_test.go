package spec

import (
	"strings"
	"testing"
)

// FuzzParseRenderRoundTrip fuzzes ParseRequirementSet over raw bytes: on
// any input, parsing must either return a precise error or a value whose
// Render (a) never panics and (b) reproduces a stable fixed point — parsing
// the rendered output again must succeed and re-rendering it must yield
// byte-identical output. No input, however adversarial, should ever panic
// either ParseRequirementSet or Render.
func FuzzParseRenderRoundTrip(f *testing.F) {
	seeds := []string{
		validLivingSpec,
		"",
		"## Requirements\n",
		"# t\n\n## Purpose\np\n",
		"### Requirement: outside\nSHALL.\n",
		"## ADDED Requirements\n### Requirement: x\nSHALL.\n",
		"## Requirements\n### Requirement: \nno name\n",
		"## Requirements\n### Requirement: A\nbody\n\n#### Scenario: \nno name\n",
		"## Requirements\n### Requirement: A\nbody\n\n### Requirement: A\nbody\n",
		"```\n### Requirement: fenced\n```\n## Requirements\n### Requirement: Real\nSHALL.\n",
		"~~~\nunterminated fence\n## Requirements\n### Requirement: Real\nSHALL.\n",
		"# t\r\n\r\n## Purpose\r\np\r\n## Requirements\r\n### Requirement: A\r\nSHALL.\r\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		rs, err := ParseRequirementSet([]byte(src))
		if err != nil {
			return // a precise error is a valid outcome for adversarial input
		}

		pass1 := rs.Render()

		rs2, err := ParseRequirementSet(pass1)
		if err != nil {
			t.Fatalf("re-parsing Render output failed: %v\nrendered:\n%s", err, pass1)
		}
		pass2 := rs2.Render()

		if string(pass1) != string(pass2) {
			t.Fatalf("render is not a fixed point:\n--- pass1 ---\n%s\n--- pass2 ---\n%s", pass1, pass2)
		}
	})
}

// FuzzDeltaNoPanic only needs to prove the parser never panics and, when it
// accepts input, that the accepted Delta is internally consistent (no
// conflicting/duplicate ops slipped past validation) — Delta has no
// renderer in this package, so there is no round-trip property to check
// here (see spec.go's package doc). Named to NOT match the "FuzzParse"
// pattern implementation-plan.md's DoD command targets, so that command
// (which errors if it matches more than one fuzz target) keeps working
// unambiguously; run this one explicitly with -fuzz=FuzzDeltaNoPanic.
// FuzzParseFoldRender fuzzes the full M1 pipeline: parse a (possibly empty,
// possibly nonsense) base spec, parse a (possibly empty, possibly nonsense)
// delta, Fold them, and Render the result. Neither Parse* nor Fold nor
// Render may ever panic. When Fold succeeds, it must not silently lose or
// gain requirements: renames and modifications are count-neutral, so the
// folded requirement count must equal
// baseCount - len(delta.Removed) + len(delta.Added) exactly (implementation-
// plan.md §12 spike 3's "conflicts detected, not silently dropped" posture
// — a fuzz-level cross-check of the same property fold_test.go locks with
// concrete named cases).
func FuzzParseFoldRender(f *testing.F) {
	baseSeeds := []string{
		validLivingSpec,
		"",
		"## Requirements\n",
		"# auth Specification\n\n## Purpose\np\n## Requirements\n### Requirement: A\nSHALL.\n\n#### Scenario: s\nb\n",
	}
	deltaSeeds := []string{
		validDelta,
		"",
		"## ADDED Requirements\n### Requirement: A\nSHALL.\n\n#### Scenario: s\nb\n",
		"## REMOVED Requirements\n### Requirement: Password Login\n",
		"## RENAMED Requirements\n- FROM: `### Requirement: Password Login`\n- TO: `### Requirement: Renamed`\n",
		"## MODIFIED Requirements\n### Requirement: Session Expiry\nSHALL.\n\n#### Scenario: s\nb\n",
	}
	for _, b := range baseSeeds {
		for _, d := range deltaSeeds {
			f.Add(b, d)
		}
	}

	f.Fuzz(func(t *testing.T, baseSrc, deltaSrc string) {
		var base *RequirementSet
		baseCount := 0
		if rs, err := ParseRequirementSet([]byte(baseSrc)); err == nil {
			base = rs
			baseCount = len(rs.Requirements)
		}

		delta, err := ParseDelta([]byte(deltaSrc))
		if err != nil {
			return // a precise error is a valid outcome for adversarial input
		}

		folded, err := Fold("fuzzcap", "999-fuzz-change", base, delta)
		if err != nil {
			return // a precise fold error (e.g. missing/duplicate target) is valid
		}

		wantCount := baseCount - len(delta.Removed) + len(delta.Added)
		if got := len(folded.Requirements); got != wantCount {
			t.Fatalf("requirement count = %d, want %d (base=%d removed=%d added=%d): silent loss/gain",
				got, wantCount, baseCount, len(delta.Removed), len(delta.Added))
		}

		_ = folded.Render() // must not panic
	})
}

func FuzzDeltaNoPanic(f *testing.F) {
	seeds := []string{
		validDelta,
		"",
		"## ADDED Requirements\n",
		"## ADDED Requirements\n### Requirement: X\nSHALL.\n\n#### Scenario: s\nb\n",
		"## RENAMED Requirements\n- FROM: `### Requirement: A`\n",
		"## RENAMED Requirements\n- TO: `### Requirement: A`\n",
		"## REMOVED Requirements\n- `### Requirement: A`\n",
		"## ADDED Requirements\n### Requirement: X\nno keyword here\n\n#### Scenario: s\nb\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		d, err := ParseDelta([]byte(src))
		if err != nil {
			return
		}
		names := map[string]bool{}
		for _, r := range d.Added {
			key := strings.ToLower(r.Name)
			if names[key] {
				t.Fatalf("accepted Delta has duplicate ADDED name %q", r.Name)
			}
			names[key] = true
		}
	})
}
