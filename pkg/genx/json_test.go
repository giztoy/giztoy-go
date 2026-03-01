package genx

import (
	"regexp"
	"testing"
)

func TestUnmarshalJSONRepairsSyntaxError(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var out payload
	if err := unmarshalJSON([]byte(`{"name":"gx",}`), &out); err != nil {
		t.Fatalf("unmarshalJSON failed: %v", err)
	}
	if out.Name != "gx" {
		t.Fatalf("unexpected parsed value: %#v", out)
	}
}

func TestHexStringFormat(t *testing.T) {
	s := hexString()
	if len(s) != 16 {
		t.Fatalf("expected len 16, got %d", len(s))
	}
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(s) {
		t.Fatalf("hexString returned invalid format: %q", s)
	}
}

func TestUnmarshalJSONReturnsNonSyntaxErrorDirectly(t *testing.T) {
	var out struct {
		N int `json:"n"`
	}

	err := unmarshalJSON([]byte(`{"n":"x"}`), &out)
	if err == nil {
		t.Fatal("expected unmarshal type error")
	}
}
