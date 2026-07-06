// init.go implements `lifecycle init`'s ordered, idempotent compose
// (implementation-plan.md §2.9): create the openspec/ tree, install the
// schema descriptor, wire openspec/config.yaml, seed lifecycle.yml,
// preflight the constitution companion, then fan out skills and managed
// pointer blocks. Every step is independently idempotent — a step that
// finds its target already in the state it wants is a no-op — so a
// re-run of the whole compose is byte-identical (M6 DoD,
// implementation-plan.md §8).

package scaffold

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/config"
	"github.com/kentra-io/spec-lifecycle/internal/constitution"
	"github.com/kentra-io/spec-lifecycle/internal/schema"
)

// defaultProjectContext seeds openspec/config.yaml's `context:` field
// (implementation-plan.md §2.9 step 4, spec-lifecycle.md §7.1). It is
// format-compatibility/documentation ONLY, mirroring OpenSpec's own
// `context` field (injected into artifact instructions by ITS runtime) —
// `lifecycle`'s own stage skills read `constitution/constitution.md`
// directly and never read this field back.
const defaultProjectContext = "This project's constitution lives at constitution/constitution.md " +
	"(the adr-sourced-constitution companion primitive). lifecycle's own stage skills " +
	"read it directly at gates 2 and 3 (design, plan) — this field exists for " +
	"OpenSpec-format compatibility and human documentation only; no `lifecycle` " +
	"code path reads it back (spec-lifecycle.md §7.1)."

// InitOptions configures a `lifecycle init` compose pass
// (implementation-plan.md §2.9).
type InitOptions struct {
	Root string

	// Force/Confirm apply to the managed-block step (h) exactly as they do
	// to a bare Refresh call (Options.Force/Options.Confirm).
	Force   bool
	Confirm func(prompt string) (bool, error)
	Stdout  io.Writer
	Stderr  io.Writer

	// Runtimes, SourceTrackingType and SourceTrackingRepo seed a FRESH
	// lifecycle.yml only (step e); an already-existing lifecycle.yml's own
	// values win on a re-run, matching the constitution's "existing config
	// wins" stance (its own cmd/constitution/init.go
	// noticeIgnoredReinitFlags). A nil/empty Runtimes uses
	// config.DefaultRuntimes.
	Runtimes           []string
	SourceTrackingType string
	SourceTrackingRepo string

	// ConstitutionBinOverride optionally overrides constitution-binary
	// resolution (internal/constitution.Locate's override argument) —
	// tests and a future `--constitution-bin` flag.
	ConstitutionBinOverride string
}

// InitResult reports what a RunInit pass did, for the CLI to print.
type InitResult struct {
	// Messages are progress lines ("created openspec/...", "wrote ...").
	Messages []string
	// Warnings are non-fatal preflight/consistency notices (missing or
	// version-mismatched constitution binary, disagreeing sourceTracking).
	Warnings []string
	// ConfigPath is the absolute path to lifecycle.yml.
	ConfigPath string
	// Fresh reports whether lifecycle.yml was freshly seeded this run (as
	// opposed to an already-existing one this run only read).
	Fresh bool
}

func (r *InitResult) msg(format string, a ...any) {
	r.Messages = append(r.Messages, fmt.Sprintf(format, a...))
}

func (r *InitResult) warn(format string, a ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, a...))
}

// RunInit runs the full §2.9 compose against o.Root. It returns a
// *scaffold.MarkerError (via errors.As) when a managed pointer-block
// target is structurally ambiguous, and a plain error for every other
// refusal (e.g. managed-block drift without --force) — callers map these
// to the CLI's exit-code contract exactly like every other verb
// (validate/archive/guard's own could-not-run vs. refused split).
func RunInit(o InitOptions) (InitResult, error) {
	var res InitResult

	// --- step a: openspec/{changes,specs} ---
	if created, err := ensureOpenSpecTree(o.Root); err != nil {
		return res, err
	} else if created {
		res.msg("created openspec/{changes,specs}")
	}

	// --- step b: schema descriptor (write only if absent — never silently
	// clobbers a hand-edited descriptor; internal/schema.Install has no
	// drift tracking of its own) ---
	installed, err := ensureSchemaDescriptor(o.Root)
	if err != nil {
		return res, err
	}
	if installed {
		res.msg("wrote %s", schema.Dir("."))
	}

	// --- steps c/d: openspec/config.yaml `schema:`+`context:` ---
	configYAMLPath := filepath.Join(o.Root, "openspec", "config.yaml")
	changed, err := EnsureProjectConfig(configYAMLPath, schema.Name, defaultProjectContext)
	if err != nil {
		return res, err
	}
	if changed {
		res.msg("wrote openspec/config.yaml")
	}

	// --- step e: lifecycle.yml ---
	lifecycleYMLPath := filepath.Join(o.Root, "lifecycle.yml")
	cfg, err := ensureLifecycleConfig(lifecycleYMLPath, o, &res)
	if err != nil {
		return res, err
	}
	res.ConfigPath = lifecycleYMLPath

	// --- step f: constitution preflight (presence + version; warn, never
	// fail — the binary is only hard-required later, at gates 2/3's
	// `lifecycle approve`, exactly like internal/approve's own posture) ---
	preflightConstitution(o.ConstitutionBinOverride, cfg.Constitution.Version, &res)

	// --- steps g/h: skill fan-out + managed pointer blocks ---
	items, err := BuildSkillItems(cfg.Runtimes)
	if err != nil {
		return res, err
	}
	blocks := BuildBlockTargets()

	refreshOpts := Options{
		Root:         o.Root,
		Mode:         ModeInit,
		Force:        o.Force,
		Confirm:      o.Confirm,
		Stdout:       discardIfNil(o.Stdout),
		Stderr:       discardIfNil(o.Stderr),
		BlockTargets: blocks,
		SkillItems:   items,
	}
	if err := PreflightBlocks(o.Root, blocks); err != nil {
		return res, err
	}
	if err := Refresh(refreshOpts); err != nil {
		return res, err
	}
	res.msg("fanned out skills and managed pointer blocks")

	return res, nil
}

// ensureOpenSpecTree creates openspec/{changes,specs} when openspec/ is
// entirely absent (implementation-plan.md §2.9 step 1: "we own the
// layout — no `openspec init` delegation"). An existing openspec/ tree
// (whatever shape it's in — a prior init, or a hand-adopted OpenSpec repo)
// is left untouched: `lifecycle` only ever ADDS the two directories it
// needs, never restructures what's there.
func ensureOpenSpecTree(root string) (created bool, err error) {
	openspecDir := filepath.Join(root, "openspec")
	if _, statErr := os.Stat(openspecDir); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}
	for _, sub := range []string{"changes", "specs"} {
		if err := os.MkdirAll(filepath.Join(openspecDir, sub), 0o755); err != nil {
			return false, fmt.Errorf("init: creating openspec/%s: %w", sub, err)
		}
	}
	return true, nil
}

// ensureSchemaDescriptor installs the embedded kentra-spec-lifecycle
// descriptor only when it isn't there yet, per implementation-plan.md
// §2.9 step 2's literal wording ("absent => install"): re-running install
// unconditionally would silently clobber a hand-edited descriptor, since
// internal/schema.Install carries no drift protection of its own.
func ensureSchemaDescriptor(root string) (installed bool, err error) {
	marker := filepath.Join(schema.Dir(root), "schema.yaml")
	if _, statErr := os.Stat(marker); statErr == nil {
		return false, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return false, statErr
	}
	if err := schema.Install(root); err != nil {
		return false, fmt.Errorf("init: %w", err)
	}
	return true, nil
}

// ensureLifecycleConfig loads an existing lifecycle.yml (its values win on
// a re-run) or seeds a fresh one from o (implementation-plan.md §2.9 step
// e, spec-lifecycle.md §10): schemaVersion 1, the pinned specFormat, a
// best-effort constitution.version pin detected from the installed
// binary, consentPolicy strict, sourceTracking, and runtimes. It also
// warns (never fails) when a sibling constitution.yml's sourceTracking
// disagrees (§2.10).
func ensureLifecycleConfig(path string, o InitOptions, res *InitResult) (*config.Config, error) {
	if _, statErr := os.Stat(path); statErr == nil {
		return config.Load(path)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return nil, statErr
	}

	cfg := config.Default()
	if len(o.Runtimes) > 0 {
		cfg.Runtimes = append([]string(nil), o.Runtimes...)
	}
	if o.SourceTrackingType != "" {
		cfg.SourceTracking.Type = o.SourceTrackingType
	}
	cfg.SourceTracking.Repo = o.SourceTrackingRepo

	if bin, lerr := constitution.Locate(o.ConstitutionBinOverride); lerr == nil {
		if v, verr := constitution.Version(bin); verr == nil {
			if pin, ok := constitution.MinorPin(v); ok {
				cfg.Constitution.Version = pin
			}
		}
	}

	warnDisagreeingSourceTracking(filepath.Dir(path), cfg, res)

	if err := cfg.Validate(path); err != nil {
		return nil, fmt.Errorf("init: seeded lifecycle.yml failed its own validation: %w", err)
	}
	if err := config.Save(path, cfg); err != nil {
		return nil, err
	}
	res.Fresh = true
	res.msg("wrote lifecycle.yml")

	// Reload so downstream code (and the caller) sees the validated,
	// on-disk verbatim, exactly like the constitution's own init.go does
	// for constitution.yml.
	return config.Load(path)
}

// warnDisagreeingSourceTracking checks a sibling constitution.yml (repo
// root, next to lifecycle.yml) for a sourceTracking.type field and warns
// when it differs from cfg's (implementation-plan.md §2.10). This is a
// best-effort, read-only peek at a handful of the companion primitive's
// config fields — spec-lifecycle never imports adr-sourced-constitution's
// Go packages (implementation-plan.md §2.12: "copy, don't couple"), and a
// missing or unparsable constitution.yml is silently skipped (no
// constitution.yml yet is the common case, not a problem to report).
func warnDisagreeingSourceTracking(root string, cfg *config.Config, res *InitResult) {
	data, err := os.ReadFile(filepath.Join(root, "constitution.yml"))
	if err != nil {
		return
	}
	var peek struct {
		SourceTracking struct {
			Type string `yaml:"type"`
		} `yaml:"sourceTracking"`
	}
	if yaml.Unmarshal(data, &peek) != nil {
		return
	}
	other := peek.SourceTracking.Type
	if other == "" || other == cfg.SourceTracking.Type {
		return
	}
	res.warn(
		"lifecycle.yml sourceTracking.type (%q) disagrees with constitution.yml's (%q) — pick one join-key convention (implementation-plan.md §2.10)",
		cfg.SourceTracking.Type, other,
	)
}

// preflightConstitution resolves the constitution binary and checks its
// version against pin, recording a warning (never an error) either way:
// spec-lifecycle.md §7 item 5 treats the constitution seam as a runtime
// process boundary whose absence only actually blocks work at gates 2/3
// (`lifecycle approve`, internal/approve's own runDeviationGate) — exactly
// like internal/approve/CheckVersion's own "presence, not version, is the
// only hard prerequisite" stance. `lifecycle init` must always succeed for
// a repo that hasn't installed the companion primitive yet.
func preflightConstitution(override, pin string, res *InitResult) {
	bin, lerr := constitution.Locate(override)
	if lerr != nil {
		res.warn(
			"constitution binary not found (%s) — gates 2/3 (`lifecycle approve --stage design|plan`) require it; install the adr-sourced-constitution companion primitive",
			lerr.Error(),
		)
		return
	}
	pf, verr := constitution.CheckVersion(bin, pin)
	if verr != nil {
		res.warn("could not confirm the constitution binary's version: %s", verr.Error())
		return
	}
	if pf.Warning != "" {
		res.warn("%s", pf.Warning)
	}
}

func discardIfNil(w io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return io.Discard
}
