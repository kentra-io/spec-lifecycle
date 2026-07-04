package archive

import "errors"

// Sentinel errors Archive returns (wrapped with %w; cmd/lifecycle maps
// them to exit codes via errors.Is — see doc.go for the exit-code
// rationale). Mirrors internal/approve's own sentinel-error shape.
var (
	// ErrGatesNotApproved means step 1 (gate-check) found at least one
	// required stage not approved/skipped, and req.ForceGates was false:
	// nothing was written.
	ErrGatesNotApproved = errors.New("archive: refused — required gate(s) not approved")
	// ErrConflict means step 2 (conflict-check) found another in-flight
	// change touching a requirement this change also targets via
	// MODIFIED/REMOVED/RENAMED, and req.ForceConflicts was false: nothing
	// was written.
	ErrConflict = errors.New("archive: refused — a conflicting in-flight change touches the same requirement")
	// ErrFoldFailed means internal/spec.Fold refused this change's delta
	// against the live capability spec (e.g. MODIFIED of a requirement
	// that no longer exists) — a content problem in the delta itself, not
	// an environment failure: nothing was written.
	ErrFoldFailed = errors.New("archive: refused — the change's delta does not apply cleanly to the live spec")
	// ErrCouldNotRun wraps every environment/usage failure that isn't one
	// of the refusals above: bad request fields, a missing change folder,
	// an unreadable approval-state.json or ledger, an already-archived
	// change folder colliding with the relocate target.
	ErrCouldNotRun = errors.New("archive: could not run")
	// ErrSelfCheckFailed means the post-write self-check (doc.go) found
	// that re-reading the just-archived folder (or a just-folded spec.md)
	// from disk does not match the ledger record Archive just wrote — an
	// internal-invariant failure, not something the caller can fix by
	// changing their request.
	ErrSelfCheckFailed = errors.New("archive: internal self-check failed after writing the ledger record")
)

// Request bundles one `lifecycle archive <change>` invocation's inputs.
type Request struct {
	// Root is the project root: the directory holding openspec/.
	Root string
	// Change is the change-folder name under openspec/changes/ (not
	// openspec/changes/archive/).
	Change string
	// ForceGates bypasses step 1's refusal when a required gate is not
	// approved/skipped (doc.go's "soft in Phase 1" reading). The override
	// is recorded, never silent: Result.GatesOverridden plus
	// Record.GatesOverridden on every appended record.
	ForceGates bool
	// ForceConflicts bypasses step 2's refusal when another in-flight
	// change touches the same requirement. Recorded the same way as
	// ForceGates (Result/Record.ConflictsOverridden).
	ForceConflicts bool
}

// Result is what a successful (or partially-completed, on a self-check
// failure) Archive call reports back to the caller.
type Result struct {
	Change              string   `json:"change"`
	Type                string   `json:"type"`
	Issue               string   `json:"issue"`
	Records             []Record `json:"records"`
	GatesOverridden     bool     `json:"gatesOverridden,omitempty"`
	ConflictsOverridden bool     `json:"conflictsOverridden,omitempty"`
	// Warnings carries non-fatal notes: gate/conflict overrides, and any
	// sibling change folder skipped during the conflict-check because its
	// own delta could not be read/parsed.
	Warnings []string `json:"warnings,omitempty"`
}

// DeltaOp is one entry in a Record's deltaOps (implementation-plan.md
// §2.4's `{"op":"ADDED","requirement":"Password login"}` shape). See
// doc.go for the RENAMED "<from> -> <to>" convention.
type DeltaOp struct {
	Op          string `json:"op"`
	Requirement string `json:"requirement"`
}

// Record is one ledger entry (spec-lifecycle.md §6.3, implementation-plan.md
// §2.4) — one per affected capability for a feature/promoted-bug archive,
// or exactly one (Capability: "") for a delta-less bug archive (doc.go).
type Record struct {
	Seq                int       `json:"seq"`
	Change             string    `json:"change"`
	Issue              string    `json:"issue"`
	Capability         string    `json:"capability"`
	PreImageSha        string    `json:"preImageSha"`
	PostImageSha       string    `json:"postImageSha"`
	DeltaOps           []DeltaOp `json:"deltaOps"`
	ArchiveManifestSha string    `json:"archiveManifestSha"`
	// GatesOverridden/ConflictsOverridden are additive beyond the pinned
	// shape (doc.go) — omitempty so an un-overridden archive's record is
	// byte-for-byte the pinned shape.
	GatesOverridden     bool `json:"gatesOverridden,omitempty"`
	ConflictsOverridden bool `json:"conflictsOverridden,omitempty"`
}

// Conflict is one requirement-level collision the conflict-check found
// between this change and another in-flight one.
type Conflict struct {
	Capability  string
	Requirement string
	OtherChange string
}
