<!--
  GENERATED FILE -- projection of the ADR log in constitution/adr/.
  Do not hand-edit; changes will be overwritten by the next "constitution
  regen". Only rule-bearing (## Rule) active ADRs project here; to change a
  rule, add, supersede, or deprecate an ADR instead.
-->

# Constitution

## architecture

### Reimplement the OpenSpec format natively in pure Go — no Node runtime

`lifecycle` MUST remain a single static Go binary with no external
language-runtime dependency. The OpenSpec on-disk format is reimplemented
natively (parse/validate/fold/render); never shell out to an OpenSpec
runtime or any other Node-based tool for this.

ADR-0001 · 2026-07-05

### Prove format compatibility with a static conformance corpus, not a runtime pin

Format compatibility with the OpenSpec on-disk convention MUST be proven
by a checked-in static conformance corpus (real fixtures + expected
fold/render output), verified byte-identical on every PR. Never reintroduce
a runtime version pin as the compatibility mechanism.

ADR-0002 · 2026-07-05

### The archive ledger's monotonic seq is the sole authoritative history order, verified by from-empty replay

The archive ledger's monotonic `seq` is the sole authoritative total order
for archived changes; on-disk folder names/dates MUST NOT be used to derive
order anywhere. `lifecycle guard` MUST support a true from-empty replay
recompute of the fold against the live projection, not only a digest-chain
comparison.

ADR-0003 · 2026-07-05
