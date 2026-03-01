package match

import (
	"strings"
	"testing"
)

func TestCompileRejectsUndefinedPlaceholder(t *testing.T) {
	_, err := Compile([]*Rule{{
		Name: "greet",
		Vars: map[string]Var{"name": {Label: "姓名", Type: "string"}},
		Patterns: []Pattern{{
			Input: "你好 [unknown]",
		}},
	}})
	if err == nil || !strings.Contains(err.Error(), "not defined in vars") {
		t.Fatalf("expected placeholder error, got: %v", err)
	}
}

func TestParseLineTypedArgs(t *testing.T) {
	m := &Matcher{specs: map[string]map[string]Var{
		"r1": {
			"age": {Type: "int"},
			"ok":  {Type: "bool"},
		},
	}}

	r, ok := m.parseLine("r1: age=12, ok=true")
	if !ok {
		t.Fatal("parseLine should return ok")
	}
	if r.Rule != "r1" {
		t.Fatalf("unexpected rule: %s", r.Rule)
	}
	if v, ok := r.Args["age"].Value.(int64); !ok || v != 12 {
		t.Fatalf("unexpected age value: %#v", r.Args["age"])
	}
	if v, ok := r.Args["ok"].Value.(bool); !ok || !v {
		t.Fatalf("unexpected ok value: %#v", r.Args["ok"])
	}
}
