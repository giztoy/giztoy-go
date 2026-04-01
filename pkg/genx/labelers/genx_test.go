package labelers

import (
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
)

func TestParseAndValidateTopKAndDedup(t *testing.T) {
	call := &genx.FuncCall{Arguments: `{"matches":[{"label":"a","score":0.9},{"label":"a","score":0.8},{"label":"b","score":0.7},{"label":"c","score":0.6}]}`}
	input := Input{Candidates: []string{"a", "b", "c"}, TopK: 2}

	r, err := parseAndValidate(call, input)
	if err != nil {
		t.Fatalf("parseAndValidate failed: %v", err)
	}
	if len(r.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(r.Matches))
	}
	if r.Matches[0].Label != "a" || r.Matches[1].Label != "b" {
		t.Fatalf("unexpected matches: %#v", r.Matches)
	}
}

func TestParseAndValidateRejectsOutOfRangeScore(t *testing.T) {
	call := &genx.FuncCall{Arguments: `{"matches":[{"label":"a","score":1.5}]}`}
	input := Input{Candidates: []string{"a"}}

	_, err := parseAndValidate(call, input)
	if err == nil || !strings.Contains(err.Error(), "invalid score") {
		t.Fatalf("expected invalid score error, got: %v", err)
	}
}
