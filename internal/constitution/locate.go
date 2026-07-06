package constitution

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// EnvBinOverride is the documented override mechanism for the constitution
// binary's location, checked when no explicit override string is passed to
// Locate (e.g. from a future `--constitution-bin` flag). Set it to an
// absolute (or PATH-relative) binary path to bypass the default PATH
// lookup — the mechanism the M3 testscript e2e suite uses to point at a
// binary built fresh from source rather than requiring the companion
// primitive be `go install`ed on the machine running the tests.
const EnvBinOverride = "LIFECYCLE_CONSTITUTION_BIN"

// Locate resolves the constitution binary's path. Precedence: override (a
// non-empty argument, reserved for a future CLI flag) wins; else the
// EnvBinOverride environment variable; else a PATH lookup for
// "constitution" (the default — the harness installs both primitives side
// by side, spec-lifecycle.md §7/§9.3). A value containing a path
// separator is stat'd directly (not looked up on PATH); a bare name is
// resolved via exec.LookPath so "constitution" and
// "./bin/constitution" both work as expected.
func Locate(override string) (string, error) {
	if override != "" {
		return resolve(override)
	}
	if env := os.Getenv(EnvBinOverride); env != "" {
		return resolve(env)
	}
	path, err := exec.LookPath("constitution")
	if err != nil {
		return "", fmt.Errorf(
			"constitution: binary not found on PATH (install the adr-sourced-constitution companion primitive, or set %s): %w",
			EnvBinOverride, err,
		)
	}
	return path, nil
}

func resolve(p string) (string, error) {
	if filepath.Base(p) != p {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("constitution: %s: %w", p, err)
		}
		return p, nil
	}
	path, err := exec.LookPath(p)
	if err != nil {
		return "", fmt.Errorf("constitution: %s: %w", p, err)
	}
	return path, nil
}
