## Context and Problem Statement

ADR-0001 committed `spec-lifecycle` to conforming to the OpenSpec on-disk
format (directory layout, markdown delta grammar, fold semantics pinned to
the v1.5.0 grammar), reimplemented natively in Go. Change #7 drops OpenSpec
conformance entirely and adopts **primitive-family consistency** instead:
the living spec and spec deltas become structured **YAML** we own end to
end, described by a **published JSON Schema**, with markdown demoted to a
deterministic read-only projection — the same YAML-source / schema /
rendered-projection shape the sibling primitives (`milestoned-plan-dag`,
`adr-sourced-constitution`) already use. ADR-0001's format-conformance
decision is therefore no longer the rule; but its *other* commitment — that
`lifecycle` stays a single static Go binary with no external language
runtime — must survive intact. Because an accepted ADR body is frozen, the
only way to change the format decision while keeping the runtime constraint
is to supersede ADR-0001 with a new rule that states both.

## Decision Drivers

- Primitive-family consistency: one YAML-source / JSON-Schema /
  markdown-projection shape across every primitive in the family.
- End-to-end ownership of the format (no OpenSpec grammar to track).
- The single-static-binary / no-language-runtime posture is non-negotiable
  and must carry forward (shared with `adr-sourced-constitution`).
- Deliberate, one-way trade of OpenSpec interop for an owned format.

## Considered Options

- Keep ADR-0001 (stay conformed to the OpenSpec on-disk format).
- Supersede ADR-0001: own a native YAML format described by a published
  JSON Schema, keeping the single-static-binary / no-runtime constraint.
- Own the YAML format *and* drop the no-runtime constraint (allow an
  embedded scripting/Node validator).

## Decision Outcome

Supersede ADR-0001. `spec-lifecycle` owns a native YAML format for the
living spec and spec deltas, described by a JSON Schema published with the
tool; markdown becomes a deterministic, read-only projection of that YAML.
OpenSpec on-disk conformance is dropped (a deliberate, one-way trade made in
change #7). The single-static-Go-binary / no-external-language-runtime
constraint is retained: YAML parsing, JSON-Schema validation, fold, and
render are all owned and in-process (an embedded Go JSON-Schema validator,
never a shelled-out Node/other runtime). The `openspec/` directory name is
kept for now (reference cleanup tracked in #6). ADR-0003 (archive `seq`
order + from-empty replay) is unaffected and survives unchanged, retargeted
onto the YAML.

## Rule

The living spec and spec deltas MUST be authored as structured YAML owned
by `spec-lifecycle` and described by a JSON Schema published with the tool;
markdown is a deterministic read-only projection, never hand-authored.
`lifecycle` MUST remain a single static Go binary with no external
language-runtime dependency — YAML parse, schema validation, fold, and
render are all in-process; never shell out to a Node or other language
runtime for them.
