package recoveryafterrestart_test

import (
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestRecoveryAfterRestartUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "704-recovery-after-restart")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("client-a").MustSucceed(t)
	if _, err := h.RunCLIUntilSuccess("ping", "--context", "client-a"); err != nil {
		t.Fatal(err)
	}

	h.StopServer()
	offline := h.RunCLI("ping", "--context", "client-a")
	if offline.Err == nil {
		t.Fatalf("expected ping to fail while server is offline:\nstdout:\n%s\nstderr:\n%s", offline.Stdout, offline.Stderr)
	}

	h.RestartServer()
	if _, err := h.RunCLIUntilSuccess("ping", "--context", "client-a"); err != nil {
		t.Fatal(err)
	}
}
