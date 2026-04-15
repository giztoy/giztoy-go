package contextstory_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestContextCreateUseListUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "100-context-create-use-list")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("beta").MustSucceed(t)
	h.CreateContext("alpha").MustSucceed(t)
	h.WaitForPing("alpha")
	h.WaitForPing("beta")

	listBefore := h.ListContexts()
	listBefore.MustSucceed(t)
	assertContextList(t, listBefore.Stdout, "alpha", "beta")
	if !strings.Contains(listBefore.Stdout, "* beta\n") {
		t.Fatalf("expected beta to be current after first create:\n%s", listBefore.Stdout)
	}

	useAlpha := h.UseContext("alpha")
	useAlpha.MustSucceed(t)
	if !strings.Contains(useAlpha.Stdout, `Switched to context "alpha".`) {
		t.Fatalf("unexpected use output:\n%s", useAlpha.Stdout)
	}

	listAfter := h.ListContexts()
	listAfter.MustSucceed(t)
	assertContextList(t, listAfter.Stdout, "alpha", "beta")
	if !strings.Contains(listAfter.Stdout, "* alpha\n") {
		t.Fatalf("expected alpha to be current after use:\n%s", listAfter.Stdout)
	}

	pingAlpha, err := h.RunCLIUntilSuccess("ping", "--context", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	assertPingOutput(t, pingAlpha.Stdout)

	pingBeta, err := h.RunCLIUntilSuccess("ping", "--context", "beta")
	if err != nil {
		t.Fatal(err)
	}
	assertPingOutput(t, pingBeta.Stdout)
}

func assertContextList(t *testing.T, stdout string, names ...string) {
	t.Helper()

	var indexes []int
	for _, name := range names {
		idx := strings.Index(stdout, name)
		if idx < 0 {
			t.Fatalf("context list missing %q:\n%s", name, stdout)
		}
		indexes = append(indexes, idx)
	}
	for i := 1; i < len(indexes); i++ {
		if indexes[i] < indexes[i-1] {
			t.Fatalf("context list not ordered as expected:\n%s", stdout)
		}
	}
}

func assertPingOutput(t *testing.T, stdout string) {
	t.Helper()

	for _, fragment := range []string{"Server Time:", "RTT:", "Clock Diff:"} {
		if !strings.Contains(stdout, fragment) {
			t.Fatalf("ping output missing %q:\n%s", fragment, stdout)
		}
	}
}
