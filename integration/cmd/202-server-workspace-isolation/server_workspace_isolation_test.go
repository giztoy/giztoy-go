package serverworkspaceisolation_test

import (
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestServerWorkspaceIsolationUserStory(t *testing.T) {
	first := clitest.NewHarness(t, "202-server-workspace-isolation")
	first.StartServerFromFixture("server_config.yaml")

	second := clitest.NewHarness(t, "202-server-workspace-isolation")
	second.StartServerFromFixture("server_config.yaml")

	if first.ServerPublicKey == second.ServerPublicKey {
		t.Fatalf("different workspaces should not share server public keys: %q", first.ServerPublicKey)
	}

	first.CreateContext("alpha").MustSucceed(t)
	second.CreateContext("beta").MustSucceed(t)

	if _, err := first.RunCLIUntilSuccess("ping", "--context", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.RunCLIUntilSuccess("ping", "--context", "beta"); err != nil {
		t.Fatal(err)
	}
}
