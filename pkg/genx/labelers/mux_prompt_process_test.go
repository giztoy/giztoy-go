package labelers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/genx/generators"
)

type labelerStub struct {
	model string
	res   *Result
	err   error
}

func (s labelerStub) Process(context.Context, Input) (*Result, error) {
	return s.res, s.err
}

func (s labelerStub) Model() string { return s.model }

type invokeOnlyGenerator struct {
	call *genx.FuncCall
	err  error
}

func (g invokeOnlyGenerator) GenerateStream(context.Context, string, genx.ModelContext) (genx.Stream, error) {
	return nil, errors.New("unused")
}

func (g invokeOnlyGenerator) Invoke(_ context.Context, _ string, _ genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if g.err != nil {
		return genx.Usage{}, nil, g.err
	}
	if g.call != nil {
		return genx.Usage{}, g.call, nil
	}
	return genx.Usage{}, tool.NewFuncCall(`{"matches":[{"label":"topic:恐龙","score":0.98}]}`), nil
}

func TestNewGenXAndProcessWithMux(t *testing.T) {
	gm := generators.NewMux()
	if err := gm.Handle("gx/mock", invokeOnlyGenerator{}); err != nil {
		t.Fatalf("register generator failed: %v", err)
	}

	l := NewGenXWithMux(Config{Generator: "gx/mock"}, gm)
	if l.Model() != "gx/mock" {
		t.Fatalf("unexpected model: %s", l.Model())
	}

	res, err := l.Process(context.Background(), Input{
		Text:       "我喜欢恐龙",
		Candidates: []string{"topic:恐龙", "topic:猫"},
		TopK:       1,
	})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Label != "topic:恐龙" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestGenXProcessEmptyAndInvokeError(t *testing.T) {
	g := NewGenX(Config{Generator: "unused"})
	res, err := g.Process(context.Background(), Input{})
	if err != nil {
		t.Fatalf("empty input should not fail: %v", err)
	}
	if res == nil || res.Matches != nil {
		t.Fatalf("unexpected empty result: %#v", res)
	}

	gm := generators.NewMux()
	if err := gm.Handle("gx/err", invokeOnlyGenerator{err: errors.New("invoke failed")}); err != nil {
		t.Fatalf("register error generator failed: %v", err)
	}

	_, err = NewGenXWithMux(Config{Generator: "gx/err"}, gm).Process(context.Background(), Input{
		Text:       "x",
		Candidates: []string{"a"},
	})
	if err == nil || !strings.Contains(err.Error(), "invoke failed") {
		t.Fatalf("expected invoke error, got: %v", err)
	}
}

func TestBuildPromptAndParseValidateBranches(t *testing.T) {
	prompt := buildPrompt(Input{
		Text:       "query",
		Candidates: []string{"a", "b"},
		Aliases: map[string][]string{
			"a": {"A1", "A2"},
		},
		TopK: 10,
	})
	if !strings.Contains(prompt, "aliases: A1, A2") || !strings.Contains(prompt, "at most 2") {
		t.Fatalf("unexpected prompt: %s", prompt)
	}

	if _, err := parseAndValidate(nil, Input{}); err == nil || !strings.Contains(err.Error(), "no function call") {
		t.Fatalf("expected nil call error, got: %v", err)
	}

	if _, err := parseAndValidate(&genx.FuncCall{Arguments: "{"}, Input{}); err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got: %v", err)
	}

	if _, err := parseAndValidate(&genx.FuncCall{Arguments: `{"matches":[{"label":"","score":0.5}]}`}, Input{Candidates: []string{"x"}}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected empty label error, got: %v", err)
	}

	if _, err := parseAndValidate(&genx.FuncCall{Arguments: `{"matches":[{"label":"y","score":0.5}]}`}, Input{Candidates: []string{"x"}}); err == nil || !strings.Contains(err.Error(), "not in candidates") {
		t.Fatalf("expected candidate mismatch error, got: %v", err)
	}

	r, err := parseAndValidate(&genx.FuncCall{Arguments: `{"matches":[]}`}, Input{Candidates: []string{"x"}})
	if err != nil {
		t.Fatalf("empty matches should succeed: %v", err)
	}
	if r == nil || r.Matches != nil {
		t.Fatalf("unexpected empty matches result: %#v", r)
	}
}

func TestMuxAndDefaultWrappers(t *testing.T) {
	m := NewMux()
	if err := m.Handle("", labelerStub{}); err == nil || !strings.Contains(err.Error(), "empty pattern") {
		t.Fatalf("expected empty pattern error, got: %v", err)
	}

	var nilLabeler Labeler
	if err := m.Handle("x", nilLabeler); err == nil || !strings.Contains(err.Error(), "nil labeler") {
		t.Fatalf("expected nil labeler error, got: %v", err)
	}

	stub := labelerStub{model: "m", res: &Result{Matches: []Match{{Label: "ok", Score: 1}}}}
	if err := m.Handle("ok", stub); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if err := m.Handle("ok", stub); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	if _, err := m.Get(""); err == nil || !strings.Contains(err.Error(), "empty pattern") {
		t.Fatalf("expected empty get error, got: %v", err)
	}
	if _, err := m.Get("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	res, err := m.Process(context.Background(), "ok", Input{})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if len(res.Matches) != 1 || res.Matches[0].Label != "ok" {
		t.Fatalf("unexpected process result: %#v", res)
	}

	old := DefaultMux
	DefaultMux = NewMux()
	t.Cleanup(func() { DefaultMux = old })

	if err := Handle("dflt", stub); err != nil {
		t.Fatalf("default handle failed: %v", err)
	}
	if _, err := Get("dflt"); err != nil {
		t.Fatalf("default get failed: %v", err)
	}
	if _, err := Process(context.Background(), "dflt", Input{}); err != nil {
		t.Fatalf("default process failed: %v", err)
	}
}
