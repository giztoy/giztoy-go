package generators

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
)

type stubGenerator struct {
	streamFn func(context.Context, string, genx.ModelContext) (genx.Stream, error)
	invokeFn func(context.Context, string, genx.ModelContext, *genx.FuncTool) (genx.Usage, *genx.FuncCall, error)
}

func (s stubGenerator) GenerateStream(ctx context.Context, pattern string, mctx genx.ModelContext) (genx.Stream, error) {
	if s.streamFn == nil {
		return nil, errors.New("streamFn not set")
	}
	return s.streamFn(ctx, pattern, mctx)
}

func (s stubGenerator) Invoke(ctx context.Context, pattern string, mctx genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
	if s.invokeFn == nil {
		return genx.Usage{}, nil, errors.New("invokeFn not set")
	}
	return s.invokeFn(ctx, pattern, mctx, tool)
}

func testTextStream(t *testing.T, text string) genx.Stream {
	t.Helper()
	ctx := (&genx.ModelContextBuilder{}).Build()
	b := genx.NewStreamBuilder(ctx, 4)
	if err := b.Add(&genx.MessageChunk{Role: genx.RoleModel, Part: genx.Text(text)}); err != nil {
		t.Fatalf("add chunk failed: %v", err)
	}
	if err := b.Done(genx.Usage{}); err != nil {
		t.Fatalf("done failed: %v", err)
	}
	return b.Stream()
}

func TestMuxHandleAndRoute(t *testing.T) {
	m := NewMux()
	calledStream := false
	calledInvoke := false

	g := stubGenerator{
		streamFn: func(_ context.Context, pattern string, _ genx.ModelContext) (genx.Stream, error) {
			if pattern != "gx/mock" {
				t.Fatalf("unexpected pattern: %s", pattern)
			}
			calledStream = true
			return testTextStream(t, "ok"), nil
		},
		invokeFn: func(_ context.Context, pattern string, _ genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
			if pattern != "gx/mock" {
				t.Fatalf("unexpected pattern: %s", pattern)
			}
			calledInvoke = true
			return genx.Usage{PromptTokenCount: 1}, tool.NewFuncCall(`{"ok":true}`), nil
		},
	}

	if err := m.Handle("gx/mock", g); err != nil {
		t.Fatalf("register generator failed: %v", err)
	}

	stream, err := m.GenerateStream(context.Background(), "gx/mock", (&genx.ModelContextBuilder{}).Build())
	if err != nil {
		t.Fatalf("generate stream failed: %v", err)
	}

	chunk, err := stream.Next()
	if err != nil {
		t.Fatalf("read first chunk failed: %v", err)
	}
	if chunk == nil || chunk.Part == nil || chunk.Part.(genx.Text) != "ok" {
		t.Fatalf("unexpected chunk: %#v", chunk)
	}
	if !calledStream {
		t.Fatal("expected streamFn to be called")
	}

	tool := genx.MustNewFuncTool[struct {
		OK bool `json:"ok"`
	}]("mock_tool", "mock tool")

	_, call, err := m.Invoke(context.Background(), "gx/mock", (&genx.ModelContextBuilder{}).Build(), tool)
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	if !calledInvoke {
		t.Fatal("expected invokeFn to be called")
	}
	if call == nil || !strings.Contains(call.Arguments, "ok") {
		t.Fatalf("unexpected call: %#v", call)
	}
}

func TestMuxHandleDuplicate(t *testing.T) {
	m := NewMux()
	g := stubGenerator{streamFn: func(context.Context, string, genx.ModelContext) (genx.Stream, error) {
		return nil, nil
	}}
	if err := m.Handle("dup", g); err != nil {
		t.Fatalf("first handle failed: %v", err)
	}
	if err := m.Handle("dup", g); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate registration error, got: %v", err)
	}
}

func TestMuxGetNotFoundAndNil(t *testing.T) {
	m := NewMux()
	if _, err := m.get("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	if err := m.mux.Set("nil", func(ptr *genx.Generator, _ bool) error { return nil }); err != nil {
		t.Fatalf("inject nil generator failed: %v", err)
	}
	if _, err := m.get("nil"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected nil generator error, got: %v", err)
	}
}

func TestDefaultMuxWrappers(t *testing.T) {
	old := DefaultMux
	DefaultMux = NewMux()
	t.Cleanup(func() { DefaultMux = old })

	if err := Handle("wrapper/mock", stubGenerator{
		streamFn: func(_ context.Context, _ string, _ genx.ModelContext) (genx.Stream, error) {
			return testTextStream(t, "wrapped"), nil
		},
		invokeFn: func(_ context.Context, _ string, _ genx.ModelContext, tool *genx.FuncTool) (genx.Usage, *genx.FuncCall, error) {
			return genx.Usage{}, tool.NewFuncCall(`{"v":1}`), nil
		},
	}); err != nil {
		t.Fatalf("default handle failed: %v", err)
	}

	stream, err := GenerateStream(context.Background(), "wrapper/mock", (&genx.ModelContextBuilder{}).Build())
	if err != nil {
		t.Fatalf("default GenerateStream failed: %v", err)
	}
	if _, err := stream.Next(); err != nil {
		t.Fatalf("default stream next failed: %v", err)
	}

	tool := genx.MustNewFuncTool[struct {
		V int `json:"v"`
	}]("wrapper_tool", "wrapper tool")
	if _, call, err := Invoke(context.Background(), "wrapper/mock", (&genx.ModelContextBuilder{}).Build(), tool); err != nil {
		t.Fatalf("default Invoke failed: %v", err)
	} else if call == nil {
		t.Fatal("expected non-nil func call")
	}
}
