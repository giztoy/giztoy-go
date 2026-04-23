package main

import "testing"

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig([]string{"-addr", ":9999"})
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.addr != ":9999" {
		t.Fatalf("addr = %q, want %q", cfg.addr, ":9999")
	}
}

func TestParseConfigErrors(t *testing.T) {
	if _, err := parseConfig([]string{"-addr", ""}); err == nil {
		t.Fatal("expected empty addr error")
	}
	if _, err := parseConfig([]string{"extra"}); err == nil {
		t.Fatal("expected positional arg error")
	}
}
