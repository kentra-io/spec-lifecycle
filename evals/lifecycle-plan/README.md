# /lifecycle-plan eval

On demand, not CI: it costs tokens and is not deterministic. Run it after any
change to `skills/lifecycle-plan/SKILL.md` or to `milestoned-plan-dag`'s
`plan-author`.

## Setup

Copy `fixture/` into a scratch repo as `openspec/changes/000-widget-intake/`.
The scratch repo needs `lifecycle init` to have run, and `milestoned-plan-dag`
on PATH — it ships no cask, so build it from its checkout:

```
go build -o <somewhere on PATH>/milestoned-plan-dag ./cmd/milestoned-plan-dag
```

The skill also runs `/plan-gate`, which belongs to `adr-sourced-constitution`,
so the scratch repo needs `constitution init` and at least one rule-bearing ADR
— otherwise the agent reaches gate 3 with nothing to grade against.

## Run

Give a **fresh** agent the `/lifecycle-plan` skill and the change folder, and
nothing else:

> Run /lifecycle-plan for the change `000-widget-intake`.

Stop it before it runs `lifecycle approve` — the eval grades the drafted
`plan.yaml`, not the gate.

## Pass conditions

1. From the **scratch** repo's root,
   `milestoned-plan-dag validate openspec/changes/000-widget-intake/plan.yaml`
   exits 0.
2. Every scenario in the delta is discharged by some milestone criterion. Run
   this one from **this** repo's root. Both paths must be absolute: `go test
   ./evals/` runs the binary with `evals/` as its cwd, and the plan lives in
   the scratch repo while the delta lives here, so the two share no root.
   ```
   EVAL_PLAN=<absolute path to the scratch repo's plan.yaml> \
   EVAL_DELTA=$PWD/evals/lifecycle-plan/fixture/specs/widget-intake/spec.yaml \
   go test ./evals/ -run TestScenarioCoverage -v
   ```
3. A **second** fresh agent, handed exactly one milestone plus the repository,
   states what it will do **without asking a question**. Binary, not a quality
   score: it asked, or it did not.

Record each run's outcome below with the date and the skill's commit.

## Runs

| Date | Skill commit | 1. valid | 2. coverage | 3. no questions |
|---|---|---|---|---|
