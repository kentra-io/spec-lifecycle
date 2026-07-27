package status

import (
	"path/filepath"
	"strings"
	"testing"
)

// writeSpec writes an openspec/specs/<cap>/spec.md projection with exactly n
// lines under root and returns the specs root.
func writeSpec(t *testing.T, specsRoot, cap string, n int) {
	t.Helper()
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	// n lines each newline-terminated -> countLines == n.
	writeFile(t, filepath.Join(specsRoot, cap, "spec.md"), strings.Join(lines, "\n")+"\n")
}

// TestCapabilityWarningsFlagsOversized is the status-reporting scenario "YAML
// status output with an oversized capability" at the derivation level: a
// capability whose spec.md exceeds the threshold is flagged with its current
// line count; one under the threshold is not.
func TestCapabilityWarningsFlagsOversized(t *testing.T) {
	specsRoot := filepath.Join(t.TempDir(), "openspec", "specs")
	writeSpec(t, specsRoot, "auth", 250)
	writeSpec(t, specsRoot, "small", 10)

	warnings, err := CapabilityWarnings(specsRoot, DefaultCapabilitySizeWarningLines)
	if err != nil {
		t.Fatalf("CapabilityWarnings: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1 (only auth is oversized): %+v", len(warnings), warnings)
	}
	if warnings[0].Capability != "auth" {
		t.Errorf("Capability = %q, want auth", warnings[0].Capability)
	}
	if warnings[0].Lines != 250 {
		t.Errorf("Lines = %d, want 250 (current line count)", warnings[0].Lines)
	}
}

// TestCapabilityWarningsDefaultThreshold verifies a zero threshold falls back
// to the documented default (200): a 200-line spec is within (not >) and a
// 201-line spec is over.
func TestCapabilityWarningsDefaultThreshold(t *testing.T) {
	if DefaultCapabilitySizeWarningLines != 200 {
		t.Fatalf("DefaultCapabilitySizeWarningLines = %d, want 200", DefaultCapabilitySizeWarningLines)
	}
	specsRoot := filepath.Join(t.TempDir(), "openspec", "specs")
	writeSpec(t, specsRoot, "atlimit", 200)
	writeSpec(t, specsRoot, "over", 201)

	warnings, err := CapabilityWarnings(specsRoot, 0)
	if err != nil {
		t.Fatalf("CapabilityWarnings: %v", err)
	}
	if len(warnings) != 1 || warnings[0].Capability != "over" || warnings[0].Lines != 201 {
		t.Fatalf("warnings = %+v, want only over@201 (200 is within threshold)", warnings)
	}
}

// TestCapabilityWarningsNoSpecsDir is not an error: a project with no live
// specs yet reports no warnings.
func TestCapabilityWarningsNoSpecsDir(t *testing.T) {
	warnings, err := CapabilityWarnings(filepath.Join(t.TempDir(), "nope"), 200)
	if err != nil {
		t.Fatalf("CapabilityWarnings: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %+v, want none", warnings)
	}
}

// TestCapabilityWarningsSortedByCapability keeps the sequence stable
// (capability-name order) regardless of directory iteration order.
func TestCapabilityWarningsSorted(t *testing.T) {
	specsRoot := filepath.Join(t.TempDir(), "openspec", "specs")
	writeSpec(t, specsRoot, "zeta", 300)
	writeSpec(t, specsRoot, "alpha", 300)

	warnings, err := CapabilityWarnings(specsRoot, 200)
	if err != nil {
		t.Fatalf("CapabilityWarnings: %v", err)
	}
	if len(warnings) != 2 || warnings[0].Capability != "alpha" || warnings[1].Capability != "zeta" {
		t.Fatalf("warnings = %+v, want [alpha, zeta] in name order", warnings)
	}
}
