package plandag

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BinName is the milestoned-plan-dag executable name looked up on PATH by
// default (the harness installs it side by side with `constitution`,
// design D6 / spec-lifecycle.md §7).
const BinName = "milestoned-plan-dag"

// EnvBinOverride is the documented override for the milestoned-plan-dag
// binary's location, checked when no explicit override string is passed to
// Locate (e.g. from `lifecycle init --plan-dag-bin`). Set it to an
// absolute (or PATH-relative) binary path to bypass the default PATH
// lookup — the injectable-path mechanism the unit tests use to point at a
// stub binary rather than requiring the companion primitive be installed.
// Mirrors internal/constitution.EnvBinOverride.
const EnvBinOverride = "LIFECYCLE_PLAN_DAG_BIN"

// Locate resolves the milestoned-plan-dag binary's path. Precedence:
// override (a non-empty argument, from `--plan-dag-bin`) wins; else the
// EnvBinOverride environment variable; else a PATH lookup for
// "milestoned-plan-dag". A value containing a path separator is stat'd
// directly (not looked up on PATH); a bare name is resolved via
// exec.LookPath. Mirrors internal/constitution.Locate.
func Locate(override string) (string, error) {
	if override != "" {
		return resolve(override)
	}
	if env := os.Getenv(EnvBinOverride); env != "" {
		return resolve(env)
	}
	path, err := exec.LookPath(BinName)
	if err != nil {
		return "", fmt.Errorf(
			"%s: binary not found on PATH (install the milestoned-plan-dag companion primitive, or set %s): %w",
			BinName, EnvBinOverride, err,
		)
	}
	return path, nil
}

func resolve(p string) (string, error) {
	if filepath.Base(p) != p {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s: %s: %w", BinName, p, err)
		}
		return p, nil
	}
	path, err := exec.LookPath(p)
	if err != nil {
		return "", fmt.Errorf("%s: %s: %w", BinName, p, err)
	}
	return path, nil
}
