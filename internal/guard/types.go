package guard

// Kind is the exact violation-type enum spec-lifecycle.md §6.3 /
// implementation-plan.md §2.4 pin: "archive_mutated | projection_drift |
// chain_break | ledger_missing". There is no catch-all — every Finding
// cites one of these four.
type Kind string

// The four pinned Kind values. See doc.go for which check(s) emit which
// kind, and for the "missing/extra capability" decision below.
const (
	// KindArchiveMutated is content-hash drift under
	// changes/archive/<change>/** against that change's ledger
	// record(s)' archiveManifestSha (check 1, "immutability") — including
	// the two edge cases doc.go documents: an archived folder on disk with
	// no ledger record at all, and a ledger record whose archived folder
	// is missing from disk. Both are, at bottom, "this archive's
	// integrity cannot be trusted", the same family as a hand-edited byte.
	KindArchiveMutated Kind = "archive_mutated"

	// KindProjectionDrift is "the live openspec/specs/ tree does not match
	// what the ledger's history says it should be" — emitted by BOTH
	// check 2's fast-path digest comparison (live hash vs latest
	// postImageSha) and check 3's from-empty replay (rendered-replay bytes
	// vs live bytes). doc.go documents the decision to reuse this SAME
	// kind (rather than invent a 5th enum member) for two structural
	// variants replay alone can detect: a live capability the ledger never
	// folds anything for, and a capability the ledger folds but which has
	// no live spec.md at all.
	KindProjectionDrift Kind = "projection_drift"

	// KindChainBreak is a per-capability preImageSha/postImageSha link
	// mismatch between consecutive ledger records (or, for a capability's
	// first-ever record, a preImageSha other than the documented
	// empty-image sentinel) — check 2's chain-of-custody half.
	KindChainBreak Kind = "chain_break"

	// KindLedgerMissing is openspec/ledger.jsonl being entirely absent
	// while at least one archived change folder exists under
	// changes/archive/ — a precondition check that runs before (and, if
	// it fires, instead of) checks 1-3, since none of them has anything to
	// check against without a ledger.
	KindLedgerMissing Kind = "ledger_missing"
)

// The three named checks a Finding's Check field identifies, plus the
// ledger-missing precondition. Free text, not a closed enum (unlike Kind) —
// only used for human/machine grouping, never switched on by this package
// itself.
const (
	CheckLedger       = "ledger"
	CheckImmutability = "immutability"
	CheckDigestChain  = "digest_chain"
	CheckReplay       = "replay"
)

// Finding is one detected problem. Capability/Change/Seq are populated
// where relevant to the specific Kind/Check and omitted (zero-valued, so
// omitted from JSON) otherwise — see each check's file for which fields it
// sets.
type Finding struct {
	Check      string `json:"check"`
	Kind       Kind   `json:"kind"`
	Capability string `json:"capability,omitempty"`
	Change     string `json:"change,omitempty"`
	Seq        int    `json:"seq,omitempty"`
	Message    string `json:"message"`
}

// Summary is the machine-readable roll-up, mirroring the sibling
// adr-sourced-constitution primitive's own guard.Summary shape
// (Checked/Violations/Clean) at the grain this domain actually has:
// changes and ledger records, not ADRs.
type Summary struct {
	ChangesChecked int  `json:"changesChecked"`
	RecordsChecked int  `json:"recordsChecked"`
	Findings       int  `json:"findings"`
	Clean          bool `json:"clean"`
}

// Result is exactly what `lifecycle guard --format json` marshals.
type Result struct {
	Findings []Finding `json:"findings"`
	Summary  Summary   `json:"summary"`
}

// Options configures one guard run.
type Options struct {
	// Root is the project root: the directory holding openspec/.
	Root string
}
