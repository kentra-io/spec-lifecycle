## ADDED Requirements

### Requirement: New-feature intake skill is shipped and fanned out

The system SHALL ship a `lifecycle-new-feature` intake skill as part of the
fanned-out skill set, so that `lifecycle init` writes a
`skills/lifecycle-new-feature/SKILL.md` into every runtime skill tree
selected by `lifecycle.yml`'s `runtimes:` list, alongside the existing stage
skills.

#### Scenario: init fans out the intake skill

- **GIVEN** `lifecycle.yml` lists `claude-code` under `runtimes:`
- **WHEN** `lifecycle init` runs
- **THEN** `.claude/skills/lifecycle-new-feature/SKILL.md` is written
  alongside the other lifecycle stage skills

#### Scenario: intake skill is absent when its runtime is not selected

- **GIVEN** `lifecycle.yml` lists only `codex` under `runtimes:`
- **WHEN** `lifecycle init` runs
- **THEN** the intake skill is written under the `codex` skill tree
  (`.agents/skills/lifecycle-new-feature/SKILL.md`) and not under
  `.claude/skills/`

### Requirement: Intake creates a source-tracking issue only on human confirmation

The `lifecycle-new-feature` intake flow SHALL, for a feature idea that is
not yet tracked, draft a GitHub issue and create it as `type: feature` only
after the human explicitly confirms the drafted text — never on the agent's
own judgment — and SHALL capture the resulting `<owner>/<repo>#<n>`
reference for the change it seeds.

#### Scenario: human confirms the drafted issue

- **GIVEN** a human describes an untracked feature idea to the intake skill
- **WHEN** the human explicitly approves the drafted issue text
- **THEN** the skill creates the GitHub issue with `type: feature` and
  records its `<owner>/<repo>#<n>` reference

#### Scenario: human has not confirmed the drafted issue

- **GIVEN** the intake skill has drafted an issue but the human has neither
  approved it nor requested it be filed
- **WHEN** the skill decides whether to create the issue
- **THEN** no GitHub issue is created

### Requirement: Intake seeds a refine-ready change folder and stops before gate 1

The intake flow SHALL create the change folder
`openspec/changes/<n>-<slug>/` following `lifecycle.yml`'s `changeNaming`,
record the source-tracking issue reference in a stub `proposal.md`
frontmatter (`issue:` and `type: feature` filled; body left for refine),
and STOP — handing off to the `refine` stage in a fresh session. It SHALL
NOT draft `proposal.md`'s body, any `specs/<capability>/spec.md` delta, or
any gate record (`approval-state.json`); those belong to refine and gate 1.

#### Scenario: folder seeded and handed off to refine

- **GIVEN** the intake skill has created issue `kentra-io/spec-lifecycle#3`
  for an untracked feature with slug `new-feature-intake`
- **WHEN** it seeds the change folder
- **THEN** `openspec/changes/003-new-feature-intake/proposal.md` exists with
  `issue: "kentra-io/spec-lifecycle#3"` and `type: feature` in its
  frontmatter, no `approval-state.json` is written, and the skill instructs
  the human to run `/lifecycle-refine` in a fresh session

#### Scenario: intake never touches the living spec or gate records

- **WHEN** the `lifecycle-new-feature` intake flow completes for a change
- **THEN** no `openspec/specs/<capability>/spec.md` file is modified and no
  gate entry is appended to that change's `approval-state.json`
