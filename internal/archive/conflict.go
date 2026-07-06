package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kentra-io/spec-lifecycle/internal/spec"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// checkConflicts implements step 2 (doc.go's "detection by construction"):
// for every OTHER live change folder under root/openspec/changes (never
// changes/archive/, never `change` itself), for every capability it
// shares with ownDeltas, parse its delta and compare the
// MODIFIED/REMOVED/RENAMED(from) requirement names (case-insensitively)
// against this change's own set for that capability. Returns every
// collision found (nil if none) plus any non-fatal warnings (a sibling
// change whose delta could not be read/parsed — skipped, not fatal to
// THIS archive).
func checkConflicts(root, change string, ownDeltas map[string]*spec.Delta) ([]Conflict, []string, error) {
	ownTargets := make(map[string]map[string]string, len(ownDeltas))
	for cap, d := range ownDeltas {
		ownTargets[cap] = targetedNames(d)
	}

	changesRoot := filepath.Join(root, "openspec", "changes")
	entries, err := os.ReadDir(changesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", changesRoot, err)
	}

	var others []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" || e.Name() == change {
			continue
		}
		others = append(others, e.Name())
	}
	sort.Strings(others)

	var conflicts []Conflict
	var warnings []string

	for _, name := range others {
		otherDir := filepath.Join(changesRoot, name)
		if !validate.HasSpecsDeltas(otherDir) {
			continue // a bug (or not-yet-refined change) with no delta can't conflict
		}

		otherCaps, cerr := discoverCapabilities(otherDir)
		if cerr != nil {
			warnings = append(warnings, fmt.Sprintf("conflict-check: skipping change %q: %v", name, cerr))
			continue
		}

		for _, cap := range otherCaps {
			own := ownTargets[cap]
			if len(own) == 0 {
				continue // this change doesn't touch that capability at all
			}

			data, rerr := os.ReadFile(filepath.Join(otherDir, "specs", cap, "spec.md"))
			if rerr != nil {
				warnings = append(warnings, fmt.Sprintf("conflict-check: skipping %s/%s: %v", name, cap, rerr))
				continue
			}
			otherDelta, perr := spec.ParseDelta(data)
			if perr != nil {
				warnings = append(warnings, fmt.Sprintf("conflict-check: skipping %s/%s (unparsable delta): %v", name, cap, perr))
				continue
			}

			otherTargets := targetedNames(otherDelta)
			for key, display := range own {
				if _, hit := otherTargets[key]; hit {
					conflicts = append(conflicts, Conflict{Capability: cap, Requirement: display, OtherChange: name})
				}
			}
		}
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Capability != conflicts[j].Capability {
			return conflicts[i].Capability < conflicts[j].Capability
		}
		if conflicts[i].Requirement != conflicts[j].Requirement {
			return conflicts[i].Requirement < conflicts[j].Requirement
		}
		return conflicts[i].OtherChange < conflicts[j].OtherChange
	})
	return conflicts, warnings, nil
}

// targetedNames returns the union of a delta's MODIFIED, REMOVED, and
// RENAMED-from requirement names, keyed case-insensitively (value is the
// original-cased display name) — the set of ALREADY-EXISTING requirements
// this delta touches (as opposed to ADDED, which names something new;
// doc.go explains why ADDED is excluded here).
func targetedNames(d *spec.Delta) map[string]string {
	out := map[string]string{}
	for _, r := range d.Modified {
		out[strings.ToLower(r.Name)] = r.Name
	}
	for _, n := range d.Removed {
		out[strings.ToLower(n)] = n
	}
	for _, rn := range d.Renamed {
		out[strings.ToLower(rn.From)] = rn.From
	}
	return out
}

// formatConflicts renders conflicts as a single human-readable line for
// an error message.
func formatConflicts(conflicts []Conflict) string {
	parts := make([]string, len(conflicts))
	for i, c := range conflicts {
		parts[i] = fmt.Sprintf("%s: requirement %q also touched by in-flight change %q", c.Capability, c.Requirement, c.OtherChange)
	}
	return strings.Join(parts, "; ")
}
