package contextisolationbetweenstoryhomes_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestContextIsolationBetweenStoryHomesUserStory(t *testing.T) {
	server := clitest.NewHarness(t, "103-context-isolation-between-story-homes")
	server.StartServerFromFixture("server_config.yaml")

	first := clitest.NewHarness(t, "103-context-isolation-between-story-homes")
	second := clitest.NewHarness(t, "103-context-isolation-between-story-homes")

	first.CreateContextWith("alpha", server.ServerAddr, server.ServerPublicKey).MustSucceed(t)
	second.CreateContextWith("beta", server.ServerAddr, server.ServerPublicKey).MustSucceed(t)

	firstList := first.ListContexts()
	firstList.MustSucceed(t)
	secondList := second.ListContexts()
	secondList.MustSucceed(t)

	if !strings.Contains(firstList.Stdout, "alpha") || strings.Contains(firstList.Stdout, "beta") {
		t.Fatalf("unexpected first home list:\n%s", firstList.Stdout)
	}
	if !strings.Contains(secondList.Stdout, "beta") || strings.Contains(secondList.Stdout, "alpha") {
		t.Fatalf("unexpected second home list:\n%s", secondList.Stdout)
	}

	if _, err := first.RunCLIUntilSuccess("ping", "--context", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.RunCLIUntilSuccess("ping", "--context", "beta"); err != nil {
		t.Fatal(err)
	}
}
