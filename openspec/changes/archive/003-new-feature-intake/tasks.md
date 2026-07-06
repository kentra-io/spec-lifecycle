## Milestone 1: Author the `lifecycle-new-feature` intake skill body
**Goal** — Write the skill that turns an untracked feature idea into a refine-ready change folder, encoding the confirm-issue → seed-folder → stop-and-handoff contract.
**Deliverables** — `skills/lifecycle-new-feature/SKILL.md`, containing:
  - YAML frontmatter (`name: lifecycle-new-feature`, non-empty `description`) matching the sibling stage skills' SKILL.md shape.
  - A body encoding the intake flow: (a) clarify the idea only enough to file a good issue — requirements stay refine's job; (b) draft the GitHub issue and create it as `type: feature` **only after explicit human confirmation**, never on the agent's own judgment; (c) derive a kebab-case slug and seed `openspec/changes/<issue-number>-<slug>/` per `lifecycle.yml`'s `changeNaming`, writing a stub `proposal.md` (frontmatter `issue:` + `type: feature` filled, body left for refine); (d) STOP and instruct the human to run `/lifecycle-refine <change>` in a fresh session.
  - A "Never" section: never create the issue without explicit confirmation; never write a spec delta, an `approval-state.json` gate record, or modify `openspec/specs/`; feature-only — route defects to `/lifecycle-bug`.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `test -f skills/lifecycle-new-feature/SKILL.md` succeeds; the file opens with YAML frontmatter containing `name: lifecycle-new-feature` and a non-empty `description:` (same shape as `skills/lifecycle-refine/SKILL.md`).
  - `grep -qi "only after" skills/lifecycle-new-feature/SKILL.md` **and** the body states the issue is created only on explicit human confirmation and never on agent judgment, and gives the not-confirmed path (no issue created) explicitly → discharges **"Intake creates a source-tracking issue only on human confirmation"** (both scenarios).
  - The body instructs seeding `openspec/changes/<n>-<slug>/` with a stub `proposal.md` frontmatter and then STOPPING with a `/lifecycle-refine` handoff, and forbids drafting the proposal body / any spec delta / any gate record → discharges **"Intake seeds a refine-ready change folder and stops before gate 1"** (both scenarios).
  - `grep -q "approval-state.json" skills/lifecycle-new-feature/SKILL.md` in a prohibition context, and the "Never" section forbids modifying `openspec/specs/` → discharges **"Intake never touches the living spec or gate records"**.
  - These four requirements are agent-behavior contracts realized by the skill body; each is graded by reading the body against the scenario checklist above (deterministic review), not by a runtime test.
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Read `skills/lifecycle-refine/SKILL.md` for the SKILL.md frontmatter + section conventions to mirror.
  2. Write the frontmatter and the intake-flow body (steps a–d).
  3. Add the "Never" guardrails (no unconfirmed issue; no delta / gate-record / living-spec writes; feature-only → `/lifecycle-bug`).
  4. Self-check the body against the scenario checklist; run the `test -f` and `grep` acceptance lines above.

## Milestone 2: Prove fan-out registration and restore dogfood parity
**Goal** — Prove `lifecycle init` fans the new skill out to every configured runtime (and only configured ones), and keep this repo's own `.claude/skills/` copy in sync.
**Deliverables** —
  - Updated `internal/scaffold/skills_test.go`: `TestBuildSkillItems_AllFiveSkillsPerRuntime` item count 15→18 with `lifecycle-new-feature` added to `wantSkills`; `TestBuildSkillItems_SingleRuntime` count 5→6; `TestSkillNamesAndContent` `want` list gains `lifecycle-new-feature` in its sorted slot (between `lifecycle-design` and `lifecycle-plan`).
  - A codex-only routing assertion (extend `TestBuildSkillItems_SingleRuntime` or add a sibling case) proving the intake skill lands at `.agents/skills/lifecycle-new-feature/SKILL.md` and NOT under `.claude/`.
  - `.claude/skills/lifecycle-new-feature/SKILL.md` — dogfood copy, byte-identical to the embedded `skills/` source.
**Validation contract** — checkable acceptance criteria, pre-committed:
  - `go test ./internal/scaffold/... -v` → PASS with the updated assertions (6 skills; 18 fanned-out items across claude-code+cursor+codex; single-runtime 6; `skillNames()` includes `lifecycle-new-feature`) → discharges **"New-feature intake skill is shipped and fanned out"** scenario 1 (claude-code present).
  - The codex-only assertion passes: intake skill present under `.agents/` and absent under `.claude/` → discharges that requirement's scenario 2 (**"intake skill is absent when its runtime is not selected"**).
  - `diff skills/lifecycle-new-feature/SKILL.md .claude/skills/lifecycle-new-feature/SKILL.md` → no output (dogfood parity).
  - `go build ./... && go vet ./... && go test ./...` → all PASS (full suite green); grep the wider test tree for any other hard-coded skill count (e.g. `init_test.go`, `scaffold_test.go`) and update any that assume 5 skills.
  - `/tmp/lifecycle guard` → exit 0 (self-guard unaffected — plan introduces no archive/living-spec change).
**Steps** — ordered breakdown, sized per `planGranularity: medium`:
  1. Update `skills_test.go` counts and name lists (15→18, single-runtime 6, add name in sorted slot).
  2. Add/extend the codex-only routing assertion for `.agents/skills/lifecycle-new-feature/SKILL.md` present + `.claude/` absent.
  3. Copy the embedded skill to `.claude/skills/lifecycle-new-feature/SKILL.md`; confirm `diff` is empty.
  4. Run `go build ./... && go vet ./... && go test ./...`; fix any other test that hard-codes the old skill count; run `/tmp/lifecycle guard`.
