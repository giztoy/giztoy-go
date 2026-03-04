package peer

import (
	"bytes"
	"errors"
	"testing"
)

func TestStampedOpusFrameEncodeDecode(t *testing.T) {
	raw := []byte{0xF8, 0xAA, 0xBB, 0xCC}
	stamp := EpochMillis(1700000000123)

	stamped := StampOpusFrame(raw, stamp)
	if got := stamped.Version(); got != OpusFrameVersion {
		t.Fatalf("Version()=%d, want %d", got, OpusFrameVersion)
	}
	if got := stamped.Stamp(); got != stamp {
		t.Fatalf("Stamp()=%d, want %d", got, stamp)
	}
	if got := stamped.Frame(); !bytes.Equal(got, raw) {
		t.Fatalf("Frame()=%v, want %v", got, raw)
	}

	parsed, err := ParseStampedOpusFrame(stamped)
	if err != nil {
		t.Fatalf("ParseStampedOpusFrame failed: %v", err)
	}
	if parsed.Stamp() != stamp {
		t.Fatalf("parsed stamp=%d, want %d", parsed.Stamp(), stamp)
	}
	if !bytes.Equal(parsed.Frame(), raw) {
		t.Fatalf("parsed frame=%v, want %v", parsed.Frame(), raw)
	}
}

func TestStampedOpusFrameValidateErrors(t *testing.T) {
	short := StampedOpusFrame{1, 2, 3}
	if got := short.Version(); got != 1 {
		t.Fatalf("short.Version()=%d, want 1", got)
	}
	if got := short.Frame(); got != nil {
		t.Fatalf("short.Frame()=%v, want nil", got)
	}

	headerOnly := StampedOpusFrame(make([]byte, 8))
	if got := headerOnly.Frame(); got == nil || len(got) != 0 {
		t.Fatalf("headerOnly.Frame() should be empty slice, got=%v", got)
	}

	empty := StampedOpusFrame(nil)
	if got := empty.Version(); got != 0 {
		t.Fatalf("empty.Version()=%d, want 0", got)
	}

	if _, err := ParseStampedOpusFrame([]byte{1, 2, 3}); !errors.Is(err, ErrOpusFrameTooShort) {
		t.Fatalf("Parse(short) err=%v, want %v", err, ErrOpusFrameTooShort)
	}

	bad := StampOpusFrame([]byte{0xF8}, 1)
	bad[0] = 2
	if _, err := ParseStampedOpusFrame(bad); !errors.Is(err, ErrInvalidOpusFrameVersion) {
		t.Fatalf("Parse(bad version) err=%v, want %v", err, ErrInvalidOpusFrameVersion)
	}
}

func TestStampedOpusFrameZeroAndMaxTimestamp(t *testing.T) {
	frame := []byte{0xF8}

	zero := StampOpusFrame(frame, 0)
	if got := zero.Stamp(); got != 0 {
		t.Fatalf("zero stamp=%d, want 0", got)
	}

	const max56 = (1 << 56) - 1
	max := StampOpusFrame(frame, EpochMillis(max56))
	if got := max.Stamp(); got != EpochMillis(max56) {
		t.Fatalf("max56 stamp=%d, want %d", got, EpochMillis(max56))
	}
}
