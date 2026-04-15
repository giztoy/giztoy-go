package adminlookupgear_test

import (
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
)

func TestAdminLookupGearUserStory(t *testing.T) {
	h := clitest.NewHarness(t, "502-admin-lookup-gear")
	h.StartServerFromFixture("server_config.yaml")

	h.CreateContext("admin-a").MustSucceed(t)
	h.CreateContext("device-a").MustSucceed(t)

	h.RegisterContext("admin-a", "admin_default", "--sn", "admin-sn").MustSucceed(t)
	h.RegisterContext(
		"device-a",
		"device_default",
		"--sn", "device-a-sn",
		"--manufacturer", "Acme",
		"--model", "Model-A",
	).MustSucceed(t)

	devicePubKey := h.ContextPublicKey("device-a")

	resolve := h.RunCLI("admin", "gears", "resolve-sn", "device-a-sn", "--context", "admin-a")
	resolve.MustSucceed(t)
	if !strings.Contains(resolve.Stdout, devicePubKey) {
		t.Fatalf("expected resolved public key %q:\n%s", devicePubKey, resolve.Stdout)
	}

	get := h.RunCLI("admin", "gears", "get", devicePubKey, "--context", "admin-a")
	get.MustSucceed(t)
	if !strings.Contains(get.Stdout, `"public_key":"`+devicePubKey+`"`) {
		t.Fatalf("expected get output to include device public key:\n%s", get.Stdout)
	}

	info := h.RunCLI("admin", "gears", "info", devicePubKey, "--context", "admin-a")
	info.MustSucceed(t)
	for _, fragment := range []string{`"sn":"device-a-sn"`, `"manufacturer":"Acme"`, `"model":"Model-A"`} {
		if !strings.Contains(info.Stdout, fragment) {
			t.Fatalf("expected info output to include %q:\n%s", fragment, info.Stdout)
		}
	}
}
