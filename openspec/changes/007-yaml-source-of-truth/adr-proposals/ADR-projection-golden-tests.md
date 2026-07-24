## Context and Problem Statement

ADR-0002 required proving format compatibility with the OpenSpec on-disk
convention via a checked-in static conformance corpus — real
OpenSpec-format fixtures plus their expected fold/render output, captured
from the reference tool at tag v1.5.0 and verified byte-identical on every
PR. Change #7 drops OpenSpec conformance (see the ADR-0001 supersession), so
there is no OpenSpec grammar left to prove compatibility with, and no reason
to maintain a corpus captured from an external reference tool. Correctness
must still be proven — but now the property under test is *our own* format's
round-trip, not fidelity to someone else's grammar. ADR-0002's rule no
longer describes reality and must be superseded.

## Decision Drivers

- The correctness property has changed: from "matches OpenSpec v1.5.0
  byte-for-byte" to "our YAML → markdown projection is deterministic and our
  fold is replay-consistent".
- Keep the byte-identical-on-every-PR discipline that made ADR-0002 strong;
  only change *what* is compared.
- Avoid an upgrade treadmill: no external reference tool to track.
- Preserve the from-empty replay guard (ADR-0003) as the fold-correctness
  proof — it already exists and is format-agnostic.

## Considered Options

- Keep ADR-0002 (maintain the OpenSpec conformance corpus with no OpenSpec
  left to conform to).
- Supersede with checked-in YAML→markdown golden projection fixtures
  (byte-identical every PR) plus from-empty replay for fold correctness.
- Prove correctness only at runtime via the replay guard, with no golden
  fixtures.

## Decision Outcome

Supersede ADR-0002. Correctness is proven by a checked-in set of golden
fixtures — source YAML paired with its expected markdown projection —
asserted byte-identical on every PR, together with the from-empty replay
(ADR-0003) that recomputes the folded YAML from the archived deltas and
compares it against the live spec. The OpenSpec conformance corpus and any
notion of a reference-tool version pin are removed. Regenerating a golden
fixture is only ever an explicit, deliberate choice (a projection format
change), never an upgrade forced by a moving external dependency —
preserving the anti-treadmill property that made ADR-0002 valuable.

## Rule

Projection and fold correctness MUST be proven by checked-in golden
fixtures — source YAML paired with its expected markdown projection —
verified byte-identical on every PR, together with `lifecycle guard`'s
from-empty replay. Never reintroduce an OpenSpec conformance corpus or an
external-reference-tool version pin as the compatibility mechanism.
