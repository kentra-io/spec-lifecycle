package archive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
)

// LedgerFileName is the append-only archive ledger's filename, a sibling
// of changes/ and specs/ under openspec/ (doc.go's location decision) —
// NOT inside any one change folder, since the ledger spans every archived
// change, past and present.
const LedgerFileName = "ledger.jsonl"

// LedgerPath returns <root>/openspec/ledger.jsonl.
func LedgerPath(root string) string {
	return filepath.Join(root, "openspec", LedgerFileName)
}

// ReadAll reads and parses every record in root's ledger, in on-disk
// (append) order. A missing ledger file is not an error: it returns a nil
// slice (an empty ledger, legitimately, before the first archive ever
// runs).
func ReadAll(root string) ([]Record, error) {
	path := LedgerPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("archive: reading %s: %w", path, err)
	}
	return parseLedger(path, data)
}

func parseLedger(path string, data []byte) ([]Record, error) {
	var records []Record
	for _, line := range bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("archive: %s: malformed ledger record: %w", path, err)
		}
		records = append(records, r)
	}
	return records, nil
}

// AppendRecords assigns each of records the next monotonic seq (1-based,
// continuing from the highest seq already in root's ledger — folder-date
// independent, spec-lifecycle.md §6.3/errata 3) and appends them, in
// order, to the ledger in a SINGLE atomic write covering the whole file
// (old bytes + every new line): a multi-capability archive's records are
// never partially durable — either all of them land, or (on any error)
// none do, the on-disk file is left exactly as it was. Returns the
// records with Seq populated, in the same order they were passed in.
func AppendRecords(root string, records []Record) ([]Record, error) {
	if len(records) == 0 {
		return nil, nil
	}

	path := LedgerPath(root)
	existingData, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("archive: reading %s: %w", path, err)
	}
	existing, err := parseLedger(path, existingData)
	if err != nil {
		return nil, err
	}

	lastSeq := 0
	for _, r := range existing {
		if r.Seq > lastSeq {
			lastSeq = r.Seq
		}
	}

	out := make([]Record, len(records))
	copy(out, records)

	var buf bytes.Buffer
	buf.Write(existingData)
	for i := range out {
		lastSeq++
		out[i].Seq = lastSeq
		line, merr := json.Marshal(out[i])
		if merr != nil {
			return nil, fmt.Errorf("archive: marshaling ledger record: %w", merr)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	if err := atomicwrite.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("archive: writing %s: %w", path, err)
	}
	return out, nil
}
