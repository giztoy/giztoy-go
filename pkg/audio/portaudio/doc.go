// Package portaudio provides device IO abstraction backed by PortAudio.
//
// The package keeps a strict split between:
//   - native backend (requires cgo on a supported platform), and
//   - portable fallback backend that returns clear unsupported-platform errors.
//
// For PCM integration, use OpenCapture/OpenPlayback with pkg/audio/pcm formats,
// or OpenPCMPlaybackWriter to write pcm.Chunk directly.
package portaudio
