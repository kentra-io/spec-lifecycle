package approve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// hashFile returns "sha256:<hex>" for the file at path.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// hashFiles hashes every relPath (slash-separated, relative to dir) and
// returns them as a map keyed by that same relative path — the shape
// approval-state.json's per-entry `artifacts` map uses (spec-lifecycle.md
// §5). Deterministic: never hand-typed, computed fresh every call.
func hashFiles(dir string, relPaths []string) (map[string]string, error) {
	out := make(map[string]string, len(relPaths))
	for _, rel := range relPaths {
		h, err := hashFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("approve: hashing %s: %w", rel, err)
		}
		out[rel] = h
	}
	return out, nil
}

// HashDrift re-hashes each path recorded in entry.Artifacts against dir's
// CURRENT content and returns every path whose hash no longer matches (or
// that no longer exists), sorted — the post-gate artifact-drift check
// spec-lifecycle.md §5/§6.2 describes ("change-folder artifacts stay
// freely editable after approval... status/guard re-hash each recorded
// gate's artifacts against the stored hash and flag drift"). A nil result
// means no drift.
func HashDrift(dir string, recorded map[string]string) []string {
	keys := make([]string, 0, len(recorded))
	for k := range recorded {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var drifted []string
	for _, rel := range keys {
		want := recorded[rel]
		got, err := hashFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || got != want {
			drifted = append(drifted, rel)
		}
	}
	return drifted
}
