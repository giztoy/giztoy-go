// Package segmentors provides conversation segmentation.
package segmentors

import "context"

// Segmentor compresses conversation messages into a structured segment.
type Segmentor interface {
	Process(ctx context.Context, input Input) (*Result, error)
	Model() string
}

// Input is the input to a [Segmentor].
type Input struct {
	Messages []string `json:"messages"`
	Schema   *Schema  `json:"schema,omitempty"`
}

// Result is the output of a [Segmentor].
type Result struct {
	Segment   SegmentOutput    `json:"segment"`
	Entities  []EntityOutput   `json:"entities"`
	Relations []RelationOutput `json:"relations"`
}

type SegmentOutput struct {
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
	Labels   []string `json:"labels"`
}

type EntityOutput struct {
	Label string         `json:"label"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

type RelationOutput struct {
	From    string `json:"from"`
	To      string `json:"to"`
	RelType string `json:"rel_type"`
}

type Schema struct {
	EntityTypes map[string]EntitySchema `json:"entity_types" yaml:"entity_types"`
}

type EntitySchema struct {
	Desc  string             `json:"desc" yaml:"desc"`
	Attrs map[string]AttrDef `json:"attrs,omitempty" yaml:"attrs,omitempty"`
}

type AttrDef struct {
	Type string `json:"type" yaml:"type"`
	Desc string `json:"desc" yaml:"desc"`
}

// Config configures a GenX segmentor implementation.
type Config struct {
	Generator     string `json:"generator" yaml:"generator"`
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
