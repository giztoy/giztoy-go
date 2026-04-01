package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/giztoy/giztoy-go/pkg/genx/profilers"
	"github.com/giztoy/giztoy-go/pkg/genx/segmentors"
)

// LLMCompressorConfig configures an [LLMCompressor].
type LLMCompressorConfig struct {
	// Segmentor is the pattern used to look up the segmentor in the mux.
	// Required — NewLLMCompressor returns an error if empty.
	Segmentor string

	// Profiler is the pattern used to look up the profiler in the mux.
	// Optional — when empty, ExtractEntities skips the profiler step.
	Profiler string

	// Schema is an optional entity schema passed to the segmentor.
	Schema *segmentors.Schema

	// Profiles are existing entity profiles passed to the profiler.
	// Keys are entity labels, values are attribute maps.
	Profiles map[string]map[string]any

	// SegmentorMux overrides the default segmentor multiplexer.
	// If nil, [segmentors.DefaultMux] is used.
	SegmentorMux *segmentors.Mux

	// ProfilerMux overrides the default profiler multiplexer.
	// If nil, [profilers.DefaultMux] is used.
	ProfilerMux *profilers.Mux
}

// LLMCompressor implements [Compressor] by delegating to registered
// segmentors and profilers via the genx mux system.
type LLMCompressor struct {
	segmentor    string
	profiler     string
	schema       *segmentors.Schema
	profiles     map[string]map[string]any
	segmentorMux *segmentors.Mux
	profilerMux  *profilers.Mux
}

// NewLLMCompressor creates a new LLM-backed [Compressor].
// The Segmentor field in cfg is required; all other fields are optional.
func NewLLMCompressor(cfg LLMCompressorConfig) (*LLMCompressor, error) {
	if cfg.Segmentor == "" {
		return nil, fmt.Errorf("memory: LLMCompressorConfig.Segmentor is required")
	}

	smux := cfg.SegmentorMux
	if smux == nil {
		smux = segmentors.DefaultMux
	}

	pmux := cfg.ProfilerMux
	if pmux == nil {
		pmux = profilers.DefaultMux
	}

	return &LLMCompressor{
		segmentor:    cfg.Segmentor,
		profiler:     cfg.Profiler,
		schema:       cfg.Schema,
		profiles:     cfg.Profiles,
		segmentorMux: smux,
		profilerMux:  pmux,
	}, nil
}

// CompressMessages runs the segmentor on messages and maps the result
// to a [CompressResult].
func (c *LLMCompressor) CompressMessages(ctx context.Context, messages []Message) (*CompressResult, error) {
	result, err := c.runSegmentor(ctx, messagesToStrings(messages))
	if err != nil {
		return nil, fmt.Errorf("memory: compress messages: %w", err)
	}
	return segResultToCompressResult(result), nil
}

// ExtractEntities runs the segmentor (and optionally the profiler) to
// extract entity and relation updates from messages.
//
// If a profiler is configured and it fails, the error is logged but does
// not block the main flow — the segmentor result is still returned.
func (c *LLMCompressor) ExtractEntities(ctx context.Context, messages []Message) (*EntityUpdate, error) {
	strs := messagesToStrings(messages)

	segResult, err := c.runSegmentor(ctx, strs)
	if err != nil {
		return nil, fmt.Errorf("memory: extract entities: %w", err)
	}

	update := segResultToEntityUpdate(segResult)

	if c.profiler != "" {
		profResult, err := c.runProfiler(ctx, strs, segResult)
		if err != nil {
			slog.Warn("memory: profiler failed (non-fatal)", "error", err)
		} else {
			mergeProfilerResult(update, profResult)
		}
	}

	return update, nil
}

// CompactSegments feeds summaries into the segmentor as input messages
// and returns the compacted result.
func (c *LLMCompressor) CompactSegments(ctx context.Context, summaries []string) (*CompressResult, error) {
	result, err := c.runSegmentor(ctx, summaries)
	if err != nil {
		return nil, fmt.Errorf("memory: compact segments: %w", err)
	}
	return segResultToCompressResult(result), nil
}

// runSegmentor dispatches to the configured segmentor via the mux.
func (c *LLMCompressor) runSegmentor(ctx context.Context, messages []string) (*segmentors.Result, error) {
	return c.segmentorMux.Process(ctx, c.segmentor, segmentors.Input{
		Messages: messages,
		Schema:   c.schema,
	})
}

// runProfiler dispatches to the configured profiler via the mux.
func (c *LLMCompressor) runProfiler(ctx context.Context, messages []string, extracted *segmentors.Result) (*profilers.Result, error) {
	return c.profilerMux.Process(ctx, c.profiler, profilers.Input{
		Messages:  messages,
		Extracted: extracted,
		Schema:    c.schema,
		Profiles:  c.profiles,
	})
}

// messagesToStrings converts memory messages to plain strings for segmentor input.
func messagesToStrings(msgs []Message) []string {
	strs := make([]string, len(msgs))
	for i, m := range msgs {
		if m.Name != "" {
			strs[i] = m.Name + ": " + m.Content
		} else {
			strs[i] = string(m.Role) + ": " + m.Content
		}
	}
	return strs
}

// segResultToCompressResult maps a segmentor result to a CompressResult.
func segResultToCompressResult(r *segmentors.Result) *CompressResult {
	return &CompressResult{
		Segments: []SegmentInput{{
			Summary:  r.Segment.Summary,
			Keywords: r.Segment.Keywords,
			Labels:   r.Segment.Labels,
		}},
		Summary: r.Segment.Summary,
	}
}

// segResultToEntityUpdate maps a segmentor result to an EntityUpdate.
func segResultToEntityUpdate(r *segmentors.Result) *EntityUpdate {
	update := &EntityUpdate{}
	for _, e := range r.Entities {
		update.Entities = append(update.Entities, EntityInput{
			Label: e.Label,
			Attrs: e.Attrs,
		})
	}
	for _, rel := range r.Relations {
		update.Relations = append(update.Relations, RelationInput{
			From:    rel.From,
			To:      rel.To,
			RelType: rel.RelType,
		})
	}
	return update
}

// mergeProfilerResult merges profiler output into an existing EntityUpdate.
// Profile updates become entity attribute merges; profiler relations are appended.
func mergeProfilerResult(update *EntityUpdate, pr *profilers.Result) {
	for label, attrs := range pr.ProfileUpdates {
		update.Entities = append(update.Entities, EntityInput{
			Label: label,
			Attrs: attrs,
		})
	}
	for _, rel := range pr.Relations {
		update.Relations = append(update.Relations, RelationInput{
			From:    rel.From,
			To:      rel.To,
			RelType: rel.RelType,
		})
	}
}
