package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// proposalFile is the refine-stage singleton artifact's filename
// (spec-lifecycle.md §4 — load-bearing, kept identical to stock OpenSpec).
const proposalFile = "proposal.md"

// validateProposal checks proposal.md's custom-artifact structure: a
// "---"-delimited YAML frontmatter block, with a non-empty `issue` field
// (spec-lifecycle.md §4's proposal row, §10's sourceTracking join key). It
// does not grade proposal.md's Why/What Changes/Impact prose — that is a
// human/agent review concern, not a machine-checkable one.
func validateProposal(dir string) ([]Finding, error) {
	path := filepath.Join(dir, proposalFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				File: path, Kind: "missing_artifact",
				Message:  proposalFile + " not found",
				Severity: SeverityError,
			}}, nil
		}
		return nil, fmt.Errorf("validate: reading %s: %w", path, err)
	}

	fm, ok := splitFrontmatter(data)
	if !ok {
		return []Finding{{
			File: path, Line: 1, Kind: "missing_frontmatter",
			Message:  `proposal.md must start with a "---"-delimited YAML frontmatter block carrying an "issue" field (spec-lifecycle.md §4, §10)`,
			Severity: SeverityError,
		}}, nil
	}

	var meta map[string]any
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return []Finding{{
			File: path, Line: 1, Kind: "malformed_frontmatter",
			Message:  fmt.Sprintf("proposal.md's frontmatter is not valid YAML: %s", err),
			Severity: SeverityError,
		}}, nil
	}

	issue, _ := meta["issue"].(string)
	if strings.TrimSpace(issue) == "" {
		return []Finding{{
			File: path, Line: 1, Kind: "missing_issue_ref",
			Message:  `proposal.md's frontmatter is missing a non-empty "issue" field (spec-lifecycle.md §10 sourceTracking join key, e.g. "kentra-io/kafka-dq#42")`,
			Severity: SeverityError,
		}}, nil
	}

	return nil, nil
}
