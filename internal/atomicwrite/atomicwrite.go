// Package atomicwrite writes a file such that a concurrent reader — or a
// crash — never observes a half-written result: the bytes are written to a
// temp file in the *same directory* as the target, flushed, and then
// atomically moved into place (implementation-plan.md §3, "Atomic writes").
//
// The move is platform-specific: os.Rename on unix (atomic same-filesystem
// replace), and MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH) on Windows,
// whose plain os.Rename cannot replace an existing file. google/renameio —
// the obvious off-the-shelf choice — exports nothing on Windows and is
// therefore disqualified here (plan §3).
//
// Guarantees, stated precisely:
//
//   - Atomicity against PROCESS DEATH, on every platform: at any kill
//     point the target path holds either the complete old content or the
//     complete new content, never a mixture. This is the property the
//     crash-injection tests exercise.
//   - Durability against POWER LOSS: on unix, the file data is fsynced
//     before the rename and the parent directory is fsynced after it, so
//     both the content and the new directory entry are durable when
//     WriteFile returns. On Windows, MoveFileEx WRITE_THROUGH "does not
//     return until the file is actually moved on the disk", but its
//     documented flush guarantee covers only moves performed as
//     copy+delete (cross-volume) — directory-entry durability for our
//     same-volume rename is not explicitly documented; see
//     rename_windows.go. The backstop there is the domain's recovery
//     story: re-run + regen self-heals.
//
// This is the write primitive for every mutating command's file
// operations (the ADR files, constitution.md, the manifest): a torn write
// is the one failure the "log is truth, regen self-heals" recovery story
// cannot paper over, so it is designed out at the syscall level.
package atomicwrite

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile atomically replaces path with data. The temp file is created
// in filepath.Dir(path) so the final move is a same-filesystem rename (a
// cross-device rename is not atomic and would fall back to copy). On any
// error before the move, the temp file is removed and path is left
// untouched.
//
// WriteFile is Prepare followed immediately by Commit — see those for the
// two-phase form a caller writing several files as one all-or-nothing
// group (e.g. internal/archive's multi-capability fold) needs instead of
// this single-file convenience wrapper.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	w, err := Prepare(path, data, perm)
	if err != nil {
		return err
	}
	if err := w.Commit(); err != nil {
		w.Discard()
		return err
	}
	return nil
}

// PreparedWrite is data staged by Prepare in a temp file, not yet visible
// at its final path until Commit moves it there.
type PreparedWrite struct {
	tmp  string
	path string
}

// Prepare stages data for an eventual atomic replace of path: it writes,
// flushes, and closes a temp file in filepath.Dir(path) (the same
// same-filesystem placement WriteFile uses) but does NOT move it into
// place. Splitting "do the (possibly failing) work" from "make it visible"
// lets a caller Prepare N files, and only Commit any of them once every one
// of the N has Prepared successfully — composing WriteFile's single-file
// atomicity into a multi-file group where a failure at Prepare time (the
// expensive, failure-prone part: allocating space, writing, flushing)
// leaves every target path untouched, not just the one that failed.
func Prepare(path string, data []byte, perm os.FileMode) (w *PreparedWrite, err error) {
	dir := filepath.Dir(path)

	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("atomicwrite: create temp file in %s: %w", dir, err)
	}
	tmp := f.Name()

	// Remove the temp file unless we hand it off via a successful Prepare.
	// Guards every early-return error path below.
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("atomicwrite: write temp file: %w", err)
	}
	// Flush to disk before the rename so the moved-into-place file has
	// durable contents even if the machine loses power right after.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("atomicwrite: sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("atomicwrite: close temp file: %w", err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return nil, fmt.Errorf("atomicwrite: chmod temp file: %w", err)
	}

	ok = true
	return &PreparedWrite{tmp: tmp, path: path}, nil
}

// Commit atomically moves w's staged content into place at w.path — the
// same platform-specific replace WriteFile itself uses. Once Commit
// returns nil, w must not be Discarded (its temp file no longer exists at
// its staged path, so Discard would be a harmless no-op, but the pairing
// is not meaningful after a successful Commit).
func (w *PreparedWrite) Commit() error {
	if err := replace(w.tmp, w.path); err != nil {
		return fmt.Errorf("atomicwrite: replace %s: %w", w.path, err)
	}
	return nil
}

// Discard removes w's staged temp file without ever moving it into place —
// the caller decided not to commit this write after all (e.g. a sibling
// Prepare in the same all-or-nothing group failed, or this same
// PreparedWrite's own Commit failed and left the temp file behind). Safe
// to call after a successful Commit too (the temp file is simply already
// gone; the os.Remove error is ignored).
func (w *PreparedWrite) Discard() {
	_ = os.Remove(w.tmp)
}
