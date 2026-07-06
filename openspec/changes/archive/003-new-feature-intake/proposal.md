---
issue: "kentra-io/spec-lifecycle#3"
designSkip: true
type: feature
---

# Add a `lifecycle-new-feature` intake skill for untracked feature ideas

## Why

Every lifecycle stage skill (`refine`/`design`/`plan`) assumes a
source-tracking issue **and** a change folder already exist: `proposal.md`
requires a non-empty `issue:` ref and is drafted *into*
`openspec/changes/<change>/`. But none of the six `lifecycle` verbs creates
a change folder, and no skill covers the step spec-lifecycle.md §3.1 names —
"GitHub issue, human-initiated" — that turns a raw, untracked idea into a
refine-ready change. Today that idea → issue → folder → refine handoff is
manual and undefined; a fresh session with an untracked idea has no
entrypoint (the exact gap that motivated this change).

## What Changes

- **feature-intake** (new capability): add a `lifecycle-new-feature`
  intake skill — the entrypoint for starting a *feature* that isn't tracked
  anywhere. It (1) clarifies the idea just enough to file a good issue (not
  full requirements — that stays refine's job), (2) creates a GitHub
  source-tracking issue as `type: feature`, but only after the human
  confirms the drafted text, (3) seeds the change folder
  `openspec/changes/<n>-<slug>/` per `lifecycle.yml`'s `changeNaming`,
  recording the issue reference in a stub `proposal.md` frontmatter, then
  (4) STOPS and hands off to `/lifecycle-refine` in a fresh session —
  preserving the "artifact is the interface" discipline (§3.1).
- Feature-scoped: defects remain the domain of `/lifecycle-bug`; the skill
  writes no gate record and no spec delta (intake precedes gate 1).

## Impact

- **New skill directory** `skills/lifecycle-new-feature/SKILL.md`. Skill
  fan-out is data-driven (`embed.go`'s `//go:embed skills`;
  `internal/scaffold/skills.go:skillNames()` reads *every* `skills/<name>/`
  entry), so the new skill auto-registers and `lifecycle init` fans it out
  to each configured runtime — **no `lifecycle` binary changes**.
- **No change** to the delta grammar, fold, archive, gate records, or any of
  the six verbs. Intake sits *above* gate 1 and produces no lifecycle
  records of its own.
- `internal/scaffold/skills_test.go` (and any golden skill-set assertion)
  updates to expect the new skill in the fanned-out set.
- **`designSkip: true` proposed.** This is one new agent-surface skill plus
  a single-capability spec delta: no binary change, no new verb or gate, no
  architecture change to the lifecycle model, no constitution ADR, and it
  does not cross capability boundaries. If you would rather a design pass
  settle the skill's finer contract (issue-body shape, duplicate-issue
  handling, slug derivation, `gh`-unauthenticated fallback), reject the
  skip at gate 1 and `design` will run.
