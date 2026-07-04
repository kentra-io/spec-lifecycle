// Command lifecycle is the CLI for spec-lifecycle: a staged, gated issue
// lifecycle in the OpenSpec on-disk format, reimplemented as a single
// static Go binary (no Node, no `openspec` runtime — see
// implementation-plan.md §0.5/"Option B"). See spec-lifecycle.md in the
// repo root for the design and implementation-plan.md for the build plan.
//
// M0 wired the binary skeleton and `--version` reporting. M2 (this
// milestone) adds `validate` (plan §2.3/§4). The remaining verbs (init,
// approve, status, archive, guard) land in later milestones (§8).
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
			validateCommand(),
		},
		// TODO(M3-M6): wire the remaining verbs (init, approve, status,
		// archive, guard) here as they land — see implementation-plan.md §4/§8.
	}
	return cmd.Run(ctx, args)
}
