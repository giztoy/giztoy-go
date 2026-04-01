package profilers

import (
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
)

func TestParseResultSuccess(t *testing.T) {
	g := &GenX{}
	call := &genx.FuncCall{Arguments: `{
		"schema_changes":[{"entity_type":"person","field":"age","def":{"type":"int","desc":"年龄"},"action":"add"}],
		"profile_updates":{"person:小明":{"age":5}},
		"relations":[{"from":"person:小明","to":"topic:恐龙","rel_type":"likes"}]
	}`}

	r, err := g.parseResult(call)
	if err != nil {
		t.Fatalf("parseResult failed: %v", err)
	}
	if len(r.SchemaChanges) != 1 || len(r.ProfileUpdates) != 1 || len(r.Relations) != 1 {
		t.Fatalf("unexpected parsed result: %#v", r)
	}
}

func TestParseResultRejectsNilCall(t *testing.T) {
	g := &GenX{}
	_, err := g.parseResult(nil)
	if err == nil || !strings.Contains(err.Error(), "no function call") {
		t.Fatalf("expected nil call error, got: %v", err)
	}
}
