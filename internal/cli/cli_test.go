package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	root := NewRootCmd()
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
	if !strings.Contains(out, "context") {
		t.Fatalf("help missing 'context': %s", out)
	}
	if !strings.Contains(out, "ping") {
		t.Fatalf("help missing 'ping': %s", out)
	}
}

func TestServeHelp(t *testing.T) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"serve", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--data-dir") {
		t.Fatalf("serve help missing '--data-dir': %s", out)
	}
	if !strings.Contains(out, "--listen") {
		t.Fatalf("serve help missing '--listen': %s", out)
	}
}

func TestContextHelp(t *testing.T) {
	root := NewRootCmd()
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
	root := NewRootCmd()
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

func TestContextCreateMissingFlags(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"context", "create", "test"})
	err := root.Execute()
	if err == nil {
		t.Fatal("context create without required flags should fail")
	}
}
