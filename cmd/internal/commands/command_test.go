package commands

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeLegacyLongFlags(t *testing.T) {
	got := normalizeLegacyLongFlags([]string{"admin", "-listen=8080", "-context", "demo", "--help", "-h"})
	want := []string{"admin", "--listen=8080", "--context", "demo", "--help", "-h"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeLegacyLongFlags() = %#v, want %#v", got, want)
	}
}

func TestRootHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "serve") {
		t.Fatalf("help missing 'serve': %s", out)
	}
	if !strings.Contains(out, "service") {
		t.Fatalf("help missing 'service': %s", out)
	}
	if !strings.Contains(out, "context") {
		t.Fatalf("help missing 'context': %s", out)
	}
	if !strings.Contains(out, "ping") {
		t.Fatalf("help missing 'ping': %s", out)
	}
	if !strings.Contains(out, "admin") {
		t.Fatalf("help missing 'admin': %s", out)
	}
	if !strings.Contains(out, "play") {
		t.Fatalf("help missing 'play': %s", out)
	}
}

func TestServeHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"serve", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "serve <dir>") {
		t.Fatalf("serve help missing '<dir>': %s", out)
	}
	if strings.Contains(out, "--data-dir") || strings.Contains(out, "--listen") || strings.Contains(out, "--config") {
		t.Fatalf("serve help should not mention removed flags: %s", out)
	}
	for _, want := range []string{"--force"} {
		if !strings.Contains(out, want) {
			t.Fatalf("serve help missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "--bg") {
		t.Fatalf("serve help should not mention '--bg': %s", out)
	}
}

func TestServiceHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"service", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"install", "start", "stop", "uninstall"} {
		if !strings.Contains(out, want) {
			t.Fatalf("service help missing %q: %s", want, out)
		}
	}
}

func TestContextHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"context", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "create") {
		t.Fatalf("context help missing 'create': %s", out)
	}
	if !strings.Contains(out, "use") {
		t.Fatalf("context help missing 'use': %s", out)
	}
	if !strings.Contains(out, "list") {
		t.Fatalf("context help missing 'list': %s", out)
	}
}

func TestPingHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"ping", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--context") {
		t.Fatalf("ping help missing '--context': %s", out)
	}
}

func TestAdminHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"admin", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "gears") || !strings.Contains(out, "firmware") {
		t.Fatalf("admin help missing subcommands: %s", out)
	}
	if !strings.Contains(out, "--listen") {
		t.Fatalf("admin help missing '--listen': %s", out)
	}
}

func TestPlayHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"play", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "serve") || !strings.Contains(out, "register") {
		t.Fatalf("play help missing subcommands: %s", out)
	}
	if !strings.Contains(out, "--listen") {
		t.Fatalf("play help missing '--listen': %s", out)
	}
}

func TestAdminGearsHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"admin", "gears", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"resolve-sn",
		"resolve-imei",
		"list-by-label",
		"list-by-certification",
		"list-by-firmware",
		"info",
		"config",
		"put-config",
		"runtime",
		"ota",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("admin gears help missing %q: %s", want, out)
		}
	}
}

func TestAdminFirmwareHelp(t *testing.T) {
	root := New()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"admin", "firmware", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"get-channel", "put-info", "upload", "rollback", "release"} {
		if !strings.Contains(out, want) {
			t.Fatalf("admin firmware help missing %q: %s", want, out)
		}
	}
}

func TestContextCreateMissingFlags(t *testing.T) {
	root := New()
	root.SetArgs([]string{"context", "create", "test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("context create without required flags should fail")
	}
}

func TestServeRequiresWorkspaceArg(t *testing.T) {
	root := New()
	root.SetArgs([]string{"serve"})
	err := root.Execute()
	if err == nil {
		t.Fatal("serve without workspace arg should fail")
	}
}
