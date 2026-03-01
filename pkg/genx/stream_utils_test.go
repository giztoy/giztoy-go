package genx

import (
	"errors"
	"io"
	"testing"
)

func TestMergeHandlesErrDone(t *testing.T) {
	m := Merge(
		&sliceStream{chunks: []*MessageChunk{{Part: Text("a")}}, doneErr: ErrDone},
		&sliceStream{chunks: []*MessageChunk{{Part: Text("b")}}, doneErr: ErrDone},
	)

	first, err := m.Next()
	if err != nil {
		t.Fatalf("first next failed: %v", err)
	}
	if first == nil || first.Part.(Text) != "a" {
		t.Fatalf("unexpected first chunk: %#v", first)
	}

	second, err := m.Next()
	if err != nil {
		t.Fatalf("second next failed: %v", err)
	}
	if second == nil || second.Part.(Text) != "b" {
		t.Fatalf("unexpected second chunk: %#v", second)
	}
}

func TestSplitHandlesErrDone(t *testing.T) {
	s := &sliceStream{chunks: []*MessageChunk{
		{Part: Text("keep")},
		{Part: Text("drop")},
	}, doneErr: ErrDone}

	matched, rest := Split(s, func(c *MessageChunk) bool {
		txt, ok := c.Part.(Text)
		return ok && txt == "keep"
	})

	v, err := matched.Next()
	if err != nil {
		t.Fatalf("matched next failed: %v", err)
	}
	if v == nil || v.Part.(Text) != "keep" {
		t.Fatalf("unexpected matched chunk: %#v", v)
	}
	if _, err := matched.Next(); !errors.Is(err, ErrDone) && !errors.Is(err, io.EOF) {
		t.Fatalf("expected stream done, got: %v", err)
	}

	v, err = rest.Next()
	if err != nil {
		t.Fatalf("rest next failed: %v", err)
	}
	if v == nil || v.Part.(Text) != "drop" {
		t.Fatalf("unexpected rest chunk: %#v", v)
	}
}
