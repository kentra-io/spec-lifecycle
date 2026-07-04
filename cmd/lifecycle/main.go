// Command lifecycle is the CLI for spec-lifecycle: a staged, gated issue
// lifecycle in the OpenSpec on-disk format, reimplemented as a single
// static Go binary (no Node, no `openspec` runtime — see
// implementation-plan.md §0.5/"Option B"). See spec-lifecycle.md in the
// repo root for the design and implementation-plan.md for the build plan.
//
// M0 (this milestone) wires the binary skeleton and `--version` reporting
// only; the six verbs (init, validate, approve, status, archive, guard —
// plan §4) are added in later milestones (§8).
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
		// TODO(M2-M6): wire the six verbs (init, validate, approve, status,
		// archive, guard) here as they land — see implementation-plan.md §4/§8.
	}
	return cmd.Run(ctx, args)
}
