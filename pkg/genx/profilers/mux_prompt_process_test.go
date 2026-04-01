package profilers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/genx/generators"
	"github.com/giztoy/giztoy-go/pkg/genx/segmentors"
)

type profilerStub struct {
	model string
	res   *Result
	err   error
}

func (s profilerStub) Process(context.Context, Input) (*Result, error) {
	return s.res, s.err
}

func (s profilerStub) Model() string { return s.model }

type profilerInvokeGenerator struct {
	call *genx.FuncCall
	err  error
}

func (g profilerInvokeGenerator) GenerateStream(context.Context, string, genx.ModelContext) (genx.Stream, error) {
	return nil, errors.New("unused")
}

func (g profilerInvokeGenerator) Invoke(_ context.Context, _ string, _ genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if g.err != nil {
		return genx.Usage{}, nil, g.err
	}
	if g.call != nil {
		return genx.Usage{}, g.call, nil
	}
	return genx.Usage{}, tool.NewFuncCall(`{"schema_changes":[],"profile_updates":{"person:小明":{"age":5}},"relations":[]}`), nil
}

func TestGenXProcessAndContextBuild(t *testing.T) {
	gm := generators.NewMux()
	if err := gm.Handle("gx/mock", profilerInvokeGenerator{}); err != nil {
		t.Fatalf("register generator failed: %v", err)
	}

	g := NewGenXWithMux(Config{Generator: "gx/mock"}, gm)
	if g.Model() != "gx/mock" {
		t.Fatalf("unexpected model: %s", g.Model())
	}

	in := Input{Messages: []string{"小明喜欢恐龙"}}
	mctx := g.buildModelContext(in)

	var prompts []*genx.Prompt
	for p := range mctx.Prompts() {
		prompts = append(prompts, p)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0].Text, "entity profile analyst") {
		t.Fatalf("unexpected prompt context: %#v", prompts)
	}

	var msgs []*genx.Message
	for m := range mctx.Messages() {
		msgs = append(msgs, m)
	}
	if len(msgs) != 1 {
		t.Fatalf("unexpected messages in model context: %#v", msgs)
	}

	res, err := g.Process(context.Background(), in)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if res.ProfileUpdates["person:小明"]["age"] != float64(5) {
		t.Fatalf("unexpected profile updates: %#v", res.ProfileUpdates)
	}
}

func TestGenXProcessInvokeErrorAndParseError(t *testing.T) {
	gm := generators.NewMux()
	if err := gm.Handle("gx/err", profilerInvokeGenerator{err: errors.New("boom")}); err != nil {
		t.Fatalf("register err generator failed: %v", err)
	}

	_, err := NewGenXWithMux(Config{Generator: "gx/err"}, gm).Process(context.Background(), Input{Messages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "invoke failed") {
		t.Fatalf("expected invoke error, got: %v", err)
	}

	gm2 := generators.NewMux()
	if err := gm2.Handle("gx/bad", profilerInvokeGenerator{call: &genx.FuncCall{Arguments: "{"}}); err != nil {
		t.Fatalf("register bad generator failed: %v", err)
	}
	_, err = NewGenXWithMux(Config{Generator: "gx/bad"}, gm2).Process(context.Background(), Input{Messages: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestPromptBuilders(t *testing.T) {
	extracted := &segmentors.Result{
		Segment: segmentors.SegmentOutput{Summary: "summary"},
		Entities: []segmentors.EntityOutput{{
			Label: "person:小明",
			Attrs: map[string]any{"age": 5},
		}},
		Relations: []segmentors.RelationOutput{{From: "person:小明", To: "topic:恐龙", RelType: "likes"}},
	}

	prompt := buildPrompt(Input{
		Messages:  []string{"hello", "world"},
		Extracted: extracted,
		Schema: &segmentors.Schema{EntityTypes: map[string]segmentors.EntitySchema{
			"person": {Desc: "人物", Attrs: map[string]segmentors.AttrDef{"age": {Type: "int", Desc: "年龄"}}},
		}},
		Profiles: map[string]map[string]any{
			"person:小明":  {"age": 5},
			"person:坏数据": {"bad": make(chan int)},
		},
	})

	if !strings.Contains(prompt, "Current Entity Schema") || !strings.Contains(prompt, "Extracted Metadata") {
		t.Fatalf("prompt missing expected sections: %s", prompt)
	}

	if got := buildConversationText([]string{"a", "b"}); got != "a\nb" {
		t.Fatalf("unexpected conversation text: %q", got)
	}

	schemaSection := buildSchemaSection(&segmentors.Schema{EntityTypes: map[string]segmentors.EntitySchema{
		"topic": {Desc: "主题", Attrs: map[string]segmentors.AttrDef{"name": {Type: "string", Desc: "名称"}}},
	}})
	if !strings.Contains(schemaSection, "### topic") {
		t.Fatalf("unexpected schema section: %s", schemaSection)
	}

	profilesSection := buildProfilesSection(map[string]map[string]any{"person:空": {}})
	if !strings.Contains(profilesSection, "(no attributes yet)") {
		t.Fatalf("unexpected profiles section: %s", profilesSection)
	}

	extractedSection := buildExtractedSection(extracted)
	if !strings.Contains(extractedSection, "Relations") || !strings.Contains(extractedSection, "person:小明") {
		t.Fatalf("unexpected extracted section: %s", extractedSection)
	}
}

func TestMuxAndDefaultWrappers(t *testing.T) {
	m := NewMux()
	stub := profilerStub{model: "m", res: &Result{ProfileUpdates: map[string]map[string]any{"x": {"k": "v"}}}}

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
	if res.ProfileUpdates["x"]["k"] != "v" {
		t.Fatalf("unexpected process result: %#v", res)
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
