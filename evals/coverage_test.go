// Package evals holds the on-demand agent evals. Nothing here runs in CI
// without its environment variables set — these tests grade an artifact a
// live agent produced, so they are opt-in by construction.
package evals

import (
	"os"
	"strings"
	"testing"

	// spec-lifecycle's YAML dependency is go.yaml.in/yaml/v3, not
	// gopkg.in/yaml.v3 — check go.mod before copying this import elsewhere.
	yaml "go.yaml.in/yaml/v3"
)

type scenario struct {
	Name string   `yaml:"name"`
	Then []string `yaml:"then"`
}

// delta mirrors the spec-delta grammar: op-tagged entries, each nesting its
// requirement under `requirement:` (see any archived change's spec.yaml).
type delta struct {
	Deltas []struct {
		Op          string `yaml:"op"`
		Requirement struct {
			Name      string     `yaml:"name"`
			Scenarios []scenario `yaml:"scenarios"`
		} `yaml:"requirement"`
	} `yaml:"deltas"`
}

type plan struct {
	Milestones []struct {
		Number   int    `yaml:"number"`
		Goal     string `yaml:"goal"`
		Contract struct {
			Criteria []struct {
				Name string `yaml:"name"`
				Then string `yaml:"then"`
			} `yaml:"criteria"`
		} `yaml:"contract"`
	} `yaml:"milestones"`
}

func norm(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// TestScenarioCoverage is eval pass-condition 2: every scenario in the change's
// delta is discharged by some milestone criterion. The link is the `then`
// clause, because the skill instructs the planner to copy given/when/then out
// in full — a paraphrase that loses the clause is itself the failure.
func TestScenarioCoverage(t *testing.T) {
	planPath, deltaPath := os.Getenv("EVAL_PLAN"), os.Getenv("EVAL_DELTA")
	if planPath == "" || deltaPath == "" {
		t.Skip("set EVAL_PLAN and EVAL_DELTA to grade a produced plan")
	}

	var d delta
	readYAML(t, deltaPath, &d)
	var p plan
	readYAML(t, planPath, &p)

	var thens []string
	for _, m := range p.Milestones {
		for _, c := range m.Contract.Criteria {
			thens = append(thens, norm(c.Then))
		}
	}
	if len(thens) == 0 {
		t.Fatalf("%s has no structured criteria at all — the plan cannot discharge any scenario", planPath)
	}

	for _, entry := range d.Deltas {
		for _, s := range entry.Requirement.Scenarios {
			for _, want := range s.Then {
				if !coveredBy(thens, norm(want)) {
					t.Errorf("scenario %q (requirement %q): no milestone criterion carries its then-clause %q",
						s.Name, entry.Requirement.Name, want)
				}
			}
		}
	}
}

// coveredBy tolerates a criterion that says more than the scenario, but not one
// that says less: the scenario's clause must appear inside some criterion.
func coveredBy(criteria []string, want string) bool {
	for _, c := range criteria {
		if strings.Contains(c, want) || strings.Contains(want, c) {
			return true
		}
	}
	return false
}

func readYAML(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := yaml.Unmarshal(data, into); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
}
