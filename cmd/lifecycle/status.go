package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/status"
)

// statusExitCouldNotRun is `lifecycle status`'s only non-zero exit code:
// it is a purely read-only reporter (spec-lifecycle.md §9.1), so there is
// no "findings" outcome distinct from "could not run" the way validate/
// approve have — a report with pending/drifted gates is still a
// successful run (exit 0).
const statusExitCouldNotRun = 2

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "report gate state across change folders (read-only)",
		Description: "Reads each change folder's approval-state.json (never writes) and\n" +
			"derives, per stage: approved/rejected (the latest entry per\n" +
			"stage-name, spec-lifecycle.md §5), pending (no entry at all), or\n" +
			"skipped (design absent + the refine entry's designSkipped:true,\n" +
			"spec-lifecycle.md §3.2). Also flags post-gate artifact drift: an\n" +
			"approved gate's recorded artifact hashes re-checked against the\n" +
			"change folder's CURRENT content (spec-lifecycle.md §5/§6.2).\n\n" +
			"The relevant stage set is derived from the change type recorded at\n" +
			"intake (proposal.md's `type: feature|bug` field, spec-lifecycle.md\n" +
			"§8/§2.11): feature -> refine/design/plan; bug -> repro/fix. A\n" +
			"promoted bug's gates[] may mix repro/fix with design/plan in the\n" +
			"same folder — both are reported.\n\n" +
			"Without --change, every change folder under openspec/changes/ (not\n" +
			"changes/archive/) is reported. Exit codes: 0 (report produced,\n" +
			"regardless of pending/drifted gates), 2 could not run (bad flags, no\n" +
			"openspec/ project root, a named --change that doesn't exist, an\n" +
			"unreadable change folder).",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "change", Usage: "report only this change (default: every change under openspec/changes/)"},
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("status: %w", err), code: statusExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runStatus(cmd)
		},
	}
}

func runStatus(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("status: --format must be %q or %q (got %q)", "text", "json", format),
			code: statusExitCouldNotRun,
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("status: %w", err), code: statusExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("status: no openspec/ directory in %s; run status from a spec-lifecycle project root", cwd),
			code: statusExitCouldNotRun,
		}
	}
	changesRoot := filepath.Join(cwd, "openspec", "changes")

	var names []string
	if name := cmd.String("change"); name != "" {
		info, err := os.Stat(filepath.Join(changesRoot, name))
		if err != nil || !info.IsDir() {
			return &exitError{
				err:  fmt.Errorf("status: change %q not found under %s", name, changesRoot),
				code: statusExitCouldNotRun,
			}
		}
		names = []string{name}
	} else {
		names, err = listChanges(changesRoot)
		if err != nil {
			return &exitError{err: fmt.Errorf("status: %w", err), code: statusExitCouldNotRun}
		}
	}

	results := make([]status.ChangeStatus, 0, len(names))
	for _, name := range names {
		cs, err := status.Change(filepath.Join(changesRoot, name))
		if err != nil {
			return &exitError{err: fmt.Errorf("status: %s: %w", name, err), code: statusExitCouldNotRun}
		}
		results = append(results, cs)
	}

	stdout := cmd.Root().Writer
	var writeErr error
	if format == "json" {
		writeErr = writeStatusJSON(stdout, results)
	} else {
		writeErr = writeStatusText(stdout, results)
	}
	if writeErr != nil {
		return &exitError{err: fmt.Errorf("status: writing output: %w", writeErr), code: statusExitCouldNotRun}
	}
	return nil
}

func writeStatusJSON(w io.Writer, results []status.ChangeStatus) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Changes []status.ChangeStatus `json:"changes"`
	}{Changes: results})
}

func writeStatusText(w io.Writer, results []status.ChangeStatus) error {
	if len(results) == 0 {
		_, err := fmt.Fprintln(w, "status: no change folders found")
		return err
	}
	for _, cs := range results {
		if _, err := fmt.Fprintf(w, "%s (%s)\n", cs.Change, cs.Type); err != nil {
			return err
		}
		for _, g := range cs.Gates {
			line := fmt.Sprintf("  %-8s %s", g.Stage, g.State)
			if len(g.Drifted) > 0 {
				line += fmt.Sprintf(" [drift: %v]", g.Drifted)
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}
