package approve

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kentra-io/spec-lifecycle/internal/config"
)

// ErrConsentRequired is returned (wrapped, via errors.Is) when a write is
// refused because consentPolicy is strict and no consent was given.
var ErrConsentRequired = errors.New("approve: refused — no consent given under the strict consent policy")

// ConsentGate decides whether `lifecycle approve` may write under the
// project's consentPolicy (spec-lifecycle.md §5's closing paragraph,
// implementation-plan.md §2.6/§2.10) — a direct mirror of the sibling
// adr-sourced-constitution primitive's cmd/constitution/consent.go, kept
// as a package-level type here (rather than in cmd/lifecycle) so Approve's
// own contract — "consentPolicy strict => refuse to write without an
// explicit --approve flag" — is unit-testable without the CLI.
//
// Under "strict" every write (an approval OR a --reject: both are
// mutations of the ground-truth record) needs an explicit human OK:
// either Approved=true (the --approve flag, for scripted/CI use) or an
// interactive "yes" at a TTY prompt. Non-interactive with neither is a
// hard refusal — nothing is written. Under "off" there is no gate.
//
// The honest limitation this mirrors from the constitution primitive:
// Layer 1 cannot verify a HUMAN typed the confirmation — the real
// architectural checkpoint is the agent-harness permission prompt around
// the Bash call that runs `lifecycle approve`. This is the CLI-level
// backstop that makes an unattended write refuse by default.
type ConsentGate struct {
	Policy   string    // config.ConsentStrict | config.ConsentOff
	Approved bool      // the --approve flag
	IsTTY    bool      // stdin is an interactive terminal
	In       io.Reader // where an interactive confirmation is read from
	Out      io.Writer // where the prompt is written
}

// Confirm returns nil if the mutation may proceed, or an error wrapping
// ErrConsentRequired otherwise. verb names the action for the prompt and
// the error message.
func (g ConsentGate) Confirm(verb string) error {
	if g.Policy == config.ConsentOff {
		return nil
	}
	if g.Approved {
		return nil
	}
	if !g.IsTTY {
		return fmt.Errorf(
			"%w: %s requires confirmation under the %q policy, but stdin is not a terminal; pass --approve to proceed non-interactively (or set consentPolicy: off)",
			ErrConsentRequired, verb, config.ConsentStrict,
		)
	}

	_, _ = fmt.Fprintf(g.Out, "About to %s. This writes to approval-state.json. Proceed? [y/N] ", verb)
	line, _ := bufio.NewReader(g.In).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return fmt.Errorf("%w: %s not confirmed; nothing was written", ErrConsentRequired, verb)
	}
}
