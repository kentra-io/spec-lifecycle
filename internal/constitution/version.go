package constitution

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// versionOutputRe strips the "constitution version " prefix urfave/cli v3
// prints for `constitution --version` (confirmed empirically: `constitution
// --version` → "constitution version (devel) (7e24d2aee640-dirty)" for a
// plain `go build`, or "constitution version v0.1.0 (abcdef012345)" for a
// GoReleaser release build — see adr-sourced-constitution's cmd/constitution
// version.go's buildVersion/buildInfoVersion).
var versionOutputRe = regexp.MustCompile(`(?i)^constitution version\s+(.*)$`)

// Version runs "<bin> --version" and returns the reported version string
// (with the "constitution version " prefix stripped, if present).
func Version(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // bin is caller-resolved (Locate), not user input from an untrusted source
	if err != nil {
		return "", fmt.Errorf("constitution --version: %w", err)
	}
	line := strings.TrimSpace(string(out))
	if m := versionOutputRe.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[1]), nil
	}
	return line, nil
}

// Preflight is the outcome of checking a resolved constitution binary
// against the lifecycle.yml `constitution: { version }` pin
// (spec-lifecycle.md §7 item 5/§10, implementation-plan.md §2.7).
type Preflight struct {
	Path    string
	Version string
	Pin     string
	// Compatible is only meaningful when Certain is true.
	Compatible bool
	// Certain is false when Version isn't a plain dotted-numeric release
	// build (e.g. a "(devel)"-tagged local build) — in that case
	// Compatible can't be evaluated, and the caller should treat this as
	// advisory-only (spec-lifecycle.md §7: "version mismatch = warning",
	// not a hard refusal — independent release cadences are expected).
	Certain bool
	// Warning is a human-readable, non-fatal note: either "version could
	// not be confirmed" (Certain=false) or "version does not satisfy the
	// pin" (Certain=true, Compatible=false). Empty when the pin is
	// unset or satisfied.
	Warning string
}

// CheckVersion resolves bin's reported version and compares it against
// pin (lifecycle.yml's constitution.version, e.g. "0.1.x" — an "x"
// wildcard component matches any value at that position). An empty pin
// always yields a satisfied, non-Warning result — presence, not version,
// is the only hard prerequisite when nothing is pinned.
func CheckVersion(bin, pin string) (Preflight, error) {
	v, err := Version(bin)
	if err != nil {
		return Preflight{}, err
	}
	pf := Preflight{Path: bin, Version: v, Pin: pin, Compatible: true, Certain: true}
	if pin == "" {
		return pf, nil
	}

	vparts, ok := leadingDottedNumeric(v)
	if !ok {
		pf.Certain = false
		pf.Compatible = false
		pf.Warning = fmt.Sprintf(
			"cannot confirm constitution %s satisfies the pinned version %q (not a dotted-numeric release build) — proceeding",
			v, pin,
		)
		return pf, nil
	}

	ok = versionSatisfiesPin(vparts, strings.Split(pin, "."))
	pf.Compatible = ok
	if !ok {
		pf.Warning = fmt.Sprintf(
			"constitution %s does not satisfy the pinned version %q (lifecycle.yml constitution.version)", v, pin,
		)
	}
	return pf, nil
}

// leadingDottedNumeric parses v's leading dotted-numeric run (optionally
// preceded by a single "v", e.g. "v0.1.0" or "0.1.0-rc1 (abcdef)" → the
// "0.1.0" portion), ok=false if v doesn't start with one at all.
func leadingDottedNumeric(v string) (parts []string, ok bool) {
	v = strings.TrimPrefix(v, "v")
	// Cut at the first byte that can't appear in a dotted-numeric run.
	end := len(v)
	for i, r := range v {
		if (r < '0' || r > '9') && r != '.' {
			end = i
			break
		}
	}
	head := strings.Trim(v[:end], ".")
	if head == "" {
		return nil, false
	}
	parts = strings.Split(head, ".")
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return nil, false
		}
	}
	return parts, true
}

// versionSatisfiesPin reports whether vparts matches pparts
// component-wise, where an "x"/"X" pin component matches anything and a
// pin with more components than the version is a mismatch.
func versionSatisfiesPin(vparts, pparts []string) bool {
	if len(pparts) > len(vparts) {
		return false
	}
	for i, p := range pparts {
		if strings.EqualFold(p, "x") {
			continue
		}
		if p != vparts[i] {
			return false
		}
	}
	return true
}
