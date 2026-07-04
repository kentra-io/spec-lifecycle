package constitution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Hash recomputes the sha256 of <root>/constitution/constitution.md,
// formatted "sha256:<hex>" — byte-identical to the algorithm
// adr-sourced-constitution's own internal/deviation.ConstitutionHash uses
// over the same file, so lifecycle's authoritative recompute
// (approval-state.json's constitutionHash, spec-lifecycle.md §5) and the
// value a deviation.json stamps (deviationConstitutionHash) are directly
// comparable without shelling out (doc.go's spike note).
//
// A missing constitution.md is not an error: lifecycle can run in a
// project before the constitution companion primitive is initialized.
// Hash then returns ("", false, nil); callers decide how to treat an
// absent hash (spec-lifecycle.md §5's example always shows a populated
// constitutionHash, which presumes the companion is already set up).
func Hash(root string) (hash string, ok bool, err error) {
	path := filepath.Join(root, "constitution", "constitution.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("constitution: reading %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), true, nil
}

// HashesEqual reports whether two constitutionHash values are equal,
// accepting either the canonical "sha256:<hex>" form or a bare 64-hex
// digest, case-insensitively on the hex — mirroring
// adr-sourced-constitution/internal/deviation's own hashMatches/
// normalizeHash (unexported there; reimplemented here since the seam is a
// process boundary, not a Go import — doc.go).
func HashesEqual(a, b string) bool {
	return normalizeHash(a) == normalizeHash(b)
}

func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(h, "sha256:")
	h = strings.TrimPrefix(h, "sha256-")
	return h
}
