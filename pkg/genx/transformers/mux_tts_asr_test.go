package transformers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/genx"
)

func TestErrorStreamAndBufferStreamLifecycle(t *testing.T) {
	es := &errorStream{err: io.EOF}
	if _, err := es.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected error stream next error: %v", err)
	}
	if err := es.Close(); err != nil {
		t.Fatalf("error stream close failed: %v", err)
	}
	if err := es.CloseWithError(errors.New("x")); err != nil {
		t.Fatalf("error stream close with error failed: %v", err)
	}

	bs := newBufferStream(2)
	if err := bs.Push(&genx.MessageChunk{Part: genx.Text("hello")}); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	chunk, err := bs.Next()
	if err != nil {
		t.Fatalf("next failed: %v", err)
	}
	if chunk == nil || chunk.Part.(genx.Text) != "hello" {
		t.Fatalf("unexpected chunk: %#v", chunk)
	}

	if err := bs.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if _, err := bs.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after close, got: %v", err)
	}

	if err := bs.CloseWithError(errors.New("boom")); err != nil {
		t.Fatalf("close with error failed: %v", err)
	}
}

func TestTTSMuxPaths(t *testing.T) {
	tts := NewTTSMux()

	if _, err := tts.Synthesize(context.Background(), "missing", "x"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}
	if _, err := tts.SynthesizeStream(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	errTF := testTransformer{fn: func(context.Context, string, genx.Stream) (genx.Stream, error) {
		return nil, errors.New("transform fail")
	}}
	if err := tts.Handle("err", errTF); err != nil {
		t.Fatalf("register err transformer failed: %v", err)
	}
	if _, err := tts.Synthesize(context.Background(), "err", "x"); err == nil || !strings.Contains(err.Error(), "transform failed") {
		t.Fatalf("expected transform error, got: %v", err)
	}

	passTF := testTransformer{fn: func(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
		return input, nil
	}}
	if err := tts.Handle("pass", passTF); err != nil {
		t.Fatalf("register pass transformer failed: %v", err)
	}
	if err := tts.Handle("pass", passTF); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	out, err := tts.Synthesize(context.Background(), "pass", "hello")
	if err != nil {
		t.Fatalf("synthesize failed: %v", err)
	}

	first, err := out.Next()
	if err != nil {
		t.Fatalf("read first synthesized chunk failed: %v", err)
	}
	if first == nil || first.Part.(genx.Text) != "hello" {
		t.Fatalf("unexpected first synthesized chunk: %#v", first)
	}

	second, err := out.Next()
	if err != nil {
		t.Fatalf("read eos chunk failed: %v", err)
	}
	if second == nil || !second.IsEndOfStream() {
		t.Fatalf("expected eos chunk, got: %#v", second)
	}

	if _, err := out.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF after synthesized stream, got: %v", err)
	}

	session, err := tts.SynthesizeStream(context.Background(), "pass")
	if err != nil {
		t.Fatalf("synthesize stream failed: %v", err)
	}
	if err := session.Send("a"); err != nil {
		t.Fatalf("session send failed: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("session close failed: %v", err)
	}

	stream := session.Output()
	if stream == nil {
		t.Fatal("session output stream should not be nil")
	}
	if err := session.CloseAll(); err != nil {
		t.Fatalf("session close all failed: %v", err)
	}
}

func TestASRMuxPaths(t *testing.T) {
	asr := NewASRMux()

	if _, err := asr.Create(context.Background(), "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got: %v", err)
	}

	errTF := testTransformer{fn: func(context.Context, string, genx.Stream) (genx.Stream, error) {
		return nil, errors.New("transform fail")
	}}
	if err := asr.Handle("err", errTF); err != nil {
		t.Fatalf("register err transformer failed: %v", err)
	}
	if _, err := asr.Create(context.Background(), "err"); err == nil || !strings.Contains(err.Error(), "transform failed") {
		t.Fatalf("expected transform error, got: %v", err)
	}

	passTF := testTransformer{fn: func(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
		return input, nil
	}}
	if err := asr.Handle("pass", passTF); err != nil {
		t.Fatalf("register pass transformer failed: %v", err)
	}
	if err := asr.Handle("pass", passTF); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	session, err := asr.Create(context.Background(), "pass")
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}
	if err := session.Send([]byte{1, 2, 3}, "audio/opus"); err != nil {
		t.Fatalf("send audio failed: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("session close failed: %v", err)
	}

	out := session.Output()
	if out == nil {
		t.Fatal("output stream should not be nil")
	}

	first, err := out.Next()
	if err != nil {
		t.Fatalf("read first chunk failed: %v", err)
	}
	blob, ok := first.Part.(*genx.Blob)
	if !ok || blob.MIMEType != "audio/opus" {
		t.Fatalf("unexpected first chunk: %#v", first)
	}

	last, err := out.Next()
	if err != nil {
		t.Fatalf("read eos chunk failed: %v", err)
	}
	if last == nil || !last.IsEndOfStream() {
		t.Fatalf("expected eos chunk, got: %#v", last)
	}

	if err := session.CloseAll(); err != nil {
		t.Fatalf("close all failed: %v", err)
	}
}

func TestDefaultASRTTSWrappers(t *testing.T) {
	oldTTS := TTSMux
	oldASR := ASRMux
	TTSMux = NewTTSMux()
	ASRMux = NewASRMux()
	t.Cleanup(func() {
		TTSMux = oldTTS
		ASRMux = oldASR
	})

	passTF := testTransformer{fn: func(_ context.Context, _ string, input genx.Stream) (genx.Stream, error) {
		return input, nil
	}}

	if err := HandleTTS("default/tts", passTF); err != nil {
		t.Fatalf("default HandleTTS failed: %v", err)
	}
	if err := HandleASR("default/asr", passTF); err != nil {
		t.Fatalf("default HandleASR failed: %v", err)
	}
}
