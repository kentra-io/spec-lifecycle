---
issue: "kentra-io/spec-lifecycle#0"
type: feature
---

# Widget intake — Proposal

## Why

Operators paste widget definitions by hand and typos reach production. Intake
should reject a malformed definition at the boundary instead.

## What Changes

- New capability `widget-intake`: validate a widget definition on submission,
  reject it with a named reason, and store only what validated.

## Impact

A new `internal/widget` package and one new CLI subcommand. No existing
capability changes.
