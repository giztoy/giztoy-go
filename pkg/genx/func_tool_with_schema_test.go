package genx

import (
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestWithSchemaOptionApplied(t *testing.T) {
	schema := &jsonschema.Schema{Type: "integer", Description: "custom"}
	tool, err := NewFuncTool[struct {
		N int `json:"n"`
	}]("with_schema", "tool with schema", WithSchema[int](schema))
	if err != nil {
		t.Fatalf("NewFuncTool with schema failed: %v", err)
	}

	if tool.typeSchemas == nil {
		t.Fatal("expected type schema map to be initialized")
	}

	typ := reflect.TypeFor[int]()
	if got := tool.typeSchemas[typ]; got != schema {
		t.Fatalf("expected schema to be registered for int type, got: %#v", got)
	}
}
