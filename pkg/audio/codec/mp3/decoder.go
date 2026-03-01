package mp3

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"

	gommp3 "github.com/hajimehoshi/go-mp3"
)

var errInvalidMP3Data = errors.New("mp3: invalid or empty stream")

// Decoder decodes MP3 audio into interleaved PCM16LE bytes.
//
// The decoder output is stereo PCM data as provided by go-mp3.
// Bitrate metadata is not exported by the underlying decoder and is 0.
type Decoder struct {
	r io.Reader

	mu       sync.Mutex
	dec      *gommp3.Decoder
	initErr  error
	inited   bool
	closed   atomic.Bool
	channels int
	bitrate  int
}

// NewDecoder creates a new MP3 decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) init() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.inited {
		return d.initErr
	}
	d.inited = true

	if d.r == nil {
		d.initErr = errors.New("mp3: nil reader")
		return d.initErr
	}

	dec, err := gommp3.NewDecoder(d.r)
	if err != nil {
		d.initErr = err
		return d.initErr
	}

	d.dec = dec
	d.channels = 2
	return nil
}

// Close marks decoder as closed. Safe to call multiple times.
func (d *Decoder) Close() error {
	if d == nil {
		return nil
	}
	if d.closed.CompareAndSwap(false, true) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.dec = nil
	}
	return nil
}

// SampleRate returns the sample rate of the stream.
// Returns 0 before the first successful initialization.
func (d *Decoder) SampleRate() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dec == nil {
		return 0
	}
	return d.dec.SampleRate()
}

// Channels returns channel count. It is 2 once initialized.
func (d *Decoder) Channels() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channels
}

// Bitrate returns bitrate in kbps.
// go-mp3 does not expose bitrate, so this is always 0.
func (d *Decoder) Bitrate() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bitrate
}

// Read reads decoded PCM bytes into p.
func (d *Decoder) Read(p []byte) (int, error) {
	if d == nil {
		return 0, errors.New("mp3: nil decoder")
	}
	if len(p) == 0 {
		return 0, nil
	}
	if d.closed.Load() {
		return 0, errors.New("mp3: decoder is closed")
	}
	if err := d.init(); err != nil {
		return 0, err
	}

	d.mu.Lock()
	dec := d.dec
	d.mu.Unlock()
	if dec == nil {
		return 0, errors.New("mp3: decoder is closed")
	}

	n, err := dec.Read(p)
	if err == io.EOF && n > 0 {
		return n, nil
	}
	return n, err
}

// DecodeFull decodes all data from r and returns PCM bytes and basic format metadata.
func DecodeFull(r io.Reader) (pcm []byte, sampleRate, channels int, err error) {
	dec := NewDecoder(r)
	defer func() {
		_ = dec.Close()
	}()

	buf := make([]byte, 8192)
	var out []byte
	for {
		n, readErr := dec.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, 0, readErr
		}
	}

	if len(out) == 0 {
		return nil, 0, 0, errInvalidMP3Data
	}

	return out, dec.SampleRate(), dec.Channels(), nil
}
