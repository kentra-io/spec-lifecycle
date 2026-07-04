package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/validate"
)

// Exit codes for `lifecycle validate` (spec-lifecycle.md §3.3,
// implementation-plan.md §2.3/§4): 0 valid, 1 findings (at least one
// error-severity Finding across the changes checked), 2 could not run at
// all (bad flags, no openspec/ project root, a named --change that
// doesn't exist, an unreadable change folder).
const (
	validateExitFindings    = 1
	validateExitCouldNotRun = 2
)

func validateCommand() *cli.Command {
	return &cli.Command{
		Name:      "validate",
		Usage:     "validate a change folder's stage artifacts (read-only)",
		ArgsUsage: " ",
		Description: "Runs the delta-grammar parser (internal/spec, over every\n" +
			"specs/**/spec.md delta) plus the custom-artifact structural checks\n" +
			"(internal/validate: proposal.md frontmatter/issue-ref, design.md's\n" +
			"NFR-discharge section, tasks.md's milestone/validation-contract\n" +
			"format — spec-lifecycle.md §3.3/§4.2) against the artifact(s) --stage\n" +
			"gates. Deterministic and read-only: writes nothing, calls nothing\n" +
			"external. Stage skills run this as the gate pre-check; `approve`\n" +
			"re-runs the same code path before writing a gate entry.\n\n" +
			"Without --change, every change folder under openspec/changes/ (not\n" +
			"changes/archive/) is checked, mirroring `lifecycle status`'s\n" +
			"convention of reporting across all changes by default.\n\n" +
			"Exit codes: 0 valid, 1 findings (at least one error), 2 could not\n" +
			"run (bad flags, no openspec/ tree, a named --change that doesn't\n" +
			"exist, an unreadable change folder). Run from a spec-lifecycle\n" +
			"project root (a directory containing openspec/).",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "stage", Required: true, Usage: "refine|design|plan"},
			&cli.StringFlag{Name: "change", Usage: "validate only this change (default: every change under openspec/changes/)"},
			&cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text|json"},
		},
		// A flag-parse/usage error is a "could not run" condition, not
		// "findings": map it to exit 2 like every other verb's usage errors
		// (guard.go, deviation.go in the sibling constitution primitive),
		// so a JSON consumer never reads a bare parse failure as exit 1.
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("validate: %w", err), code: validateExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runValidate(cmd)
		},
	}
}

func runValidate(cmd *cli.Command) error {
	format := cmd.String("format")
	if format != "text" && format != "json" {
		return &exitError{
			err:  fmt.Errorf("validate: --format must be %q or %q (got %q)", "text", "json", format),
			code: validateExitCouldNotRun,
		}
	}

	stage := validate.Stage(cmd.String("stage"))
	if !isRecognizedStage(stage) {
		return &exitError{
			err:  fmt.Errorf("validate: --stage must be one of %v (got %q)", validate.Stages, cmd.String("stage")),
			code: validateExitCouldNotRun,
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("validate: %w", err), code: validateExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("validate: no openspec/ directory in %s; run validate from a spec-lifecycle project root", cwd),
			code: validateExitCouldNotRun,
		}
	}
	changesRoot := filepath.Join(cwd, "openspec", "changes")

	var names []string
	if name := cmd.String("change"); name != "" {
		info, err := os.Stat(filepath.Join(changesRoot, name))
		if err != nil || !info.IsDir() {
			return &exitError{
				err:  fmt.Errorf("validate: change %q not found under %s", name, changesRoot),
				code: validateExitCouldNotRun,
			}
		}
		names = []string{name}
	} else {
		names, err = listChanges(changesRoot)
		if err != nil {
			return &exitError{err: fmt.Errorf("validate: %w", err), code: validateExitCouldNotRun}
		}
	}

	results := make([]changeResult, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(changesRoot, name)
		findings, err := validate.Change(dir, stage)
		if err != nil {
			return &exitError{err: fmt.Errorf("validate: %s: %w", name, err), code: validateExitCouldNotRun}
		}
		results = append(results, changeResult{Change: name, Findings: findings})
	}

	stdout := cmd.Root().Writer
	sum := summarize(results)
	var writeErr error
	if format == "json" {
		writeErr = writeValidateJSON(stdout, string(stage), results, sum)
	} else {
		writeErr = writeValidateText(stdout, string(stage), results, sum)
	}
	if writeErr != nil {
		return &exitError{err: fmt.Errorf("validate: writing output: %w", writeErr), code: validateExitCouldNotRun}
	}

	if sum.Errors > 0 {
		return &exitError{
			err:  fmt.Errorf("validate: %d error(s), %d warning(s) across %d change(s)", sum.Errors, sum.Warnings, sum.Changes),
			code: validateExitFindings,
		}
	}
	return nil
}

func isRecognizedStage(s validate.Stage) bool {
	for _, st := range validate.Stages {
		if st == s {
			return true
		}
	}
	return false
}

// listChanges lists every change-folder name directly under changesRoot,
// excluding "archive" (the append-only event log, not a live change — spec
// §6.2), sorted. A changesRoot that doesn't exist yet yields an empty list,
// not an error: `lifecycle validate` can legitimately run before any
// change folder exists.
func listChanges(changesRoot string) ([]string, error) {
	entries, err := os.ReadDir(changesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", changesRoot, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// changeResult is one change folder's findings — the JSON payload's
// per-change unit and the text formatter's per-change grouping.
type changeResult struct {
	Change   string             `json:"change"`
	Findings []validate.Finding `json:"findings"`
}

type validateSummary struct {
	Changes  int `json:"changes"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

func summarize(results []changeResult) validateSummary {
	s := validateSummary{Changes: len(results)}
	for _, r := range results {
		for _, f := range r.Findings {
			if f.Severity == validate.SeverityWarning {
				s.Warnings++
			} else {
				s.Errors++
			}
		}
	}
	return s
}

// writeValidateJSON emits ONLY the machine payload on stdout (mirroring
// guard's "JSON-only on stdout, pipeable" convention in the sibling
// constitution primitive) — stage, per-change findings, and a summary.
// This shape is the stable, documented --format json contract.
func writeValidateJSON(w io.Writer, stage string, results []changeResult, s validateSummary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Stage   string          `json:"stage"`
		Changes []changeResult  `json:"changes"`
		Summary validateSummary `json:"summary"`
	}{Stage: stage, Changes: results, Summary: s})
}

func writeValidateText(w io.Writer, stage string, results []changeResult, s validateSummary) error {
	if len(results) == 0 {
		_, err := fmt.Fprintf(w, "validate: no change folders found for stage %q\n", stage)
		return err
	}
	for _, r := range results {
		if len(r.Findings) == 0 {
			if _, err := fmt.Fprintf(w, "%s: ok\n", r.Change); err != nil {
				return err
			}
			continue
		}
		for _, f := range r.Findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			if _, err := fmt.Fprintf(w, "[%s] %s: %s\n", f.Severity, loc, f.Message); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "validate: %d change(s), %d error(s), %d warning(s)\n", s.Changes, s.Errors, s.Warnings)
	return err
}
