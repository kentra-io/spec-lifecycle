# Warn when a capability's living spec grows past ~200 lines — Design

## Context

`openspec/specs/<capability>/spec.md` accumulates requirements forever via
the fold (spec-lifecycle.md §6.1); nothing today tells a human a capability
has grown large enough to be a split candidate. spec-lifecycle.md §6.4
already commits to this warning (mirroring the constitution primitive's own
projection-size warning) but it was never implemented. `internal/status`
and `internal/guard` both already walk `openspec/specs/` for other reasons,
so this is an additive check over data they already have in hand, not a new
read path.

## Goals / Non-Goals

**Goals:**
- Surface a `capability_oversized` advisory from both `lifecycle status`
  and `lifecycle guard` when a capability's `spec.md` exceeds a configurable
  line-count threshold.
- Make the threshold configurable per project (`lifecycle.yml`), defaulting
  to spec-lifecycle.md §6.4's ~200 lines.
- Keep the check purely advisory: no exit-code change on either verb.

**Non-Goals:**
- No automatic capability splitting or refactoring — a human decides how
  to act on the warning.
- No change to the fold, delta grammar, or archive pipeline.
- No retroactive sweep/report command of its own — the warning rides along
  inside `status`/`guard`'s existing reports.

## Decisions

- **Where the check lives:** a small shared helper in `internal/spec`
  (line-counting a rendered `spec.md`) called from both `internal/status`
  and `internal/guard`, so the two verbs can never drift on what counts as
  "oversized." Considered duplicating the check in each package instead —
  rejected, since the two verbs already share the fold/render code and this
  is the same kind of shared primitive.
- **Threshold configuration:** a new optional `capabilitySizeWarningLines`
  key in `lifecycle.yml` (default 200 when absent), following the existing
  `config` package's plain-struct, no-Viper convention. Considered a
  hardcoded constant instead — rejected, since the constitution primitive's
  analogous projection-size warning is not configurable and that has
  already been a minor friction point there; making it configurable here
  from the start is cheap.
- **Reporting shape:** `--format text` gets one line per oversized
  capability appended to the existing report; `--format json` gets a new
  top-level `capabilityWarnings` array. Considered folding it into the
  existing gate/finding structures instead — rejected, since a capability
  warning is not tied to any one change folder or gate, unlike every
  existing `status`/`guard` finding type.

## NFR Discharge

- `Advisory-only, non-blocking`: enforced by construction — the new check
  only appends to the existing report/warnings slice; it never returns an
  error or contributes to either verb's exit-code computation. Proven by a
  testscript case with a deliberately oversized fixture capability where
  `status`/`guard` still exit 0.
- `No new read path / no performance regression`: the check reuses the
  `spec.md` bytes `status`/`guard` already read for their existing
  responsibilities; no additional file walk is introduced.

## ADR proposals

(none) — this is a reporting addition over already-owned data; it does not
touch the pure-Go-engine, conformance-corpus, or ledger-ordering
invariants this repo's constitution governs (ADR-0001..0003).

## Risks / Trade-offs

- **Threshold bikeshedding** → default matches the number the spec already
  committed to (~200); a project can retune it without a code change.
- **Warning fatigue if many capabilities sit just over the line** → out of
  scope for this change; a future change can add hysteresis/grouping if it
  becomes a real problem.
