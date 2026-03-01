//go:build cgo && ((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64)))

package mp3

/*
#cgo darwin,arm64 CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/lame/darwin-arm64/include
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/lame/darwin-arm64/lib -lmp3lame -liconv -lm

#cgo darwin,amd64 CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/lame/darwin-amd64/include
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/lame/darwin-amd64/lib -lmp3lame -liconv -lm

#cgo linux,amd64 CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/lame/linux-amd64/include
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/lame/linux-amd64/lib -lmp3lame -lm

#cgo linux,arm64 CFLAGS: -I${SRCDIR}/../../../../third_party/audio/prebuilt/lame/linux-arm64/include
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../../third_party/audio/prebuilt/lame/linux-arm64/lib -lmp3lame -lm

#include <lame/lame.h>

static int lame_encode_interleaved_wrapper(lame_t gf, const short* pcm, int num_samples, unsigned char* mp3buf, int mp3buf_size) {
    return lame_encode_buffer_interleaved(gf, (short*)pcm, num_samples, mp3buf, mp3buf_size);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

const nativeEncoderEnabled = true

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
// If kbps <= 0, VBR mode is used.
func WithBitrate(kbps int) EncoderOption {
	return func(e *Encoder) {
		if e == nil {
			return
		}
		e.bitrate = kbps
	}
}

// Encoder encodes PCM16LE bytes into MP3 stream data.
//
// Input PCM must be interleaved by channel.
type Encoder struct {
	w io.Writer

	mu         sync.Mutex
	lame       C.lame_t
	sampleRate int
	channels   int
	quality    Quality
	bitrate    int
	inited     bool
	closed     atomic.Bool
	mp3buf     []byte
	pending    []byte
}

func clampQuality(q Quality) Quality {
	if q < QualityBest {
		return QualityBest
	}
	if q > QualityWorst {
		return QualityWorst
	}
	return q
}

// NewEncoder creates an MP3 encoder writing encoded bytes into w.
func NewEncoder(w io.Writer, sampleRate, channels int, opts ...EncoderOption) (*Encoder, error) {
	if w == nil {
		return nil, errors.New("mp3: writer is nil")
	}
	if sampleRate <= 0 {
		return nil, fmt.Errorf("mp3: invalid sample rate %d", sampleRate)
	}
	if channels != 1 && channels != 2 {
		return nil, errors.New("mp3: channels must be 1 or 2")
	}

	e := &Encoder{
		w:          w,
		sampleRate: sampleRate,
		channels:   channels,
		quality:    QualityMedium,
		mp3buf:     make([]byte, 8192),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}

	e.quality = clampQuality(e.quality)
	if e.bitrate < 0 {
		e.bitrate = 0
	}

	return e, nil
}

func (e *Encoder) init() error {
	if e.inited {
		return nil
	}

	lame := C.lame_init()
	if lame == nil {
		return errors.New("mp3: failed to initialize LAME")
	}

	if C.lame_set_in_samplerate(lame, C.int(e.sampleRate)) != 0 {
		C.lame_close(lame)
		return errors.New("mp3: failed to set input sample rate")
	}

	if C.lame_set_num_channels(lame, C.int(e.channels)) != 0 {
		C.lame_close(lame)
		return errors.New("mp3: failed to set channel count")
	}

	if e.channels == 1 {
		if C.lame_set_mode(lame, C.MONO) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set mono mode")
		}
	} else {
		if C.lame_set_mode(lame, C.JOINT_STEREO) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set stereo mode")
		}
	}

	if e.bitrate > 0 {
		if C.lame_set_VBR(lame, C.vbr_off) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set CBR mode")
		}
		if C.lame_set_brate(lame, C.int(e.bitrate)) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set bitrate")
		}
	} else {
		if C.lame_set_VBR(lame, C.vbr_default) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set VBR mode")
		}
		if C.lame_set_VBR_quality(lame, C.float(e.quality)) != 0 {
			C.lame_close(lame)
			return errors.New("mp3: failed to set VBR quality")
		}
	}

	if C.lame_init_params(lame) < 0 {
		C.lame_close(lame)
		return errors.New("mp3: failed to initialize LAME parameters")
	}

	e.lame = lame
	e.inited = true
	runtime.SetFinalizer(e, func(enc *Encoder) {
		_ = enc.Close()
	})
	return nil
}

// Write encodes PCM16LE input bytes.
func (e *Encoder) Write(pcm []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed.Load() {
		return 0, errors.New("mp3: encoder is closed")
	}
	if err := e.init(); err != nil {
		return 0, err
	}
	if len(pcm) == 0 {
		return 0, nil
	}

	bytesPerSample := 2 * e.channels
	combined := pcm
	if len(e.pending) > 0 {
		combined = make([]byte, len(e.pending)+len(pcm))
		copy(combined, e.pending)
		copy(combined[len(e.pending):], pcm)
	}

	alignedLen := len(combined) - (len(combined) % bytesPerSample)
	if alignedLen == 0 {
		e.pending = append(e.pending[:0], combined...)
		return len(pcm), nil
	}

	encodeInput := combined[:alignedLen]
	remainder := combined[alignedLen:]
	numSamples := alignedLen / bytesPerSample

	requiredSize := numSamples*5/4 + 7200
	if len(e.mp3buf) < requiredSize {
		e.mp3buf = make([]byte, requiredSize)
	}

	var encoded C.int
	if e.channels == 2 {
		encoded = C.lame_encode_interleaved_wrapper(
			e.lame,
			(*C.short)(unsafe.Pointer(&encodeInput[0])),
			C.int(numSamples),
			(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
			C.int(len(e.mp3buf)),
		)
	} else {
		encoded = C.lame_encode_buffer(
			e.lame,
			(*C.short)(unsafe.Pointer(&encodeInput[0])),
			nil,
			C.int(numSamples),
			(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
			C.int(len(e.mp3buf)),
		)
	}

	if encoded < 0 {
		return 0, fmt.Errorf("mp3: encode failed with code %d", int(encoded))
	}

	if encoded > 0 {
		if _, err := e.w.Write(e.mp3buf[:encoded]); err != nil {
			return 0, err
		}
	}

	if len(remainder) == 0 {
		e.pending = e.pending[:0]
	} else {
		e.pending = append(e.pending[:0], remainder...)
	}

	return len(pcm), nil
}

// Flush flushes internal encoder buffers.
func (e *Encoder) Flush() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed.Load() || !e.inited || e.lame == nil {
		return nil
	}
	if len(e.pending) > 0 {
		return fmt.Errorf("mp3: incomplete pcm frame: %d trailing bytes", len(e.pending))
	}

	if len(e.mp3buf) < 7200 {
		e.mp3buf = make([]byte, 7200)
	}

	encoded := C.lame_encode_flush(
		e.lame,
		(*C.uchar)(unsafe.Pointer(&e.mp3buf[0])),
		C.int(len(e.mp3buf)),
	)

	if encoded < 0 {
		return fmt.Errorf("mp3: flush failed with code %d", int(encoded))
	}

	if encoded > 0 {
		if _, err := e.w.Write(e.mp3buf[:encoded]); err != nil {
			return err
		}
	}

	return nil
}

// Close releases native encoder resources. Safe to call multiple times.
func (e *Encoder) Close() error {
	if e == nil {
		return nil
	}
	if e.closed.CompareAndSwap(false, true) {
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.lame != nil {
			C.lame_close(e.lame)
			e.lame = nil
		}
		e.pending = nil
		runtime.SetFinalizer(e, nil)
	}
	return nil
}

// EncodePCMStream encodes all PCM bytes from pcm into MP3 written to w.
func EncodePCMStream(w io.Writer, pcm io.Reader, sampleRate, channels int, opts ...EncoderOption) (written int64, err error) {
	if pcm == nil {
		return 0, errors.New("mp3: reader is nil")
	}

	enc, err := NewEncoder(w, sampleRate, channels, opts...)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = enc.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, readErr := pcm.Read(buf)
		if n > 0 {
			wn, writeErr := enc.Write(buf[:n])
			written += int64(wn)
			if writeErr != nil {
				return written, writeErr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, readErr
		}
	}

	if err := enc.Flush(); err != nil {
		return written, err
	}

	return written, nil
}
