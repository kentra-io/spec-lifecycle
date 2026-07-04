// Package schema embeds the natively-owned kentra-spec-lifecycle schema
// descriptor (spec-lifecycle.md §4, implementation-plan.md §2.2): the
// artifact set (proposal -> specs -> design -> tasks), its requires: DAG,
// and the four artifact templates (spec-lifecycle.md §4's stage/content
// table; the tasks.md template carries §4.2's milestone/validation-contract
// grammar verbatim).
//
// The schema.yaml + templates/*.md shape mirrors OpenSpec v1.5.0's own
// [experimental] project-local schema descriptor layout —
// <projectRoot>/openspec/schemas/<name>/{schema.yaml,templates/*.md} —
// confirmed against a v1.5.0 checkout's src/core/artifact-graph/resolver.ts
// (getProjectSchemasDir/resolveSchema) and its packaged
// schemas/spec-driven/{schema.yaml,templates/*.md}. That shape is copied
// for format-compatibility and human documentation ONLY: nothing at
// runtime reads this descriptor back. Stage ordering and the artifact DAG
// are enforced by lifecycle's own gate records (approval-state.json) and
// `lifecycle validate` (internal/validate), never by a schema interpreter
// — deliberately, so this primitive never grows the "young `[experimental]`
// subsystem" risk class OpenSpec's own schema loader carries
// (implementation-plan.md §0.5/§11).
//
// Install writes the descriptor tree to a project directory; Verify checks
// an already-installed tree still matches the embedded assets
// byte-for-byte (the drift check a future `lifecycle init`/regen path, M6,
// needs). Neither function is `lifecycle init` itself — that verb (and its
// idempotent compose of this package with config.yaml wiring, constitution
// preflight, and skill fan-out) is M6, per implementation-plan.md §8's
// milestone map; M2 only needs the schema descriptor + its writer/verifier
// to exist and be tested.
package schema

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
)

// Name is the schema's identifier: both the schema.yaml `name:` field and
// the directory name under openspec/schemas/ (spec-lifecycle.md §12 item
// 4 — "the format-compatible schema is kentra-branded").
const Name = "kentra-spec-lifecycle"

//go:embed schema.yaml templates/*.md
var assets embed.FS

// relPaths returns every embedded asset's path relative to the descriptor
// root (e.g. "schema.yaml", "templates/proposal.md"), sorted for
// deterministic iteration — the single enumeration Install and Verify both
// walk, so adding a template later never requires updating a second,
// hand-maintained file list.
func relPaths() ([]string, error) {
	var paths []string
	err := fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("schema: walking embedded assets: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

// Dir returns the descriptor's install root under a project directory:
// <dir>/openspec/schemas/kentra-spec-lifecycle.
func Dir(dir string) string {
	return filepath.Join(dir, "openspec", "schemas", Name)
}

// Install writes the embedded schema descriptor to
// <dir>/openspec/schemas/kentra-spec-lifecycle/{schema.yaml,templates/*.md},
// creating directories as needed. Every file is written atomically
// (internal/atomicwrite: a torn write here is exactly the "log is truth"
// failure mode that primitive is designed out at the syscall level).
// Install is idempotent — re-running it against an already-installed,
// unmodified tree rewrites the same bytes — and does not itself check for
// drift; callers that need drift detection call Verify first.
func Install(dir string) error {
	root := Dir(dir)
	paths, err := relPaths()
	if err != nil {
		return err
	}
	for _, rel := range paths {
		data, err := assets.ReadFile(rel)
		if err != nil {
			return fmt.Errorf("schema: reading embedded %s: %w", rel, err)
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("schema: creating %s: %w", filepath.Dir(target), err)
		}
		if err := atomicwrite.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("schema: writing %s: %w", target, err)
		}
	}
	return nil
}

// Mismatch names one installed file that fails to verify against the
// embedded asset it should mirror.
type Mismatch struct {
	// Rel is the path relative to the descriptor root (e.g.
	// "templates/tasks.md").
	Rel string
	// Reason is "missing" (file absent) or "modified" (present, differs).
	Reason string
}

func (m Mismatch) String() string {
	return fmt.Sprintf("%s: %s", m.Rel, m.Reason)
}

// Verify reports every embedded asset that is missing from, or
// byte-differs under, <dir>/openspec/schemas/kentra-spec-lifecycle. A nil
// slice means the installed tree matches exactly. Verify never flags EXTRA
// files present under the descriptor root beyond the embedded set — it
// only checks that every embedded asset is present and unmodified.
func Verify(dir string) ([]Mismatch, error) {
	root := Dir(dir)
	paths, err := relPaths()
	if err != nil {
		return nil, err
	}
	var mismatches []Mismatch
	for _, rel := range paths {
		want, err := assets.ReadFile(rel)
		if err != nil {
			return nil, fmt.Errorf("schema: reading embedded %s: %w", rel, err)
		}
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				mismatches = append(mismatches, Mismatch{Rel: rel, Reason: "missing"})
				continue
			}
			return nil, fmt.Errorf("schema: reading installed %s: %w", rel, err)
		}
		if !bytes.Equal(want, got) {
			mismatches = append(mismatches, Mismatch{Rel: rel, Reason: "modified"})
		}
	}
	return mismatches, nil
}
