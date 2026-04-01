package portaudio

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/giztoy/giztoy-go/pkg/audio/pcm"
)

// PCMPlaybackWriter adapts PlaybackStream to pcm.WriteCloser.
type PCMPlaybackWriter struct {
	stream *PlaybackStream
	format pcm.Format
}

// OpenPCMPlaybackWriter opens playback and returns a pcm.WriteCloser.
func OpenPCMPlaybackWriter(format pcm.Format, opts PlaybackOptions) (pcm.WriteCloser, error) {
	stream, err := OpenPlayback(format, opts)
	if err != nil {
		return nil, err
	}
	return &PCMPlaybackWriter{stream: stream, format: format}, nil
}

// Write writes a pcm.Chunk into playback stream.
func (w *PCMPlaybackWriter) Write(chunk pcm.Chunk) error {
	if w == nil || w.stream == nil {
		return errors.New("portaudio: nil pcm playback writer")
	}
	if chunk == nil {
		return errors.New("portaudio: nil pcm chunk")
	}
	if got := chunk.Format(); got != w.format {
		return fmt.Errorf("portaudio: pcm chunk format mismatch: got %s, want %s", got, w.format)
	}
	_, err := chunk.WriteTo(w.stream)
	return err
}

// Close closes underlying playback stream.
func (w *PCMPlaybackWriter) Close() error {
	if w == nil || w.stream == nil {
		return nil
	}
	return w.stream.Close()
}

// ReadPCMChunk reads a fixed-duration chunk from a capture stream.
func ReadPCMChunk(stream io.Reader, format pcm.Format, duration time.Duration) (pcm.Chunk, error) {
	if stream == nil {
		return nil, errors.New("portaudio: capture reader is nil")
	}
	if duration <= 0 {
		return nil, fmt.Errorf("portaudio: duration must be > 0, got %s", duration)
	}
	return format.ReadChunk(stream, duration)
}
