## Milestone 1: Shared size-check helper + config key
**Goal** — a shared, testable helper that says whether a capability's `spec.md` is oversized, and a project-tunable threshold to check it against.
**Deliverables** — `internal/spec` gains a line-counting helper over a rendered capability spec; `internal/config`'s `Config` struct gains an optional `CapabilitySizeWarningLines int` field (`lifecycle.yml` key `capabilitySizeWarningLines`, default 200 when absent or zero).
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/spec/... ./internal/config/...` passes, including a new case asserting the helper flags a >200-line fixture and does not flag a <=200-line one, and a config-load case asserting the default applies when the key is absent.
  - Makes the "Machine-readable capability warnings" scenario's precondition satisfiable (a capability whose line count is known and comparable to the threshold).
**Steps** — ordered breakdown, sized per `planGranularity` (lifecycle.yml, spec-lifecycle.md §10):
  1. Add the line-counting helper to `internal/spec`, taking rendered `spec.md` bytes and a threshold, returning an oversized bool + line count.
  2. Add `CapabilitySizeWarningLines` to `internal/config.Config` with a `default200IfZero` accessor so callers never hand-roll the default.
  3. Unit-test both with fixtures at exactly-200, 201, and a trivially small spec.

## Milestone 2: Wire the check into `status` and `guard`
**Goal** — `lifecycle status` and `lifecycle guard` both surface `capability_oversized` warnings without changing either verb's exit code.
**Deliverables** — `internal/status`'s report walk and `internal/guard`'s check pipeline each call the Milestone-1 helper per capability under `openspec/specs/` and append a warning entry; `--format text` prints one line per oversized capability; `--format json` adds a `capabilityWarnings` array to both verbs' JSON output.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - A new testscript case (`cmd/lifecycle/testdata/script/status_capability_oversized.txtar` or equivalent) with a deliberately oversized fixture capability: `lifecycle status` exits `0` and its text output names the capability; `lifecycle status --format json` exits `0` and its `capabilityWarnings` array contains that capability.
  - An equivalent `lifecycle guard` case: guard still exits `0` on an otherwise-clean project with one oversized capability, and the oversized capability is named in both `--format text` and `--format json` output.
  - Makes the "A capability's spec.md exceeds the threshold" and "JSON status output with an oversized capability" scenarios (`specs/status-reporting/spec.md`) pass.
  - `go test ./internal/status/... ./internal/guard/...` and the full `go test ./...` remain green; `golangci-lint run` clean.
**Steps** — ordered breakdown, sized per `planGranularity` (lifecycle.yml, spec-lifecycle.md §10):
  1. Call the helper from `internal/status`'s per-capability loop; append to its existing warnings collection; extend its JSON struct with `capabilityWarnings`.
  2. Call the same helper from `internal/guard`'s check pipeline as a fourth, purely-advisory pass that can never itself return a `1`/`2` exit; extend guard's JSON struct the same way.
  3. Add the text-format line(s) for both verbs, matching the existing report style.
  4. Add the two testscript cases above with an oversized fixture capability; confirm the "Every capability is within the threshold" scenario (no warning) still holds on the existing clean fixtures.
