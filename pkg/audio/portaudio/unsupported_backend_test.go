//go:build !(cgo && ((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))))

package portaudio

import (
	"strings"
	"testing"
)

func TestUnsupportedBackendValidation(t *testing.T) {
	b := unsupportedBackend{}
	if got := b.Name(); got != "unsupported" {
		t.Fatalf("Name=%q, want unsupported", got)
	}
	if err := b.Terminate(); err != nil {
		t.Fatalf("Terminate should be nil, got %v", err)
	}
	if _, err := b.ListDevices(); err == nil || !strings.Contains(err.Error(), "unsupported platform") {
		t.Fatalf("expected unsupported platform error, got %v", err)
	}
	if _, err := b.OpenStream(directionInput, StreamConfig{SampleRate: 0, Channels: 1, FramesPerBuffer: 1}); err == nil {
		t.Fatal("expected config validation error")
	}
	if err := b.IsFormatSupported(directionInput, StreamConfig{SampleRate: 16000, Channels: 1, FramesPerBuffer: 320}); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
