package segmentors

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/genx/generators"
)

type segmentorStub struct {
	model string
	res   *Result
	err   error
}

func (s segmentorStub) Process(context.Context, Input) (*Result, error) {
	return s.res, s.err
}

func (s segmentorStub) Model() string { return s.model }

type segmentInvokeGenerator struct {
	call *genx.FuncCall
	err  error
}

func (g segmentInvokeGenerator) GenerateStream(context.Context, string, genx.ModelContext) (genx.Stream, error) {
	return nil, errors.New("unused")
}

func (g segmentInvokeGenerator) Invoke(_ context.Context, _ string, _ genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if g.err != nil {
		return genx.Usage{}, nil, g.err
	}
	if g.call != nil {
		return genx.Usage{}, g.call, nil
	}
	return genx.Usage{}, tool.NewFuncCall(`{
		"segment":{"summary":"小明喜欢恐龙","keywords":["恐龙"],"labels":["person:小明","topic:恐龙"]},
		"entities":[{"label":"person:小明","attrs":[{"key":"age","value":"5"}]}],
		"relations":[{"from":"person:小明","to":"topic:恐龙","rel_type":"likes"}]
	}`), nil
}

func TestGenXProcessAndPromptBuilders(t *testing.T) {
	gm := generators.NewMux()
	if err := gm.Handle("gx/mock", segmentInvokeGenerator{}); err != nil {
		t.Fatalf("register generator failed: %v", err)
	}

	g := NewGenXWithMux(Config{Generator: "gx/mock"}, gm)
	if g.Model() != "gx/mock" {
		t.Fatalf("unexpected model: %s", g.Model())
	}

	in := Input{Messages: []string{"小明喜欢恐龙"}, Schema: &Schema{EntityTypes: map[string]EntitySchema{
		"person": {Desc: "人物", Attrs: map[string]AttrDef{"age": {Type: "int", Desc: "年龄"}}},
	}}}

	mctx := g.buildModelContext(in)
	var prompts []*genx.Prompt
	for p := range mctx.Prompts() {
		prompts = append(prompts, p)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0].Text, "conversation segmentor") {
		t.Fatalf("unexpected prompt in context: %#v", prompts)
	}

	res, err := g.Process(context.Background(), in)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if res.Segment.Summary != "小明喜欢恐龙" || res.Entities[0].Attrs["age"] != "5" {
		t.Fatalf("unexpected result: %#v", res)
	}

	prompt := buildPrompt(in)
	if !strings.Contains(prompt, "Entity Schema Hint") {
		t.Fatalf("prompt missing schema hint: %s", prompt)
	}

	if got := buildConversationText([]string{"a", "b"}); got != "a\nb" {
		t.Fatalf("unexpected conversation text: %q", got)
	}

	hint := buildSchemaHint(in.Schema)
	if !strings.Contains(hint, "### person") || !strings.Contains(hint, "Attributes") {
		t.Fatalf("unexpected schema hint: %s", hint)
	}
}

func TestGenXProcessInvokeAndParseError(t *testing.T) {
	gm := generators.NewMux()
	if err := gm.Handle("gx/err", segmentInvokeGenerator{err: errors.New("invoke err")}); err != nil {
		t.Fatalf("register err generator failed: %v", err)
	}

	_, err := NewGenXWithMux(Config{Generator: "gx/err"}, gm).Process(context.Background(), Input{Messages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "invoke failed") {
		t.Fatalf("expected invoke error, got: %v", err)
	}

	gm2 := generators.NewMux()
	if err := gm2.Handle("gx/bad", segmentInvokeGenerator{call: &genx.FuncCall{Arguments: "{"}}); err != nil {
		t.Fatalf("register bad generator failed: %v", err)
	}

	_, err = NewGenXWithMux(Config{Generator: "gx/bad"}, gm2).Process(context.Background(), Input{Messages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestMuxAndDefaultWrappers(t *testing.T) {
	m := NewMux()
	stub := segmentorStub{model: "m", res: &Result{Segment: SegmentOutput{Summary: "ok"}}}

	if err := m.Handle("gx", stub); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	if err := m.Handle("gx", stub); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	if _, err := m.Get("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	res, err := m.Process(context.Background(), "gx", Input{})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if res.Segment.Summary != "ok" {
		t.Fatalf("unexpected result: %#v", res)
	}

	old := DefaultMux
	DefaultMux = NewMux()
	t.Cleanup(func() { DefaultMux = old })

	if err := Handle("default", stub); err != nil {
		t.Fatalf("default handle failed: %v", err)
	}
	if _, err := Get("default"); err != nil {
		t.Fatalf("default get failed: %v", err)
	}
	if _, err := Process(context.Background(), "default", Input{}); err != nil {
		t.Fatalf("default process failed: %v", err)
	}
}
