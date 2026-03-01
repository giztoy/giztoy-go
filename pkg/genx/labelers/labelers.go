// Package labelers provides query-time label selection for memory recall.
package labelers

import "context"

// Labeler selects labels from candidates for a query text.
type Labeler interface {
	Process(ctx context.Context, input Input) (*Result, error)
	Model() string
}

// Input is the input to a [Labeler].
type Input struct {
	Text       string              `json:"text"`
	Candidates []string            `json:"candidates"`
	Aliases    map[string][]string `json:"aliases,omitempty"`
	TopK       int                 `json:"top_k,omitempty"`
}

// Match is a selected candidate label.
type Match struct {
	Label string  `json:"label"`
	Score float64 `json:"score,omitempty"`
}

// Result is the output of a [Labeler].
type Result struct {
	Matches []Match `json:"matches"`
}

// Config configures a GenX labeler implementation.
type Config struct {
	Generator     string `json:"generator" yaml:"generator"`
	PromptVersion string `json:"prompt_version,omitempty" yaml:"prompt_version,omitempty"`
}
