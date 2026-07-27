package archive

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/plandag"
)

// TestMain implements the classic os/exec "fake subprocess" idiom (mirror
// of internal/constitution's main_test.go): when re-exec'd with
// GO_WANT_HELPER_PROCESS=1 this test binary behaves like a
// milestoned-plan-dag binary, so the archive step-completion gate's
// `milestoned-plan-dag resolve` shell-out can be exercised without the real
// companion primitive installed.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT")) //nolint:errcheck
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR")) //nolint:errcheck
		code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakeResolveBin points internal/plandag.Locate at this test binary (via
// the LIFECYCLE_PLAN_DAG_BIN env override), configured to exit 0 and print
// resolveYAML as its `resolve` output — the injectable stub the archive
// step-completion gate tests use.
func fakeResolveBin(t *testing.T, resolveYAML string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", "0")
	t.Setenv("HELPER_STDOUT", resolveYAML)
	t.Setenv("HELPER_STDERR", "")
	t.Setenv(plandag.EnvBinOverride, os.Args[0])
}
