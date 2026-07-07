package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// Exit codes for `lifecycle apply <change>` (harness orchestration.md
// §5.5's machine-readable plan surface): 0 ok (tasks.md parsed and
// surfaced), 1 refused (tasks.md fails the SAME plan-stage structural
// validation `lifecycle validate --stage plan` runs — a missing artifact,
// a missing milestone label, an empty Validation contract, or a malformed
// ```contract block — nothing is surfaced from a plan that isn't already
// trustworthy), 2 could not run (bad flags, no openspec/ project root, a
// named <change> that doesn't exist, an unreadable change folder).
const (
	applyExitRefused     = 1
	applyExitCouldNotRun = 2
)

func applyCommand() *cli.Command {
	return &cli.Command{
		Name:      "apply",
		Usage:     "surface a change's milestones and validation contracts as data (read-only)",
		ArgsUsage: "<change>",
		Description: "Reads <change>'s tasks.md and projects it into the machine-readable\n" +
			"shape an execution engine (e.g. the orchestration module's read_plan\n" +
			"step, harness orchestration.md §5.5) consumes without doing its own\n" +
			"bespoke markdown parsing: every milestone's id/title, its Steps (with\n" +
			"opt-in checkbox tracking — internal/validate's addendum to\n" +
			"spec-lifecycle.md §4.2), and its optional structured validation\n" +
			"contract (an executable acceptance-check command, plain-language\n" +
			"criteria, and the allowed path-set / diff-confined-paths\n" +
			"declaration).\n\n" +
			"`apply` never surfaces a plan `lifecycle validate --stage plan` would\n" +
			"reject: it runs that exact same structural validation first and\n" +
			"refuses (exit 1) on any error-severity finding, so a downstream\n" +
			"consumer never has to re-derive \"is this plan well-formed\" itself —\n" +
			"the milestone/contract data `apply` returns is always already\n" +
			"trustworthy.\n\n" +
			"This verb does not require any gate to be approved (like validate/\n" +
			"status, it is read-only and deterministic) — it reports whatever the\n" +
			"change folder currently holds.\n\n" +
			"Exit codes: 0 ok, 1 refused (tasks.md fails plan-stage validation —\n" +
			"see `lifecycle validate --stage plan` for the same findings), 2 could\n" +
			"not run (bad flags, no openspec/ tree, missing change folder, an\n" +
			"unreadable change folder). Run from a spec-lifecycle project root (a\n" +
			"directory containing openspec/).",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("apply: %w", err), code: applyExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runApply(cmd)
		},
	}
}

// applyMilestone is one milestone in the `apply` JSON/text surface —
// validate.Milestone plus nothing else: the contract fields harness
// orchestration.md §5.5 asks for (id/title, acceptance-check command,
// criteria, allowed path-set) are exactly validate.Contract's fields,
// reused rather than re-declared.
type applyMilestone = validate.Milestone

// applyResult is the whole `lifecycle apply <change> --format json`
// payload — the stable, documented contract a consumer like
// orchestration's read_plan step codes against.
type applyResult struct {
	Change     string           `json:"change"`
	Type       string           `json:"type"`
	Issue      string           `json:"issue"`
	Milestones []applyMilestone `json:"milestones"`
}

func runApply(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("apply: --format must be %q or %q (got %q)", "text", "json", format),
			code: applyExitCouldNotRun,
		}
	}
	if cmd.Args().Len() != 1 {
		return &exitError{
			err:  fmt.Errorf("apply: exactly one <change> argument is required"),
			code: applyExitCouldNotRun,
		}
	}
	change := cmd.Args().First()

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("apply: %w", err), code: applyExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("apply: no openspec/ directory in %s; run apply from a spec-lifecycle project root", cwd),
			code: applyExitCouldNotRun,
		}
	}
	changesRoot := filepath.Join(cwd, "openspec", "changes")
	dir := filepath.Join(changesRoot, change)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return &exitError{
			err:  fmt.Errorf("apply: change %q not found under %s", change, changesRoot),
			code: applyExitCouldNotRun,
		}
	}

	findings, err := validate.Change(dir, validate.StagePlan)
	if err != nil {
		return &exitError{err: fmt.Errorf("apply: %s: %w", change, err), code: applyExitCouldNotRun}
	}
	sum := summarize([]changeResult{{Change: change, Findings: findings}})
	if sum.Errors > 0 {
		stderr := cmd.Root().ErrWriter
		for _, f := range findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			fmt.Fprintf(stderr, "apply: [%s] %s: %s\n", f.Severity, loc, f.Message) //nolint:errcheck
		}
		return &exitError{
			err:  fmt.Errorf("apply: refused — %s's tasks.md fails plan-stage validation (%d error(s); see `lifecycle validate --stage plan` for details)", change, sum.Errors),
			code: applyExitRefused,
		}
	}

	milestones, _, err := validate.ParseMilestones(dir)
	if err != nil {
		return &exitError{err: fmt.Errorf("apply: %s: %w", change, err), code: applyExitCouldNotRun}
	}
	meta, err := validate.ReadProposalMeta(dir)
	if err != nil {
		return &exitError{err: fmt.Errorf("apply: %s: %w", change, err), code: applyExitCouldNotRun}
	}

	res := applyResult{Change: change, Type: meta.Type, Issue: meta.Issue, Milestones: milestones}

	stdout := cmd.Root().Writer
	var writeErr error
	if format == "json" {
		writeErr = writeApplyJSON(stdout, res)
	} else {
		writeErr = writeApplyText(stdout, res)
	}
	if writeErr != nil {
		return &exitError{err: fmt.Errorf("apply: writing output: %w", writeErr), code: applyExitCouldNotRun}
	}
	return nil
}

func writeApplyJSON(w io.Writer, res applyResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeApplyText(w io.Writer, res applyResult) error {
	if _, err := fmt.Fprintf(w, "%s (%s)\n", res.Change, res.Type); err != nil {
		return err
	}
	if len(res.Milestones) == 0 {
		_, err := fmt.Fprintln(w, "  (no milestones)")
		return err
	}
	for _, m := range res.Milestones {
		tracked, checked := 0, 0
		for _, s := range m.Steps {
			if s.Tracked {
				tracked++
				if s.Checked {
					checked++
				}
			}
		}
		if _, err := fmt.Fprintf(w, "  Milestone %d: %s", m.ID, m.Title); err != nil {
			return err
		}
		if tracked > 0 {
			if _, err := fmt.Fprintf(w, " [%d/%d steps checked]", checked, tracked); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if m.Contract != nil {
			if _, err := fmt.Fprintf(w, "    check:  %s\n", m.Contract.Check); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "    paths:  %v\n", m.Contract.Paths); err != nil {
				return err
			}
		}
	}
	return nil
}
