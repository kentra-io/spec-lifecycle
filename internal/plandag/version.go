package plandag

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// versionOutputRe strips the "milestoned-plan-dag version " prefix the
// companion primitive prints for `milestoned-plan-dag --version`
// (urfave/cli v3, the same shape internal/constitution parses for its own
// binary — e.g. "milestoned-plan-dag version 0.1.0 (abcdef012345)").
var versionOutputRe = regexp.MustCompile(`(?i)^milestoned-plan-dag version\s+(.*)$`)

// Version runs "<bin> --version" and returns the reported version string
// (with the "milestoned-plan-dag version " prefix stripped, if present).
func Version(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // bin is caller-resolved (Locate), not untrusted input
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", BinName, err)
	}
	line := strings.TrimSpace(string(out))
	if m := versionOutputRe.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[1]), nil
	}
	return line, nil
}

// Preflight is the outcome of checking a resolved milestoned-plan-dag
// binary against an optional version pin — the warn-not-fail preflight
// `lifecycle init` runs, mirroring internal/constitution.Preflight
// (spec-lifecycle.md §7 item 5, design D6).
type Preflight struct {
	Path    string
	Version string
	Pin     string
	// Compatible is only meaningful when Certain is true.
	Compatible bool
	// Certain is false when Version isn't a plain dotted-numeric release
	// build (e.g. a "(devel)" local build) — then Compatible can't be
	// evaluated and the caller treats it as advisory-only.
	Certain bool
	// Warning is a human-readable, non-fatal note; empty when the pin is
	// unset or satisfied.
	Warning string
}

// CheckVersion resolves bin's reported version and compares it against pin
// (an "x" wildcard component matches any value at that position). An empty
// pin always yields a satisfied, non-Warning result — presence, not
// version, is the only hard prerequisite when nothing is pinned. Mirrors
// internal/constitution.CheckVersion.
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
			"cannot confirm %s %s satisfies the pinned version %q (not a dotted-numeric release build) — proceeding",
			BinName, v, pin,
		)
		return pf, nil
	}

	ok = versionSatisfiesPin(vparts, strings.Split(pin, "."))
	pf.Compatible = ok
	if !ok {
		pf.Warning = fmt.Sprintf(
			"%s %s does not satisfy the pinned version %q", BinName, v, pin,
		)
	}
	return pf, nil
}

// leadingDottedNumeric parses v's leading dotted-numeric run (optionally
// preceded by a single "v"), ok=false if v doesn't start with one.
func leadingDottedNumeric(v string) (parts []string, ok bool) {
	v = strings.TrimPrefix(v, "v")
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

// versionSatisfiesPin reports whether vparts matches pparts component-wise,
// where an "x"/"X" pin component matches anything and a pin with more
// components than the version is a mismatch.
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
