package repeatserverrestart_test

import (
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestRepeatServerRestartUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "602-repeat-server-restart")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("client-a").MustSucceed(t)
	for i := 0; i < 3; i++ {
		if _, err := h.RunCLIUntilSuccess("ping", "--context", "client-a"); err != nil {
			t.Fatal(err)
		}
		h.StopServer()
		h.RestartServer()
	}
	if _, err := h.RunCLIUntilSuccess("ping", "--context", "client-a"); err != nil {
		t.Fatal(err)
	}
}
