package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx/profilers"
	"github.com/giztoy/giztoy-go/pkg/genx/segmentors"
)

// ---------------------------------------------------------------------------
// Fake segmentor / profiler for unit tests
// ---------------------------------------------------------------------------

type fakeSegmentor struct {
	result *segmentors.Result
	err    error
	calls  int
}

func (f *fakeSegmentor) Process(_ context.Context, _ segmentors.Input) (*segmentors.Result, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeSegmentor) Model() string { return "fake-seg" }

type fakeProfiler struct {
	result *profilers.Result
	err    error
	calls  int
}

func (f *fakeProfiler) Process(_ context.Context, _ profilers.Input) (*profilers.Result, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeProfiler) Model() string { return "fake-prof" }

// newTestMuxes creates isolated segmentor/profiler muxes with the given fakes
// registered under the specified patterns.
func newTestMuxes(segPattern string, seg segmentors.Segmentor, profPattern string, prof profilers.Profiler) (*segmentors.Mux, *profilers.Mux) {
	smux := segmentors.NewMux()
	if seg != nil {
		_ = smux.Handle(segPattern, seg)
	}
	pmux := profilers.NewMux()
	if prof != nil {
		_ = pmux.Handle(profPattern, prof)
	}
	return smux, pmux
}

// ---------------------------------------------------------------------------
// NewLLMCompressor
// ---------------------------------------------------------------------------

func TestNewLLMCompressor_SegmentorRequired(t *testing.T) {
	_, err := NewLLMCompressor(LLMCompressorConfig{})
	if err == nil {
		t.Fatal("expected error when Segmentor is empty")
	}
}

func TestNewLLMCompressor_OK(t *testing.T) {
	smux, pmux := newTestMuxes("test/seg", &fakeSegmentor{}, "test/prof", &fakeProfiler{})
	c, err := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		Profiler:     "test/prof",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})
	if err != nil {
		t.Fatalf("NewLLMCompressor: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil compressor")
	}
}

func TestNewLLMCompressor_DefaultMuxes(t *testing.T) {
	c, err := NewLLMCompressor(LLMCompressorConfig{
		Segmentor: "some/pattern",
	})
	if err != nil {
		t.Fatalf("NewLLMCompressor: %v", err)
	}
	if c.segmentorMux != segmentors.DefaultMux {
		t.Error("expected default segmentor mux")
	}
	if c.profilerMux != profilers.DefaultMux {
		t.Error("expected default profiler mux")
	}
}

// ---------------------------------------------------------------------------
// CompressMessages
// ---------------------------------------------------------------------------

func TestCompressMessages_OK(t *testing.T) {
	seg := &fakeSegmentor{result: &segmentors.Result{
		Segment: segmentors.SegmentOutput{
			Summary:  "聊了恐龙的话题",
			Keywords: []string{"恐龙", "霸王龙"},
			Labels:   []string{"person:小明", "topic:恐龙"},
		},
	}}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, err := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})
	if err != nil {
		t.Fatal(err)
	}

	msgs := []Message{
		{Role: RoleUser, Name: "小明", Content: "我喜欢恐龙"},
		{Role: RoleModel, Content: "恐龙很酷！"},
	}

	result, err := c.CompressMessages(context.Background(), msgs)
	if err != nil {
		t.Fatalf("CompressMessages: %v", err)
	}

	if len(result.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(result.Segments))
	}
	if result.Segments[0].Summary != "聊了恐龙的话题" {
		t.Errorf("summary = %q, want %q", result.Segments[0].Summary, "聊了恐龙的话题")
	}
	if result.Summary != "聊了恐龙的话题" {
		t.Errorf("result.Summary = %q, want %q", result.Summary, "聊了恐龙的话题")
	}
	if seg.calls != 1 {
		t.Errorf("segmentor calls = %d, want 1", seg.calls)
	}
}

func TestCompressMessages_SegmentorError(t *testing.T) {
	seg := &fakeSegmentor{err: errors.New("segmentor boom")}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	_, err := c.CompressMessages(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("expected error from segmentor")
	}
}

// ---------------------------------------------------------------------------
// ExtractEntities
// ---------------------------------------------------------------------------

func TestExtractEntities_SegmentorOnly(t *testing.T) {
	seg := &fakeSegmentor{result: &segmentors.Result{
		Entities: []segmentors.EntityOutput{
			{Label: "person:小明", Attrs: map[string]any{"age": float64(8)}},
		},
		Relations: []segmentors.RelationOutput{
			{From: "person:小明", To: "topic:恐龙", RelType: "likes"},
		},
	}}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	update, err := c.ExtractEntities(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("ExtractEntities: %v", err)
	}

	if len(update.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(update.Entities))
	}
	if update.Entities[0].Label != "person:小明" {
		t.Errorf("entity label = %q, want %q", update.Entities[0].Label, "person:小明")
	}
	if len(update.Relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(update.Relations))
	}
	if update.Relations[0].RelType != "likes" {
		t.Errorf("relation type = %q, want %q", update.Relations[0].RelType, "likes")
	}
}

func TestExtractEntities_WithProfiler(t *testing.T) {
	seg := &fakeSegmentor{result: &segmentors.Result{
		Entities: []segmentors.EntityOutput{
			{Label: "person:小明", Attrs: map[string]any{"age": float64(8)}},
		},
	}}
	prof := &fakeProfiler{result: &profilers.Result{
		ProfileUpdates: map[string]map[string]any{
			"person:小明": {"hobby": "恐龙"},
		},
		Relations: []segmentors.RelationOutput{
			{From: "person:小明", To: "topic:恐龙", RelType: "expert_in"},
		},
	}}
	smux, pmux := newTestMuxes("test/seg", seg, "test/prof", prof)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		Profiler:     "test/prof",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	update, err := c.ExtractEntities(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("ExtractEntities: %v", err)
	}

	// Segmentor entity + profiler entity merge.
	if len(update.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(update.Entities))
	}

	// Profiler relation should be merged in.
	if len(update.Relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(update.Relations))
	}
	if update.Relations[0].RelType != "expert_in" {
		t.Errorf("relation type = %q, want %q", update.Relations[0].RelType, "expert_in")
	}

	if prof.calls != 1 {
		t.Errorf("profiler calls = %d, want 1", prof.calls)
	}
}

func TestExtractEntities_ProfilerFailureNonFatal(t *testing.T) {
	seg := &fakeSegmentor{result: &segmentors.Result{
		Entities: []segmentors.EntityOutput{
			{Label: "person:小明"},
		},
	}}
	prof := &fakeProfiler{err: errors.New("profiler boom")}
	smux, pmux := newTestMuxes("test/seg", seg, "test/prof", prof)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		Profiler:     "test/prof",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	update, err := c.ExtractEntities(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("expected no error when profiler fails, got: %v", err)
	}

	// Segmentor result should still be returned.
	if len(update.Entities) != 1 {
		t.Fatalf("entities = %d, want 1 (segmentor result)", len(update.Entities))
	}
	if prof.calls != 1 {
		t.Errorf("profiler calls = %d, want 1", prof.calls)
	}
}

func TestExtractEntities_SegmentorError(t *testing.T) {
	seg := &fakeSegmentor{err: errors.New("segmentor boom")}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	_, err := c.ExtractEntities(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err == nil {
		t.Fatal("expected error from segmentor")
	}
}

// ---------------------------------------------------------------------------
// CompactSegments
// ---------------------------------------------------------------------------

func TestCompactSegments_OK(t *testing.T) {
	seg := &fakeSegmentor{result: &segmentors.Result{
		Segment: segmentors.SegmentOutput{
			Summary:  "一周的恐龙回忆",
			Keywords: []string{"恐龙", "一周"},
			Labels:   []string{"person:小明"},
		},
	}}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	summaries := []string{"day1: 聊了恐龙", "day2: 画了恐龙", "day3: 看了恐龙化石"}
	result, err := c.CompactSegments(context.Background(), summaries)
	if err != nil {
		t.Fatalf("CompactSegments: %v", err)
	}

	if len(result.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(result.Segments))
	}
	if result.Segments[0].Summary != "一周的恐龙回忆" {
		t.Errorf("summary = %q, want %q", result.Segments[0].Summary, "一周的恐龙回忆")
	}
}

func TestCompactSegments_SegmentorError(t *testing.T) {
	seg := &fakeSegmentor{err: errors.New("segmentor boom")}
	smux, pmux := newTestMuxes("test/seg", seg, "", nil)

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		SegmentorMux: smux,
		ProfilerMux:  pmux,
	})

	_, err := c.CompactSegments(context.Background(), []string{"summary"})
	if err == nil {
		t.Fatal("expected error from segmentor")
	}
}

// ---------------------------------------------------------------------------
// messagesToStrings
// ---------------------------------------------------------------------------

func TestMessagesToStrings(t *testing.T) {
	msgs := []Message{
		{Role: RoleUser, Name: "小明", Content: "你好"},
		{Role: RoleModel, Content: "你好呀"},
		{Role: RoleUser, Content: "恐龙"},
	}

	strs := messagesToStrings(msgs)

	if len(strs) != 3 {
		t.Fatalf("len = %d, want 3", len(strs))
	}
	if strs[0] != "小明: 你好" {
		t.Errorf("strs[0] = %q, want %q", strs[0], "小明: 你好")
	}
	if strs[1] != "model: 你好呀" {
		t.Errorf("strs[1] = %q, want %q", strs[1], "model: 你好呀")
	}
	if strs[2] != "user: 恐龙" {
		t.Errorf("strs[2] = %q, want %q", strs[2], "user: 恐龙")
	}
}

// ---------------------------------------------------------------------------
// mergeProfilerResult
// ---------------------------------------------------------------------------

func TestMergeProfilerResult(t *testing.T) {
	update := &EntityUpdate{
		Entities: []EntityInput{
			{Label: "person:小明", Attrs: map[string]any{"age": float64(8)}},
		},
	}

	pr := &profilers.Result{
		ProfileUpdates: map[string]map[string]any{
			"person:小明": {"hobby": "恐龙"},
			"person:小红": {"hobby": "画画"},
		},
		Relations: []segmentors.RelationOutput{
			{From: "person:小明", To: "person:小红", RelType: "sibling"},
		},
	}

	mergeProfilerResult(update, pr)

	// Original entity + 2 from profiler.
	if len(update.Entities) != 3 {
		t.Fatalf("entities = %d, want 3", len(update.Entities))
	}
	if len(update.Relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(update.Relations))
	}
	if update.Relations[0].RelType != "sibling" {
		t.Errorf("relation type = %q, want %q", update.Relations[0].RelType, "sibling")
	}
}

// ---------------------------------------------------------------------------
// Schema and Profiles pass-through
// ---------------------------------------------------------------------------

func TestLLMCompressor_SchemaPassedToSegmentor(t *testing.T) {
	var capturedInput segmentors.Input
	seg := &capturingSegmentor{
		result: &segmentors.Result{
			Segment: segmentors.SegmentOutput{Summary: "test"},
		},
		capture: func(input segmentors.Input) { capturedInput = input },
	}
	smux := segmentors.NewMux()
	_ = smux.Handle("test/seg", seg)

	schema := &segmentors.Schema{
		EntityTypes: map[string]segmentors.EntitySchema{
			"person": {Desc: "a person"},
		},
	}

	c, _ := NewLLMCompressor(LLMCompressorConfig{
		Segmentor:    "test/seg",
		Schema:       schema,
		SegmentorMux: smux,
		ProfilerMux:  profilers.NewMux(),
	})

	_, _ = c.CompressMessages(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})

	if capturedInput.Schema == nil {
		t.Fatal("expected schema to be passed to segmentor")
	}
	if _, ok := capturedInput.Schema.EntityTypes["person"]; !ok {
		t.Error("expected person entity type in schema")
	}
}

type capturingSegmentor struct {
	result  *segmentors.Result
	capture func(segmentors.Input)
}

func (s *capturingSegmentor) Process(_ context.Context, input segmentors.Input) (*segmentors.Result, error) {
	if s.capture != nil {
		s.capture(input)
	}
	return s.result, nil
}

func (s *capturingSegmentor) Model() string { return "capturing-seg" }
