package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	yaml "go.yaml.in/yaml/v3"

	"github.com/kentra-io/spec-lifecycle/internal/schema"
)

// Canonical `$id` URLs of the two published JSON Schemas (must match the
// `$id` fields in the embedded schema files, and the cross-file `$ref` the
// delta schema uses to reach the shared requirement/scenario sub-shape —
// design D4). The compiler registers each embedded schema under these URLs
// so `$ref` resolves in-process, with no network fetch.
const (
	livingSpecSchemaURL = "https://schemas.kentra.io/spec-lifecycle/living-spec.schema.json"
	specDeltaSchemaURL  = "https://schemas.kentra.io/spec-lifecycle/spec-delta.schema.json"
)

// LivingSpecSchema returns the compiled living-spec JSON Schema (draft
// 2020-12), compiled in-process from the embedded, published schema bytes —
// no external process, no language runtime (constitution ADR-0004).
func LivingSpecSchema() (*jsonschema.Schema, error) {
	living, _, err := compileSchemas()
	return living, err
}

// SpecDeltaSchema returns the compiled spec-delta JSON Schema (draft
// 2020-12), compiled in-process from the embedded, published schema bytes.
func SpecDeltaSchema() (*jsonschema.Schema, error) {
	_, delta, err := compileSchemas()
	return delta, err
}

// compileSchemas registers both published schemas with one compiler (so the
// delta schema's cross-file `$ref` into the living-spec schema's shared
// sub-shape resolves) and compiles both. Both are recompiled per call; that
// is cheap relative to gate I/O and keeps the package free of process-global
// mutable state.
func compileSchemas() (living, delta *jsonschema.Schema, err error) {
	c := jsonschema.NewCompiler()
	resources := []struct{ url, name string }{
		{livingSpecSchemaURL, schema.LivingSpecSchemaName},
		{specDeltaSchemaURL, schema.SpecDeltaSchemaName},
	}
	for _, r := range resources {
		raw, e := schema.PublishedSchema(r.name)
		if e != nil {
			return nil, nil, fmt.Errorf("spec: reading embedded schema %s: %w", r.name, e)
		}
		doc, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if e != nil {
			return nil, nil, fmt.Errorf("spec: parsing embedded schema %s: %w", r.name, e)
		}
		if e := c.AddResource(r.url, doc); e != nil {
			return nil, nil, fmt.Errorf("spec: registering schema %s: %w", r.name, e)
		}
	}
	if living, err = c.Compile(livingSpecSchemaURL); err != nil {
		return nil, nil, fmt.Errorf("spec: compiling living-spec schema: %w", err)
	}
	if delta, err = c.Compile(specDeltaSchemaURL); err != nil {
		return nil, nil, fmt.Errorf("spec: compiling spec-delta schema: %w", err)
	}
	return living, delta, nil
}

// ValidateYAML validates an already-decoded YAML document against a compiled
// published JSON Schema. It returns nil when the document conforms, and an
// error naming the offending JSON path (e.g. `/deltas/0/op`) when it does
// not — so a schema violation points the author at the exact node. Obtain
// sch from LivingSpecSchema or SpecDeltaSchema.
//
// doc is the value produced by yaml.Unmarshal into an `any`/map — the
// decoded document, not raw bytes. It is normalized through JSON so YAML's
// scalar types line up with the JSON data model the validator expects.
func ValidateYAML(doc any, sch *jsonschema.Schema) error {
	norm, err := toJSONValue(doc)
	if err != nil {
		return err
	}
	if err := sch.Validate(norm); err != nil {
		return asPathError(err)
	}
	return nil
}

// ValidateYAMLBytes decodes raw YAML bytes and validates the result against
// sch. It is the convenience entry point for callers that hold the file
// bytes (a malformed-YAML input is reported as a decode error, distinct from
// a schema violation).
func ValidateYAMLBytes(data []byte, sch *jsonschema.Schema) error {
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("spec: not valid YAML: %w", err)
	}
	return ValidateYAML(doc, sch)
}

// toJSONValue round-trips a YAML-decoded value through encoding/json into the
// value shape github.com/santhosh-tekuri/jsonschema expects (map[string]any,
// []any, float64/json.Number, string, bool, nil).
func toJSONValue(doc any) (any, error) {
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("spec: normalizing decoded document: %w", err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("spec: normalizing decoded document: %w", err)
	}
	return v, nil
}

// asPathError prefixes a jsonschema validation failure with the JSON path of
// the deepest offending instance node, so the returned error names the
// offending path up front regardless of the library's default multi-line
// formatting (which is preserved via %w). Non-validation errors pass through
// unchanged.
func asPathError(err error) error {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	return fmt.Errorf("schema validation failed at %q: %w", leafInstanceLocation(ve), err)
}

// leafInstanceLocation walks to the deepest cause and renders its instance
// location as a JSON Pointer (e.g. "/deltas/0/op", or "/" for the document
// root).
func leafInstanceLocation(ve *jsonschema.ValidationError) string {
	cur := ve
	for len(cur.Causes) > 0 {
		cur = cur.Causes[0]
	}
	if len(cur.InstanceLocation) == 0 {
		return "/"
	}
	return "/" + strings.Join(cur.InstanceLocation, "/")
}
