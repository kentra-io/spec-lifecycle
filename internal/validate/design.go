package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// designFile is the design-stage singleton artifact's filename
// (spec-lifecycle.md §4).
const designFile = "design.md"

// nfrDischargeHeadingRe recognizes design.md's required "## NFR Discharge"
// heading, tolerant of a hyphen or extra trailing words (e.g.
// "## NFR-Discharge (per §4.1)") but not of it being missing entirely.
var nfrDischargeHeadingRe = regexp.MustCompile(`(?im)^##\s+NFR[\s-]*Discharge\b`)

// validateDesign checks design.md's custom-artifact structure: an
// explicit NFR-discharge section is present (spec-lifecycle.md §4's design
// row, §4.1's NFR routing rule — design MUST account for every declared
// NFR). It does not grade the design's technical content, nor whether
// every individual NFR is actually discharged — that is a human/agent
// review concern, not a machine-checkable one.
func validateDesign(dir string) ([]Finding, error) {
	path := filepath.Join(dir, designFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: path, Kind: "missing_artifact",
				Message:  designFile + " not found",
				Severity: SeverityError,
			}}, nil
		}
		return nil, fmt.Errorf("validate: reading %s: %w", path, err)
	}

	if !nfrDischargeHeadingRe.Match(data) {
		return []Finding{{
			File: path, Kind: "missing_nfr_discharge",
			Message:  `design.md is missing an explicit NFR-discharge section (a "## NFR Discharge" heading) — design MUST account for every declared NFR (spec-lifecycle.md §4.1)`,
			Severity: SeverityError,
		}}, nil
	}
	return nil, nil
}
