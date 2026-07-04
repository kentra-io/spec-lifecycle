package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
	"github.com/kentra-io/spec-lifecycle/internal/spec"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// foldedCapability is one capability's in-memory fold result, computed
// BEFORE any write happens (see Archive's step ordering: every fold is
// attempted, and any failure refuses the whole archive, before a single
// byte is written to disk).
type foldedCapability struct {
	capability string
	preImage   string
	postImage  string
	rendered   []byte
	deltaOps   []DeltaOp
}

// Archive runs the 5-step archive pipeline (doc.go) for req.Change:
// gate-check, conflict-check, pre-image + fold (in memory), write the
// folded specs + relocate the change folder, then post-image + ledger
// append, and finally the post-write self-check.
func Archive(req Request) (Result, error) {
	if strings.TrimSpace(req.Root) == "" {
		return Result{}, fmt.Errorf("%w: Root is required", ErrCouldNotRun)
	}
	if strings.TrimSpace(req.Change) == "" {
		return Result{}, fmt.Errorf("%w: Change is required", ErrCouldNotRun)
	}

	changeDir := filepath.Join(req.Root, "openspec", "changes", req.Change)
	if info, err := os.Stat(changeDir); err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf(
			"%w: change %q not found under %s",
			ErrCouldNotRun, req.Change, filepath.Join(req.Root, "openspec", "changes"),
		)
	}

	meta, err := validate.ReadProposalMeta(changeDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
	}

	result := Result{Change: req.Change, Type: meta.Type, Issue: meta.Issue}

	// --- Step 1: GATE CHECK ---
	violations, err := checkGates(changeDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: checking gates: %w", ErrCouldNotRun, err)
	}
	if len(violations) > 0 {
		if !req.ForceGates {
			return Result{}, fmt.Errorf("%w: %s", ErrGatesNotApproved, strings.Join(violations, "; "))
		}
		result.GatesOverridden = true
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("gate check overridden (--force-gates): %s", strings.Join(violations, "; ")))
	}

	hasDelta := validate.HasSpecsDeltas(changeDir)

	var capabilities []string
	ownDeltas := map[string]*spec.Delta{}
	if hasDelta {
		capabilities, err = discoverCapabilities(changeDir)
		if err != nil {
			return Result{}, fmt.Errorf("%w: discovering capabilities: %w", ErrCouldNotRun, err)
		}
		for _, cap := range capabilities {
			deltaPath := filepath.Join(changeDir, "specs", cap, "spec.md")
			data, rerr := os.ReadFile(deltaPath)
			if rerr != nil {
				return Result{}, fmt.Errorf("%w: reading delta %s: %w", ErrCouldNotRun, deltaPath, rerr)
			}
			d, perr := spec.ParseDelta(data)
			if perr != nil {
				return Result{}, fmt.Errorf("%w: parsing delta %s: %w", ErrFoldFailed, deltaPath, perr)
			}
			ownDeltas[cap] = d
		}

		// --- Step 2: CONFLICT CHECK ---
		conflicts, cwarnings, cerr := checkConflicts(req.Root, req.Change, ownDeltas)
		result.Warnings = append(result.Warnings, cwarnings...)
		if cerr != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, cerr)
		}
		if len(conflicts) > 0 {
			if !req.ForceConflicts {
				return Result{}, fmt.Errorf("%w: %s", ErrConflict, formatConflicts(conflicts))
			}
			result.ConflictsOverridden = true
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("conflict check overridden (--force-conflicts): %s", formatConflicts(conflicts)))
		}
	}

	// --- Steps 3+4: PRE-IMAGE + FOLD (entirely in memory; nothing written
	// yet, so any failure here leaves the project untouched) ---
	var folded []foldedCapability
	for _, cap := range capabilities {
		specPath := filepath.Join(req.Root, "openspec", "specs", cap, "spec.md")

		var base *spec.RequirementSet
		preImage := emptyImageSHA
		if data, rerr := os.ReadFile(specPath); rerr == nil {
			preImage = hashBytes(data)
			base, err = spec.ParseRequirementSet(data)
			if err != nil {
				return Result{}, fmt.Errorf("%w: parsing live spec %s: %w", ErrCouldNotRun, specPath, err)
			}
		} else if !os.IsNotExist(rerr) {
			return Result{}, fmt.Errorf("%w: reading live spec %s: %w", ErrCouldNotRun, specPath, rerr)
		}

		foldedSet, ferr := spec.Fold(cap, req.Change, base, ownDeltas[cap])
		if ferr != nil {
			return Result{}, fmt.Errorf("%w: folding capability %q: %w", ErrFoldFailed, cap, ferr)
		}
		rendered := foldedSet.Render()

		folded = append(folded, foldedCapability{
			capability: cap,
			preImage:   preImage,
			postImage:  hashBytes(rendered),
			rendered:   rendered,
			deltaOps:   deltaOpsFromDelta(ownDeltas[cap]),
		})
	}

	// --- Write the folded specs as ONE all-or-nothing group (doc.go's
	// "prepare, then commit" note): every capability's write is first
	// staged (the failure-prone part — allocating, writing, flushing a
	// temp file) before ANY of them is made visible, so a Prepare failure
	// for capability N leaves capabilities 1..N-1 untouched too — not just
	// N itself. Only once every capability has staged cleanly do we commit
	// them, each via the same atomic replace WriteFile itself uses. ---
	prepared := make([]*atomicwrite.PreparedWrite, 0, len(folded))
	committed := 0
	defer func() {
		for _, w := range prepared[committed:] {
			w.Discard()
		}
	}()
	for _, fc := range folded {
		specPath := filepath.Join(req.Root, "openspec", "specs", fc.capability, "spec.md")
		if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
			return Result{}, fmt.Errorf("%w: creating %s: %w", ErrCouldNotRun, filepath.Dir(specPath), err)
		}
		w, err := atomicwrite.Prepare(specPath, fc.rendered, 0o644)
		if err != nil {
			return Result{}, fmt.Errorf("%w: preparing write for %s: %w", ErrCouldNotRun, specPath, err)
		}
		prepared = append(prepared, w)
	}
	for _, w := range prepared {
		if err := w.Commit(); err != nil {
			return Result{}, fmt.Errorf("%w: committing write: %w", ErrCouldNotRun, err)
		}
		committed++
	}

	// --- Relocate the change folder (the "commit point": once this
	// succeeds, the change is archived) ---
	archiveRoot := filepath.Join(req.Root, "openspec", "changes", "archive")
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("%w: creating %s: %w", ErrCouldNotRun, archiveRoot, err)
	}
	archiveDir := filepath.Join(archiveRoot, req.Change)
	if _, err := os.Stat(archiveDir); err == nil {
		return Result{}, fmt.Errorf("%w: %s already exists (change %q already archived?)", ErrCouldNotRun, archiveDir, req.Change)
	}
	if err := os.Rename(changeDir, archiveDir); err != nil {
		return Result{}, fmt.Errorf("%w: relocating %s to %s: %w", ErrCouldNotRun, changeDir, archiveDir, err)
	}

	// --- Step 5: POST-IMAGE + LEDGER APPEND ---
	manifestSha, merr := ManifestSHA(archiveDir)
	if merr != nil {
		return Result{}, fmt.Errorf("%w: hashing archived folder %s: %w", ErrCouldNotRun, archiveDir, merr)
	}

	var records []Record
	if hasDelta {
		for _, fc := range folded {
			records = append(records, Record{
				Change:              req.Change,
				Issue:               meta.Issue,
				Capability:          fc.capability,
				PreImageSha:         fc.preImage,
				PostImageSha:        fc.postImage,
				DeltaOps:            fc.deltaOps,
				ArchiveManifestSha:  manifestSha,
				GatesOverridden:     result.GatesOverridden,
				ConflictsOverridden: result.ConflictsOverridden,
			})
		}
	} else {
		// Delta-less bug archive (doc.go): exactly one record, no
		// capability affected.
		records = append(records, Record{
			Change:              req.Change,
			Issue:               meta.Issue,
			Capability:          "",
			PreImageSha:         emptyImageSHA,
			PostImageSha:        emptyImageSHA,
			DeltaOps:            []DeltaOp{},
			ArchiveManifestSha:  manifestSha,
			GatesOverridden:     result.GatesOverridden,
			ConflictsOverridden: result.ConflictsOverridden,
		})
	}

	appended, aerr := AppendRecords(req.Root, records)
	if aerr != nil {
		return Result{}, fmt.Errorf("%w: appending ledger record(s): %w", ErrCouldNotRun, aerr)
	}
	result.Records = appended

	// --- Post-write self-check (doc.go) ---
	if scErr := selfCheck(req.Root, archiveDir, appended, hasDelta); scErr != nil {
		return result, scErr
	}

	return result, nil
}

// deltaOpsFromDelta renders d's ops in the fold's own fixed order
// (RENAMED -> REMOVED -> MODIFIED -> ADDED, spec-lifecycle.md §6.1) — see
// doc.go for the RENAMED "<from> -> <to>" convention.
func deltaOpsFromDelta(d *spec.Delta) []DeltaOp {
	ops := make([]DeltaOp, 0, len(d.Renamed)+len(d.Removed)+len(d.Modified)+len(d.Added))
	for _, rn := range d.Renamed {
		ops = append(ops, DeltaOp{Op: string(spec.OpRenamed), Requirement: rn.From + " -> " + rn.To})
	}
	for _, name := range d.Removed {
		ops = append(ops, DeltaOp{Op: string(spec.OpRemoved), Requirement: name})
	}
	for _, r := range d.Modified {
		ops = append(ops, DeltaOp{Op: string(spec.OpModified), Requirement: r.Name})
	}
	for _, r := range d.Added {
		ops = append(ops, DeltaOp{Op: string(spec.OpAdded), Requirement: r.Name})
	}
	return ops
}

// selfCheck re-reads, from disk, the archived folder's manifest and (when
// a fold happened) every folded capability spec, and compares them
// against the just-appended records — doc.go's "post-write self-check".
func selfCheck(root, archiveDir string, records []Record, hasDelta bool) error {
	if len(records) == 0 {
		return nil
	}

	wantManifest := records[0].ArchiveManifestSha
	gotManifest, err := ManifestSHA(archiveDir)
	if err != nil {
		return fmt.Errorf("%w: recomputing archive manifest for %s: %w", ErrSelfCheckFailed, archiveDir, err)
	}
	if gotManifest != wantManifest {
		return fmt.Errorf(
			"%w: archived folder %s manifest %s does not match the just-written ledger record's archiveManifestSha %s",
			ErrSelfCheckFailed, archiveDir, gotManifest, wantManifest,
		)
	}

	if !hasDelta {
		return nil
	}
	for _, r := range records {
		specPath := filepath.Join(root, "openspec", "specs", r.Capability, "spec.md")
		got, err := hashFile(specPath)
		if err != nil {
			return fmt.Errorf("%w: re-reading %s: %w", ErrSelfCheckFailed, specPath, err)
		}
		if got != r.PostImageSha {
			return fmt.Errorf(
				"%w: %s hash %s does not match ledger record (seq %d) postImageSha %s",
				ErrSelfCheckFailed, specPath, got, r.Seq, r.PostImageSha,
			)
		}
	}
	return nil
}
