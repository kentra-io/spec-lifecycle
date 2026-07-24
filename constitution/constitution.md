<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only rule-bearing (## Rule) active ADRs project here; to change a
  rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## architecture

### The archive ledger's monotonic seq is the sole authoritative history order, verified by from-empty replay

The archive ledger's monotonic `seq` is the sole authoritative total order
for archived changes; on-disk folder names/dates MUST NOT be used to derive
order anywhere. `lifecycle guard` MUST support a true from-empty replay
recompute of the fold against the live projection, not only a digest-chain
comparison.

ADR-0003 · 2026-07-05

### Own a native YAML spec format described by a published JSON Schema

The living spec and spec deltas MUST be authored as structured YAML owned
by `spec-lifecycle` and described by a JSON Schema published with the tool;
markdown is a deterministic read-only projection, never hand-authored.
`lifecycle` MUST remain a single static Go binary with no external
language-runtime dependency — YAML parse, schema validation, fold, and
render are all in-process; never shell out to a Node or other language
runtime for them.

ADR-0004 · 2026-07-24

### Prove correctness with golden projection fixtures and from-empty replay

Projection and fold correctness MUST be proven by checked-in golden
fixtures — source YAML paired with its expected markdown projection —
verified byte-identical on every PR, together with `lifecycle guard`'s
from-empty replay. Never reintroduce an OpenSpec conformance corpus or an
external-reference-tool version pin as the compatibility mechanism.

ADR-0005 · 2026-07-24
