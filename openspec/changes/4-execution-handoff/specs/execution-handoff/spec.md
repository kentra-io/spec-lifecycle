## ADDED Requirements

### Requirement: Structured validation contract on a milestone
A milestone's Validation contract section MAY carry a single fenced
` ```contract ` YAML block declaring `check` (a single executable
acceptance-check command), `criteria` (plain-language acceptance
criteria), and `paths` (a non-empty list of repo-relative allowed-path
globs — the diff-confined-paths declaration). The system SHALL enforce
this block is well-formed whenever it is present, and SHALL NOT require
it: a milestone with only the pre-existing free-text Validation-contract
bullets and no ` ```contract ` block SHALL still validate.

#### Scenario: Well-formed contract block validates
- **GIVEN** a milestone's Validation contract section carries a
  ` ```contract ` block with non-empty `check`, non-empty `criteria`, and
  at least one repo-relative `paths` entry
- **WHEN** `lifecycle validate --stage plan` runs
- **THEN** no finding is reported for that block

#### Scenario: Malformed contract block is rejected
- **GIVEN** a milestone's ` ```contract ` block is missing `check`,
  `criteria`, or `paths`, is not valid YAML, declares more than one
  ` ```contract ` block, or declares a `paths` entry that is absolute or
  contains `..`
- **WHEN** `lifecycle validate --stage plan` runs
- **THEN** the system SHALL report an error-severity finding naming the
  specific problem, and the command SHALL exit non-zero

#### Scenario: A contract-less milestone still validates
- **GIVEN** a milestone carries only the pre-existing free-text
  Validation-contract bullets, with no ` ```contract ` block at all
- **WHEN** `lifecycle validate --stage plan` runs
- **THEN** no contract-related finding is reported

### Requirement: Archive refuses on incomplete tracked tasks
A Steps-list line MAY carry a `[ ]`/`[x]` checkbox right after its
ordinal number. The system SHALL refuse `lifecycle archive <change>` when
any milestone in that change's `tasks.md` has at least one
checkbox-tracked Steps item that is not checked, unless
`--force-incomplete-tasks` is given, in which case the archive SHALL
proceed and SHALL record the override (`tasksIncompleteOverridden: true`)
on every ledger record it appends. A `tasks.md` with no checkbox-tracked
steps at all, or no `tasks.md`, SHALL NOT be refused by this gate.

#### Scenario: Archive is refused on an unchecked tracked step
- **GIVEN** a change's `tasks.md` has a milestone with one checked and one
  unchecked tracked Steps item, and every required gate is approved
- **WHEN** `lifecycle archive <change>` runs without
  `--force-incomplete-tasks`
- **THEN** the system SHALL refuse (exit 1), name the unchecked step in
  its error message, and SHALL NOT relocate the change folder, fold any
  delta, or append any ledger record

#### Scenario: Archive proceeds when every tracked step is checked
- **GIVEN** a change's `tasks.md` has only checked tracked Steps items (or
  no tracked items at all), and every required gate is approved
- **WHEN** `lifecycle archive <change>` runs
- **THEN** the system SHALL archive the change exactly as it would have
  before this requirement existed

#### Scenario: The override is recorded, never silent
- **GIVEN** a change's `tasks.md` has an unchecked tracked Steps item
- **WHEN** `lifecycle archive --force-incomplete-tasks <change>` runs
- **THEN** the archive SHALL proceed, and every ledger record it appends
  SHALL carry `tasksIncompleteOverridden: true`

### Requirement: `lifecycle apply` surfaces milestones and contracts as JSON
The system SHALL provide a `lifecycle apply <change> [--format
text|json]` verb that projects a change's `tasks.md` milestones —
id/title, Steps (with checkbox-tracked state), and the optional
structured contract (`check`/`criteria`/`paths`) — as text or JSON,
without requiring a consumer to parse markdown itself. The system SHALL
refuse (exit 1) to surface a plan that fails the same plan-stage
structural validation `lifecycle validate --stage plan` runs.

#### Scenario: JSON surface includes the contract fields
- **GIVEN** a change's `tasks.md` has a milestone with a well-formed
  ` ```contract ` block
- **WHEN** `lifecycle apply <change> --format json` runs
- **THEN** the JSON output includes that milestone's `id`, `title`, and a
  `contract` object with `check`, `criteria`, and `paths`

#### Scenario: apply refuses an invalid plan
- **GIVEN** a change's `tasks.md` has a malformed ` ```contract ` block
- **WHEN** `lifecycle apply <change> --format json` runs
- **THEN** the system SHALL refuse (exit 1) and SHALL NOT emit a
  `milestones` payload
