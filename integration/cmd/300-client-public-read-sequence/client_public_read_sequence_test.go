package clientpublicreadsequence_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestClientPublicReadSequenceUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "300-client-public-read-sequence")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("device-a").MustSucceed(t)
	h.RegisterContext(
		"device-a",
		"device_default",
		"--name", "device-a",
		"--sn", "device-a-sn",
		"--manufacturer", "Acme",
		"--model", "Model-A",
		"--depot", "demo",
		"--firmware-semver", "1.0.0",
	).MustSucceed(t)

	config := h.RunCLI("play", "config", "--context", "device-a")
	config.MustSucceed(t)
	if !strings.Contains(config.Stdout, `"firmware"`) {
		t.Fatalf("expected play config output to include firmware config:\n%s", config.Stdout)
	}

	if _, err := h.RunCLIUntilSuccess("ping", "--context", "device-a"); err != nil {
		t.Fatal(err)
	}
}
