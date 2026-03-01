package match

import "testing"

func TestParseRuleYAML(t *testing.T) {
	data := []byte(`
name: test_rule
vars:
  who:
    label: 人名
    type: string
patterns:
  - "你好 [who]"
  - ["早上好 [who]", "test_rule: who=[人名]"]
examples:
  - ["test_rule"]
`)

	r, err := ParseRuleYAML(data)
	if err != nil {
		t.Fatalf("ParseRuleYAML failed: %v", err)
	}
	if r.Name != "test_rule" {
		t.Fatalf("unexpected rule name: %s", r.Name)
	}
	if len(r.Patterns) != 2 || len(r.Examples) != 1 {
		t.Fatalf("unexpected parsed rule: %#v", r)
	}
}
