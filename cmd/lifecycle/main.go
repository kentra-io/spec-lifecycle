// Command lifecycle is the CLI for spec-lifecycle: a staged, gated issue
// lifecycle in the OpenSpec on-disk format, reimplemented as a single
// static Go binary (no Node, no `openspec` runtime — see
// implementation-plan.md §0.5/"Option B"). See spec-lifecycle.md in the
// repo root for the design and implementation-plan.md for the build plan.
//
// M0 wired the binary skeleton and `--version` reporting. M2 added
// `validate` (plan §2.3/§4). M3 added `approve` and `status` (plan §2.6).
// M4 added `archive` (plan §2.5) and the baseline ledger. M5 added `guard`
// (plan §2.4) and wired it as a post-archive self-check. M6 added `init`
// (plan §2.9/§4): the native scaffold and integration wiring — all 6 v1
// verbs were live. Change 007 (Milestone 6) retired the `lifecycle apply`
// verb: the machine-readable plan surface is now `milestoned-plan-dag`,
// not `lifecycle` (specs/plan-integration "The machine plan surface is
// milestoned-plan-dag, not lifecycle").
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	if err := run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}

// rootCommand builds the CLI's command tree. It is factored out of run so a
// test can walk the real registered commands and flags rather than a
// hand-maintained copy of them (see skilldrift_test.go).
func rootCommand() *cli.Command {
	return &cli.Command{
		Name:    "lifecycle",
		Usage:   "stage-gated OpenSpec-format change lifecycle",
		Version: buildVersion(),
		Commands: []*cli.Command{
			initCommand(),
			validateCommand(),
			approveCommand(),
			statusCommand(),
			archiveCommand(),
			guardCommand(),
		},
	}
}

func run(ctx context.Context, args []string) error {
	return rootCommand().Run(ctx, args)
}
