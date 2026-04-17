package servecmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestServeCommandRequiresSingleWorkspaceArg(t *testing.T) {
	cmd := NewCmd()
	if err := cmd.Args(cmd, []string{"workspace-dir"}); err != nil {
		t.Fatalf("Args(valid) error = %v", err)
	}
	if err := cmd.Args(cmd, nil); err == nil {
		t.Fatal("Args(nil) should fail")
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Fatal("Args(two args) should fail")
	}
}

func TestServeCommandHelpIncludesBackgroundFlags(t *testing.T) {
	cmd := NewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"--force", "-f"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "--bg") {
		t.Fatalf("help should not mention --bg: %s", out)
	}
}
