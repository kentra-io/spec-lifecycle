package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/approve"
	"github.com/kentra-io/spec-lifecycle/internal/config"
	"github.com/kentra-io/spec-lifecycle/internal/constitution"
)

// Exit codes for `lifecycle approve` (spec-lifecycle.md §3.3/§9.1,
// implementation-plan.md §2.6/§4): 0 ok (a gate entry was written), 1
// refused (consent withheld, the gated artifact failed validation, or
// deviation.json is invalid — nothing was written), 2 could not run (bad
// flags/usage, no openspec/ project root, an unrecognized --stage, a
// missing change folder, or an environment failure such as a design/plan
// gate's deviation.json being unreadable or the constitution binary
// failing to run at all).
const (
	approveExitRefused     = 1
	approveExitCouldNotRun = 2
)

func approveCommand() *cli.Command {
	return &cli.Command{
		Name:      "approve",
		Usage:     "record a gate decision in a change's approval-state.json",
		ArgsUsage: "<change>",
		Description: "Resolves --stage's generated artifact(s) via the embedded\n" +
			"kentra-spec-lifecycle schema, hashes them, recomputes constitutionHash,\n" +
			"and — at gates 2/3 (design, plan) — runs `constitution deviation\n" +
			"validate` against <change>/deviation.json before appending one entry\n" +
			"to <change>/approval-state.json (spec-lifecycle.md §5). Runs the SAME\n" +
			"validation code path `lifecycle validate` uses first: an invalid\n" +
			"artifact is never approved (a --reject bypasses this — you can reject\n" +
			"an invalid or unwanted artifact without fixing it first).\n\n" +
			"Consent (spec-lifecycle.md §5's closing paragraph): under\n" +
			"consentPolicy: strict (lifecycle.yml, the default), this refuses to\n" +
			"write without --approve (or an interactive y/N confirmation on a\n" +
			"TTY) — the human-permission-prompt boundary. consentPolicy: off\n" +
			"removes the gate.\n\n" +
			"Exit codes: 0 ok, 1 refused (consent withheld, invalid artifact, or\n" +
			"invalid deviation.json — nothing written), 2 could not run (bad\n" +
			"flags, no openspec/ project root, unrecognized --stage, missing\n" +
			"change folder, or an environment failure). Run from a spec-lifecycle\n" +
			"project root (a directory containing openspec/).",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "stage", Required: true, Usage: fmt.Sprintf("one of %v", approve.Stages)},
			&cli.BoolFlag{Name: "reject", Usage: "record a rejection instead of an approval"},
			&cli.BoolFlag{Name: "design-skip", Usage: "propose/record a design-stage skip (--stage refine only)"},
			&cli.StringFlag{Name: "notes", Usage: "free-form notes recorded on the gate entry"},
			&cli.StringFlag{Name: "approved-by", Usage: "defaults to the OS account name ($USER/$USERNAME)"},
			&cli.BoolFlag{Name: "approve", Usage: "confirm this write under consentPolicy: strict (the human-permission-prompt boundary)"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("approve: %w", err), code: approveExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runApprove(cmd)
		},
	}
}

func runApprove(cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return &exitError{
			err:  fmt.Errorf("approve: exactly one <change> argument is required"),
			code: approveExitCouldNotRun,
		}
	}
	change := cmd.Args().First()

	stage := approve.Stage(cmd.String("stage"))
	if !stage.IsRecognized() {
		return &exitError{
			err:  fmt.Errorf("approve: --stage must be one of %v (got %q)", approve.Stages, cmd.String("stage")),
			code: approveExitCouldNotRun,
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("approve: %w", err), code: approveExitCouldNotRun}
	}
	if _, err := os.Stat(filepath.Join(cwd, "openspec")); err != nil {
		return &exitError{
			err:  fmt.Errorf("approve: no openspec/ directory in %s; run approve from a spec-lifecycle project root", cwd),
			code: approveExitCouldNotRun,
		}
	}

	cfg, err := config.Load(filepath.Join(cwd, "lifecycle.yml"))
	if err != nil {
		return &exitError{err: fmt.Errorf("approve: %w", err), code: approveExitCouldNotRun}
	}

	stderr := cmd.Root().ErrWriter

	bin, locateErr := constitution.Locate("")
	if locateErr != nil {
		bin = "" // presence is only fatal if this stage's gate actually needs it (approve.Approve enforces that)
	} else if cfg.Constitution.Version != "" {
		if pf, verr := constitution.CheckVersion(bin, cfg.Constitution.Version); verr == nil && pf.Warning != "" {
			fmt.Fprintln(stderr, "approve: "+pf.Warning) //nolint:errcheck
		}
	}

	approvedBy := cmd.String("approved-by")
	if approvedBy == "" {
		approvedBy = defaultApprovedBy()
	}

	gate := approve.ConsentGate{
		Policy:   cfg.ConsentPolicy,
		Approved: cmd.Bool("approve"),
		IsTTY:    isTerminal(os.Stdin),
		In:       os.Stdin,
		Out:      stderr,
	}

	req := approve.Request{
		Root:            cwd,
		Change:          change,
		Stage:           stage,
		Reject:          cmd.Bool("reject"),
		DesignSkip:      cmd.Bool("design-skip"),
		Notes:           cmd.String("notes"),
		ApprovedBy:      approvedBy,
		Consent:         gate,
		ConstitutionBin: bin,
	}

	res, err := approve.Approve(req)
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, "approve: warning: "+w) //nolint:errcheck
	}

	if err != nil {
		return mapApproveError(res, err)
	}

	stdout := cmd.Root().Writer
	verb := "approved"
	if res.Entry.Status == approve.StatusRejected {
		verb = "rejected"
	}
	fmt.Fprintf(stdout, "approve: %s stage %q for %s (%d artifact(s) hashed)\n", //nolint:errcheck
		verb, res.Entry.Stage, change, len(res.Entry.Artifacts))
	return nil
}

func mapApproveError(res approve.Result, err error) error {
	switch {
	case errors.Is(err, approve.ErrConsentRequired),
		errors.Is(err, approve.ErrInvalidArtifact),
		errors.Is(err, approve.ErrDeviationInvalid):
		for _, f := range res.Findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			fmt.Fprintf(os.Stderr, "approve: [%s] %s: %s\n", f.Severity, loc, f.Message) //nolint:errcheck
		}
		return &exitError{err: err, code: approveExitRefused}
	default:
		return &exitError{err: err, code: approveExitCouldNotRun}
	}
}

// defaultApprovedBy resolves the acting approver's name when --approved-by
// is not given: the OS account username (os/user), falling back to
// $USER/$USERNAME, and finally "" (approval-state.json's approvedBy field
// is then simply empty — not a hard failure; the timestamp + artifact
// hashes are the load-bearing audit trail).
func defaultApprovedBy() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := os.Getenv("USER"); v != "" {
		return v
	}
	return os.Getenv("USERNAME")
}
