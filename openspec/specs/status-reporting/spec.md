# status-reporting Specification

## Purpose
TBD - created by archiving change 001-capability-size-warning. Update Purpose after archive.
## Requirements
### Requirement: Oversized-capability warning
The system SHALL surface an advisory warning identifying any capability
whose `openspec/specs/<capability>/spec.md` exceeds
`capabilitySizeWarningLines` (default 200) lines, from both
`lifecycle status` and `lifecycle guard`, without changing either verb's
exit code.

#### Scenario: A capability's spec.md exceeds the threshold
- **GIVEN** `openspec/specs/auth/spec.md` is 250 lines long and
  `capabilitySizeWarningLines` is unset (default 200)
- **WHEN** `lifecycle status` runs
- **THEN** the report includes a warning naming `auth` and its current
  line count, and the command still exits `0`

#### Scenario: Every capability is within the threshold
- **GIVEN** every capability's `spec.md` is under the configured threshold
- **WHEN** `lifecycle status` or `lifecycle guard` runs
- **THEN** no `capability_oversized` warning is reported

### Requirement: Machine-readable capability warnings
The system SHALL include oversized-capability warnings in
`--format json` output as a `capabilityWarnings` array (each entry naming
the capability and its line count), matching the same data surfaced in
`--format text`.

#### Scenario: JSON status output with an oversized capability
- **GIVEN** `openspec/specs/auth/spec.md` exceeds the configured threshold
- **WHEN** `lifecycle status --format json` runs
- **THEN** the JSON output's `capabilityWarnings` array contains an entry
  for `auth` with its current line count

