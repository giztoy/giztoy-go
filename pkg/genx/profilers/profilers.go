// Package profilers provides entity profile management.
package profilers

import (
	"context"

	"github.com/giztoy/giztoy-go/pkg/genx/segmentors"
)

// Profiler evolves schemas and updates profiles.
type Profiler interface {
	Process(ctx context.Context, input Input) (*Result, error)
	Model() string
}

// Input is the input to a [Profiler].
type Input struct {
	Messages  []string                  `json:"messages"`
	Extracted *segmentors.Result        `json:"extracted"`
	Schema    *segmentors.Schema        `json:"schema,omitempty"`
	Profiles  map[string]map[string]any `json:"profiles,omitempty"`
}

// Result is the output of a [Profiler].
type Result struct {
	SchemaChanges  []SchemaChange              `json:"schema_changes"`
	ProfileUpdates map[string]map[string]any   `json:"profile_updates"`
	Relations      []segmentors.RelationOutput `json:"relations"`
}

// SchemaChange proposes a schema modification.
type SchemaChange struct {
	EntityType string             `json:"entity_type"`
	Field      string             `json:"field"`
	Def        segmentors.AttrDef `json:"def"`
	Action     string             `json:"action"`
}

// Config configures a GenX profiler implementation.
type Config struct {
	Generator     string `json:"generator" yaml:"generator"`
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
