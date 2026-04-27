package realservicesmoke_test

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	clitest "github.com/GizClaw/gizclaw-go/integration/cmd"
	itest "github.com/GizClaw/gizclaw-go/integration/testutil"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func TestRealServiceUISmoke(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	workspaceRoot := filepath.Join(repoRoot, "integration", ".workspace", "ui-real-service")
	h := clitest.NewPersistentHarnessForRoot(t, "integration/ui", "900-real-service-smoke", workspaceRoot)
	h.StartServerFromFixture("server_config.yaml")
	h.EnsureContext("admin-a").MustSucceed(t)
	h.EnsureContext("device-a").MustSucceed(t)

	adminClient := h.ConnectClientFromContext("admin-a")
	defer adminClient.Close()
	adminSeed, err := itest.LoadRegistrationSeed("admin")
	if err != nil {
		t.Fatalf("load admin registration seed: %v", err)
	}
	registerGear(t, adminClient, itest.RegistrationRequest(h.ContextPublicKey("admin-a"), adminSeed))

	deviceClient := h.ConnectClientFromContext("device-a")
	defer deviceClient.Close()
	deviceSeed, err := itest.LoadRegistrationSeed("device")
	if err != nil {
		t.Fatalf("load device registration seed: %v", err)
	}
	devicePublicKey := h.ContextPublicKey("device-a")
	registerGear(t, deviceClient, itest.RegistrationRequest(devicePublicKey, deviceSeed))
	_ = deviceClient.Close()

	adminAPI, err := adminClient.ServerAdminClient()
	if err != nil {
		t.Fatalf("create admin API client: %v", err)
	}
	seedCtx, cancel := context.WithTimeout(context.Background(), itest.ReadyTimeout)
	defer cancel()
	if err := itest.ApplyAdminCatalogSeed(seedCtx, adminAPI); err != nil {
		t.Fatalf("apply admin catalog seed: %v", err)
	}
	if err := itest.ApplyWorkspaceSeed(seedCtx, adminAPI); err != nil {
		t.Fatalf("apply workspace seed: %v", err)
	}
	if err := itest.ApplyFirmwareSeed(seedCtx, adminAPI); err != nil {
		t.Fatalf("apply firmware seed: %v", err)
	}
	if err := itest.ApplyDeviceConfigSeed(seedCtx, adminAPI, devicePublicKey); err != nil {
		t.Fatalf("apply device config seed: %v", err)
	}
	_ = adminClient.Close()

	adminURL := h.StartUI("admin", "admin-a")
	playURL := h.StartUI("play", "device-a")
	runPlaywright(t, repoRoot, adminURL, playURL, devicePublicKey)
}

func registerGear(t testing.TB, client *gizclaw.Client, request serverpublic.RegistrationRequest) {
	t.Helper()

	api, err := client.ServerPublicClient()
	if err != nil {
		t.Fatalf("create public API client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), itest.ReadyTimeout)
	defer cancel()
	resp, err := api.RegisterGearWithResponse(ctx, request)
	if err != nil {
		t.Fatalf("register %q: %v", request.PublicKey, err)
	}
	if resp.JSON200 != nil || resp.StatusCode() == http.StatusConflict {
		return
	}
	t.Fatalf("register %q got status %d: %s", request.PublicKey, resp.StatusCode(), strings.TrimSpace(string(resp.Body)))
}

func runPlaywright(t testing.TB, repoRoot, adminURL, playURL, devicePublicKey string) {
	t.Helper()

	cmd := exec.Command("npm", "exec", "playwright", "test", "--", "900-real-service-smoke/real_service_smoke.real.spec.ts")
	cmd.Dir = filepath.Join(repoRoot, "integration", "ui")
	cmd.Env = append(os.Environ(),
		"ADMIN_UI_URL="+adminURL,
		"PLAY_UI_URL="+playURL,
		"DEVICE_PUBLIC_KEY="+devicePublicKey,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run real-service Playwright: %v\n%s", err, string(output))
	}
	t.Logf("real-service Playwright output:\n%s", string(output))
}

func mustRepoRoot(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve real-service UI test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve repo root %q: %v", root, err)
	}
	return root
}
