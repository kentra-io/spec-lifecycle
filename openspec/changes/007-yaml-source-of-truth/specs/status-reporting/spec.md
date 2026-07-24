## MODIFIED Requirements

### Requirement: Machine-readable capability warnings
The system SHALL include oversized-capability warnings in `lifecycle
status`'s YAML machine output (`--format yaml`) as a `capabilityWarnings`
sequence (each entry naming the capability and its line count), matching the
same data surfaced in `--format text`. The `--format json` option is
removed.

#### Scenario: YAML status output with an oversized capability
- **GIVEN** `openspec/specs/auth/spec.md` exceeds the configured threshold
- **WHEN** `lifecycle status --format yaml` runs
- **THEN** the YAML output's `capabilityWarnings` sequence contains an entry
  for `auth` with its current line count
