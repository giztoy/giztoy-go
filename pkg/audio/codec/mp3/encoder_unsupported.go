//go:build !cgo || !(linux || darwin) || ((linux || darwin) && cgo && !amd64 && !arm64)

package mp3

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
)

const nativeEncoderEnabled = false

// Quality is the VBR quality level for MP3 encoding.
// 0 is best quality and 9 is worst quality.
type Quality int

const (
	QualityBest   Quality = 0
	QualityHigh   Quality = 2
	QualityMedium Quality = 5
	QualityLow    Quality = 7
	QualityWorst  Quality = 9
)

// EncoderOption configures an Encoder.
type EncoderOption func(*Encoder)

// WithQuality configures VBR quality mode.
func WithQuality(q Quality) EncoderOption {
	return func(e *Encoder) {
		if e == nil {
			return
		}
		e.quality = q
	}
}

// WithBitrate configures CBR mode in kbps.
func WithBitrate(kbps int) EncoderOption {
	return func(e *Encoder) {
		if e == nil {
			return
		}
		e.bitrate = kbps
	}
}

func unsupportedEncoderErr() error {
	return fmt.Errorf(
		"mp3: unsupported platform %s/%s for encoder: supports %s with cgo",
		runtime.GOOS,
		runtime.GOARCH,
		supportedPlatformDescription,
	)
}

// Encoder is unavailable on unsupported platforms.
type Encoder struct {
	quality Quality
	bitrate int
	closed  atomic.Bool
}

// NewEncoder returns unsupported platform error.
func NewEncoder(w io.Writer, sampleRate, channels int, opts ...EncoderOption) (*Encoder, error) {
	_ = sampleRate
	_ = opts
	if w == nil {
		return nil, errors.New("mp3: writer is nil")
	}
	if channels != 1 && channels != 2 {
		return nil, errors.New("mp3: channels must be 1 or 2")
	}
	return nil, unsupportedEncoderErr()
}

// Write returns unsupported platform error.
func (e *Encoder) Write(_ []byte) (int, error) {
	if e == nil || e.closed.Load() {
		return 0, errors.New("mp3: encoder is closed")
	}
	return 0, unsupportedEncoderErr()
}

// Flush returns unsupported platform error.
func (e *Encoder) Flush() error {
	if e == nil || e.closed.Load() {
		return errors.New("mp3: encoder is closed")
	}
	return unsupportedEncoderErr()
}

// Close marks encoder closed.
func (e *Encoder) Close() error {
	if e == nil {
		return nil
	}
	e.closed.Store(true)
	return nil
}

// EncodePCMStream returns unsupported platform error.
func EncodePCMStream(w io.Writer, pcm io.Reader, sampleRate, channels int, opts ...EncoderOption) (int64, error) {
	_ = pcm
	if _, err := NewEncoder(w, sampleRate, channels, opts...); err != nil {
		return 0, err
	}
	return 0, unsupportedEncoderErr()
}
