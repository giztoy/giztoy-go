package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/audio/codec/mp3"
	"github.com/giztoy/giztoy-go/pkg/audio/codec/ogg"
	"github.com/giztoy/giztoy-go/pkg/audio/codec/opus"
	"github.com/giztoy/giztoy-go/pkg/audio/pcm"
	"github.com/giztoy/giztoy-go/pkg/audio/portaudio"
	"github.com/giztoy/giztoy-go/pkg/audio/resampler"
)

func patchDeps(t *testing.T) {
	t.Helper()

	origOpenPlayback := openPlaybackFn
	origOpenCapture := openCaptureFn
	origNewMP3Encoder := newMP3EncoderFn
	origDecodeMP3 := decodeMP3Fn
	origNewResampler := newResamplerFn
	origOpusRuntimeSupported := opusRuntimeSupportedFn
	origNewOpusEncoder := newOpusEncoderFn
	origNewOpusDecoder := newOpusDecoderFn
	origNewOGGStreamWriter := newOGGStreamWriterFn
	origReadAllOGGPackets := readAllOGGPacketsFn
	origNow := nowFn

	t.Cleanup(func() {
		openPlaybackFn = origOpenPlayback
		openCaptureFn = origOpenCapture
		newMP3EncoderFn = origNewMP3Encoder
		decodeMP3Fn = origDecodeMP3
		newResamplerFn = origNewResampler
		opusRuntimeSupportedFn = origOpusRuntimeSupported
		newOpusEncoderFn = origNewOpusEncoder
		newOpusDecoderFn = origNewOpusDecoder
		newOGGStreamWriterFn = origNewOGGStreamWriter
		readAllOGGPacketsFn = origReadAllOGGPackets
		nowFn = origNow
	})
}

type fakePlayback struct {
	buf      bytes.Buffer
	writeErr error
	closeErr error
}

func (p *fakePlayback) Write(b []byte) (int, error) {
	if p.writeErr != nil {
		return 0, p.writeErr
	}
	return p.buf.Write(b)
}

func (p *fakePlayback) Close() error {
	return p.closeErr
}

type fakeCapture struct {
	cfg      portaudio.StreamConfig
	chunks   [][]byte
	idx      int
	closeErr error
}

func (c *fakeCapture) Config() portaudio.StreamConfig {
	return c.cfg
}

func (c *fakeCapture) Read(p []byte) (int, error) {
	if c.idx >= len(c.chunks) {
		return 0, io.EOF
	}
	n := copy(p, c.chunks[c.idx])
	c.idx++
	return n, nil
}

func (c *fakeCapture) Close() error {
	return c.closeErr
}

type fakeMP3Encoder struct {
	bytesWritten int
	writeErr     error
	flushErr     error
	closeErr     error
	flushed      bool
	closed       bool
}

func (e *fakeMP3Encoder) Write(p []byte) (int, error) {
	if e.writeErr != nil {
		return 0, e.writeErr
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

func (e *fakeMP3Encoder) Flush() error {
	e.flushed = true
	return e.flushErr
}

func (e *fakeMP3Encoder) Close() error {
	e.closed = true
	return e.closeErr
}

type fakeOpusEncoder struct {
	err error
}

func (e *fakeOpusEncoder) Encode(samples []int16, frameSize int) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	_ = frameSize
	return int16ToBytes(samples), nil
}

func (e *fakeOpusEncoder) Close() error { return nil }

type fakeOpusDecoder struct {
	err error
}

func (d *fakeOpusDecoder) Decode(packet []byte, frameSize int, fec bool) ([]int16, error) {
	if d.err != nil {
		return nil, d.err
	}
	_, _ = frameSize, fec
	return bytesToInt16(packet), nil
}

func (d *fakeOpusDecoder) Close() error { return nil }

type fakeOGGWriter struct {
	packets [][]byte
	eos     []bool
	err     error
}

func (w *fakeOGGWriter) WritePacket(packet []byte, granulePos uint64, eos bool) (int, error) {
	_ = granulePos
	if w.err != nil {
		return 0, w.err
	}
	w.packets = append(w.packets, append([]byte(nil), packet...))
	w.eos = append(w.eos, eos)
	return len(packet), nil
}

func TestParsePCMFormat(t *testing.T) {
	cases := []struct {
		in   string
		want pcm.Format
		ok   bool
	}{
		{in: "16k", want: pcm.L16Mono16K, ok: true},
		{in: "16000", want: pcm.L16Mono16K, ok: true},
		{in: "24k", want: pcm.L16Mono24K, ok: true},
		{in: "48k", want: pcm.L16Mono48K, ok: true},
		{in: "44k", ok: false},
	}

	for _, tc := range cases {
		got, err := parsePCMFormat(tc.in)
		if tc.ok {
			if err != nil {
				t.Fatalf("parsePCMFormat(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("parsePCMFormat(%q)=%v, want %v", tc.in, got, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("parsePCMFormat(%q) expected error", tc.in)
		}
	}
}

func TestParseSongIDs(t *testing.T) {
	ids, err := parseSongIDs("twinkle_star", "")
	if err != nil {
		t.Fatalf("parseSongIDs default error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "twinkle_star" {
		t.Fatalf("parseSongIDs default=%v", ids)
	}

	ids, err = parseSongIDs("ignored", "twinkle_star, canon,twinkle_star,  canon_3voice")
	if err != nil {
		t.Fatalf("parseSongIDs csv error: %v", err)
	}
	got := strings.Join(ids, ",")
	want := "twinkle_star,canon,canon_3voice"
	if got != want {
		t.Fatalf("parseSongIDs csv=%q, want %q", got, want)
	}

	if _, err := parseSongIDs("", ""); err == nil {
		t.Fatal("parseSongIDs expected empty error")
	}
	if _, err := parseSongIDs("ignored", " , , "); err == nil {
		t.Fatal("parseSongIDs expected empty csv error")
	}
}

func TestResolveSongs(t *testing.T) {
	selected, err := resolveSongs([]string{"twinkle_star", "happy_birthday"})
	if err != nil {
		t.Fatalf("resolveSongs valid error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("resolveSongs len=%d, want 2", len(selected))
	}

	if _, err := resolveSongs(nil); err == nil {
		t.Fatal("resolveSongs expected empty ids error")
	}
	if _, err := resolveSongs([]string{"not_exists"}); err == nil {
		t.Fatal("resolveSongs expected unknown id error")
	}
}

func TestClampVolume(t *testing.T) {
	if got := clampVolume(-1); got != 0 {
		t.Fatalf("clampVolume(-1)=%v, want 0", got)
	}
	if got := clampVolume(2); got != 1 {
		t.Fatalf("clampVolume(2)=%v, want 1", got)
	}
	if got := clampVolume(0.3); got != 0.3 {
		t.Fatalf("clampVolume(0.3)=%v, want 0.3", got)
	}
}

func TestConfigValidate(t *testing.T) {
	patchDeps(t)
	nowFn = func() time.Time { return time.Date(2026, 3, 2, 15, 4, 5, 0, time.UTC) }

	cfg := config{
		mode:    modePlaySong,
		songID:  "twinkle_star",
		volume:  0.5,
		bitrate: 96,
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate play-song: %v", err)
	}

	cfg = config{
		mode:    modeRecordMic,
		volume:  0.4,
		bitrate: 96,
		timeout: 3 * time.Second,
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate record-mic: %v", err)
	}
	if cfg.outputMP3 != "recording-20260302-150405.mp3" {
		t.Fatalf("validate record-mic should set deterministic output, got %q", cfg.outputMP3)
	}

	cfg = config{mode: modeRecordMic, volume: 0.4, bitrate: 96, timeout: 0}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate expected timeout error")
	}

	cfg = config{mode: modePlayMP3, volume: 0.4, bitrate: 96}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate expected input mp3 required error")
	}

	cfg = config{mode: modeRecordOGG, volume: 0.4, bitrate: 96, timeout: 3 * time.Second}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate record-ogg: %v", err)
	}
	if cfg.outputOGG != "recording-20260302-150405.ogg" {
		t.Fatalf("validate record-ogg should set deterministic output, got %q", cfg.outputOGG)
	}

	cfg = config{mode: modePlayOGG, volume: 0.4, bitrate: 96}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate expected input ogg required error")
	}

	cfg = config{mode: mode("unknown"), volume: 0.4, bitrate: 96}
	if err := cfg.validate(); err == nil {
		t.Fatal("validate expected mode error")
	}
}

func TestParseConfig(t *testing.T) {
	patchDeps(t)

	cfg, err := parseConfig([]string{"-mode", "play-song", "-song", "twinkle_star", "-format", "24k"})
	if err != nil {
		t.Fatalf("parseConfig play-song: %v", err)
	}
	if cfg.mode != modePlaySong {
		t.Fatalf("mode=%q, want %q", cfg.mode, modePlaySong)
	}
	if cfg.format != pcm.L16Mono24K {
		t.Fatalf("format=%v, want %v", cfg.format, pcm.L16Mono24K)
	}

	if _, err := parseConfig([]string{"-mode", "play-mp3"}); err == nil {
		t.Fatal("parseConfig play-mp3 expected input required error")
	}

	if _, err := parseConfig([]string{"-mode", "play-ogg"}); err == nil {
		t.Fatal("parseConfig play-ogg expected input required error")
	}

	cfg, err = parseConfig([]string{"-mode", "record-ogg", "-timeout", "2s", "-output-ogg", "out/test.ogg"})
	if err != nil {
		t.Fatalf("parseConfig record-ogg: %v", err)
	}
	if cfg.outputOGG != "out/test.ogg" {
		t.Fatalf("outputOGG=%q, want out/test.ogg", cfg.outputOGG)
	}

	if _, err := parseConfig([]string{"-mode", "record-mic", "-timeout", "-1s"}); err == nil {
		t.Fatal("parseConfig record-mic expected timeout error")
	}

	if _, err := parseConfig([]string{"-mode", "play-song", "-format", "44k"}); err == nil {
		t.Fatal("parseConfig expected format error")
	}
}

func TestPlayReaderWithPortAudio(t *testing.T) {
	patchDeps(t)

	fake := &fakePlayback{}
	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_ = format
		_ = opts
		return fake, nil
	}

	cfg := config{format: pcm.L16Mono16K}
	if err := playReaderWithPortAudio(cfg, bytes.NewReader(make([]byte, 640))); err != nil {
		t.Fatalf("playReaderWithPortAudio: %v", err)
	}
	if fake.buf.Len() == 0 {
		t.Fatal("playback should receive bytes")
	}

	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_, _ = format, opts
		return nil, errors.New("unsupported platform")
	}
	if err := playReaderWithPortAudio(cfg, bytes.NewReader(make([]byte, 640))); err == nil || !strings.Contains(err.Error(), "CGO_ENABLED=1") {
		t.Fatalf("expected portaudio hint error, got %v", err)
	}
}

func TestRecordMicrophoneToMP3(t *testing.T) {
	patchDeps(t)

	encoder := &fakeMP3Encoder{}
	newMP3EncoderFn = func(w io.Writer, sampleRate, channels int, opts ...mp3.EncoderOption) (mp3Encoder, error) {
		_, _ = w, opts
		if sampleRate != 16000 || channels != 1 {
			t.Fatalf("unexpected encoder params sampleRate=%d channels=%d", sampleRate, channels)
		}
		return encoder, nil
	}

	openCaptureFn = func(format pcm.Format, opts portaudio.CaptureOptions) (captureReader, error) {
		_ = opts
		if format != pcm.L16Mono16K {
			t.Fatalf("unexpected capture format=%v", format)
		}
		return &fakeCapture{
			cfg: portaudio.StreamConfig{SampleRate: 16000, Channels: 1, FramesPerBuffer: 160},
			chunks: [][]byte{
				make([]byte, 320),
				make([]byte, 320),
			},
		}, nil
	}

	out := filepath.Join(t.TempDir(), "mic.mp3")
	cfg := config{
		mode:      modeRecordMic,
		format:    pcm.L16Mono16K,
		timeout:   200 * time.Millisecond,
		outputMP3: out,
		bitrate:   96,
	}

	if err := recordMicrophoneToMP3(cfg); err != nil {
		t.Fatalf("recordMicrophoneToMP3: %v", err)
	}
	if !encoder.flushed {
		t.Fatal("encoder should be flushed")
	}
	if encoder.bytesWritten == 0 {
		t.Fatal("encoder should receive pcm bytes")
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output file should exist: %v", err)
	}
}

func TestDecodeAndResampleMP3AndPlayMP3(t *testing.T) {
	patchDeps(t)

	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return make([]byte, 640), 16000, 1, nil
	}

	newResamplerFn = func(src io.Reader, srcFmt, dstFmt resampler.Format) (io.ReadCloser, error) {
		if srcFmt.SampleRate != 16000 || srcFmt.Stereo {
			t.Fatalf("unexpected srcFmt=%+v", srcFmt)
		}
		if dstFmt.SampleRate != 16000 || dstFmt.Stereo {
			t.Fatalf("unexpected dstFmt=%+v", dstFmt)
		}
		b, err := io.ReadAll(src)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}

	fake := &fakePlayback{}
	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_, _ = format, opts
		return fake, nil
	}

	in := filepath.Join(t.TempDir(), "in.mp3")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write input mp3 failed: %v", err)
	}

	cfg := config{mode: modePlayMP3, format: pcm.L16Mono16K, inputMP3: in, bitrate: 96, volume: 0.5}
	if err := playMP3File(cfg); err != nil {
		t.Fatalf("playMP3File: %v", err)
	}
	if fake.buf.Len() == 0 {
		t.Fatal("playMP3File should write bytes to playback")
	}
}

func TestNewOpusLoopbackReader(t *testing.T) {
	patchDeps(t)

	opusRuntimeSupportedFn = func() bool { return true }
	newOpusEncoderFn = func(sampleRate, channels int, app opus.Application) (opusEncoder, error) {
		if sampleRate != 16000 || channels != 1 || app != opus.ApplicationAudio {
			t.Fatalf("unexpected opus encoder args: %d %d %d", sampleRate, channels, app)
		}
		return &fakeOpusEncoder{}, nil
	}
	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		if sampleRate != 16000 || channels != 1 {
			t.Fatalf("unexpected opus decoder args: %d %d", sampleRate, channels)
		}
		return &fakeOpusDecoder{}, nil
	}

	input := bytes.Repeat([]byte{1, 2}, 500)
	r, err := newOpusLoopbackReader(bytes.NewReader(input), pcm.L16Mono16K)
	if err != nil {
		t.Fatalf("newOpusLoopbackReader: %v", err)
	}
	defer func() {
		_ = r.Close()
	}()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read loopback: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("loopback output should not be empty")
	}

	opusRuntimeSupportedFn = func() bool { return false }
	if _, err := newOpusLoopbackReader(bytes.NewReader(input), pcm.L16Mono16K); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}

func TestMiscHelpers(t *testing.T) {
	if err := ensureParentDir(""); err == nil {
		t.Fatal("ensureParentDir empty path should error")
	}
	if err := ensureParentDir("out.mp3"); err != nil {
		t.Fatalf("ensureParentDir current dir path failed: %v", err)
	}

	p := filepath.Join(t.TempDir(), "a", "b", "c.mp3")
	if err := ensureParentDir(p); err != nil {
		t.Fatalf("ensureParentDir failed: %v", err)
	}

	if got := defaultRecordingFileName("mp3"); !strings.HasPrefix(got, "recording-") || !strings.HasSuffix(got, ".mp3") {
		t.Fatalf("defaultRecordingFileName=%q", got)
	}

	if err := withPortAudioHint(errors.New("unsupported platform")); err == nil || !strings.Contains(err.Error(), "CGO_ENABLED=1") {
		t.Fatalf("withPortAudioHint should append hint, got %v", err)
	}
	if err := withPortAudioHint(errors.New("other")); err == nil || strings.Contains(err.Error(), "CGO_ENABLED=1") {
		t.Fatalf("withPortAudioHint should keep non-platform errors, got %v", err)
	}

	var sb strings.Builder
	if err := listSongs(&sb); err != nil {
		t.Fatalf("listSongs: %v", err)
	}
	if sb.Len() == 0 {
		t.Fatal("listSongs output should not be empty")
	}
	if err := listSongs(nil); err == nil {
		t.Fatal("listSongs nil writer should error")
	}

	sb.Reset()
	printUsage(&sb)
	if !strings.Contains(sb.String(), "-mode play-song") {
		t.Fatalf("printUsage output mismatch: %s", sb.String())
	}
}

func TestPlaySongsAndRunModes(t *testing.T) {
	patchDeps(t)

	fake := &fakePlayback{}
	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_, _ = format, opts
		return fake, nil
	}

	// single song
	if err := playSongs(config{
		format:    pcm.L16Mono16K,
		songID:    "twinkle_star",
		volume:    0.4,
		richSound: true,
		bitrate:   96,
	}); err != nil {
		t.Fatalf("playSongs(single): %v", err)
	}

	// multi-track mix
	if err := playSongs(config{
		format:    pcm.L16Mono16K,
		songsCSV:  "twinkle_star,canon",
		volume:    0.4,
		richSound: true,
		bitrate:   96,
	}); err != nil {
		t.Fatalf("playSongs(multi): %v", err)
	}

	if err := run(config{mode: modeListSongs}); err != nil {
		t.Fatalf("run(list): %v", err)
	}
	if err := run(config{mode: modePlaySong, format: pcm.L16Mono16K, songID: "twinkle_star", volume: 0.5, richSound: true, bitrate: 96}); err != nil {
		t.Fatalf("run(play-song): %v", err)
	}

	in := filepath.Join(t.TempDir(), "in.mp3")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write input failed: %v", err)
	}
	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return make([]byte, 640), 16000, 1, nil
	}
	newResamplerFn = func(src io.Reader, srcFmt, dstFmt resampler.Format) (io.ReadCloser, error) {
		b, err := io.ReadAll(src)
		if err != nil {
			return nil, err
		}
		_ = srcFmt
		_ = dstFmt
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	if err := run(config{mode: modePlayMP3, format: pcm.L16Mono16K, inputMP3: in, bitrate: 96, volume: 0.5}); err != nil {
		t.Fatalf("run(play-mp3): %v", err)
	}

	openCaptureFn = func(format pcm.Format, opts portaudio.CaptureOptions) (captureReader, error) {
		_, _ = format, opts
		return &fakeCapture{cfg: portaudio.StreamConfig{SampleRate: 16000, Channels: 1, FramesPerBuffer: 160}}, nil
	}
	newMP3EncoderFn = func(w io.Writer, sampleRate, channels int, opts ...mp3.EncoderOption) (mp3Encoder, error) {
		_, _, _, _ = w, sampleRate, channels, opts
		return &fakeMP3Encoder{}, nil
	}
	out := filepath.Join(t.TempDir(), "record.mp3")
	if err := run(config{mode: modeRecordMic, format: pcm.L16Mono16K, timeout: 20 * time.Millisecond, outputMP3: out, bitrate: 96, volume: 0.5}); err != nil {
		t.Fatalf("run(record-mic): %v", err)
	}

	if err := run(config{mode: mode("unknown")}); err == nil {
		t.Fatal("run(unknown) expected error")
	}
}

func TestDecodeAndResampleMP3Errors(t *testing.T) {
	patchDeps(t)

	if _, err := decodeAndResampleMP3(filepath.Join(t.TempDir(), "not-exists.mp3"), pcm.L16Mono16K); err == nil {
		t.Fatal("decodeAndResampleMP3 expected open error")
	}

	in := filepath.Join(t.TempDir(), "in.mp3")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write input failed: %v", err)
	}

	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return nil, 0, 0, errors.New("decode failed")
	}
	if _, err := decodeAndResampleMP3(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "decode mp3 failed") {
		t.Fatalf("expected decode failure, got %v", err)
	}

	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return make([]byte, 10), 16000, 3, nil
	}
	if _, err := decodeAndResampleMP3(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "unsupported decoded mp3 channels") {
		t.Fatalf("expected channels error, got %v", err)
	}

	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return make([]byte, 10), 16000, 1, nil
	}
	newResamplerFn = func(src io.Reader, srcFmt, dstFmt resampler.Format) (io.ReadCloser, error) {
		_, _, _ = src, srcFmt, dstFmt
		return nil, errors.New("resampler failed")
	}
	if _, err := decodeAndResampleMP3(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "create resampler failed") {
		t.Fatalf("expected resampler error, got %v", err)
	}
}

func TestRecordAndPlayErrorBranches(t *testing.T) {
	patchDeps(t)

	openCaptureFn = func(format pcm.Format, opts portaudio.CaptureOptions) (captureReader, error) {
		_, _ = format, opts
		return nil, errors.New("unsupported platform")
	}
	err := recordMicrophoneToMP3(config{format: pcm.L16Mono16K, timeout: 10 * time.Millisecond, outputMP3: filepath.Join(t.TempDir(), "a.mp3"), bitrate: 96})
	if err == nil || !strings.Contains(err.Error(), "CGO_ENABLED=1") {
		t.Fatalf("expected portaudio hint error, got %v", err)
	}

	openCaptureFn = func(format pcm.Format, opts portaudio.CaptureOptions) (captureReader, error) {
		_, _ = format, opts
		return &fakeCapture{cfg: portaudio.StreamConfig{SampleRate: 16000, Channels: 1, FramesPerBuffer: 160}}, nil
	}
	newMP3EncoderFn = func(w io.Writer, sampleRate, channels int, opts ...mp3.EncoderOption) (mp3Encoder, error) {
		_, _, _, _ = w, sampleRate, channels, opts
		return nil, errors.New("encoder failed")
	}
	err = recordMicrophoneToMP3(config{format: pcm.L16Mono16K, timeout: 10 * time.Millisecond, outputMP3: filepath.Join(t.TempDir(), "b.mp3"), bitrate: 96})
	if err == nil || !strings.Contains(err.Error(), "create mp3 encoder failed") {
		t.Fatalf("expected create encoder error, got %v", err)
	}

	decodeMP3Fn = func(r io.Reader) ([]byte, int, int, error) {
		_, _ = io.Copy(io.Discard, r)
		return make([]byte, 640), 16000, 1, nil
	}
	newResamplerFn = func(src io.Reader, srcFmt, dstFmt resampler.Format) (io.ReadCloser, error) {
		b, _ := io.ReadAll(src)
		_, _ = srcFmt, dstFmt
		return io.NopCloser(bytes.NewReader(b)), nil
	}
	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_, _ = format, opts
		return nil, errors.New("unsupported platform")
	}
	in := filepath.Join(t.TempDir(), "c.mp3")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write input failed: %v", err)
	}
	err = playMP3File(config{format: pcm.L16Mono16K, inputMP3: in, bitrate: 96, volume: 0.5})
	if err == nil || !strings.Contains(err.Error(), "CGO_ENABLED=1") {
		t.Fatalf("expected playback hint error, got %v", err)
	}
}

func TestOpusLoopbackErrorBranches(t *testing.T) {
	patchDeps(t)

	opusRuntimeSupportedFn = func() bool { return true }
	newOpusEncoderFn = func(sampleRate, channels int, app opus.Application) (opusEncoder, error) {
		_, _, _ = sampleRate, channels, app
		return nil, errors.New("encoder create failed")
	}
	if _, err := newOpusLoopbackReader(bytes.NewReader(make([]byte, 640)), pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "create opus encoder failed") {
		t.Fatalf("expected create encoder error, got %v", err)
	}

	newOpusEncoderFn = func(sampleRate, channels int, app opus.Application) (opusEncoder, error) {
		_, _, _ = sampleRate, channels, app
		return &fakeOpusEncoder{}, nil
	}
	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		_, _ = sampleRate, channels
		return nil, errors.New("decoder create failed")
	}
	if _, err := newOpusLoopbackReader(bytes.NewReader(make([]byte, 640)), pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "create opus decoder failed") {
		t.Fatalf("expected create decoder error, got %v", err)
	}

	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		_, _ = sampleRate, channels
		return &fakeOpusDecoder{}, nil
	}
	newOpusEncoderFn = func(sampleRate, channels int, app opus.Application) (opusEncoder, error) {
		_, _, _ = sampleRate, channels, app
		return &fakeOpusEncoder{err: errors.New("encode failed")}, nil
	}
	r, err := newOpusLoopbackReader(bytes.NewReader(make([]byte, 640)), pcm.L16Mono16K)
	if err != nil {
		t.Fatalf("newOpusLoopbackReader setup: %v", err)
	}
	defer func() {
		_ = r.Close()
	}()
	if _, err := io.ReadAll(r); err == nil || !strings.Contains(err.Error(), "opus encode failed") {
		t.Fatalf("expected encode runtime error, got %v", err)
	}
}

func TestRecordMicrophoneToOGGAndPlayOGG(t *testing.T) {
	patchDeps(t)

	opusRuntimeSupportedFn = func() bool { return true }
	openCaptureFn = func(format pcm.Format, opts portaudio.CaptureOptions) (captureReader, error) {
		_, _ = format, opts
		return &fakeCapture{
			cfg: portaudio.StreamConfig{SampleRate: 16000, Channels: 1, FramesPerBuffer: 160},
			chunks: [][]byte{
				make([]byte, 640),
				make([]byte, 640),
			},
		}, nil
	}
	newOpusEncoderFn = func(sampleRate, channels int, app opus.Application) (opusEncoder, error) {
		if sampleRate != 16000 || channels != 1 || app != opus.ApplicationAudio {
			t.Fatalf("unexpected opus encoder args: %d %d %d", sampleRate, channels, app)
		}
		return &fakeOpusEncoder{}, nil
	}
	w := &fakeOGGWriter{}
	newOGGStreamWriterFn = func(writer io.Writer, serial uint32) (oggStreamWriter, error) {
		_, _ = writer, serial
		return w, nil
	}

	out := filepath.Join(t.TempDir(), "mic.ogg")
	if err := recordMicrophoneToOGG(config{format: pcm.L16Mono16K, timeout: 20 * time.Millisecond, outputOGG: out}); err != nil {
		t.Fatalf("recordMicrophoneToOGG: %v", err)
	}
	if len(w.packets) < 3 {
		t.Fatalf("expected opus head/tags/audio packets, got %d", len(w.packets))
	}
	if !w.eos[len(w.eos)-1] {
		t.Fatalf("last packet should set eos")
	}

	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		if sampleRate != 16000 || channels != 1 {
			t.Fatalf("unexpected opus decoder args: %d %d", sampleRate, channels)
		}
		return &fakeOpusDecoder{}, nil
	}
	readAllOGGPacketsFn = func(r io.Reader) ([]ogg.Packet, error) {
		_, _ = io.Copy(io.Discard, r)
		return []ogg.Packet{
			{Data: buildOpusTagsPacket("test")},
			{Data: mustOpusHeadPacketForTest(t, 16000, 1)},
			{Data: []byte{1, 2, 3, 4}},
		}, nil
	}
	openPlaybackFn = func(format pcm.Format, opts portaudio.PlaybackOptions) (playbackWriter, error) {
		_, _ = format, opts
		return &fakePlayback{}, nil
	}

	in := filepath.Join(t.TempDir(), "in.ogg")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write ogg input failed: %v", err)
	}
	if err := playOGGFile(config{format: pcm.L16Mono16K, inputOGG: in, bitrate: 96, volume: 0.5}); err != nil {
		t.Fatalf("playOGGFile: %v", err)
	}
}

func TestDecodeAndResampleOGGErrorsAndHelpers(t *testing.T) {
	patchDeps(t)

	if _, _, err := parseOpusHeadPacket([]byte("bad")); err == nil {
		t.Fatal("parseOpusHeadPacket should fail for invalid packet")
	}
	if _, err := buildOpusHeadPacket(0, 1); err == nil {
		t.Fatal("buildOpusHeadPacket should fail on invalid sample rate")
	}
	if _, err := buildOpusHeadPacket(16000, 3); err == nil {
		t.Fatal("buildOpusHeadPacket should fail on invalid channels")
	}
	if got := buildOpusTagsPacket("vendor"); !isOpusTagsPacket(got) {
		t.Fatalf("expected OpusTags packet, got %q", got)
	}

	in := filepath.Join(t.TempDir(), "in.ogg")
	if err := os.WriteFile(in, []byte("dummy"), 0o600); err != nil {
		t.Fatalf("write input failed: %v", err)
	}

	readAllOGGPacketsFn = func(r io.Reader) ([]ogg.Packet, error) {
		_, _ = io.Copy(io.Discard, r)
		return nil, errors.New("read packets failed")
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "read ogg packets failed") {
		t.Fatalf("expected read packets error, got %v", err)
	}

	readAllOGGPacketsFn = func(r io.Reader) ([]ogg.Packet, error) {
		_, _ = io.Copy(io.Discard, r)
		return []ogg.Packet{}, nil
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "empty ogg packet stream") {
		t.Fatalf("expected empty packets error, got %v", err)
	}

	readAllOGGPacketsFn = func(r io.Reader) ([]ogg.Packet, error) {
		_, _ = io.Copy(io.Discard, r)
		return []ogg.Packet{{Data: mustOpusHeadPacketForTest(t, 16000, 1)}}, nil
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "no opus audio packets") {
		t.Fatalf("expected no audio packets error, got %v", err)
	}

	readAllOGGPacketsFn = func(r io.Reader) ([]ogg.Packet, error) {
		_, _ = io.Copy(io.Discard, r)
		return []ogg.Packet{{Data: mustOpusHeadPacketForTest(t, 16000, 1)}, {Data: []byte{1, 2, 3, 4}}}, nil
	}
	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		_, _ = sampleRate, channels
		return nil, errors.New("decoder failed")
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "create opus decoder failed") {
		t.Fatalf("expected decoder create error, got %v", err)
	}

	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		_, _ = sampleRate, channels
		return &fakeOpusDecoder{err: errors.New("decode failed")}, nil
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono16K); err == nil || !strings.Contains(err.Error(), "decode ogg opus packet") {
		t.Fatalf("expected decode packet error, got %v", err)
	}

	newOpusDecoderFn = func(sampleRate, channels int) (opusDecoder, error) {
		_, _ = sampleRate, channels
		return &fakeOpusDecoder{}, nil
	}
	newResamplerFn = func(src io.Reader, srcFmt, dstFmt resampler.Format) (io.ReadCloser, error) {
		_, _, _ = src, srcFmt, dstFmt
		return nil, errors.New("resampler failed")
	}
	if _, err := decodeAndResampleOGG(in, pcm.L16Mono24K); err == nil || !strings.Contains(err.Error(), "create resampler failed") {
		t.Fatalf("expected resampler error, got %v", err)
	}
}

func mustOpusHeadPacketForTest(t *testing.T, sampleRate, channels int) []byte {
	t.Helper()
	p, err := buildOpusHeadPacket(sampleRate, channels)
	if err != nil {
		t.Fatalf("buildOpusHeadPacket: %v", err)
	}
	return p
}
