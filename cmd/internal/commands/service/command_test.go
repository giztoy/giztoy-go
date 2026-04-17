package servicecmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestServiceCommandHelpIncludesSubcommands(t *testing.T) {
	cmd := NewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"install", "start", "stop", "uninstall"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q: %s", want, out)
		}
	}
}

func TestServiceSubcommandsRequireWorkspaceArg(t *testing.T) {
	install := newInstallCmd()
	if err := install.Args(install, []string{"workspace"}); err != nil {
		t.Fatalf("install Args(valid) error = %v", err)
	}
	if err := install.Args(install, nil); err == nil {
		t.Fatal("install Args(nil) should fail")
	}

	for _, cmd := range []*cobra.Command{newStartCmd(), newStopCmd(), newUninstallCmd()} {
		if err := cmd.Args(cmd, nil); err != nil {
			t.Fatalf("%s Args(nil) error = %v", cmd.Name(), err)
		}
		if err := cmd.Args(cmd, []string{"workspace"}); err == nil {
			t.Fatalf("%s Args(extra arg) should fail", cmd.Name())
		}
	}
}
