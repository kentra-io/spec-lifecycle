package guard

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
)

// checkImmutability implements check 1 (doc.go): every archived change
// folder's content must still hash to its ledger record(s)'
// archiveManifestSha, via internal/archive.ManifestSHA — the exact
// algorithm Archive used to compute that hash when it first appended the
// record(s).
//
// byChange is every ledger record grouped by Change (guard.go); archiveSet
// is the set of directory names actually present under
// openspec/changes/archive/. Three shapes are possible per change name (the
// union of both):
//
//   - present on disk, referenced by the ledger, hashes match: clean.
//   - present on disk, referenced by the ledger, hashes DON'T match: a hand
//     edit — the textbook KindArchiveMutated case.
//   - present on disk, but the ledger has no record for it at all:
//     immutability can't be established (nothing to compare against) — also
//     KindArchiveMutated, since the practical consequence is identical
//     ("this archived folder's integrity cannot be trusted").
//   - referenced by the ledger, but missing from disk entirely (the folder
//     was deleted/renamed after archiving): also KindArchiveMutated — an
//     even more thorough violation of "the archive directory is thereafter
//     append-only" (spec-lifecycle.md §6.2) than an in-place edit.
func checkImmutability(root string, byChange map[string][]archive.Record, archiveSet map[string]bool) ([]Finding, error) {
	names := make(map[string]bool, len(byChange)+len(archiveSet))
	for name := range byChange {
		names[name] = true
	}
	for name := range archiveSet {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)

	var findings []Finding
	for _, name := range sorted {
		recs := byChange[name]
		onDisk := archiveSet[name]

		switch {
		case onDisk && len(recs) == 0:
			findings = append(findings, Finding{
				Check:  CheckImmutability,
				Kind:   KindArchiveMutated,
				Change: name,
				Message: fmt.Sprintf(
					"archived change %q exists under openspec/changes/archive/ but no ledger record references it; immutability cannot be verified",
					name,
				),
			})
		case !onDisk && len(recs) > 0:
			findings = append(findings, Finding{
				Check:  CheckImmutability,
				Kind:   KindArchiveMutated,
				Change: name,
				Seq:    recs[0].Seq,
				Message: fmt.Sprintf(
					"the ledger has %d record(s) for change %q, but openspec/changes/archive/%s is missing from disk",
					len(recs), name, name,
				),
			})
		default:
			archiveDir := filepath.Join(root, "openspec", "changes", "archive", name)
			manifest, err := archive.ManifestSHA(archiveDir)
			if err != nil {
				return nil, fmt.Errorf("guard: hashing archived change %q: %w", name, err)
			}
			for _, r := range recs {
				if manifest == r.ArchiveManifestSha {
					continue
				}
				findings = append(findings, Finding{
					Check:  CheckImmutability,
					Kind:   KindArchiveMutated,
					Change: name,
					Seq:    r.Seq,
					Message: fmt.Sprintf(
						"archived change %q content hash %s does not match ledger record (seq %d) archiveManifestSha %s",
						name, manifest, r.Seq, r.ArchiveManifestSha,
					),
				})
			}
		}
	}
	return findings, nil
}
