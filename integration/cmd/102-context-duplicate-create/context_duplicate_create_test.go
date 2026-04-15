package contextduplicatecreate_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestContextDuplicateCreateUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "102-context-duplicate-create")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("alpha").MustSucceed(t)

	duplicate := h.CreateContext("alpha")
	if duplicate.Err == nil {
		t.Fatalf("expected duplicate context create to fail:\nstdout:\n%s\nstderr:\n%s", duplicate.Stdout, duplicate.Stderr)
	}
	if !strings.Contains(duplicate.Stderr+duplicate.Stdout, "exists") {
		t.Fatalf("unexpected duplicate context error:\nstdout:\n%s\nstderr:\n%s", duplicate.Stdout, duplicate.Stderr)
	}

	if _, err := h.RunCLIUntilSuccess("ping", "--context", "alpha"); err != nil {
		t.Fatal(err)
	}
}
