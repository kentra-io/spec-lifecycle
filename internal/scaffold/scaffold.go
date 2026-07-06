package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
)

// Mode selects the drift policy Refresh applies (plan §2.2, §6, mirrored
// from the constitution):
//   - ModeInit: a drifted target is rewritten only after --force or an
//     interactive confirm; otherwise Refresh refuses (returns an error).
//   - ModeWarn: a drifted target is never rewritten — Refresh warns and
//     leaves it, so drift in a user file can never block whatever mutating
//     verb triggers a refresh.
type Mode int

// Refresh modes (see Mode).
const (
	ModeInit Mode = iota
	ModeWarn
)

// BlockTarget names a managed-block target (a file such as CLAUDE.md or
// AGENTS.md) and the interior content Refresh should keep it pointed at.
//
// TODO(M6): in the constitution, blockTargets is derived from config.Config
// (which agent-instruction files are enabled, and their fixed interior
// text — e.g. the `@constitution/constitution.md` import). spec-lifecycle
// has no `lifecycle.yml`-driven config yet (that lands with internal/config,
// M3/M6): callers construct BlockTarget values directly for now. M6 wires
// this from the parsed config the same way.
type BlockTarget struct {
	Rel      string // repo-root-relative, slash form
	Interior string
}

// SkillItem names a single fanned-out skill file (an embedded SKILL.md
// copied into an agent's skills tree) and its content.
//
// TODO(M6/M7): in the constitution, skillItems() enumerates an embedded
// skills/ filesystem (go:embed) and crosses it with the skills trees the
// config selects (.claude/.agents/.cursor). spec-lifecycle's own skills/
// bundle and its go:embed wiring don't exist until M7 authors the SKILL.md
// bodies; until then, callers construct SkillItem values directly. TreeDir
// below is kept as the (config-independent) tree-name → directory mapping
// so M6/M7 wiring has a stable helper to call into.
type SkillItem struct {
	Rel     string // repo-root-relative, slash form
	Content []byte
}

// Skill tree keys — mirrors the constitution's config.SkillTree* constants.
// TODO(M6): move these into internal/config once lifecycle.yml parsing
// exists; kept here for now so TreeDir has something to switch on.
const (
	SkillTreeClaude = "claude"
	SkillTreeAgents = "agents"
	SkillTreeCursor = "cursor"
)

// TreeDir maps a skills-tree key to its on-disk directory (plan §6).
func TreeDir(tree string) (string, bool) {
	switch tree {
	case SkillTreeClaude:
		return ".claude", true
	case SkillTreeAgents:
		return ".agents", true
	case SkillTreeCursor:
		return ".cursor", true
	}
	return "", false
}

// Options configures a Refresh pass.
type Options struct {
	Root    string
	Mode    Mode
	Force   bool                              // ModeInit: overwrite drift without prompting
	Confirm func(prompt string) (bool, error) // ModeInit interactive confirm; nil ⇒ never confirm
	Stdout  io.Writer                         // progress ("wrote ...")
	Stderr  io.Writer                         // warnings

	// BlockTargets and SkillItems are the managed-block and fanned-out-file
	// targets a Refresh pass should reconcile. TODO(M6): once
	// `lifecycle.yml` + the embedded skills bundle exist, callers derive
	// these from config the way the constitution's blockTargets(cfg) /
	// skillItems(cfg) do; for M0 they are supplied directly.
	BlockTargets []BlockTarget
	SkillItems   []SkillItem
}

// Refresh writes/updates the managed pointer blocks and fanned-out skill
// copies named by o.BlockTargets/o.SkillItems, drift-protected via
// openspec/.state. It is the shared engine behind `init` (ModeInit) and
// whatever regen-equivalent verb spec-lifecycle grows (ModeWarn). It
// manages only what the caller selects: no targets and no skill items is a
// no-op that never creates a .state file.
func Refresh(o Options) error {
	st := loadStateOrEmpty(o.Root, o.Stderr)

	for _, t := range o.BlockTargets {
		if err := o.refreshBlock(st, t); err != nil {
			return err
		}
	}

	for _, it := range o.SkillItems {
		if err := o.refreshFile(st, it); err != nil {
			return err
		}
	}

	// Don't materialize an empty .state in a repo that manages nothing —
	// that keeps non-integrated repos' trees byte-clean. Once anything is
	// managed (or a .state already exists), persist.
	if st.empty() && !stateExists(o.Root) {
		return nil
	}
	return st.Save(o.Root)
}

// loadStateOrEmpty loads openspec/.state, degrading to a fresh empty state
// (with a prominent stderr warning) when the file is corrupt or its
// schemaVersion is unrecognized. This upholds the invariant that a refresh
// can never be blocked by the state of CLI-owned bookkeeping (plan §2.2,
// §6): a hard error here would propagate out of Refresh and make the
// triggering verb exit non-zero after its real work already landed, lying
// via the exit code. Degrading is safe because drift detection is
// content-hash based: against an empty state a matching interior is still a
// no-op, and a drifted one still triggers the refuse/--force path (init) or
// the warn path. The .state is rebuilt on the next successful write.
func loadStateOrEmpty(root string, stderr io.Writer) *State {
	st, err := LoadState(root)
	if err != nil {
		warnf(stderr, "warning: %s; ignoring it and proceeding with an empty drift state (managed files will be reconciled on the next `lifecycle init`)", err.Error())
		return newState()
	}
	return st
}

// PreflightBlocks checks every managed block target for a malformed marker
// pair without writing anything, so `init` can refuse a structurally
// ambiguous file (exit 2) before it seeds or renders. A clean, absent, or
// simply drifted block is fine here — only a broken marker pair is an error.
func PreflightBlocks(repoRoot string, targets []BlockTarget) error {
	for _, t := range targets {
		content, _, err := readFileIfExists(filepath.Join(repoRoot, filepath.FromSlash(t.Rel)))
		if err != nil {
			return err
		}
		if _, _, _, _, lerr := LocateBlock(content); lerr != nil {
			var me *MarkerError
			if errors.As(lerr, &me) {
				me.Path = t.Rel
			}
			return lerr
		}
	}
	return nil
}

func (o Options) refreshBlock(st *State, t BlockTarget) error {
	fpath := filepath.Join(o.Root, filepath.FromSlash(t.Rel))
	content, _, err := readFileIfExists(fpath)
	if err != nil {
		return err
	}

	found, _, _, curInterior, lerr := LocateBlock(content)
	if lerr != nil {
		var me *MarkerError
		if errors.As(lerr, &me) {
			me.Path = t.Rel
		}
		if o.Mode == ModeWarn {
			warnf(o.Stderr, "regen: %s; leaving it untouched", lerr.Error())
			return nil
		}
		return lerr
	}

	desiredHash := hashContent([]byte(t.Interior))
	write := func() error {
		out, err := ApplyBlock(content, t.Interior)
		if err != nil {
			return err
		}
		if err := writeAtomic(fpath, out); err != nil {
			return err
		}
		st.set(t.Rel, desiredHash)
		o.progressf("wrote managed block in %s", t.Rel)
		return nil
	}

	switch {
	case found && curInterior == t.Interior:
		// Already exactly what we want: record the hash, write nothing.
		st.set(t.Rel, desiredHash)
		return nil
	case !found:
		// No block yet — create or append. Not drift.
		return write()
	}

	stored, hasStored := st.get(t.Rel)
	if hasStored && stored == hashContent([]byte(curInterior)) {
		// Our own previous interior; the CLI's desired content changed
		// (a version bump) — safe to update without a drift prompt.
		return write()
	}
	return o.handleDrift(st, t.Rel, write, stored, hasStored)
}

func (o Options) refreshFile(st *State, it SkillItem) error {
	fpath := filepath.Join(o.Root, filepath.FromSlash(it.Rel))
	content, existed, err := readFileIfExists(fpath)
	if err != nil {
		return err
	}

	desiredHash := hashContent(it.Content)
	write := func() error {
		if err := writeAtomic(fpath, it.Content); err != nil {
			return err
		}
		st.set(it.Rel, desiredHash)
		o.progressf("wrote %s", it.Rel)
		return nil
	}

	switch {
	case existed && bytes.Equal(content, it.Content):
		st.set(it.Rel, desiredHash)
		return nil
	case !existed:
		return write()
	}

	stored, hasStored := st.get(it.Rel)
	if hasStored && stored == hashContent(content) {
		return write()
	}
	return o.handleDrift(st, it.Rel, write, stored, hasStored)
}

// handleDrift applies the mode's policy to a target whose on-disk content
// diverged from what the CLI last wrote (plan §2.2, §6).
func (o Options) handleDrift(st *State, rel string, write func() error, stored string, hasStored bool) error {
	if o.Mode == ModeWarn {
		warnf(o.Stderr, "regen: %s drifted from what `lifecycle init` last wrote; leaving it untouched (run `lifecycle init` to reconcile)", rel)
		// Preserve the prior recorded hash so a later `init` still sees the
		// drift; if we never wrote this target, record nothing for it.
		if hasStored {
			st.set(rel, stored)
		}
		return nil
	}
	if o.Force {
		return write()
	}
	if o.Confirm != nil {
		ok, err := o.Confirm(fmt.Sprintf("%s drifted from what `lifecycle init` last wrote; overwrite the managed content?", rel))
		if err != nil {
			return err
		}
		if ok {
			return write()
		}
	}
	return fmt.Errorf(
		"%s drifted from what `lifecycle init` last wrote; refusing to overwrite it. Re-run with --force to overwrite the managed content (any edits outside the managed region are preserved), or reconcile the file by hand",
		rel,
	)
}

func readFileIfExists(fpath string) (content []byte, existed bool, err error) {
	data, err := os.ReadFile(fpath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func writeAtomic(fpath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		return err
	}
	return atomicwrite.WriteFile(fpath, data, 0o644)
}

func stateExists(repoRoot string) bool {
	_, err := os.Stat(statePath(repoRoot))
	return err == nil
}

func warnf(w io.Writer, format string, a ...any) {
	if w != nil {
		// Best-effort diagnostic; a failed warning write must not derail a
		// refresh (matching the consent gate's `_, _ =` convention).
		_, _ = fmt.Fprintf(w, format+"\n", a...)
	}
}

func (o Options) progressf(format string, a ...any) {
	if o.Stdout != nil {
		_, _ = fmt.Fprintf(o.Stdout, format+"\n", a...)
	}
}
