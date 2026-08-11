# Widget intake — Design

## Context

Widget definitions arrive as YAML from an operator-facing CLI. Nothing
validates them today.

## Goals / Non-Goals

**Goals:** reject malformed definitions at the boundary, with a named reason.

**Non-Goals:** no schema migration; no change to how stored widgets are read.

## Decisions

Validation is a pure function over the parsed definition, separate from
storage, so it can be tested without a store. The store stays an interface so
the CLI can be tested against an in-memory implementation.

## Components & Interfaces

| Component | File | Responsibility |
|---|---|---|
| `Definition` | `internal/widget/definition.go` | The parsed widget definition: `Name string`, `Size int`. Data only. |
| `Validate` | `internal/widget/validate.go` | `func Validate(Definition) error` — returns `ErrEmptyName` or `ErrNonPositiveSize`, naming the failing field. Pure. |
| `Store` | `internal/widget/store.go` | `interface { Put(Definition) (string, error); Get(string) (Definition, bool) }` — persistence seam. |
| `MemStore` | `internal/widget/store.go` | In-memory `Store` used by tests and by the CLI's dry-run mode. |
| `Intake` | `internal/widget/intake.go` | `func (Intake) Submit(Definition) (string, error)` — validates, then stores; the one entry point the CLI calls. |

## NFR Discharge

(none declared)

## ADR proposals

(none)

## Risks / Trade-offs

[A second validation rule lands later and is added only to the CLI] → Mitigation:
`Validate` is the only rule site; the CLI never inspects fields itself.
