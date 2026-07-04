package approve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// StatePath returns <changeDir>/approval-state.json.
func StatePath(changeDir string) string {
	return filepath.Join(changeDir, StateFileName)
}

// ReadState reads and parses changeDir's approval-state.json. A missing
// file is not an error: it returns a zero-gates StateFile with
// SchemaVersion already set (every stage is then legitimately "no entry
// yet", internal/status's pending case) — approval-state.json is only
// created on the FIRST approve call, mirroring approval-state.json's own
// append-only nature (nothing to append to yet is not a failure).
func ReadState(changeDir string) (StateFile, error) {
	path := StatePath(changeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			sf := StateFile{SchemaVersion: SchemaVersion, Change: filepath.Base(changeDir)}
			if meta, merr := validate.ReadProposalMeta(changeDir); merr == nil {
				sf.Issue = meta.Issue
			}
			return sf, nil
		}
		return StateFile{}, fmt.Errorf("approve: reading %s: %w", path, err)
	}

	var sf StateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return StateFile{}, fmt.Errorf("approve: %s: not valid JSON: %w", path, err)
	}
	if sf.SchemaVersion != SchemaVersion {
		return StateFile{}, fmt.Errorf(
			"approve: %s: unsupported schemaVersion %d (this build only supports schemaVersion %d)",
			path, sf.SchemaVersion, SchemaVersion,
		)
	}
	return sf, nil
}

// appendEntry appends entry to changeDir's approval-state.json (creating
// it, schema-versioned, if absent) and writes it atomically. Append-only:
// no prior entry is ever modified or removed (spec-lifecycle.md §5:
// "--reject appends; consumers take the latest entry per stage-name").
func appendEntry(changeDir string, entry Entry) error {
	sf, err := ReadState(changeDir)
	if err != nil {
		return err
	}
	sf.Gates = append(sf.Gates, entry)

	out, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("approve: marshaling %s: %w", StateFileName, err)
	}
	out = append(out, '\n')
	return atomicwrite.WriteFile(StatePath(changeDir), out, 0o644)
}

// LatestPerStage reduces gates to (at most) one Entry per Stage: the
// entry with the highest ApprovedAt (RFC3339 timestamps compare
// correctly as strings); ties break on ARRAY ORDER, the later entry
// winning (spec-lifecycle.md §5: "consumers take the latest entry per
// stage-name"). Shared by internal/status (deriving gate state) and by
// any future caller needing the same "what does this stage's record say
// right now" reduction.
func LatestPerStage(gates []Entry) map[Stage]Entry {
	latest := make(map[Stage]Entry, len(gates))
	for _, e := range gates {
		cur, ok := latest[e.Stage]
		if !ok || e.ApprovedAt >= cur.ApprovedAt {
			latest[e.Stage] = e
		}
	}
	return latest
}
