package plandag

import (
	"fmt"
	"os"
	"strconv"
	"testing"
)

// TestMain implements the classic os/exec "fake subprocess" idiom (mirror
// of internal/constitution's own main_test.go): when re-exec'd with
// GO_WANT_HELPER_PROCESS=1, this test binary behaves like a
// milestoned-plan-dag binary (printing HELPER_STDOUT/HELPER_STDERR and
// exiting HELPER_EXIT) instead of running `go test`, so the Locate /
// Version / Validate / Resolve seam can be exercised without depending on
// the real companion primitive being installed.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT")) //nolint:errcheck
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR")) //nolint:errcheck
		code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeBin configures the current test binary to behave, when re-exec'd as a
// subprocess, like the milestoned-plan-dag CLI exiting with code and
// printing stdout/stderr. The returned path (os.Args[0], absolute) is a
// path with a separator, so Locate stat's it directly.
func fakeBin(t *testing.T, code int, stdout, stderr string) string {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", strconv.Itoa(code))
	t.Setenv("HELPER_STDOUT", stdout)
	t.Setenv("HELPER_STDERR", stderr)
	return os.Args[0]
}
