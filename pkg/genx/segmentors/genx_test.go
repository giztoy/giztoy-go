package segmentors

import (
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
)

func TestParseResultConvertsAttrsArray(t *testing.T) {
	g := &GenX{}
	call := &genx.FuncCall{Arguments: `{
		"segment":{"summary":"s","keywords":["k"],"labels":["person:小明"]},
		"entities":[{"label":"person:小明","attrs":[{"key":"age","value":"5"}]}],
		"relations":[{"from":"person:小明","to":"topic:恐龙","rel_type":"likes"}]
	}`}

	r, err := g.parseResult(call)
	if err != nil {
		t.Fatalf("parseResult failed: %v", err)
	}
	if len(r.Entities) != 1 || r.Entities[0].Attrs["age"] != "5" {
		t.Fatalf("unexpected entity attrs: %#v", r.Entities)
	}
}

func TestParseResultRejectsNilCall(t *testing.T) {
	g := &GenX{}
	_, err := g.parseResult(nil)
	if err == nil || !strings.Contains(err.Error(), "no function call") {
		t.Fatalf("expected nil call error, got: %v", err)
	}
}
