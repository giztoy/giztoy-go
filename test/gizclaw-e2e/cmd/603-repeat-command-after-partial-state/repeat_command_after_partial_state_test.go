package repeatcommandafterpartialstate_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/test/gizclaw-e2e/cmd"
)

func TestRepeatCommandAfterPartialStateUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "603-repeat-command-after-partial-state")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("device-a").MustSucceed(t)
	first := h.RegisterContext("device-a", "--sn", "device-a")
	first.MustSucceed(t)

	second := h.RegisterContext("device-a", "--sn", "device-a-retry")
	second.MustSucceed(t)
	if !strings.Contains(second.Stdout, `"sn":"device-a-retry"`) {
		t.Fatalf("second register should update auto-registered device info:\n%s", second.Stdout)
	}

	if _, err := h.RunCLIUntilSuccess("ping", "--context", "device-a"); err != nil {
		t.Fatal(err)
	}
}
