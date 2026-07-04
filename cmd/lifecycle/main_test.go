package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets testscript scripts under testdata/script `exec lifecycle
// ...` against the real CLI logic, in-process, per the standard
// testscript.Main pattern: this test binary re-execs itself as `lifecycle`
// when os.Args[0] matches.
//
// The registered wrapper mirrors main() exactly — including
// os.Exit(exitCode(err)) — so black-box scripts observe the real exit
// contract as later milestones give it teeth (0 clean, 1 violations, 2
// could-not-run; plan §2.4), not a flattened "1 on any error".
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"lifecycle": func() {
			if err := run(context.Background(), os.Args); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(exitCode(err))
			}
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			// exitcode <n> <command> [args...] runs a command and asserts its
			// process exit code equals <n>, capturing stdout/stderr so the
			// usual stdout/stderr builtins still match afterwards.
			// testscript's own `! exec` only distinguishes zero from
			// non-zero; `validate`'s contract needs 1 (findings) told apart
			// from 2 (could not run), so this is how the e2e suite asserts
			// the exact code (mirrors the sibling adr-sourced-constitution
			// primitive's cmd/constitution/main_test.go).
			"exitcode": cmdExitcode,
			// injecthash <jsonfile> replaces the literal token __HASH__ in
			// <jsonfile> with "sha256:<hex>" of constitution/constitution.md
			// (relative to the script's working directory), so a
			// deviation.json fixture's constitutionHash can be made to
			// actually match the rendered projection without depending on a
			// platform sha256 binary — verbatim mirror of the sibling
			// adr-sourced-constitution primitive's own cmd/constitution/
			// main_test.go helper of the same name, used by
			// approve_design_gate_real_constitution.txtar.
			"injecthash": cmdInjecthash,
		},
		// Setup/Condition wire the one testscript that exercises the REAL
		// constitution binary (approve_design_gate_real_constitution.txtar,
		// per implementation-plan.md M3's DoD: "at least one testscript
		// must exercise the REAL constitution binary built from
		// adr-sourced-constitution"). Every other script in this suite is
		// hermetic (never needs "constitution" on PATH at all — they either
		// stay at stage refine/repro/fix, which have no deviation-gate step,
		// or use approve's own unit tests' fake-subprocess coverage for the
		// deviation-gate LOGIC instead). Setup is harmless to run
		// unconditionally: it only prepends a PATH entry when the real
		// binary built successfully.
		Setup: func(e *testscript.Env) error {
			if dir, err := realConstitutionBinDir(); err == nil {
				e.Setenv("PATH", dir+string(os.PathListSeparator)+e.Getenv("PATH"))
			}
			return nil
		},
		Condition: func(cond string) (bool, error) {
			if cond != "realconstitution" {
				return false, fmt.Errorf("unknown condition %q", cond)
			}
			_, err := realConstitutionBinDir()
			return err == nil, nil
		},
	})
}

var (
	realConstitutionOnce sync.Once
	realConstitutionDir  string
	realConstitutionErr  error
)

// realConstitutionBinDir builds the REAL constitution binary from the
// sibling adr-sourced-constitution repo (harness AGENTS.md: both
// primitives live side by side as submodules) exactly once per test run,
// to a scratch directory outside that repo (never writing into it, per
// this milestone's hard rule), and returns the directory holding it. A
// non-nil error means the build was not possible in this environment
// (the sibling repo isn't checked out, or `go build` failed) — every
// script needing the real binary is written to skip with a clear reason
// in that case ([!realconstitution] skip '...'), never to fail the suite.
func realConstitutionBinDir() (string, error) {
	realConstitutionOnce.Do(func() {
		constDir := os.Getenv("LIFECYCLE_TEST_CONSTITUTION_REPO")
		if constDir == "" {
			_, thisFile, _, ok := runtime.Caller(0)
			if !ok {
				realConstitutionErr = fmt.Errorf("realConstitutionBinDir: runtime.Caller failed")
				return
			}
			// thisFile: .../harness/spec-lifecycle/cmd/lifecycle/main_test.go
			// -> up to .../harness, then into the sibling submodule.
			harnessDir := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
			constDir = filepath.Join(harnessDir, "adr-sourced-constitution")
		}
		if _, err := os.Stat(filepath.Join(constDir, "go.mod")); err != nil {
			realConstitutionErr = fmt.Errorf(
				"adr-sourced-constitution sibling repo not found at %s (set LIFECYCLE_TEST_CONSTITUTION_REPO to override): %w",
				constDir, err)
			return
		}

		binDir, err := os.MkdirTemp("", "lifecycle-real-constitution-")
		if err != nil {
			realConstitutionErr = err
			return
		}
		binPath := filepath.Join(binDir, "constitution")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}

		cmd := exec.Command("go", "build", "-o", binPath, "./cmd/constitution")
		cmd.Dir = constDir // building FROM the sibling repo, output OUTSIDE it — never writes there
		if out, err := cmd.CombinedOutput(); err != nil {
			realConstitutionErr = fmt.Errorf("building constitution from %s: %w: %s", constDir, err, out)
			return
		}
		realConstitutionDir = binDir
	})
	return realConstitutionDir, realConstitutionErr
}

func cmdInjecthash(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: injecthash <jsonfile>")
	}
	md, err := os.ReadFile(ts.MkAbs("constitution/constitution.md"))
	if err != nil {
		ts.Fatalf("injecthash: reading constitution.md: %v", err)
	}
	sum := sha256.Sum256(md)
	hash := "sha256:" + hex.EncodeToString(sum[:])

	target := ts.MkAbs(args[0])
	data, err := os.ReadFile(target)
	if err != nil {
		ts.Fatalf("injecthash: reading %s: %v", args[0], err)
	}
	out := strings.ReplaceAll(string(data), "__HASH__", hash)
	if err := os.WriteFile(target, []byte(out), 0o644); err != nil {
		ts.Fatalf("injecthash: writing %s: %v", args[0], err)
	}
}

func cmdExitcode(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: exitcode <n> <command> [args...]")
	}
	want, err := strconv.Atoi(args[0])
	if err != nil {
		ts.Fatalf("exitcode: first argument must be a number, got %q", args[0])
	}

	runErr := ts.Exec(args[1], args[2:]...)
	got := 0
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			got = ee.ExitCode()
		} else {
			ts.Fatalf("exitcode: running %v: %v", args[1:], runErr)
		}
	}

	if neg {
		if got == want {
			ts.Fatalf("exitcode: %v exited %d, did not want %d", args[1:], got, want)
		}
		return
	}
	if got != want {
		ts.Fatalf("exitcode: %v exited %d, want %d", args[1:], got, want)
	}
}
