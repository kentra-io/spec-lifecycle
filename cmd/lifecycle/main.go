// Command lifecycle is the CLI for spec-lifecycle: a staged, gated issue
// lifecycle in the OpenSpec on-disk format, reimplemented as a single
// static Go binary (no Node, no `openspec` runtime — see
// implementation-plan.md §0.5/"Option B"). See spec-lifecycle.md in the
// repo root for the design and implementation-plan.md for the build plan.
//
// M0 wired the binary skeleton and `--version` reporting. M2 added
// `validate` (plan §2.3/§4). M3 added `approve` and `status` (plan §2.6).
// M4 added `archive` (plan §2.5) and the baseline ledger. M5 added `guard`
// (plan §2.4) and wired it as a post-archive self-check. M6 (this
// milestone) adds `init` (plan §2.9/§4): the native scaffold and
// integration wiring — all 6 v1 verbs are now live.
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

func run(ctx context.Context, args []string) error {
	cmd := &cli.Command{
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
	return cmd.Run(ctx, args)
}
