---
name: lifecycle-init
description: Bootstraps spec-lifecycle in a project — installs the `lifecycle` CLI if missing, elicits the few seeding choices (runtimes, source tracking), runs the idempotent `lifecycle init` compose, and orients the human to the stage flow. Invoke explicitly with /lifecycle-init in a repo that should adopt staged, gated, spec-driven planning.
disable-model-invocation: true
---

# lifecycle-init

Bootstrap this repository for **spec-lifecycle**: a staged, human-gated,
spec-driven issue lifecycle (refine → design → plan → archive) over an
owned YAML spec format (living specs and deltas are structured YAML;
markdown is a read-only projection). The heavy lifting is one idempotent
CLI command —
your job is to install the CLI if needed, ask the two or three seeding
questions that only the human can answer, run the compose, and explain
what got installed.

First read the state of the repo. If `lifecycle.yml` already exists, this
project is initialized: tell the human what is already there and ask whether
they want a refresh before running anything. A re-run is safe — it refreshes
scaffolding and skills and leaves `lifecycle.yml`, in-flight changes, gate
records, and the archive ledger untouched — so the reason to ask is consent,
not risk.

## Ensure the `lifecycle` CLI is available

Everything below shells out to `lifecycle`. If it is not on PATH, install
the prebuilt release — do **not** build from source or install a Go
toolchain just for this:

```
brew install kentra-io/tap/lifecycle
```

(No Homebrew? Grab the platform archive from
https://github.com/kentra-io/spec-lifecycle/releases and put `lifecycle`
on PATH.)

## Elicit the seeding choices

Ask the human — do not guess (these are written once into a fresh
`lifecycle.yml` and ignored on re-runs):

1. **Runtimes** — which agent trees should receive the Layer-2 stage
   skills: `claude-code`, `cursor`, `codex` (default: all three).
   `--runtimes` is repeatable.
2. **Source tracking** — where changes are sourced from:
   `--source-type github-issue --source-repo <owner>/<repo>` for the
   common case; `generic`, `jira`, or `none` otherwise.

## Run the compose

From the directory that should become the project root:

```
lifecycle init --runtimes claude-code --source-type github-issue --source-repo <owner>/<repo>
```

Every step is independently idempotent. It scaffolds `openspec/`
(changes/specs), installs the `kentra-spec-lifecycle` schema descriptor,
seeds `openspec/config.yaml` + `lifecycle.yml`, preflights the companion
`constitution` binary (a missing one is a WARNING here — it only blocks
later at gates 2/3), fans the stage skills out to the configured runtime
trees, and writes managed pointer blocks into CLAUDE.md / AGENTS.md. If
`init` refuses over a drifted pointer block, show the human the diff and
let them decide about `--force` — never force on your own.

## Orient the human

After a clean run, confirm with `lifecycle status` (empty is correct on a
fresh project) and hand over the flow:

- `/lifecycle-new-feature` — intake for a new feature idea (issue + stub).
- `/lifecycle-bug` — repro-first intake for a defect in specced behavior.
- `/lifecycle-refine`, `/lifecycle-design` — the staged artifacts, each
  gated by `lifecycle approve` (never hand-edit `approval-state.json`).
  The plan stage's `plan.yaml` is authored with `milestoned-plan-dag`'s
  `/plan-author` skill and gated the same way.
- `/lifecycle-archive` — folds an implemented change into the living spec.

## Never

- Never run `lifecycle init --force` without the human's explicit go-ahead.
- Never hand-create any file under `openspec/` or `lifecycle.yml` yourself —
  the CLI owns that tree end to end.
- Never proceed past a failed install or a refused compose by improvising —
  surface the error and stop.
