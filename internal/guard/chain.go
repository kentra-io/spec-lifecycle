package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
)

// checkDigestChain implements check 2 (doc.go), the fast path: per
// capability (delta-less bug records, Capability == "", carry no
// projection and are skipped), every record AFTER a capability's first
// must have a preImageSha equal to the PRIOR record's postImageSha —
// KindChainBreak on mismatch — and the live openspec/specs/<cap>/spec.md
// hash must equal that capability's LATEST postImageSha —
// KindProjectionDrift on mismatch (spec-lifecycle.md §6.3 item 2 is
// explicit that a live-spec mismatch is projection_drift, not chain_break,
// even though this same check computes it).
//
// A capability's FIRST-EVER ledger record is deliberately NOT checked
// against archive.EmptyImageSHA here: internal/archive's own Archive
// legitimately records a real (non-empty) preImageSha on a capability's
// first record whenever that capability's spec.md already existed on disk
// before the ledger ever recorded anything about it (a brownfield
// capability folded into lifecycle's tracked history for the first time —
// exactly what cmd/lifecycle's own archive_conformance_case02 fixture
// exercises: an ADDED requirement folded into an ALREADY-EXISTING
// capability spec.md). Requiring every capability's history to originate
// from empty would misreport that legitimate, ordinary case as a
// chain_break; spec-lifecycle.md §6.3's own wording only ever requires
// consecutive records to link to EACH OTHER, never that a capability's
// very first record link to nothing.
func checkDigestChain(root string, records []archive.Record) ([]Finding, error) {
	byCap := map[string][]archive.Record{}
	for _, r := range records {
		if r.Capability == "" {
			continue
		}
		byCap[r.Capability] = append(byCap[r.Capability], r)
	}

	caps := make([]string, 0, len(byCap))
	for cap := range byCap {
		caps = append(caps, cap)
	}
	sort.Strings(caps)

	var findings []Finding
	for _, cap := range caps {
		recs := byCap[cap]
		var prev string
		for i, r := range recs {
			if i > 0 && r.PreImageSha != prev {
				findings = append(findings, Finding{
					Check:      CheckDigestChain,
					Kind:       KindChainBreak,
					Capability: cap,
					Change:     r.Change,
					Seq:        r.Seq,
					Message: fmt.Sprintf(
						"capability %q: record seq %d's preImageSha %s does not match the prior record's postImageSha %s",
						cap, r.Seq, r.PreImageSha, prev,
					),
				})
			}
			prev = r.PostImageSha
		}

		liveSha, err := liveSpecHash(root, cap)
		if err != nil {
			return nil, fmt.Errorf("guard: hashing live spec for capability %q: %w", cap, err)
		}
		if liveSha != prev {
			findings = append(findings, Finding{
				Check:      CheckDigestChain,
				Kind:       KindProjectionDrift,
				Capability: cap,
				Message: fmt.Sprintf(
					"capability %q: live openspec/specs/%s/spec.md hash %s does not match the latest ledger record's postImageSha %s",
					cap, cap, liveSha, prev,
				),
			})
		}
	}
	return findings, nil
}

// liveSpecHash hashes root's live openspec/specs/<cap>/spec.md with the
// exact algorithm (archive.HashBytes) Archive used to produce postImageSha
// — archive.EmptyImageSHA if the capability has no live spec.md at all
// (the same "capability doesn't exist yet" sentinel Archive's own
// pre-image logic uses, archive/doc.go).
func liveSpecHash(root, capability string) (string, error) {
	path := filepath.Join(root, "openspec", "specs", capability, "spec.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return archive.EmptyImageSHA, nil
		}
		return "", err
	}
	return archive.HashBytes(data), nil
}
