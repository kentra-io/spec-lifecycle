package status

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
)

// DefaultCapabilitySizeWarningLines is the line-count threshold above which a
// capability's live spec.md projection is flagged oversized (status-reporting
// spec, requirement "Oversized-capability warning": default 200 lines). The
// threshold is a configured value; when unset, this default applies.
const DefaultCapabilitySizeWarningLines = 200

// CapabilityWarning is one oversized-capability advisory: the capability name
// and the current line count of its live openspec/specs/<cap>/spec.md
// projection (status-reporting spec, "Machine-readable capability warnings").
type CapabilityWarning struct {
	Capability string `json:"capability" yaml:"capability"`
	Lines      int    `json:"lines" yaml:"lines"`
}

// CapabilityWarnings scans each capability directory under specsRoot
// (openspec/specs) and returns, in capability-name order, a warning for every
// capability whose spec.md projection exceeds threshold lines. A threshold
// <= 0 falls back to DefaultCapabilitySizeWarningLines. A missing specsRoot
// (no live specs yet) or a capability directory without a spec.md is not an
// error — such capabilities simply contribute no warning.
func CapabilityWarnings(specsRoot string, threshold int) ([]CapabilityWarning, error) {
	if threshold <= 0 {
		threshold = DefaultCapabilitySizeWarningLines
	}

	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var warnings []CapabilityWarning
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specPath := filepath.Join(specsRoot, e.Name(), "spec.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if n := countLines(data); n > threshold {
			warnings = append(warnings, CapabilityWarning{Capability: e.Name(), Lines: n})
		}
	}

	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].Capability < warnings[j].Capability
	})
	return warnings, nil
}

// countLines returns the number of lines in data: one per '\n', plus one more
// for a final line that is non-empty but unterminated. An empty file is zero
// lines.
func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}
