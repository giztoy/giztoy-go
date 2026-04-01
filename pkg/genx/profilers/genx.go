package profilers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/genx/generators"
	"github.com/giztoy/giztoy-go/pkg/genx/segmentors"
)

var _ Profiler = (*GenX)(nil)

type profileArg struct {
	SchemaChanges  []SchemaChange              `json:"schema_changes"`
	ProfileUpdates map[string]map[string]any   `json:"profile_updates"`
	Relations      []segmentors.RelationOutput `json:"relations"`
}

var profileTool = genx.MustNewFuncTool[profileArg](
	"update_profiles",
	"Update entity profiles and propose schema changes based on conversation analysis.",
)

// GenX implements [Profiler] using a genx.Generator.
type GenX struct {
	generator string
	mux       *generators.Mux
}

func NewGenX(cfg Config) *GenX {
	return &GenX{generator: cfg.Generator}
}

func NewGenXWithMux(cfg Config, mux *generators.Mux) *GenX {
	return &GenX{generator: cfg.Generator, mux: mux}
}

func (g *GenX) Model() string {
	return g.generator
}

func (g *GenX) Process(ctx context.Context, input Input) (*Result, error) {
	mctx := g.buildModelContext(input)

	var (
		usage genx.Usage
		call  *genx.FuncCall
		err   error
	)
	if g.mux != nil {
		usage, call, err = g.mux.Invoke(ctx, g.generator, mctx, profileTool)
	} else {
		usage, call, err = generators.Invoke(ctx, g.generator, mctx, profileTool)
	}
	if err != nil {
		return nil, fmt.Errorf("profilers: invoke failed: %w", err)
	}
	_ = usage

	return g.parseResult(call)
}

func (g *GenX) buildModelContext(input Input) genx.ModelContext {
	var mcb genx.ModelContextBuilder
	mcb.PromptText("profiler", buildPrompt(input))
	mcb.UserText("conversation", buildConversationText(input.Messages))
	return mcb.Build()
}

func (g *GenX) parseResult(call *genx.FuncCall) (*Result, error) {
	if call == nil {
		return nil, fmt.Errorf("profilers: no function call returned")
	}

	var arg profileArg
	if err := json.Unmarshal([]byte(call.Arguments), &arg); err != nil {
		return nil, fmt.Errorf("profilers: failed to parse profile result: %w", err)
	}

	return &Result{
		SchemaChanges:  arg.SchemaChanges,
		ProfileUpdates: arg.ProfileUpdates,
		Relations:      arg.Relations,
	}, nil
}
