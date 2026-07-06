package approve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kentra-io/spec-lifecycle/internal/constitution"
	"github.com/kentra-io/spec-lifecycle/internal/schema"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// Sentinel errors Approve returns (wrapped with %w; test/CLI callers use
// errors.Is). Their exit-code mapping (implementation-plan.md's "exit
// codes consistent with validate: 0 ok / 1 findings-or-refusal / 2
// usage-env") lives in cmd/lifecycle, not here — this package stays
// exec-independent.
var (
	// ErrInvalidArtifact means validateForStage found at least one
	// error-severity Finding; nothing was written (Result.Findings
	// carries them).
	ErrInvalidArtifact = errors.New("approve: refused — the gated artifact has validation errors")
	// ErrDeviationInvalid means constitution deviation validate exited 1
	// (invalid deviation.json); nothing was written.
	ErrDeviationInvalid = errors.New("approve: refused — deviation.json is invalid")
	// ErrCouldNotRun wraps every environment/usage failure that isn't a
	// validation or consent refusal: bad flags, a missing change folder,
	// an unreadable approval-state.json, a constitution binary that
	// could not run at all (exit 2 in its own contract).
	ErrCouldNotRun = errors.New("approve: could not run")
)

// nowFunc is swappable in tests.
var nowFunc = time.Now

// Request bundles one `lifecycle approve` invocation's inputs.
type Request struct {
	// Root is the project root: the directory holding openspec/,
	// lifecycle.yml, and (if the companion primitive is set up)
	// constitution/.
	Root   string
	Change string // change folder name under openspec/changes/
	Stage  Stage
	Reject bool
	// DesignSkip records designSkipped:true on a StageRefine entry
	// (spec-lifecycle.md §3.2). Only valid together with StageRefine.
	DesignSkip bool
	Notes      string
	ApprovedBy string
	// Consent decides whether this write may proceed at all (consent.go).
	Consent ConsentGate
	// ConstitutionBin is the resolved constitution binary path, or "" if
	// it could not be located — only fatal when this Stage
	// requiresDeviation (design/plan).
	ConstitutionBin string
}

// Result is what a successful (or refused-with-findings) Approve call
// reports back to the caller.
type Result struct {
	Entry    Entry
	Findings []validate.Finding // non-nil only alongside ErrInvalidArtifact
	Warnings []string           // non-fatal notes (e.g. a constitutionHash mismatch, a stale constitution version)
}

// Approve resolves req.Stage's artifact set, validates it (unless
// req.Reject), hashes it, recomputes constitutionHash, runs the
// constitution plan-gate's deviation validate at gates 2/3, and appends
// one Entry to req.Change's approval-state.json. See package doc.go for
// the bug-flow composition and consent.go for the consent contract.
func Approve(req Request) (Result, error) {
	if !req.Stage.IsRecognized() {
		return Result{}, fmt.Errorf("%w: unrecognized stage %q (want one of %v)", ErrCouldNotRun, req.Stage, Stages)
	}
	if req.DesignSkip && req.Stage != StageRefine {
		return Result{}, fmt.Errorf("%w: --design-skip is only valid with --stage refine", ErrCouldNotRun)
	}

	verb := fmt.Sprintf("record a %s decision for change %q, stage %q", statusVerb(req.Reject), req.Change, req.Stage)
	if err := req.Consent.Confirm(verb); err != nil {
		return Result{}, err
	}

	changeDir := filepath.Join(req.Root, "openspec", "changes", req.Change)
	if info, err := os.Stat(changeDir); err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf(
			"%w: change %q not found under %s",
			ErrCouldNotRun, req.Change, filepath.Join(req.Root, "openspec", "changes"),
		)
	}

	var result Result

	if !req.Reject {
		findings, err := validateForStage(changeDir, req.Stage)
		if err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
		}
		if hasError(findings) {
			return Result{Findings: findings}, ErrInvalidArtifact
		}
	}

	def, err := schema.Load()
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
	}
	globs, err := ArtifactGlobs(def, req.Stage)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
	}
	var relFiles []string
	for _, g := range globs {
		files, err := resolveArtifactFiles(changeDir, g)
		if err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
		}
		relFiles = append(relFiles, files...)
	}
	artifacts, err := hashFiles(changeDir, relFiles)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
	}

	entry := Entry{
		Stage:         req.Stage,
		Status:        StatusApproved,
		DesignSkipped: req.Stage == StageRefine && req.DesignSkip,
		Artifacts:     artifacts,
		ApprovedBy:    req.ApprovedBy,
		ApprovedAt:    nowFunc().UTC().Format(time.RFC3339),
		Notes:         req.Notes,
	}
	if req.Reject {
		entry.Status = StatusRejected
	}

	hash, ok, herr := constitution.Hash(req.Root)
	if herr != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, herr)
	}
	if ok {
		entry.ConstitutionHash = hash
	} else {
		result.Warnings = append(result.Warnings,
			"constitution/constitution.md not found; constitutionHash omitted (initialize the constitution companion primitive)")
	}

	if requiresDeviation(req.Stage) && !req.Reject {
		warnings, derr := runDeviationGate(req, changeDir, &entry)
		result.Warnings = append(result.Warnings, warnings...)
		if derr != nil {
			return Result{Warnings: result.Warnings}, derr
		}
	}

	if err := appendEntry(changeDir, entry); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrCouldNotRun, err)
	}

	result.Entry = entry
	return result, nil
}

func statusVerb(reject bool) string {
	if reject {
		return "reject"
	}
	return "approve"
}

// runDeviationGate implements spec-lifecycle.md §3.3/§7 item 5's gate-2/3
// step: deviation.json must be present, `constitution deviation validate`
// must not report it invalid or unable to run, and its own
// constitutionHash is compared against entry.ConstitutionHash (a mismatch
// is a warning, not a failure — the constitution moved via an accepted
// ADR at gate 2, §7 item 5's documented case). On success entry's
// DeviationRef/DeviationConstitutionHash are populated in place.
func runDeviationGate(req Request, changeDir string, entry *Entry) (warnings []string, err error) {
	devPath := filepath.Join(changeDir, "deviation.json")
	data, rerr := os.ReadFile(devPath)
	if rerr != nil {
		return nil, fmt.Errorf(
			"%w: gate %q requires %s (run the constitution plan-gate skill first): %w",
			ErrCouldNotRun, req.Stage, devPath, rerr,
		)
	}
	if req.ConstitutionBin == "" {
		return nil, fmt.Errorf(
			"%w: gate %q requires the constitution binary to validate deviation.json, but it was not found",
			ErrCouldNotRun, req.Stage,
		)
	}

	absPath, aerr := filepath.Abs(devPath)
	if aerr != nil {
		return nil, fmt.Errorf("%w: %w", ErrCouldNotRun, aerr)
	}
	devRes, derr := constitution.DeviationValidate(req.ConstitutionBin, req.Root, absPath)
	if derr != nil {
		return nil, fmt.Errorf("%w: %w", ErrCouldNotRun, derr)
	}
	if strings.TrimSpace(devRes.Stderr) != "" {
		warnings = append(warnings, strings.TrimSpace(devRes.Stderr))
	}
	switch devRes.ExitCode {
	case constitution.DeviationCouldNotRun:
		return warnings, fmt.Errorf(
			"%w: constitution deviation validate could not run: %s",
			ErrCouldNotRun, strings.TrimSpace(devRes.Stderr),
		)
	case constitution.DeviationInvalid:
		return warnings, fmt.Errorf("%w: %s", ErrDeviationInvalid, strings.TrimSpace(devRes.Stderr))
	}

	ref := "deviation.json"
	entry.DeviationRef = &ref

	if devHash, ok := readDeviationConstitutionHash(data); ok {
		entry.DeviationConstitutionHash = &devHash
		if entry.ConstitutionHash != "" && !constitution.HashesEqual(devHash, entry.ConstitutionHash) {
			warnings = append(warnings, fmt.Sprintf(
				"constitutionHash changed since the plan-gate ran (deviation.json: %s, recomputed: %s) — "+
					"the constitution moved via an accepted ADR; both are kept (spec-lifecycle.md §7 item 5)",
				devHash, entry.ConstitutionHash))
		}
	}
	return warnings, nil
}

// readDeviationConstitutionHash extracts deviation.json's own
// constitutionHash field without depending on the constitution
// primitive's (unexported, different-module) Report type — the seam is a
// process boundary (doc.go), so this package only ever reads the one
// field it needs.
func readDeviationConstitutionHash(data []byte) (string, bool) {
	var doc struct {
		ConstitutionHash string `json:"constitutionHash"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.ConstitutionHash == "" {
		return "", false
	}
	return doc.ConstitutionHash, true
}
