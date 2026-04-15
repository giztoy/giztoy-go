package multiclienthomeisolation_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestMultiClientHomeIsolationUserStory(t *testing.T) {
	server := clitest.NewHarness(t, "402-multi-client-home-isolation")
	server.StartServerFromFixture("server_config.yaml")

	first := clitest.NewHarness(t, "402-multi-client-home-isolation")
	second := clitest.NewHarness(t, "402-multi-client-home-isolation")

	first.CreateContextWith("alpha", server.ServerAddr, server.ServerPublicKey).MustSucceed(t)
	second.CreateContextWith("beta", server.ServerAddr, server.ServerPublicKey).MustSucceed(t)

	firstList := first.ListContexts()
	firstList.MustSucceed(t)
	secondList := second.ListContexts()
	secondList.MustSucceed(t)

	if !strings.Contains(firstList.Stdout, "alpha") || strings.Contains(firstList.Stdout, "beta") {
		t.Fatalf("unexpected first home contexts:\n%s", firstList.Stdout)
	}
	if !strings.Contains(secondList.Stdout, "beta") || strings.Contains(secondList.Stdout, "alpha") {
		t.Fatalf("unexpected second home contexts:\n%s", secondList.Stdout)
	}

	if _, err := first.RunCLIUntilSuccess("ping", "--context", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.RunCLIUntilSuccess("ping", "--context", "beta"); err != nil {
		t.Fatal(err)
	}
}
