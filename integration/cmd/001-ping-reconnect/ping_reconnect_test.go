package pingreconnect_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestPingReconnectUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "001-ping-reconnect")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("client-a").MustSucceed(t)
	h.WaitForPing("client-a")

	beforeRestart, err := h.RunCLIUntilSuccess("ping", "--context", "client-a")
	if err != nil {
		t.Fatal(err)
	}
	assertPingOutput(t, beforeRestart.Stdout)

	originalAddr := h.ServerAddr
	originalPubKey := h.ServerPublicKey

	h.StopServer()

	duringRestart := h.RunCLI("ping", "--context", "client-a")
	if duringRestart.Err == nil {
		t.Fatalf("ping should fail while server is stopped:\nstdout:\n%s\nstderr:\n%s", duringRestart.Stdout, duringRestart.Stderr)
	}

	h.RestartServer()

	if h.ServerAddr != originalAddr {
		t.Fatalf("server addr changed after restart: got %q want %q", h.ServerAddr, originalAddr)
	}
	if h.ServerPublicKey != originalPubKey {
		t.Fatalf("server public key changed after restart: got %q want %q", h.ServerPublicKey, originalPubKey)
	}

	afterRestart, err := h.RunCLIUntilSuccess("ping", "--context", "client-a")
	if err != nil {
		t.Fatal(err)
	}
	assertPingOutput(t, afterRestart.Stdout)
}

func assertPingOutput(t *testing.T, stdout string) {
	t.Helper()

	for _, fragment := range []string{"Server Time:", "RTT:", "Clock Diff:"} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("ping output missing %q:\n%s", fragment, stdout)
		}
	}
}
