package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// hashBytes returns "sha256:<hex>" for data — the same "sha256:" prefix
// convention internal/approve's own (unexported) hash.go uses, kept as a
// small duplicated helper rather than a cross-package import: this
// primitive already copies rather than shares small frozen helpers across
// its own repo boundary (implementation-plan.md §2.12's precedent, applied
// here at the internal-package grain).
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// hashFile returns hashBytes of path's content.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

// emptyImageSHA is the documented sentinel for "no live spec.md existed
// yet for this capability" (doc.go's pre-image section): the sha256 of
// the empty byte string, computed once here rather than hand-typed.
var emptyImageSHA = hashBytes(nil)

// EmptyImageSHA is the exported form of emptyImageSHA: internal/guard's
// digest-chain check (spec-lifecycle.md §6.3, implementation-plan.md §2.4
// item 2) needs the SAME sentinel — a first ledger record's preImageSha,
// and a capability with no live spec.md yet, are both checked against this
// value — and the task's own instruction is "REUSE its record types,
// manifest algorithm, and readers; do not duplicate or re-derive hashing".
// Exported rather than recomputed so the two packages can never define the
// sentinel two different ways.
var EmptyImageSHA = emptyImageSHA

// HashFile is the exported form of hashFile, for the same cross-package
// reuse reason as EmptyImageSHA above (internal/guard hashes live
// openspec/specs/<cap>/spec.md files with the exact algorithm archive
// itself used to produce postImageSha at fold time).
func HashFile(path string) (string, error) { return hashFile(path) }

// HashBytes is the exported form of hashBytes, for the same cross-package
// reuse reason as EmptyImageSHA/HashFile above.
func HashBytes(data []byte) string { return hashBytes(data) }

// ManifestSHA content-hashes every regular file under dir (recursively)
// into a single digest: a sorted, slash-separated "relpath\tsha256\n"
// manifest, then hashBytes of that joined manifest. Deterministic across
// platforms (paths are always rendered with filepath.ToSlash before
// hashing, so a Windows checkout's os.PathSeparator never changes the
// result) and across raw-byte content (this repo's fixtures are
// byte-canonical LF, "* -text" in .gitattributes, so no CRLF
// normalization is needed or performed here).
//
// This is the same manifest shape `lifecycle guard`'s immutability check
// (M5, spec-lifecycle.md §6.3 item 1) will recompute against a Record's
// ArchiveManifestSha — exported so that future package can call it
// without duplicating the walk/hash logic.
func ManifestSHA(dir string) (string, error) {
	var lines []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		lines = append(lines, filepath.ToSlash(rel)+"\t"+hashBytes(data))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	return hashBytes([]byte(strings.Join(lines, "\n"))), nil
}
