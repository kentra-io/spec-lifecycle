---
id: ADR-0001
title: Reimplement the OpenSpec format natively in pure Go — no Node runtime
category: architecture
date: 2026-07-05
status: accepted
---

## Context and Problem Statement

Established at project bootstrap by `constitution init`.

## Considered Options

- Adopt this founding principle
- Leave the convention implicit

## Decision Outcome

`spec-lifecycle` conforms to the OpenSpec on-disk format (directory layout,
delta grammar, fold semantics, pinned to the v1.5.0 grammar) but does not
depend on `@fission-ai/openspec` or any Node runtime to do it. The parser,
validator, fold, and renderer are all owned, in-process, in Go
(implementation-plan.md §0.5/§2.3/§2.4, Option B). This reverses an earlier
design that treated OpenSpec-the-tool as a required runtime `lifecycle`
shells out to; that runtime turned out to have no safe programmatic fold
entrypoint, an `[experimental]` custom-schema loader, and open silent-data-
loss bugs (#1246). Owning the engine removes all of that as an external
risk and keeps `lifecycle` a single static binary with no language-runtime
dependency, matching the companion `adr-sourced-constitution` primitive's
posture.

## Rule

`lifecycle` MUST remain a single static Go binary with no external
language-runtime dependency. The OpenSpec on-disk format is reimplemented
natively (parse/validate/fold/render); never shell out to an OpenSpec
runtime or any other Node-based tool for this.
