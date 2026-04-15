package repeatcommandafterpartialstate_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestRepeatCommandAfterPartialStateUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "603-repeat-command-after-partial-state")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("device-a").MustSucceed(t)
	first := h.RunCLI("play", "register", "--context", "device-a", "--token", "device_default", "--sn", "device-a")
	first.MustSucceed(t)

	second := h.RunCLI("play", "register", "--context", "device-a", "--token", "device_default", "--sn", "device-a")
	if second.Err == nil {
		t.Fatalf("expected second register to fail:\nstdout:\n%s\nstderr:\n%s", second.Stdout, second.Stderr)
	}
	if !strings.Contains(second.Stderr+second.Stdout, "already") && !strings.Contains(second.Stderr+second.Stdout, "exists") {
		t.Fatalf("unexpected duplicate registration error:\nstdout:\n%s\nstderr:\n%s", second.Stdout, second.Stderr)
	}

	if _, err := h.RunCLIUntilSuccess("ping", "--context", "device-a"); err != nil {
		t.Fatal(err)
	}
}
