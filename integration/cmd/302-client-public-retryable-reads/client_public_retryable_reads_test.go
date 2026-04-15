package clientpublicretryablereads_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestClientPublicRetryableReadsUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "302-client-public-retryable-reads")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("device-a").MustSucceed(t)
	h.RegisterContext("device-a", "device_default", "--sn", "device-a-sn").MustSucceed(t)

	for i := 0; i < 4; i++ {
		config := h.RunCLI("play", "config", "--context", "device-a")
		config.MustSucceed(t)
		if !strings.Contains(config.Stdout, `"firmware"`) {
			t.Fatalf("expected play config output to include firmware config:\n%s", config.Stdout)
		}
		if _, err := h.RunCLIUntilSuccess("ping", "--context", "device-a"); err != nil {
			t.Fatal(err)
		}
	}
}
