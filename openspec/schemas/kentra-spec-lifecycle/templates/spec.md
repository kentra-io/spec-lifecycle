# One capability delta per file, authored as structured YAML:
#   openspec/changes/<change>/specs/<capability>/spec.yaml
#
# The YAML is the hand-edited source of truth (change 007, design D2). The
# markdown spec.md under openspec/specs/<capability>/ is a deterministic,
# read-only projection regenerated from the folded YAML — never hand-edited.
#
# Functional and non-functional requirements are merged here
# (spec-lifecycle.md §4.1): a measurable, behavior-observable NFR is an
# ordinary requirement below, not a separate document.
#
# `deltas:` is a sequence of op-tagged entries applied keyed by requirement
# name in the fixed op order RENAMED -> REMOVED -> MODIFIED -> ADDED. This
# file demonstrates the ADDED case; an entry may instead be:
#   - op: MODIFIED — a `requirement:` mapping carrying the requirement's
#     ENTIRE intended post-change shape (the fold replaces, not merges —
#     spec-lifecycle.md §6.1).
#   - op: REMOVED — a `requirement:` mapping with only its `name:`.
#   - op: RENAMED — `from:`/`to:` naming the old and new requirement, and
#     no `requirement:`.

capability: <capability>
deltas:
  - op: ADDED
    requirement:
      name: <requirement name>
      text: |
        The system SHALL <behavior>.
      scenarios:
        - name: <scenario name>
          given:
            - <precondition>
          when:
            - <action>
          then:
            - <outcome>
