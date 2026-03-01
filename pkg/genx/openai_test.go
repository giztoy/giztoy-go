package genx

import (
	"slices"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestFormatOpenAISchemaObjectFieldsBecomeNullableAndRequired(t *testing.T) {
	s := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"name": {Type: "string"},
		},
	}

	out := FormatOpenAISchema(s)
	if out.AdditionalProperties == nil {
		t.Fatal("expected AdditionalProperties to be set")
	}
	if !slices.Contains(out.Required, "name") {
		t.Fatalf("expected required to contain name, got: %#v", out.Required)
	}
	if !slices.Contains(out.Properties["name"].Types, "null") {
		t.Fatalf("expected name to include null type, got: %#v", out.Properties["name"])
	}
}

func TestOpenAIPatchSchemaUsesCustomFormatter(t *testing.T) {
	in := &jsonschema.Schema{Type: "object"}
	g := &OpenAIGenerator{
		SchemaFormatter: func(m *jsonschema.Schema) *jsonschema.Schema {
			m.Description = "patched"
			return m
		},
	}

	out := g.patchSchema(in)
	if out == in {
		t.Fatal("expected patchSchema to clone input schema")
	}
	if out.Description != "patched" {
		t.Fatalf("unexpected description: %q", out.Description)
	}
}
