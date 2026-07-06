package scaffold

import (
	"bytes"
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/atomicwrite"
)

// EnsureProjectConfig implements `lifecycle init`'s openspec/config.yaml
// wiring (implementation-plan.md §2.9 steps 3/4, spec-lifecycle.md §7.1):
//
//   - The top-level `schema:` key is checked/set to schemaName on every
//     call (idempotent — setting the same value twice writes the same
//     bytes) — the descriptor `lifecycle` owns and every re-init keeps
//     pointed at kentra-spec-lifecycle, mirroring OpenSpec's own
//     `openspec/config.yaml` shape (project-config.ts's ProjectConfigSchema:
//     `schema: string`).
//   - The top-level `context:` key is seeded with contextText ONLY when
//     absent — once written, it is exactly the kind of free-text,
//     human-owned field a re-init must never clobber (a stronger version of
//     the same "don't clobber user edits below it" principle the plan
//     states for `schema:`).
//
// Both edits are surgical: performed on the parsed yaml.Node tree, not a
// full struct round-trip, so every OTHER top-level key (`rules:`,
// `references:`, `store:`, or anything a future OpenSpec config field adds)
// and every comment attached to a node survives byte-for-byte. This is the
// documented constraint: go.yaml.in/yaml/v3's Node encoder preserves
// Head/Line/FootComment text attached to nodes it already parsed, but it is
// NOT a lossless byte-for-byte YAML formatter — blank-line spacing and
// scalar quoting style on nodes we don't touch may still be
// re-serialized (a known, accepted limitation of every YAML-node-based
// editor, not just this one). An absent or empty config.yaml is created
// fresh as a single-mapping document.
//
// Returns changed=true iff the file's bytes were rewritten (an absent file
// counts as a change). A malformed existing file (not valid YAML, or a
// non-mapping root) is a hard error — surgical editing cannot proceed
// without knowing where the mapping is.
func EnsureProjectConfig(path, schemaName, contextText string) (changed bool, err error) {
	before, existed, err := readFileIfExists(path)
	if err != nil {
		return false, err
	}

	doc, err := decodeOrEmptyMapping(path, before)
	if err != nil {
		return false, err
	}
	mapping := doc.Content[0]

	setSchema := setMappingScalar(mapping, "schema", schemaName, true)
	setContext := setMappingScalar(mapping, "context", contextText, false)

	out, merr := yaml.Marshal(doc)
	if merr != nil {
		return false, fmt.Errorf("%s: %w", path, merr)
	}

	if existed && !setSchema && !setContext && bytes.Equal(before, out) {
		return false, nil
	}
	if err := atomicwrite.WriteFile(path, out, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// decodeOrEmptyMapping parses data as a YAML document, treating an absent
// or blank file as a fresh single-mapping document ready to receive keys.
// The root must be (or become) a mapping — any other shape is refused,
// since there would be nowhere well-defined to add `schema:`/`context:`.
func decodeOrEmptyMapping(path string, data []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}},
		}, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: not valid YAML: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return nil, fmt.Errorf("%s: could not locate a single YAML document root", path)
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top level must be a YAML mapping (got %s)", path, kindName(doc.Content[0].Kind))
	}
	return &doc, nil
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.SequenceNode:
		return "a sequence"
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.AliasNode:
		return "an alias"
	default:
		return "an unrecognized node"
	}
}

// setMappingScalar sets key's value to value inside mapping (a
// yaml.MappingNode). When overwrite is false and the key already exists,
// it is left untouched (returns false). When overwrite is true, an
// existing scalar with the exact same value is left untouched too (no
// spurious rewrite). A multi-line value is rendered with block literal
// style (`|`) so it reads as prose, not an escaped one-liner. Returns
// whether the mapping's content actually changed.
func setMappingScalar(mapping *yaml.Node, key, value string, overwrite bool) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		existing := mapping.Content[i+1]
		if !overwrite {
			return false
		}
		if existing.Kind == yaml.ScalarNode && existing.Value == value {
			return false
		}
		replacement := scalarNode(value)
		existing.Kind = replacement.Kind
		existing.Tag = replacement.Tag
		existing.Value = replacement.Value
		existing.Style = replacement.Style
		existing.Content = replacement.Content
		existing.Anchor = replacement.Anchor
		existing.Alias = replacement.Alias
		// Head/Line/FootComment are deliberately left untouched:
		// overwriting a scalar's value must not silently drop a
		// user-authored comment attached to it.
		return true
	}
	mapping.Content = append(mapping.Content, plainScalarNode(key), scalarNode(value))
	return true
}

func plainScalarNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func scalarNode(s string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	if strings.Contains(s, "\n") {
		n.Style = yaml.LiteralStyle
	}
	return n
}
