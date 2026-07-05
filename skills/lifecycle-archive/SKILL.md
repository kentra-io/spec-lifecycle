---
name: lifecycle-archive
description: Archive a fully-gated spec-lifecycle change — conflict-check, native fold into the living spec, relocate to changes/archive/, and a post-archive guard run. Invoke explicitly with /lifecycle-archive.
disable-model-invocation: true
---

# lifecycle-archive

**Stub (M6).** This bundle ships with correct frontmatter and a pointer to
the right verbs and docs so `lifecycle init` fans out a real, loadable
skill; the full prompt body lands in M7 (implementation-plan.md §8's
milestone map). Do not treat this file as the finished skill.

## What this does (spec-lifecycle.md §6.2)

Run `lifecycle status --change <change>` first to confirm every required
gate for this change's type is approved (or, for `design`, legitimately
skipped). Then run:

```
lifecycle archive <change>
```

which gate-checks, conflict-checks (refuses loudly if another in-flight
change touches a requirement this change also touches — never a silent
drop), folds the delta into `openspec/specs/<capability>/spec.md` (a
delta-less bug change skips the fold), relocates the folder to
`openspec/changes/archive/<change>/`, appends the ledger record(s), and
runs the full post-archive `lifecycle guard` self-check.

There is no `openspec archive` to call — everything here is native.

## Never

- Never hand-edit anything under `openspec/changes/archive/` — it is the
  append-only event log.
- Never pass `--force-gates`/`--force-conflicts` without the human's
  explicit, informed approval; both are recorded on the resulting ledger
  record(s).
- Never treat a successful archive as done if the post-archive
  `lifecycle guard` self-check reports a problem — surface it to the human.
