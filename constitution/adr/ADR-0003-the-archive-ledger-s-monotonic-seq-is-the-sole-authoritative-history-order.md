---
id: ADR-0003
title: The archive ledger's monotonic seq is the sole authoritative history order, verified by from-empty replay
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

`openspec/ledger.jsonl` records one append-only entry per archived
capability fold, each carrying a monotonic `seq`, pre/post-image digests,
and an archive-manifest digest (implementation-plan.md §2.4/§2.5). On-disk
folder-name date prefixes are cosmetic and never read back for ordering —
OpenSpec itself has no native archive total-order (#409/#1192), so
`lifecycle` supplies one. `lifecycle guard` treats the ledger's `seq` order
as ground truth and, because the fold is owned in-process, can recompute
`fold(all archived deltas, in seq order, from empty)` and diff it against
the live projection — a true from-empty replay, not just a digest
comparison, as the gold-standard fidelity check (spec-lifecycle.md §6.3).

## Rule

The archive ledger's monotonic `seq` is the sole authoritative total order
for archived changes; on-disk folder names/dates MUST NOT be used to derive
order anywhere. `lifecycle guard` MUST support a true from-empty replay
recompute of the fold against the live projection, not only a digest-chain
comparison.
