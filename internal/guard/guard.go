package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
)

// Run performs one guard check (doc.go) and returns every Finding pooled
// across all checks that ran. A non-nil error means guard could not run at
// all (exit 2: a malformed ledger, or an I/O failure reading a file guard
// needs) — Result is always zero-valued in that case. A nil error with a
// non-empty Result.Findings means the check ran to completion and found
// problems (exit 1); a nil error with zero findings is clean (exit 0).
// Run itself never exits or logs.
func Run(opts Options) (Result, error) {
	root := opts.Root
	archiveRoot := filepath.Join(root, "openspec", "changes", "archive")

	archiveNames, err := listArchivedChangeNames(archiveRoot)
	if err != nil {
		return Result{}, fmt.Errorf("guard: listing %s: %w", archiveRoot, err)
	}

	ledgerPath := archive.LedgerPath(root)
	if _, statErr := os.Stat(ledgerPath); statErr != nil {
		if !os.IsNotExist(statErr) {
			return Result{}, fmt.Errorf("guard: checking %s: %w", ledgerPath, statErr)
		}
		// Precondition (doc.go): no ledger file at all.
		var findings []Finding
		if len(archiveNames) > 0 {
			findings = append(findings, Finding{
				Check: CheckLedger,
				Kind:  KindLedgerMissing,
				Message: fmt.Sprintf(
					"%s is absent but %d archived change folder(s) exist under %s",
					ledgerPath, len(archiveNames), archiveRoot,
				),
			})
		}
		return finalizeResult(findings, 0, 0), nil
	}

	records, err := archive.ReadAll(root)
	if err != nil {
		// archive.ReadAll's own error already names the file and the
		// malformed line precisely (doc.go: "not a finding, an environment
		// failure").
		return Result{}, fmt.Errorf("guard: %w", err)
	}

	byChange := groupByChange(records)
	archiveSet := make(map[string]bool, len(archiveNames))
	for _, n := range archiveNames {
		archiveSet[n] = true
	}

	var findings []Finding

	imFindings, err := checkImmutability(root, byChange, archiveSet)
	if err != nil {
		return Result{}, err
	}
	findings = append(findings, imFindings...)

	chainFindings, err := checkDigestChain(root, records)
	if err != nil {
		return Result{}, err
	}
	findings = append(findings, chainFindings...)

	replayFindings, err := checkReplay(root, records)
	if err != nil {
		return Result{}, err
	}
	findings = append(findings, replayFindings...)

	sortFindings(findings)
	return finalizeResult(findings, len(byChange), len(records)), nil
}

// listArchivedChangeNames returns the sorted set of directory names
// directly under archiveRoot (openspec/changes/archive/) — a missing
// archiveRoot (no archive has ever happened) yields a nil slice, not an
// error, mirroring internal/archive's discoverCapabilities convention for
// an absent directory.
func listArchivedChangeNames(archiveRoot string) ([]string, error) {
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// groupByChange buckets records by their Change field, preserving each
// bucket's original (ledger-append, i.e. seq) order.
func groupByChange(records []archive.Record) map[string][]archive.Record {
	m := map[string][]archive.Record{}
	for _, r := range records {
		m[r.Change] = append(m[r.Change], r)
	}
	return m
}

// finalizeResult builds a Result, guaranteeing Findings is a non-nil
// (possibly empty) slice: a machine JSON consumer should never need a nil
// check to range over it.
func finalizeResult(findings []Finding, changesChecked, recordsChecked int) Result {
	if findings == nil {
		findings = []Finding{}
	}
	return Result{
		Findings: findings,
		Summary: Summary{
			ChangesChecked: changesChecked,
			RecordsChecked: recordsChecked,
			Findings:       len(findings),
			Clean:          len(findings) == 0,
		},
	}
}

// sortFindings imposes a total, deterministic order on a run's pooled
// findings (Check, then Kind, then Change, then Capability, then Seq) —
// idempotence (guard_test.go's TestRun_Idempotent) and reproducible
// `--format json` output both depend on this, mirroring the sibling
// adr-sourced-constitution primitive's own guard package (internal/guard's
// sortViolations there).
func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Change != b.Change {
			return a.Change < b.Change
		}
		if a.Capability != b.Capability {
			return a.Capability < b.Capability
		}
		return a.Seq < b.Seq
	})
}
