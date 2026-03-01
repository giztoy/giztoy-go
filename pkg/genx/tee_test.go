package genx

import (
	"errors"
	"io"
	"testing"
)

func TestTeeTreatsErrDoneAsNormalEnd(t *testing.T) {
	src := &sliceStream{chunks: []*MessageChunk{{Part: Text("hello")}}, doneErr: ErrDone}
	builder := NewStreamBuilder((&ModelContextBuilder{}).Build(), 8)
	tee := Tee(src, builder)

	if _, err := tee.Next(); err != nil {
		t.Fatalf("tee next failed: %v", err)
	}
	if _, err := tee.Next(); !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone from tee, got: %v", err)
	}

	out := builder.Stream()
	v, err := out.Next()
	if err != nil {
		t.Fatalf("builder stream next failed: %v", err)
	}
	if v == nil || v.Part.(Text) != "hello" {
		t.Fatalf("unexpected mirrored chunk: %#v", v)
	}
	if _, err := out.Next(); !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone from mirrored stream, got: %v", err)
	}
}

func TestStreamBuilderUnknownToolCallDoesNotDropChunk(t *testing.T) {
	b := NewStreamBuilder((&ModelContextBuilder{}).Build(), 8)
	chunk := &MessageChunk{
		Role: RoleModel,
		ToolCall: &ToolCall{
			ID:       "call-1",
			FuncCall: &FuncCall{Name: "unknown_tool", Arguments: `{"k":"v"}`},
		},
	}

	if err := b.Add(chunk); err != nil {
		t.Fatalf("add chunk failed: %v", err)
	}
	if err := b.Done(Usage{}); err != nil {
		t.Fatalf("done failed: %v", err)
	}

	out := b.Stream()
	v, err := out.Next()
	if err != nil {
		t.Fatalf("unexpected error reading chunk: %v", err)
	}
	if v == nil || v.ToolCall == nil || v.ToolCall.FuncCall == nil {
		t.Fatalf("unexpected chunk payload: %#v", v)
	}
	if v.ToolCall.FuncCall.Name != "unknown_tool" {
		t.Fatalf("unexpected tool call name: %s", v.ToolCall.FuncCall.Name)
	}
}

func TestTeeTreatsEOFAsNormalEnd(t *testing.T) {
	src := &sliceStream{chunks: []*MessageChunk{{Part: Text("hello")}}, doneErr: io.EOF}
	builder := NewStreamBuilder((&ModelContextBuilder{}).Build(), 8)
	tee := Tee(src, builder)

	if _, err := tee.Next(); err != nil {
		t.Fatalf("tee next failed: %v", err)
	}
	if _, err := tee.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF from tee, got: %v", err)
	}

	out := builder.Stream()
	if _, err := out.Next(); err != nil {
		t.Fatalf("builder stream first next failed: %v", err)
	}
	if _, err := out.Next(); !errors.Is(err, ErrDone) {
		t.Fatalf("expected ErrDone from mirrored stream, got: %v", err)
	}
}
