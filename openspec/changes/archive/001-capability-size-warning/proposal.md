---
issue: "kentra-io/spec-lifecycle#1"
designSkip: false
type: feature
---

# Warn when a capability's living spec grows past ~200 lines

## Why

spec-lifecycle.md §6.4 commits to a size-discipline warning — mirroring the
constitution primitive's own projection-size warning — so an
`openspec/specs/<capability>/spec.md` that has grown large enough to be a
split candidate is surfaced to the human instead of silently accumulating
forever. This was speced but never built; nothing today tells a human a
capability has outgrown a single file.

## What Changes

- **status-reporting** (new capability): `lifecycle status` and
  `lifecycle guard` gain a size-discipline check — when a capability's
  `openspec/specs/<capability>/spec.md` exceeds a configurable line-count
  threshold (default ~200 lines, per spec-lifecycle.md §6.4), the report
  surfaces a `capability_oversized` warning naming the capability and its
  current line count. The check is advisory only: it never changes an exit
  code (0 for `status`; guard's existing 0/1/2 contract is unaffected — an
  oversized capability is a warning, not a `guard` finding) and never blocks
  approval or archive.
- Both `--format text` and `--format json` surface the same warning
  (text: a human-readable line per oversized capability; json: a
  `capabilityWarnings` array alongside the existing gate/guard fields).

## Impact

- `internal/status` (adds the size check to the existing report walk over
  `openspec/specs/`).
- `internal/guard` (same check, reported alongside its existing three
  checks — additive, no change to guard's existing exit-code semantics).
- `lifecycle.yml` schema: a new optional `capabilitySizeWarningLines`
  key (default 200) so a project can tune the threshold.
- No change to the delta grammar, fold, or archive — this is purely a
  reporting addition over data `lifecycle` already reads.
