package serverconfigboot_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestServerConfigBootUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "200-server-config-boot")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("client-a").MustSucceed(t)
	result, err := h.RunCLIUntilSuccess("ping", "--context", "client-a")
	if err != nil {
		t.Fatal(err)
	}
	if h.ServerAddr == "" {
		t.Fatal("ServerAddr should not be empty")
	}
	for _, fragment := range []string{"Server Time:", "RTT:", "Clock Diff:"} {
		if !strings.Contains(result.Stdout, fragment) {
			t.Fatalf("ping output missing %q:\n%s", fragment, result.Stdout)
		}
	}
}
