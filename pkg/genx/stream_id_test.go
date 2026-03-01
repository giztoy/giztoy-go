package genx

import (
	"regexp"
	"testing"
)

func TestBase62EncodeUint32(t *testing.T) {
	if got := base62EncodeUint32(0); got != "0" {
		t.Fatalf("expected 0, got %q", got)
	}
	if got := base62EncodeUint32(61); got != "z" {
		t.Fatalf("expected z, got %q", got)
	}
	if got := base62EncodeUint32(62); got != "10" {
		t.Fatalf("expected 10, got %q", got)
	}
}

func TestBase62EncodeBytesAndStreamID(t *testing.T) {
	if got := base62Encode(nil); got != "" {
		t.Fatalf("expected empty string for nil input, got %q", got)
	}
	if got := base62Encode([]byte{0}); got != "0" {
		t.Fatalf("expected 0 for zero bytes, got %q", got)
	}
	if got := base62Encode([]byte{1, 0}); got == "" {
		t.Fatal("expected non-empty base62 for non-zero bytes")
	}

	id := NewStreamID()
	if len(id) < 6 {
		t.Fatalf("stream id too short: %q", id)
	}
	if !regexp.MustCompile(`^[0-9A-Za-z]+$`).MatchString(id) {
		t.Fatalf("stream id has invalid charset: %q", id)
	}
}
