package wrongserverpublickey_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestWrongServerPublicKeyUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "701-wrong-server-public-key")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContextWith("broken", h.ServerAddr, strings.Repeat("1", len(h.ServerPublicKey))).MustSucceed(t)

	result := h.RunCLI("ping", "--context", "broken")
	if result.Err == nil {
		t.Fatalf("expected ping to fail with wrong server public key:\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
	}
}
