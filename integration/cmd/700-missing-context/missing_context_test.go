package missingcontext_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestMissingContextUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "700-missing-context")

	list := h.ListContexts()
	list.MustSucceed(t)
	if !strings.Contains(list.Stdout, "No contexts found.") {
		t.Fatalf("unexpected context list output:\n%s", list.Stdout)
	}

	ping := h.RunCLI("ping")
	if ping.Err == nil {
		t.Fatalf("ping without a context should fail:\nstdout:\n%s\nstderr:\n%s", ping.Stdout, ping.Stderr)
	}
	if !strings.Contains(ping.Stderr, "no active context; run 'gizclaw context create' first") {
		t.Fatalf("unexpected ping error output:\n%s", ping.Stderr)
	}
}
