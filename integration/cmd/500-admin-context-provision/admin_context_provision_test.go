package admincontextprovision_test

import (
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestAdminContextProvisionUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "500-admin-context-provision")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("admin-a").MustSucceed(t)
	h.RegisterContext("admin-a", "admin_default", "--sn", "admin-sn").MustSucceed(t)

	after := h.RunCLI("admin", "gears", "list", "--context", "admin-a")
	after.MustSucceed(t)
}
