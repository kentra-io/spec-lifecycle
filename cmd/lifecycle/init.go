package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/kentra-io/spec-lifecycle/internal/config"
	"github.com/kentra-io/spec-lifecycle/internal/scaffold"
)

// Exit codes for `lifecycle init` (implementation-plan.md §2.9/§4): 0 ok
// (the compose ran; any constitution-preflight/sourceTracking notices are
// warnings on stderr, not failures), 1 refused (a managed pointer-block
// target drifted from what a previous `init` wrote and neither --force
// nor an interactive confirm accepted overwriting it — nothing further
// was written), 2 could not run (bad flags, a structurally ambiguous
// managed-block marker pair, or an environment failure). This mirrors
// every other verb's existing 0/1/2 split (validate.go, archive.go).
const initExitCouldNotRun = 2

func initCommand() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "scaffold a spec-lifecycle project: openspec/ tree, schema, config, skills, pointer blocks",
		ArgsUsage: " ", // no positional args
		Description: "Runs the §2.9 native compose, in order (implementation-plan.md):\n" +
			"  1. openspec/{changes,specs} — created if openspec/ is entirely absent.\n" +
			"  2. openspec/schemas/kentra-spec-lifecycle/ — installed if absent (a\n" +
			"     format-compat/documentation descriptor; nothing reads it back).\n" +
			"  3. openspec/config.yaml — schema: set/kept pointed at\n" +
			"     kentra-spec-lifecycle; context: seeded once, never overwritten.\n" +
			"     Both edits are surgical: every other key and comment survives.\n" +
			"  4. lifecycle.yml — seeded once (schemaVersion, specFormat, a\n" +
			"     best-effort constitution.version pin detected from the\n" +
			"     installed constitution binary, consentPolicy strict,\n" +
			"     sourceTracking, runtimes); an existing lifecycle.yml wins on a\n" +
			"     re-run (--runtimes/--source-type/--source-repo are then ignored).\n" +
			"  5. constitution preflight — presence + version vs the pin. A\n" +
			"     missing or version-mismatched binary is a WARNING, never a\n" +
			"     failure: it only actually blocks work later, at gates 2/3\n" +
			"     (`lifecycle approve --stage design|plan`).\n" +
			"  6. Layer-2 skills fan out to each configured runtime's tree\n" +
			"     (.claude/, .cursor/, .agents/skills/), and managed pointer\n" +
			"     blocks are written into CLAUDE.md and AGENTS.md.\n\n" +
			"Every step is independently idempotent, so a re-run of the whole\n" +
			"compose is byte-identical. A managed pointer block that drifted\n" +
			"from what a previous `init` wrote is refused (exit 1) unless\n" +
			"--force is given (or, on a terminal, an interactive y/N confirm is\n" +
			"accepted); a structurally ambiguous marker pair is refused outright\n" +
			"(exit 2) rather than guessed at. Run from the directory that should\n" +
			"become the project root.\n\n" +
			"Ownership (implementation-plan.md §2.9): `lifecycle` owns the entire\n" +
			"openspec/ tree, lifecycle.yml, every approval-state.json, and the\n" +
			"archive ledger (openspec/ledger.jsonl) end to end — nothing else\n" +
			"writes there. `init` only ever adds scaffolding it finds missing;\n" +
			"it never rewrites an existing project's already-existing\n" +
			"lifecycle-owned records (in-flight changes, gate state, archived\n" +
			"changes, the ledger).",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "runtimes", Usage: "skill fan-out target: claude-code|cursor|codex (repeatable; default: all three; a fresh lifecycle.yml only)"},
			&cli.StringFlag{Name: "source-type", Usage: "sourceTracking.type to seed: github-issue|generic|jira|none (a fresh lifecycle.yml only)"},
			&cli.StringFlag{Name: "source-repo", Usage: "sourceTracking.repo to seed, e.g. kentra-io/kafka-dq (a fresh lifecycle.yml only)"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite a managed pointer block that drifted from what init last wrote"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return &exitError{err: fmt.Errorf("init: %w", err), code: initExitCouldNotRun}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			return runInit(cmd)
		},
	}
}

func runInit(cmd *cli.Command) error {
	runtimes, err := normalizeRuntimes(cmd.StringSlice("runtimes"))
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return &exitError{err: fmt.Errorf("init: %w", err), code: initExitCouldNotRun}
	}
	stdout := cmd.Root().Writer
	stderr := cmd.Root().ErrWriter

	opts := scaffold.InitOptions{
		Root:               cwd,
		Force:              cmd.Bool("force"),
		Stdout:             stdout,
		Stderr:             stderr,
		Runtimes:           runtimes,
		SourceTrackingType: cmd.String("source-type"),
		SourceTrackingRepo: cmd.String("source-repo"),
	}
	if !cmd.Bool("force") && isTerminal(os.Stdin) {
		opts.Confirm = interactiveConfirm(os.Stdin, stderr)
	}

	res, err := scaffold.RunInit(opts)
	if err != nil {
		var me *scaffold.MarkerError
		if errors.As(err, &me) {
			return &exitError{err: fmt.Errorf("init: %w", err), code: initExitCouldNotRun}
		}
		return fmt.Errorf("init: %w", err)
	}

	for _, m := range res.Messages {
		if _, werr := fmt.Fprintln(stdout, m); werr != nil {
			return &exitError{err: werr, code: initExitCouldNotRun}
		}
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, "init: warning: "+w) //nolint:errcheck
	}

	_, err = fmt.Fprintln(stdout, "init: spec-lifecycle project ready")
	return err
}

// normalizeRuntimes validates --runtimes against config's vocabulary,
// preserving order and collapsing duplicates; an empty flag list leaves
// runtimes nil so scaffold.RunInit falls back to config.DefaultRuntimes.
func normalizeRuntimes(given []string) ([]string, error) {
	if len(given) == 0 {
		return nil, nil
	}
	allowed := map[string]bool{config.RuntimeClaudeCode: true, config.RuntimeCursor: true, config.RuntimeCodex: true}
	seen := map[string]bool{}
	out := make([]string, 0, len(given))
	for _, r := range given {
		if !allowed[r] {
			return nil, &exitError{
				err: fmt.Errorf("init: --runtimes: unknown value %q (allowed: %q, %q, %q)",
					r, config.RuntimeClaudeCode, config.RuntimeCursor, config.RuntimeCodex),
				code: initExitCouldNotRun,
			}
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out, nil
}

// interactiveConfirm builds a y/N prompt reader for init's drift confirm on
// a terminal — verbatim mirror of the sibling adr-sourced-constitution
// primitive's own cmd/constitution/init.go helper of the same name.
func interactiveConfirm(in *os.File, out io.Writer) func(string) (bool, error) {
	reader := bufio.NewReader(in)
	return func(prompt string) (bool, error) {
		if _, err := fmt.Fprintf(out, "%s [y/N] ", prompt); err != nil {
			return false, err
		}
		line, _ := reader.ReadString('\n')
		s := strings.ToLower(strings.TrimSpace(line))
		return s == "y" || s == "yes", nil
	}
}
