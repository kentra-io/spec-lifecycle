---
name: lifecycle-refine
description: Conduct the refine stage of a spec-lifecycle change — proposal.md plus a specs/<capability>/spec.md delta, gated at gate 1. Invoke explicitly with /lifecycle-refine.
disable-model-invocation: true
---

# lifecycle-refine

**Stub (M6).** This bundle ships with correct frontmatter and a pointer to
the right verbs and docs so `lifecycle init` fans out a real, loadable
skill; the full prompt body — the actual refine-stage conversation
discipline — lands in M7 (implementation-plan.md §8's milestone map). Do
not treat this file as the finished skill.

## What this stage produces

- `proposal.md` — Why / What Changes / Impact, with the source-tracking
  issue reference in frontmatter, and (optionally) a proposal to skip the
  `design` stage for small, architecturally inert work.
- `specs/<capability>/spec.md` — one delta per capability this change
  touches (ADDED/MODIFIED/REMOVED/RENAMED Requirements, RFC-2119, with
  GIVEN/WHEN/THEN scenarios).

See spec-lifecycle.md §3.1/§3.2/§4 and §4.1 (NFR routing), and the embedded
`kentra-spec-lifecycle` schema's `proposal`/`specs` artifact instructions
(`openspec/schemas/kentra-spec-lifecycle/schema.yaml` after `lifecycle
init`) for the authoritative content shape.

## Gate mechanics (spec-lifecycle.md §3.3)

1. Draft the artifacts above in a fresh change folder
   (`openspec/changes/<change>/`).
2. Run `lifecycle validate --stage refine` and fix any findings.
3. Surface the artifacts (and any proposed design-skip) to the human.
4. On approval, run `lifecycle approve --stage refine` (add `--design-skip`
   if the human accepted skipping `design`). This verb is mutating —
   never pre-grant it in `allowed-tools`.

## Never

- Never run `lifecycle approve` without the human's explicit approval of
  what you are about to record.
- Never hand-edit `approval-state.json` or ledger files.
