package main

import (
	"context"
	"fmt"
	"os"
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
	})
}
