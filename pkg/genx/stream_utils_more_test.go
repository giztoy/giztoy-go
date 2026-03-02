package genx

import (
	"errors"
	"io"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/buffer"
)

type closeTrackStream struct {
	chunks          []*MessageChunk
	err             error
	idx             int
	closeCalled     bool
	closeErrCalled  bool
	closeErrArg     error
	closeReturnErr  error
	closeWithErrRet error
}

func (s *closeTrackStream) Next() (*MessageChunk, error) {
	if s.idx < len(s.chunks) {
		v := s.chunks[s.idx]
		s.idx++
		return v, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, ErrDone
}

func (s *closeTrackStream) Close() error {
	s.closeCalled = true
	return s.closeReturnErr
}

func (s *closeTrackStream) CloseWithError(err error) error {
	s.closeErrCalled = true
	s.closeErrArg = err
	return s.closeWithErrRet
}

func TestMIMETypeMatcherAndEmptyStream(t *testing.T) {
	matcher := MIMETypeMatcher("audio/")
	if matcher(nil) {
		t.Fatal("nil chunk should not match")
	}
	if matcher(&MessageChunk{Part: Text("x")}) {
		t.Fatal("text chunk should not match audio prefix")
	}
	if !matcher(&MessageChunk{Part: &Blob{MIMEType: "audio/wav", Data: []byte{1}}}) {
		t.Fatal("audio blob should match")
	}

	es := &emptyStream{}
	if _, err := es.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF from empty stream, got: %v", err)
	}
	if err := es.Close(); err != nil {
		t.Fatalf("empty stream close failed: %v", err)
	}
	if err := es.CloseWithError(errors.New("x")); err != nil {
		t.Fatalf("empty stream close with error failed: %v", err)
	}
}

func TestCompositeSeqAndMergeInterleaved(t *testing.T) {
	s1 := &sliceStream{chunks: []*MessageChunk{{Part: Text("a")}}, doneErr: ErrDone}
	s2 := &sliceStream{chunks: []*MessageChunk{{Part: Text("b")}}, doneErr: io.EOF}
	out := CompositeSeq(s1, s2)

	first, err := out.Next()
	if err != nil || first.Part.(Text) != "a" {
		t.Fatalf("unexpected first chunk: chunk=%#v err=%v", first, err)
	}
	mid, err := out.Next()
	if err != nil || !mid.IsEndOfStream() {
		t.Fatalf("expected EOS marker between streams, got: %#v err=%v", mid, err)
	}
	second, err := out.Next()
	if err != nil || second.Part.(Text) != "b" {
		t.Fatalf("unexpected second chunk: chunk=%#v err=%v", second, err)
	}
	if _, err := out.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF from composite stream, got: %v", err)
	}

	errWant := errors.New("boom")
	errStream := CompositeSeq(&sliceStream{doneErr: errWant})
	if _, err := errStream.Next(); !errors.Is(err, errWant) {
		t.Fatalf("expected propagated stream error, got: %v", err)
	}

	inter := MergeInterleaved(
		&sliceStream{chunks: []*MessageChunk{{Part: Text("1")}, {Part: Text("3")}}, doneErr: ErrDone},
		&sliceStream{chunks: []*MessageChunk{{Part: Text("2")}, {Part: Text("4")}}, doneErr: io.EOF},
	)
	parts := make([]string, 0, 4)
	for {
		chunk, err := inter.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("unexpected merge interleaved error: %v", err)
		}
		parts = append(parts, string(chunk.Part.(Text)))
	}
	if got := parts; len(got) != 4 || got[0] != "1" || got[1] != "2" || got[2] != "3" || got[3] != "4" {
		t.Fatalf("unexpected interleaved order: %#v", got)
	}

	interErr := MergeInterleaved(&sliceStream{doneErr: errWant}, &sliceStream{doneErr: ErrDone})
	if _, err := interErr.Next(); !errors.Is(err, errWant) {
		t.Fatalf("expected propagated interleaved error, got: %v", err)
	}
}

func TestMergeCloseAndBufferStreamCloseWithError(t *testing.T) {
	s1 := &closeTrackStream{chunks: []*MessageChunk{{Part: Text("a")}}}
	s2 := &closeTrackStream{chunks: []*MessageChunk{{Part: Text("b")}}}
	m := Merge(s1, s2)

	if _, err := m.Next(); err != nil {
		t.Fatalf("merge next failed: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("merge close failed: %v", err)
	}
	if !s1.closeCalled || !s2.closeCalled {
		t.Fatalf("expected underlying streams to be closed: s1=%v s2=%v", s1.closeCalled, s2.closeCalled)
	}

	m2 := Merge(&closeTrackStream{}, &closeTrackStream{})
	want := errors.New("close-with-error")
	if err := m2.CloseWithError(want); err != nil {
		t.Fatalf("merge close with error failed: %v", err)
	}

	b := &bufferStream{buf: buffer.N[*MessageChunk](1)}
	if err := b.CloseWithError(want); err != nil {
		t.Fatalf("bufferStream CloseWithError failed: %v", err)
	}
	if _, err := b.Next(); err == nil {
		t.Fatal("expected error after CloseWithError")
	}
	if err := b.Close(); err != nil {
		t.Fatalf("bufferStream Close failed: %v", err)
	}
}
