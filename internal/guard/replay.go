package guard

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
	"github.com/kentra-io/spec-lifecycle/internal/spec"
)

// checkReplay implements check 3 (doc.go), the deep check: recompute
// fold(all archived deltas, in ledger seq order, from empty) entirely
// in-process via internal/spec, then diff the rendered result against the
// live projection, byte for byte.
//
// Per capability, folding is done from an empty base (nil *RequirementSet)
// through every one of that capability's records IN SEQ ORDER, reading
// each record's delta straight from the preserved archived file
// (openspec/changes/archive/<record.Change>/specs/<capability>/spec.md —
// never from the record's own summary DeltaOps, which is
// informational/audit only and does not carry full requirement bodies,
// archive/doc.go). Since a single capability's fold history is
// self-contained (folding capability A never reads or writes capability
// B's state), replaying capability-by-capability in each capability's own
// seq order is equivalent to one global seq-ordered interleaved replay.
//
// A record whose archived delta cannot be read/parsed, or whose fold
// itself errors (e.g. a hand-edited delta that no longer matches the live
// history it once produced), "taints" that capability's replay for the
// rest of this run: one Finding is recorded for the specific failure and
// the capability is excluded from the live-projection comparison below
// (there is nothing trustworthy left to compare) — but it is NOT reported
// a second time by the "extra live capability" pass, since it plainly does
// have ledger history, just unreplayable history.
//
// # Brownfield capabilities are exempt from the byte-diff, by design
//
// A capability whose FIRST-EVER ledger record has a preImageSha other than
// archive.EmptyImageSHA predates lifecycle's own tracked history — its
// original baseline content existed on disk before any archive ever ran
// (cmd/lifecycle's own archive_conformance_case02/archive_conflict
// fixtures exercise exactly this: an existing openspec/specs/auth/spec.md
// present from the start, with no genesis "capability didn't exist yet"
// record). Those original bytes were never captured anywhere in the
// ledger (only their hash was, as that first preImageSha) — a TRUE
// from-empty replay of such a capability's full history can therefore
// never byte-match the live projection, not because anything is wrong,
// but because the starting point is structurally unrecoverable from
// ledger data alone. This check accordingly folds from empty, and
// byte-diffs against live, ONLY for a capability whose entire history
// originated inside the ledger (first record's preImageSha ==
// EmptyImageSHA); a brownfield capability is still marked "attempted"
// (so it is correctly excluded from the separate "extra live capability"
// pass below — it plainly HAS ledger history) but is not byte-diffed here.
// Its ledger-tracked portion is still held to full fidelity by check 2's
// digest chain (chain.go), which needs no pre-ledger baseline to verify a
// chain of hash links.
func checkReplay(root string, records []archive.Record) ([]Finding, error) {
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
	rendered := map[string][]byte{}
	attempted := map[string]bool{}

	for _, cap := range caps {
		attempted[cap] = true
		if byCap[cap][0].PreImageSha != archive.EmptyImageSHA {
			continue // brownfield origin (see doc above) — not byte-diffable
		}
		var current *spec.RequirementSet
		tainted := false
		for _, r := range byCap[cap] {
			deltaPath := filepath.Join(root, "openspec", "changes", "archive", r.Change, "specs", cap, "spec.md")
			data, err := os.ReadFile(deltaPath)
			if err != nil {
				findings = append(findings, replayFinding(cap, r, fmt.Sprintf("could not read archived delta %s: %v", deltaPath, err)))
				tainted = true
				break
			}
			d, err := spec.ParseDelta(data)
			if err != nil {
				findings = append(findings, replayFinding(cap, r, fmt.Sprintf("could not parse archived delta %s: %v", deltaPath, err)))
				tainted = true
				break
			}
			folded, err := spec.Fold(cap, r.Change, current, d)
			if err != nil {
				findings = append(findings, replayFinding(cap, r, fmt.Sprintf("from-empty replay could not fold: %v", err)))
				tainted = true
				break
			}
			current = folded
		}
		if tainted || current == nil {
			continue
		}
		rendered[cap] = current.Render()
	}

	for _, cap := range caps {
		want, ok := rendered[cap]
		if !ok {
			continue // tainted above; already has its own Finding
		}
		specPath := filepath.Join(root, "openspec", "specs", cap, "spec.md")
		got, err := os.ReadFile(specPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("guard: reading %s: %w", specPath, err)
			}
			findings = append(findings, Finding{
				Check:      CheckReplay,
				Kind:       KindProjectionDrift,
				Capability: cap,
				Message: fmt.Sprintf(
					"capability %q: from-empty replay produced content but %s does not exist",
					cap, specPath,
				),
			})
			continue
		}
		if !bytes.Equal(got, want) {
			findings = append(findings, Finding{
				Check:      CheckReplay,
				Kind:       KindProjectionDrift,
				Capability: cap,
				Message: fmt.Sprintf(
					"capability %q: %s does not byte-match the from-empty replay of its archived history",
					cap, specPath,
				),
			})
		}
	}

	liveCaps, err := listLiveCapabilities(root)
	if err != nil {
		return nil, fmt.Errorf("guard: listing %s: %w", filepath.Join(root, "openspec", "specs"), err)
	}
	for _, cap := range liveCaps {
		if attempted[cap] {
			continue
		}
		findings = append(findings, Finding{
			Check:      CheckReplay,
			Kind:       KindProjectionDrift,
			Capability: cap,
			Message: fmt.Sprintf(
				"capability %q has a live openspec/specs/%s/spec.md but no ledger record ever folds it",
				cap, cap,
			),
		})
	}

	return findings, nil
}

func replayFinding(capability string, r archive.Record, message string) Finding {
	return Finding{
		Check:      CheckReplay,
		Kind:       KindProjectionDrift,
		Capability: capability,
		Change:     r.Change,
		Seq:        r.Seq,
		Message:    message,
	}
}

// listLiveCapabilities returns the sorted set of capability names under
// openspec/specs/ that have a spec.md — mirroring
// internal/archive.discoverCapabilities' own "directory containing
// spec.md" test, just rooted at specs/ instead of a change's specs/. A
// missing openspec/specs/ directory (no capability has ever been created)
// yields a nil slice, not an error.
func listLiveCapabilities(root string) ([]string, error) {
	specsRoot := filepath.Join(root, "openspec", "specs")
	entries, err := os.ReadDir(specsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var caps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(specsRoot, e.Name(), "spec.md")); err != nil {
			continue
		}
		caps = append(caps, e.Name())
	}
	sort.Strings(caps)
	return caps, nil
}
