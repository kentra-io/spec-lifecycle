package constitution

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

// TestMain implements the classic os/exec "fake subprocess" idiom: when
// re-exec'd with GO_WANT_HELPER_PROCESS=1, this test binary behaves like
// a `constitution` binary (printing HELPER_STDOUT/HELPER_STDERR and
// exiting HELPER_EXIT) instead of running `go test`. Every unit test in
// this package that needs a "constitution" executable uses
// fakeConstitutionBin instead of building/depending on the real one — the
// real binary is exercised only by cmd/lifecycle's testscript e2e suite
// (implementation-plan.md M3's DoD), keeping this package's own tests in
// the hermetic Go tier (plan §9).
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT")) //nolint:errcheck
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR")) //nolint:errcheck
		code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeConstitutionBin configures the current test binary to behave, when
// re-exec'd as a subprocess, like the constitution CLI exiting with code
// and printing stdout/stderr.
func fakeConstitutionBin(t *testing.T, code int, stdout, stderr string) string {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", strconv.Itoa(code))
	t.Setenv("HELPER_STDOUT", stdout)
	t.Setenv("HELPER_STDERR", stderr)
	return os.Args[0]
}
