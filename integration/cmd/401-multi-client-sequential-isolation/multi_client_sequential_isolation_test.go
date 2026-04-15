package multiclientsequentialisolation_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestMultiClientSequentialIsolationUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "401-multi-client-sequential-isolation")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("alpha").MustSucceed(t)
	h.CreateContext("beta").MustSucceed(t)
	h.UseContext("alpha").MustSucceed(t)

	if _, err := h.RunCLIUntilSuccess("ping"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.RunCLIUntilSuccess("ping", "--context", "beta"); err != nil {
		t.Fatal(err)
	}

	list := h.ListContexts()
	list.MustSucceed(t)
	if !strings.Contains(list.Stdout, "* alpha") {
		t.Fatalf("expected alpha to remain current after explicit beta command:\n%s", list.Stdout)
	}
	if _, err := h.RunCLIUntilSuccess("ping"); err != nil {
		t.Fatal(err)
	}
}
