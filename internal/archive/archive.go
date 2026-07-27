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
	// rendered is the deterministic markdown PROJECTION written to the live
	// openspec/specs/<cap>/spec.md; postImage is its hash (the value guard's
	// digest chain and from-empty replay compare against). source is the
	// owned YAML the fold produced, written to spec.yaml so a subsequent
	// archive of this capability can read it back as its fold base.
	rendered []byte
	source   []byte
	deltaOps []DeltaOp
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

	// --- Step 1b: TASKS-COMPLETION GATE (harness orchestration.md §5.5) ---
	incomplete, err := checkTasksComplete(changeDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: checking tasks completion: %w", ErrCouldNotRun, err)
	}
	if len(incomplete) > 0 {
		if !req.ForceIncompleteTasks {
			return Result{}, fmt.Errorf("%w: %s", ErrTasksIncomplete, strings.Join(incomplete, "; "))
		}
		result.TasksIncompleteOverridden = true
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("tasks-completion gate overridden (--force-incomplete-tasks): %s", strings.Join(incomplete, "; ")))
	}

	hasDelta := validate.HasSpecsDeltas(changeDir)

	var capabilities []string
	ownDeltas := map[string]*spec.SpecDelta{}
	if hasDelta {
		capabilities, err = discoverCapabilities(changeDir)
		if err != nil {
			return Result{}, fmt.Errorf("%w: discovering capabilities: %w", ErrCouldNotRun, err)
		}
		for _, cap := range capabilities {
			deltaPath := filepath.Join(changeDir, "specs", cap, "spec.yaml")
			data, rerr := os.ReadFile(deltaPath)
			if rerr != nil {
				return Result{}, fmt.Errorf("%w: reading delta %s: %w", ErrCouldNotRun, deltaPath, rerr)
			}
			d, perr := spec.ParseDeltaYAML(data)
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
		// The pre-image is the hash of the live markdown PROJECTION
		// (spec.md), the same basis guard's digest chain compares a prior
		// record's postImageSha against; the fold BASE, by contrast, is read
		// from the owned YAML source (spec.yaml), since the projection is
		// one-way and can never be re-parsed into the model (design D3).
		projPath := filepath.Join(req.Root, "openspec", "specs", cap, "spec.md")
		sourcePath := filepath.Join(req.Root, "openspec", "specs", cap, "spec.yaml")

		preImage := emptyImageSHA
		if data, rerr := os.ReadFile(projPath); rerr == nil {
			preImage = hashBytes(data)
		} else if !os.IsNotExist(rerr) {
			return Result{}, fmt.Errorf("%w: reading live spec %s: %w", ErrCouldNotRun, projPath, rerr)
		}

		var base *spec.LivingSpec
		if data, rerr := os.ReadFile(sourcePath); rerr == nil {
			base, err = spec.ParseLivingSpecYAML(data)
			if err != nil {
				return Result{}, fmt.Errorf("%w: parsing live spec source %s: %w", ErrCouldNotRun, sourcePath, err)
			}
		} else if !os.IsNotExist(rerr) {
			return Result{}, fmt.Errorf("%w: reading live spec source %s: %w", ErrCouldNotRun, sourcePath, rerr)
		}

		foldedSet, ferr := spec.FoldYAML(cap, base, ownDeltas[cap])
		if ferr != nil {
			return Result{}, fmt.Errorf("%w: folding capability %q: %w", ErrFoldFailed, cap, ferr)
		}
		rendered := foldedSet.RenderProjection()
		source, serr := foldedSet.RenderSource()
		if serr != nil {
			return Result{}, fmt.Errorf("%w: rendering folded source for capability %q: %w", ErrCouldNotRun, cap, serr)
		}

		folded = append(folded, foldedCapability{
			capability: cap,
			preImage:   preImage,
			postImage:  hashBytes(rendered),
			rendered:   rendered,
			source:     source,
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
		capDir := filepath.Join(req.Root, "openspec", "specs", fc.capability)
		if err := os.MkdirAll(capDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("%w: creating %s: %w", ErrCouldNotRun, capDir, err)
		}
		// Write the owned YAML source (spec.yaml) AND its deterministic
		// markdown projection (spec.md) as part of the same all-or-nothing
		// group: the source is the fold base for any later archive; the
		// projection is what guard reads for its digest chain and from-empty
		// replay (design D3/D9).
		sourcePath := filepath.Join(capDir, "spec.yaml")
		ws, err := atomicwrite.Prepare(sourcePath, fc.source, 0o644)
		if err != nil {
			return Result{}, fmt.Errorf("%w: preparing write for %s: %w", ErrCouldNotRun, sourcePath, err)
		}
		prepared = append(prepared, ws)

		projPath := filepath.Join(capDir, "spec.md")
		wp, err := atomicwrite.Prepare(projPath, fc.rendered, 0o644)
		if err != nil {
			return Result{}, fmt.Errorf("%w: preparing write for %s: %w", ErrCouldNotRun, projPath, err)
		}
		prepared = append(prepared, wp)
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
				Change:                    req.Change,
				Issue:                     meta.Issue,
				Capability:                fc.capability,
				PreImageSha:               fc.preImage,
				PostImageSha:              fc.postImage,
				DeltaOps:                  fc.deltaOps,
				ArchiveManifestSha:        manifestSha,
				GatesOverridden:           result.GatesOverridden,
				ConflictsOverridden:       result.ConflictsOverridden,
				TasksIncompleteOverridden: result.TasksIncompleteOverridden,
			})
		}
	} else {
		// Delta-less bug archive (doc.go): exactly one record, no
		// capability affected.
		records = append(records, Record{
			Change:                    req.Change,
			Issue:                     meta.Issue,
			Capability:                "",
			PreImageSha:               emptyImageSHA,
			PostImageSha:              emptyImageSHA,
			DeltaOps:                  []DeltaOp{},
			ArchiveManifestSha:        manifestSha,
			GatesOverridden:           result.GatesOverridden,
			ConflictsOverridden:       result.ConflictsOverridden,
			TasksIncompleteOverridden: result.TasksIncompleteOverridden,
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
// doc.go for the RENAMED "<from> -> <to>" convention. The source is the
// structured YAML delta (spec.SpecDelta); its entries are grouped by op
// into the fixed order here regardless of their author order, matching the
// order FoldYAML applies them and the markdown engine's original output.
func deltaOpsFromDelta(d *spec.SpecDelta) []DeltaOp {
	var renamed, removed, modified, added []DeltaOp
	for _, e := range d.Deltas {
		switch e.Op {
		case spec.OpRenamed:
			renamed = append(renamed, DeltaOp{Op: string(spec.OpRenamed), Requirement: e.From + " -> " + e.To})
		case spec.OpRemoved:
			removed = append(removed, DeltaOp{Op: string(spec.OpRemoved), Requirement: e.Requirement.Name})
		case spec.OpModified:
			modified = append(modified, DeltaOp{Op: string(spec.OpModified), Requirement: e.Requirement.Name})
		case spec.OpAdded:
			added = append(added, DeltaOp{Op: string(spec.OpAdded), Requirement: e.Requirement.Name})
		}
	}
	ops := make([]DeltaOp, 0, len(renamed)+len(removed)+len(modified)+len(added))
	ops = append(ops, renamed...)
	ops = append(ops, removed...)
	ops = append(ops, modified...)
	ops = append(ops, added...)
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
