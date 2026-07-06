// Package testutil holds shared test guards for OS-dependent filesystem
// behavior. CI runs {ubuntu, macos, windows}; any test that provokes an
// error via Unix filesystem semantics must use one of these guards instead
// of a bare runtime.GOOS / Geteuid check, so the skip policy stays uniform.
package testutil

import (
	"os"
	"runtime"
	"testing"
)

// SkipUnlessPermissionEnforcement skips tests that provoke errors via
// permission bits (chmod read-only dirs, 0o000 files). Windows does not
// enforce those bits, and root bypasses them.
func SkipUnlessPermissionEnforcement(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows: permission bits are not enforced")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks are bypassed")
	}
}

// SkipUnlessUnixFSErrors skips tests that depend on Unix error
// classification (e.g. ENOTDIR from reading a regular file as a
// directory). Windows maps some of these to not-exist, which defensive
// production code treats as absence rather than failure.
func SkipUnlessUnixFSErrors(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("windows: Unix filesystem error classification not available")
	}
}
