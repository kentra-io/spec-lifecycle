## ADDED Requirements

### Requirement: YAML is the source of truth for the living spec and spec deltas

The living spec and every per-change spec delta SHALL be authored as
structured YAML — the canonical, hand-edited source of truth — replacing
the markdown OpenSpec delta grammar. No markdown spec artifact is
hand-authored or read as authoritative.

#### Scenario: a spec delta is authored and read as YAML

- **GIVEN** a change that touches a capability's requirements
- **WHEN** its spec delta is authored
- **THEN** the delta is a YAML document (requirements and scenarios as
  structured YAML), and the tool reads that YAML — not any markdown file —
  as the authoritative delta

### Requirement: A JSON Schema describing the spec YAML is published

The tool SHALL publish a JSON Schema that describes the living-spec and
spec-delta YAML, so the YAML can be validated against a declared shape.

#### Scenario: YAML is validated against the published schema

- **GIVEN** a spec-delta YAML document and the published JSON Schema
- **WHEN** the document is validated
- **THEN** a document that conforms passes and one that violates the
  schema is rejected with the offending path

### Requirement: Markdown is a deterministic read-only projection with derived slugs

The tool SHALL render markdown as a deterministic, read-only projection of
the source YAML — regenerated from the YAML, never hand-edited — and the
projection SHALL surface a derived kebab-slug field for every requirement
and every scenario.

#### Scenario: the projection is regenerated and carries slugs

- **GIVEN** a living-spec YAML with a requirement `New feature intake` and a
  scenario `Human confirms the drafted issue`
- **WHEN** the markdown projection is rendered
- **THEN** the same YAML always renders byte-identical markdown, and the
  projection includes derived slugs `new-feature-intake` and
  `human-confirms-the-drafted-issue` for that requirement and scenario

### Requirement: The fold and replay engine operates on the spec YAML

The delta/fold/replay engine SHALL apply change deltas to the living spec
over the structured YAML (keyed by requirement, in the fixed op order), and
`lifecycle guard` SHALL support a true from-empty replay that recomputes the
fold from the archived deltas and compares it against the live YAML
projection.

#### Scenario: from-empty replay confirms the live spec

- **GIVEN** an archive ledger of change deltas and the current living-spec
  YAML
- **WHEN** `lifecycle guard` runs a from-empty replay
- **THEN** it recomputes the folded YAML from the deltas and reports
  agreement with the live spec, or names the divergence if they differ
