package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/guard"
)

// Exit codes for `lifecycle guard` (spec-lifecycle.md §6.3,
// implementation-plan.md §2.4): 0 clean, 1 findings (the ledger, archive,
// or live projection disagree with each other), 2 could not run (bad
// flags/usage, no openspec/ project root, a malformed ledger, or any other
// environment failure).
const (
	guardExitFindings    = 1
	guardExitCouldNotRun = 2
)

func guardCommand() *cli.Command {
	return &cli.Command{
		Name:  "guard",
		Usage: "check the archive ledger, archived changes, and live specs for drift (spec-lifecycle.md §6.3)",
		Description: "Runs, deterministically and with no LLM involved, three checks (in\n" +
			"order) over the whole project history, pooling every problem found\n" +
			"rather than stopping at the first:\n" +
			"  1. immutability  — content-hash every archived change folder under\n" +
			"                     openspec/changes/archive/** and compare against its\n" +
			"                     ledger record(s)' archiveManifestSha.\n" +
			"  2. digest chain  — per capability, the live openspec/specs/<cap>/spec.md\n" +
			"                     hash must equal that capability's latest\n" +
			"                     postImageSha, and each record's preImageSha must equal\n" +
			"                     the prior record's postImageSha (first record: the\n" +
			"                     empty-image sentinel).\n" +
			"  3. from-empty replay — recompute fold(all archived deltas, in ledger\n" +
			"                     seq order, from empty) in-process and diff the result\n" +
			"                     against the live projection, byte for byte.\n\n" +
			"If openspec/ledger.jsonl is entirely absent while archived changes exist,\n" +
			"that alone is reported (checks 1-3 do not run, since there is nothing to\n" +
			"check against). A present-but-malformed ledger is a could-not-run error\n" +
			"(exit 2), not a finding.\n\n" +
			"Exit codes: 0 clean, 1 findings, 2 could not run. Run from a\n" +
			"spec-lifecycle project root (a directory containing openspec/); also run\n" +
			"automatically after every successful `lifecycle archive` (§2.5 step 5).",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runGuard(cmd)
		},
	}
}

func runGuard(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("guard: --format must be %q or %q (got %q)", "text", "json", format),
			code: guardExitCouldNotRun,
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("guard: no openspec/ directory in %s; run guard from a spec-lifecycle project root", cwd),
			code: guardExitCouldNotRun,
		}
	}

	res, err := guard.Run(guard.Options{Root: cwd})
	if err != nil {
		return &exitError{err: fmt.Errorf("guard: %w", err), code: guardExitCouldNotRun}
	}

	stdout := cmd.Root().Writer
	var werr error
	if format == "json" {
		werr = writeGuardJSON(stdout, res)
	} else {
		werr = writeGuardText(stdout, res)
	}
	if werr != nil {
		return &exitError{err: fmt.Errorf("guard: writing output: %w", werr), code: guardExitCouldNotRun}
	}

	if !res.Summary.Clean {
		return &exitError{
			err:  fmt.Errorf("guard: %d finding(s)", res.Summary.Findings),
			code: guardExitFindings,
		}
	}
	return nil
}

func writeGuardJSON(w io.Writer, res guard.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

func writeGuardText(w io.Writer, res guard.Result) error {
	if res.Summary.Clean {
		_, err := fmt.Fprintf(w, "guard: clean (%d change(s), %d ledger record(s) checked)\n",
			res.Summary.ChangesChecked, res.Summary.RecordsChecked)
		return err
	}
	for _, f := range res.Findings {
		if _, err := fmt.Fprintf(w, "[%s/%s]", f.Check, f.Kind); err != nil {
			return err
		}
		if f.Change != "" {
			if _, err := fmt.Fprintf(w, " change=%s", f.Change); err != nil {
				return err
			}
		}
		if f.Capability != "" {
			if _, err := fmt.Fprintf(w, " capability=%s", f.Capability); err != nil {
				return err
			}
		}
		if f.Seq != 0 {
			if _, err := fmt.Fprintf(w, " seq=%d", f.Seq); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "\n  %s\n", f.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "guard: %d change(s), %d ledger record(s) checked, %d finding(s)\n",
		res.Summary.ChangesChecked, res.Summary.RecordsChecked, res.Summary.Findings)
	return err
}
