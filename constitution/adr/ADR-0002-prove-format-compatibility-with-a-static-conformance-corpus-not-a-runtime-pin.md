---
id: ADR-0002
title: Prove format compatibility with a static conformance corpus, not a runtime pin
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

Because there is no OpenSpec runtime installed to version-check against,
format compatibility is proven differently: a static corpus of real
OpenSpec-format change folders plus their expected fold/render outputs,
captured once from the reference tool at tag v1.5.0 (implementation-plan.md
§2.8/§9). Every PR asserts the Go parser/fold/render reproduces the corpus
byte-identically. Format drift is only ever an explicit, deliberate choice
to regenerate the corpus against a newer grammar — never an upgrade
treadmill forced by a moving dependency.

## Rule

Format compatibility with the OpenSpec on-disk convention MUST be proven
by a checked-in static conformance corpus (real fixtures + expected
fold/render output), verified byte-identical on every PR. Never reintroduce
a runtime version pin as the compatibility mechanism.
