package spec

import (
	"flag"
	"os"
	"strings"
	"testing"
)

// update regenerates the checked-in golden projections instead of comparing
// against them (`go test ./internal/spec/ -update`). Golden fixtures are the
// projection's correctness proof (constitution ADR-0005), so refreshing them
// is always a deliberate, reviewed act.
var update = flag.Bool("update", false, "update golden files in testdata/")

// goldenBytes asserts got is byte-identical to the golden file at path, or
// rewrites that file when -update is set.
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

// TestSlug is the D5 slug-derivation table test: lowercase, each maximal run
// of non-[a-z0-9] collapses to a single "-", leading/trailing "-" trimmed.
func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		// The spec-format scenario's required derivations.
		{"New feature intake", "new-feature-intake"},
		{"Human confirms the drafted issue", "human-confirms-the-drafted-issue"},
		// Leading/trailing separator runs are trimmed.
		{"  --Leading & Trailing punctuation!!  ", "leading-trailing-punctuation"},
		// Interior punctuation runs collapse to a single dash.
		{"a...b", "a-b"},
		{"Mixed CASE and   multiple   spaces", "mixed-case-and-multiple-spaces"},
		// Digit runs are preserved; surrounding punctuation collapses.
		{"Digits 123 and codes v2.0", "digits-123-and-codes-v2-0"},
		{"already-kebab-case", "already-kebab-case"},
		// Degenerate inputs.
		{"", ""},
		{"!!!", ""},
		{"---", ""},
		{"MixedCASE", "mixedcase"},
		// Non-ASCII letters are outside [a-z0-9] and act as separators.
		{"café münchen", "caf-m-nchen"},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// projectionFixtures pairs each source YAML with its expected markdown golden.
var projectionFixtures = []struct{ name, yaml, golden string }{
	{"feature-intake", "testdata/projection/feature-intake.spec.yaml", "testdata/projection/feature-intake.spec.md"},
	{"edge-slugs", "testdata/projection/edge-slugs.spec.yaml", "testdata/projection/edge-slugs.spec.md"},
}

// TestRenderProjection_Golden proves the projection is byte-stable: each source
// YAML renders byte-identical to its checked-in golden markdown. Run with
// `-update` to regenerate the goldens after an intentional format change.
func TestRenderProjection_Golden(t *testing.T) {
	for _, f := range projectionFixtures {
		t.Run(f.name, func(t *testing.T) {
			data, err := os.ReadFile(f.yaml)
			if err != nil {
				t.Fatalf("reading source YAML: %v", err)
			}
			ls, err := ParseLivingSpecYAML(data)
			if err != nil {
				t.Fatalf("ParseLivingSpecYAML(%s): %v", f.yaml, err)
			}
			goldenBytes(t, f.golden, ls.RenderProjection())
		})
	}
}

// TestRenderProjection_Idempotent proves rendering the same YAML twice is
// byte-identical (determinism — no clock, filesystem, or map-order input).
func TestRenderProjection_Idempotent(t *testing.T) {
	for _, f := range projectionFixtures {
		data, err := os.ReadFile(f.yaml)
		if err != nil {
			t.Fatalf("reading source YAML: %v", err)
		}
		ls, err := ParseLivingSpecYAML(data)
		if err != nil {
			t.Fatalf("ParseLivingSpecYAML(%s): %v", f.yaml, err)
		}
		first := string(ls.RenderProjection())
		second := string(ls.RenderProjection())
		if first != second {
			t.Fatalf("render not idempotent for %s:\n--- first ---\n%s\n--- second ---\n%s", f.name, first, second)
		}
	}
}

// TestRenderProjection_CarriesDerivedSlugs is the validation-contract assertion:
// the feature-intake projection surfaces the derived requirement slug
// `new-feature-intake` and scenario slug `human-confirms-the-drafted-issue`.
func TestRenderProjection_CarriesDerivedSlugs(t *testing.T) {
	data, err := os.ReadFile("testdata/projection/feature-intake.spec.yaml")
	if err != nil {
		t.Fatalf("reading source YAML: %v", err)
	}
	ls, err := ParseLivingSpecYAML(data)
	if err != nil {
		t.Fatalf("ParseLivingSpecYAML: %v", err)
	}
	got := string(ls.RenderProjection())
	for _, want := range []string{
		"### Requirement: New feature intake",
		"<!-- slug: new-feature-intake -->",
		"#### Scenario: Human confirms the drafted issue",
		"<!-- slug: human-confirms-the-drafted-issue -->",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("projection missing %q\n--- projection ---\n%s", want, got)
		}
	}
}

// TestRenderProjection_ByteStabilityRules guards the D5 rules directly: a
// managed read-only header first, LF endings only, no trailing whitespace on
// any line, and exactly one trailing newline.
func TestRenderProjection_ByteStabilityRules(t *testing.T) {
	data, err := os.ReadFile("testdata/projection/feature-intake.spec.yaml")
	if err != nil {
		t.Fatalf("reading source YAML: %v", err)
	}
	ls, err := ParseLivingSpecYAML(data)
	if err != nil {
		t.Fatalf("ParseLivingSpecYAML: %v", err)
	}
	out := string(ls.RenderProjection())

	if !strings.HasPrefix(out, projectionHeader+"\n") {
		t.Errorf("projection does not start with the managed read-only header")
	}
	if strings.Contains(out, "\r") {
		t.Errorf("projection contains CR — endings must be LF only")
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Errorf("projection must end with exactly one trailing newline")
	}
	for i, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.TrimRight(line, " \t") != line {
			t.Errorf("line %d has trailing whitespace: %q", i+1, line)
		}
	}
}
