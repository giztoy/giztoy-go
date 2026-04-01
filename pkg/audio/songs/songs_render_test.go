package songs

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/audio/pcm"
)

func TestGenerateChordAndHelpers(t *testing.T) {
	samples := DurationSamples(100, 16000)

	chord := GenerateChord([]float64{C4, E4, G4}, samples, 16000, 0.6)
	if len(chord) != samples {
		t.Fatalf("GenerateChord len=%d want=%d", len(chord), samples)
	}
	hasNonZero := false
	for _, v := range chord {
		if v != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Fatal("GenerateChord should produce non-zero samples")
	}

	zeros := GenerateChord([]float64{Rest, Rest}, samples, 16000, 0.6)
	for i, v := range zeros {
		if v != 0 {
			t.Fatalf("GenerateChord rest result[%d]=%d, want 0", i, v)
		}
	}

	if got := DurationSamplesFormat(250, pcm.L16Mono16K); got != 4000 {
		t.Fatalf("DurationSamplesFormat()=%d want=4000", got)
	}

	notes := []Note{{Freq: A4, Dur: 120}, {Freq: Rest, Dur: 80}}
	if got := TotalDuration(notes); got != 200 {
		t.Fatalf("TotalDuration()=%d want=200", got)
	}
}

func TestRenderDefaultsAndRenderBytes(t *testing.T) {
	opts := DefaultRenderOptions()
	if opts.Format != DefaultFormat {
		t.Fatalf("default format mismatch: got %v want %v", opts.Format, DefaultFormat)
	}
	if opts.Volume <= 0 || opts.Volume > 1 {
		t.Fatalf("default volume out of range: %f", opts.Volume)
	}

	opts.Metronome = true
	data, err := SongHappyBirthday.RenderBytes(opts)
	if err != nil {
		t.Fatalf("RenderBytes() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("RenderBytes() returned empty data")
	}
	if len(data)%2 != 0 {
		t.Fatalf("RenderBytes length must align int16, got %d", len(data))
	}
}

func TestRenderAndVoiceToChunk(t *testing.T) {
	voice := Voice{Notes: []Note{{Freq: C4, Dur: 120}, {Freq: E4, Dur: 120}, {Freq: Rest, Dur: 50}}}
	chunk := VoiceToChunk(voice, pcm.L16Mono16K, 0.5)
	if chunk.Len() == 0 {
		t.Fatal("VoiceToChunk() should produce non-empty chunk")
	}
	if chunk.Format() != pcm.L16Mono16K {
		t.Fatalf("VoiceToChunk format mismatch: got %v", chunk.Format())
	}

	r := SongTwinkleStar.Render(RenderOptions{Format: pcm.L16Mono16K, Volume: 0.4, Metronome: false, RichSound: true})
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Render() read error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("Render() should produce output")
	}
}

func TestRenderVolumeZeroIsMute(t *testing.T) {
	r := SongTwinkleStar.Render(RenderOptions{Format: pcm.L16Mono16K, Volume: 0, Metronome: false, RichSound: true})
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Render() read error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Render() should still output muted frame data")
	}
	for i, b := range data {
		if b != 0 {
			t.Fatalf("muted render byte[%d]=%d, want 0", i, b)
		}
	}
}

func TestVoiceToChunkRichSoundSwitch(t *testing.T) {
	voice := Voice{Notes: []Note{{Freq: A4, Dur: 120}, {Freq: C5, Dur: 120}}}

	rich := voiceToChunk(voice, pcm.L16Mono16K, 0.5, true)
	sine := voiceToChunk(voice, pcm.L16Mono16K, 0.5, false)

	var richBuf bytes.Buffer
	if _, err := rich.WriteTo(&richBuf); err != nil {
		t.Fatalf("rich chunk write error: %v", err)
	}
	var sineBuf bytes.Buffer
	if _, err := sine.WriteTo(&sineBuf); err != nil {
		t.Fatalf("sine chunk write error: %v", err)
	}

	if bytes.Equal(richBuf.Bytes(), sineBuf.Bytes()) {
		t.Fatal("rich and sine render should differ for tonal notes")
	}
}

func TestVoiceLabelAndEmptyReader(t *testing.T) {
	if got := voiceLabel(0); got != "melody" {
		t.Fatalf("voiceLabel(0)=%q", got)
	}
	if got := voiceLabel(1); got != "bass" {
		t.Fatalf("voiceLabel(1)=%q", got)
	}
	if got := voiceLabel(2); got != "harmony" {
		t.Fatalf("voiceLabel(2)=%q", got)
	}
	if got := voiceLabel(3); got != "voice" {
		t.Fatalf("voiceLabel(3)=%q", got)
	}

	var er emptyReader
	n, err := er.Read(make([]byte, 8))
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("emptyReader.Read()=(%d,%v), want (0,EOF)", n, err)
	}
}

func TestRenderWithNoVoices(t *testing.T) {
	s := Song{
		ID:    "empty",
		Name:  "empty",
		Tempo: Tempo{BPM: 120, Signature: Time4_4},
		Voices: func() []BeatVoice {
			return nil
		},
	}

	r := s.Render(DefaultRenderOptions())
	n, err := r.Read(make([]byte, 16))
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("empty song render read=(%d,%v), want (0,EOF)", n, err)
	}
}
