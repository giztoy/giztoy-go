package transformers

import (
	"context"
	"fmt"

	"github.com/giztoy/giztoy-go/pkg/genx"
	"github.com/giztoy/giztoy-go/pkg/trie"
)

// TTSMux is the default TTS transformer multiplexer.
var TTSMux = NewTTSMux()

// HandleTTS registers a TTS transformer for the given pattern to the default mux.
func HandleTTS(pattern string, t genx.Transformer) error {
	return TTSMux.Handle(pattern, t)
}

// TTS routes synthesis requests to registered transformers.
type TTS struct {
	mux *trie.Trie[genx.Transformer]
}

// NewTTSMux creates a new TTS transformer multiplexer.
func NewTTSMux() *TTS {
	return &TTS{mux: trie.New[genx.Transformer]()}
}

// Handle registers a TTS transformer for the given pattern.
func (m *TTS) Handle(pattern string, t genx.Transformer) error {
	return m.mux.Set(pattern, func(ptr *genx.Transformer, existed bool) error {
		if existed {
			return fmt.Errorf("tts: transformer already registered for %s", pattern)
		}
		*ptr = t
		return nil
	})
}

// Synthesize creates a TTS stream for the given model pattern and text.
func (m *TTS) Synthesize(ctx context.Context, pattern string, text string) (genx.Stream, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}

	inputStream := newBufferStream(10)

	textChunk := &genx.MessageChunk{Part: genx.Text(text)}
	if err := inputStream.Push(textChunk); err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: push text failed: %w", err)
	}

	eosChunk := genx.NewTextEndOfStream()
	if err := inputStream.Push(eosChunk); err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: push eos failed: %w", err)
	}

	inputStream.Close()

	outputStream, err := t.Transform(ctx, pattern, inputStream)
	if err != nil {
		return nil, fmt.Errorf("tts: transform failed: %w", err)
	}

	return outputStream, nil
}

// SynthesizeStream creates a TTS session for streaming text input.
func (m *TTS) SynthesizeStream(ctx context.Context, pattern string) (*TTSSession, error) {
	ptr, ok := m.mux.Get(pattern)
	if !ok {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}
	t := *ptr
	if t == nil {
		return nil, fmt.Errorf("tts: transformer not found for %s", pattern)
	}

	inputStream := newBufferStream(100)

	outputStream, err := t.Transform(ctx, pattern, inputStream)
	if err != nil {
		inputStream.Close()
		return nil, fmt.Errorf("tts: transform failed: %w", err)
	}

	return &TTSSession{input: inputStream, output: outputStream}, nil
}

// TTSSession represents an active TTS session.
type TTSSession struct {
	input  *bufferStream
	output genx.Stream
}

// Send sends text to the TTS session.
func (s *TTSSession) Send(text string) error {
	chunk := &genx.MessageChunk{Part: genx.Text(text)}
	return s.input.Push(chunk)
}

// Close signals the end of text input.
func (s *TTSSession) Close() error {
	eosChunk := genx.NewTextEndOfStream()
	if err := s.input.Push(eosChunk); err != nil {
		return err
	}
	return s.input.Close()
}

// Output returns the output stream for receiving audio chunks.
func (s *TTSSession) Output() genx.Stream {
	return s.output
}

// CloseAll closes both input and output streams.
func (s *TTSSession) CloseAll() error {
	s.input.Close()
	if s.output != nil {
		return s.output.Close()
	}
	return nil
}
