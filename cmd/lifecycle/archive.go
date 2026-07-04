package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/archive"
	"github.com/kentra-io/spec-lifecycle/internal/guard"
)

// Exit codes for `lifecycle archive` (spec-lifecycle.md §6.2,
// implementation-plan.md §2.5, internal/archive/doc.go): 0 ok (the
// change was archived: fold + relocate + ledger append all succeeded), 1
// refused (a required gate is not approved, a conflicting in-flight
// change was found, or the delta does not fold cleanly — nothing was
// written; --force-gates/--force-conflicts bypass the first two), 2
// could not run (bad flags/usage, no openspec/ project root, a missing
// change folder, an environment failure, or the post-write self-check
// failing).
const (
	archiveExitRefused     = 1
	archiveExitCouldNotRun = 2
)

func archiveCommand() *cli.Command {
	return &cli.Command{
		Name:      "archive",
		Usage:     "fold a change's delta into openspec/specs/, relocate it, and append the ledger record(s)",
		ArgsUsage: "<change>",
		Description: "Runs the 5-step archive pipeline (spec-lifecycle.md §6.2):\n" +
			"  1. gate-check   — every stage internal/status reports as required for\n" +
			"                    this change's type must be approved (or, for design,\n" +
			"                    legitimately skipped). Refuses by default; --force-gates\n" +
			"                    overrides (recorded).\n" +
			"  2. conflict-check — refuses if another in-flight change's delta touches\n" +
			"                    (MODIFIED/REMOVED/RENAMED) a requirement this change\n" +
			"                    also touches. --force-conflicts overrides (recorded).\n" +
			"  3. pre-image    — sha256 of each affected capability's live spec.md\n" +
			"                    (or a documented empty-byte-string sentinel if the\n" +
			"                    capability doesn't exist yet).\n" +
			"  4. fold+relocate — native Go fold (internal/spec) into\n" +
			"                    openspec/specs/<cap>/spec.md; the change folder moves\n" +
			"                    to openspec/changes/archive/<change>/. A delta-less\n" +
			"                    bug skips the fold entirely.\n" +
			"  5. post-image + ledger — appends one record per affected capability\n" +
			"                    (or exactly one, capability-less, for a delta-less bug)\n" +
			"                    to openspec/ledger.jsonl with the next monotonic seq,\n" +
			"                    then re-reads the archived folder and any folded specs\n" +
			"                    from disk to self-check they match what was just\n" +
			"                    recorded, then runs the FULL `lifecycle guard` (§6.3)\n" +
			"                    over the whole project as a comprehensive post-archive\n" +
			"                    self-check (§2.5 step 5) — a guard problem is surfaced\n" +
			"                    loudly (non-zero exit) even though the archive itself\n" +
			"                    already committed.\n\n" +
			"Exit codes: 0 ok, 1 refused (gate/conflict/fold — nothing written), 2\n" +
			"could not run (bad flags, no openspec/ tree, missing change folder, an\n" +
			"environment failure, an internal self-check failure, or a post-archive\n" +
			"guard problem). Run from a spec-lifecycle project root (a directory\n" +
			"containing openspec/).",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force-gates", Usage: "archive even though a required gate is not approved (recorded on every ledger record)"},
			&cli.BoolFlag{Name: "force-conflicts", Usage: "archive even though another in-flight change touches the same requirement (recorded on every ledger record)"},
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("archive: %w", err), code: archiveExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runArchive(cmd)
		},
	}
}

func runArchive(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("archive: --format must be %q or %q (got %q)", "text", "json", format),
			code: archiveExitCouldNotRun,
		}
	}
	if cmd.Args().Len() != 1 {
		return &exitError{
			err:  fmt.Errorf("archive: exactly one <change> argument is required"),
			code: archiveExitCouldNotRun,
		}
	}
	change := cmd.Args().First()

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("archive: %w", err), code: archiveExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("archive: no openspec/ directory in %s; run archive from a spec-lifecycle project root", cwd),
			code: archiveExitCouldNotRun,
		}
	}

	req := archive.Request{
		Root:           cwd,
		Change:         change,
		ForceGates:     cmd.Bool("force-gates"),
		ForceConflicts: cmd.Bool("force-conflicts"),
	}

	res, err := archive.Archive(req)

	stderr := cmd.Root().ErrWriter
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, "archive: warning: "+w) //nolint:errcheck
	}

	if err != nil {
		return mapArchiveError(err)
	}

	stdout := cmd.Root().Writer
	if format == "json" {
		if werr := writeArchiveJSON(stdout, res); werr != nil {
			return &exitError{err: fmt.Errorf("archive: writing output: %w", werr), code: archiveExitCouldNotRun}
		}
	} else if werr := writeArchiveText(stdout, change, res); werr != nil {
		return &exitError{err: fmt.Errorf("archive: writing output: %w", werr), code: archiveExitCouldNotRun}
	}

	// Post-archive guard self-check (implementation-plan.md §2.5 step 5,
	// archive/doc.go's "post-write self-check" section): the archive
	// already committed successfully above; this is a comprehensive,
	// whole-project detection pass layered on top (guard/doc.go's
	// "Archive integration"), never a rollback. A problem here must
	// surface loudly — a non-zero exit — even though res above already
	// printed a successful archive.
	if gerr := postArchiveGuard(cmd, cwd); gerr != nil {
		return gerr
	}
	return nil
}

// postArchiveGuard runs the full `lifecycle guard` (internal/guard) over
// root immediately after a successful archive.Archive call, per
// implementation-plan.md §2.5 step 5. Returns nil when guard ran and found
// nothing; otherwise an *exitError mapped to archiveExitCouldNotRun (the
// archive itself already wrote its state — this is detection, not refusal
// — so it shares archive's existing "something is wrong" exit code rather
// than inventing a new one).
func postArchiveGuard(cmd *cli.Command, root string) error {
	gres, gerr := guard.Run(guard.Options{Root: root})
	if gerr != nil {
		return &exitError{
			err:  fmt.Errorf("archive: post-archive guard self-check could not run: %w", gerr),
			code: archiveExitCouldNotRun,
		}
	}
	if gres.Summary.Clean {
		return nil
	}

	stderr := cmd.Root().ErrWriter
	fmt.Fprintln(stderr, "archive: warning: post-archive guard self-check found problems with the archive it just wrote:") //nolint:errcheck
	for _, f := range gres.Findings {
		fmt.Fprintf(stderr, "  [%s/%s] %s\n", f.Check, f.Kind, f.Message) //nolint:errcheck
	}
	return &exitError{
		err:  fmt.Errorf("archive: post-archive guard self-check found %d problem(s) — see above", len(gres.Findings)),
		code: archiveExitCouldNotRun,
	}
}

func mapArchiveError(err error) error {
	switch {
	case errors.Is(err, archive.ErrGatesNotApproved),
		errors.Is(err, archive.ErrConflict),
		errors.Is(err, archive.ErrFoldFailed):
		return &exitError{err: err, code: archiveExitRefused}
	default:
		return &exitError{err: err, code: archiveExitCouldNotRun}
	}
}

func writeArchiveJSON(w io.Writer, res archive.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeArchiveText(w io.Writer, change string, res archive.Result) error {
	if _, err := fmt.Fprintf(w, "archive: archived %q (%s) — %d ledger record(s) appended\n",
		change, res.Type, len(res.Records)); err != nil {
		return err
	}
	for _, r := range res.Records {
		label := r.Capability
		if label == "" {
			label = "(no capability — delta-less)"
		}
		if _, err := fmt.Fprintf(w, "  seq %d  %s\n", r.Seq, label); err != nil {
			return err
		}
	}
	return nil
}
