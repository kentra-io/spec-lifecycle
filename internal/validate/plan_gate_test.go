package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kentra-io/spec-lifecycle/internal/plandag"
)

// TestMain implements the classic os/exec "fake subprocess" idiom (mirror
// of internal/constitution's main_test.go): when re-exec'd with
// GO_WANT_HELPER_PROCESS=1 this test binary behaves like a
// milestoned-plan-dag binary, so the plan-stage gate's shell-out can be
// exercised without the real companion primitive installed.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT")) //nolint:errcheck
		fmt.Fprint(os.Stderr, os.Getenv("HELPER_STDERR")) //nolint:errcheck
		code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// fakePlanDAGBin points internal/plandag.Locate at this test binary (via
// the LIFECYCLE_PLAN_DAG_BIN env override) configured to exit `code` and
// print `stdout`/`stderr` — the injectable stub the plan-stage gate tests
// use.
func fakePlanDAGBin(t *testing.T, code int, stdout, stderr string) {
	t.Helper()
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_EXIT", strconv.Itoa(code))
	t.Setenv("HELPER_STDOUT", stdout)
	t.Setenv("HELPER_STDERR", stderr)
	t.Setenv(plandag.EnvBinOverride, os.Args[0])
}

func TestValidatePlanValidPasses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plan.yaml"), "milestones: []\n")
	fakePlanDAGBin(t, 0, "plan is valid\n", "")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %+v, want none (milestoned-plan-dag reported the plan valid)", findings)
	}
}

func TestValidatePlanInvalidFailsSurfacingMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plan.yaml"), "milestones: []\n")
	fakePlanDAGBin(t, 1, "", "milestone 3: dependency cycle A -> B -> A\n")

	findings, err := Change(dir, StagePlan)
	if err != nil {
		t.Fatalf("Change: %v (an invalid plan is a Finding, not a run failure)", err)
	}
	if len(findings) != 1 || findings[0].Kind != "invalid_plan" {
		t.Fatalf("findings = %+v, want exactly one invalid_plan finding", findings)
	}
	if !strings.Contains(findings[0].Message, "dependency cycle A -> B -> A") {
		t.Errorf("finding message = %q, want the primitive's message surfaced verbatim", findings[0].Message)
	}
}

func TestValidatePlanBinaryAbsentIsCouldNotRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "plan.yaml"), "milestones: []\n")
	t.Setenv(plandag.EnvBinOverride, "")
	t.Setenv("PATH", t.TempDir())

	if _, err := Change(dir, StagePlan); err == nil {
		t.Fatal("Change: err = nil, want a could-not-run error when the plan primitive is absent")
	}
}
