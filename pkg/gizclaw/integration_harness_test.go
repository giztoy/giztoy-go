package gizclaw_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/integration/testutil"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

type testServer struct {
	server *gizclaw.Server
	addr   string
	errCh  chan error
}

func startTestServer(t *testing.T) *testServer {
	t.Helper()

	root := t.TempDir()
	firmwareRoot := filepath.Join(root, "firmware")
	if err := os.MkdirAll(firmwareRoot, 0o755); err != nil {
		t.Fatalf("mkdir firmware root: %v", err)
	}

	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(server) error: %v", err)
	}

	srv := &gizclaw.Server{
		KeyPair:   keyPair,
		GearStore: kv.NewMemory(nil),
		RegistrationTokens: map[string]gearservice.GearRole{
			"admin_default":  gearservice.GearRoleAdmin,
			"device_default": gearservice.GearRoleDevice,
		},
		DepotStore: depotstore.Dir(firmwareRoot),
	}

	ts := &testServer{
		server: srv,
		addr:   testutil.AllocateUDPAddr(t),
		errCh:  make(chan error, 1),
	}
	go func() {
		ts.errCh <- srv.ListenAndServe(nil, giznet.WithBindAddr(ts.addr))
	}()

	if err := waitForServerReady(ts.addr, srv.PublicKey(), ts.errCh); err != nil {
		_ = srv.Close()
		select {
		case <-ts.errCh:
		case <-time.After(500 * time.Millisecond):
		}
		t.Fatalf("test server did not become ready: %v", err)
	}

	t.Cleanup(func() { _ = ts.server.Close() })
	return ts
}

func newTestClient(t *testing.T, ts *testServer) *gizclaw.Client {
	t.Helper()

	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(client) error: %v", err)
	}

	client := &gizclaw.Client{KeyPair: keyPair}
	startTestClient(t, client, ts.server.PublicKey(), ts.addr)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func waitForServerReady(addr string, pk giznet.PublicKey, errCh <-chan error) error {
	return testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		select {
		case err := <-errCh:
			return fmt.Errorf("test server exited before ready: %w", err)
		default:
		}

		keyPair, err := giznet.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("GenerateKeyPair(ready check): %w", err)
		}

		client := &gizclaw.Client{KeyPair: keyPair}
		dialErrCh := make(chan error, 1)
		go func() {
			dialErrCh <- client.DialAndServe(pk, addr)
		}()

		for i := 0; i < 20; i++ {
			select {
			case err := <-dialErrCh:
				_ = client.Close()
				if err != nil {
					return fmt.Errorf("dial ready check: %w", err)
				}
				return fmt.Errorf("dial ready check: client stopped before ready")
			default:
			}

			if err := probeServerPublicReady(client); err == nil {
				_ = client.Close()
				return nil
			}
			time.Sleep(50 * time.Millisecond)
		}

		_ = client.Close()
		return fmt.Errorf("probe server public ready: not ready")
	})
}

func startTestClient(t *testing.T, c *gizclaw.Client, serverPK giznet.PublicKey, addr string) {
	t.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.DialAndServe(serverPK, addr)
	}()

	if err := testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
			return fmt.Errorf("client stopped before ready")
		default:
		}
		return probeServerPublicReady(c)
	}); err != nil {
		t.Fatalf("test client did not become ready: %v", err)
	}
}

func probeServerPublicReady(c *gizclaw.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), testutil.ProbeTimeout)
	defer cancel()
	_, err := getServerInfo(ctx, c)
	return err
}

func register(ctx context.Context, c *gizclaw.Client, req serverpublic.RegistrationRequest) (serverpublic.RegistrationResult, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.RegistrationResult{}, err
	}
	if req.PublicKey == "" && c != nil && c.KeyPair != nil {
		req.PublicKey = c.KeyPair.Public.String()
	}
	resp, err := api.RegisterGearWithResponse(ctx, req)
	if err != nil {
		return serverpublic.RegistrationResult{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.RegistrationResult{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON409)
}

func getServerInfo(ctx context.Context, c *gizclaw.Client) (serverpublic.ServerInfo, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.ServerInfo{}, err
	}
	resp, err := api.GetServerInfoWithResponse(ctx)
	if err != nil {
		return serverpublic.ServerInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.ServerInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func getInfo(ctx context.Context, c *gizclaw.Client) (serverpublic.DeviceInfo, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	resp, err := api.GetInfoWithResponse(ctx)
	if err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func putInfo(ctx context.Context, c *gizclaw.Client, info serverpublic.DeviceInfo) (serverpublic.DeviceInfo, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	resp, err := api.PutInfoWithResponse(ctx, info)
	if err != nil {
		return serverpublic.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON404)
}

func getRuntime(ctx context.Context, c *gizclaw.Client) (serverpublic.Runtime, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.Runtime{}, err
	}
	resp, err := api.GetRuntimeWithResponse(ctx)
	if err != nil {
		return serverpublic.Runtime{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.Runtime{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func getRegistration(ctx context.Context, c *gizclaw.Client) (serverpublic.Registration, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.Registration{}, err
	}
	resp, err := api.GetRegistrationWithResponse(ctx)
	if err != nil {
		return serverpublic.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func getConfig(ctx context.Context, c *gizclaw.Client) (serverpublic.Configuration, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.Configuration{}, err
	}
	resp, err := api.GetConfigWithResponse(ctx)
	if err != nil {
		return serverpublic.Configuration{}, err
	}
	if resp.JSON200 != nil {
		cfg := *resp.JSON200
		if cfg.Firmware == nil {
			cfg.Firmware = &serverpublic.FirmwareConfig{}
		}
		return cfg, nil
	}
	return serverpublic.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func getOTA(ctx context.Context, c *gizclaw.Client) (serverpublic.OTASummary, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return serverpublic.OTASummary{}, err
	}
	resp, err := api.GetOTAWithResponse(ctx)
	if err != nil {
		return serverpublic.OTASummary{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return serverpublic.OTASummary{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func downloadFirmware(ctx context.Context, c *gizclaw.Client, path string) ([]byte, http.Header, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return nil, nil, err
	}
	resp, err := api.DownloadFirmware(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}
		return data, resp.Header.Clone(), nil
	}
	body, _ := io.ReadAll(resp.Body)
	return nil, nil, responseError(resp.StatusCode, body)
}

func listFirmwares(ctx context.Context, c *gizclaw.Client) ([]adminservice.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListDepotsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Items, nil
	}
	return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
}

func getFirmwareDepot(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return adminservice.Depot{}, err
	}
	resp, err := api.GetDepotWithResponse(ctx, depot)
	if err != nil {
		return adminservice.Depot{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return adminservice.Depot{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func getFirmwareChannel(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel) (adminservice.DepotRelease, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return adminservice.DepotRelease{}, err
	}
	resp, err := api.GetChannelWithResponse(ctx, depot, channel)
	if err != nil {
		return adminservice.DepotRelease{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return adminservice.DepotRelease{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func putFirmwareInfo(ctx context.Context, c *gizclaw.Client, depot string, info adminservice.DepotInfo) (adminservice.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return adminservice.Depot{}, err
	}
	resp, err := api.PutDepotInfoWithResponse(ctx, depot, info)
	if err != nil {
		return adminservice.Depot{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return adminservice.Depot{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON409, resp.JSON500)
}

func uploadFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel, data []byte) (adminservice.DepotRelease, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return adminservice.DepotRelease{}, err
	}
	resp, err := api.PutChannelWithBodyWithResponse(ctx, depot, channel, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return adminservice.DepotRelease{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return adminservice.DepotRelease{}, responseError(resp.StatusCode(), resp.Body, resp.JSON409)
}

func releaseFirmware(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://gizclaw/firmwares/"+url.PathEscape(depot)+"/@release", nil)
	if err != nil {
		return adminservice.Depot{}, err
	}
	resp, err := c.HTTPClient(gizclaw.ServiceAdmin).Do(req)
	if err != nil {
		return adminservice.Depot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return adminservice.Depot{}, err
	}
	if resp.StatusCode == http.StatusOK {
		var out adminservice.Depot
		if err := json.Unmarshal(body, &out); err != nil {
			return adminservice.Depot{}, err
		}
		return out, nil
	}
	return adminservice.Depot{}, responseError(resp.StatusCode, body)
}

func rollbackFirmware(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://gizclaw/firmwares/"+url.PathEscape(depot)+"/@rollback", nil)
	if err != nil {
		return adminservice.Depot{}, err
	}
	resp, err := c.HTTPClient(gizclaw.ServiceAdmin).Do(req)
	if err != nil {
		return adminservice.Depot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return adminservice.Depot{}, err
	}
	if resp.StatusCode == http.StatusOK {
		var out adminservice.Depot
		if err := json.Unmarshal(body, &out); err != nil {
			return adminservice.Depot{}, err
		}
		return out, nil
	}
	return adminservice.Depot{}, responseError(resp.StatusCode, body)
}

func listGears(ctx context.Context, c *gizclaw.Client) ([]gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListGearsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Items, nil
	}
	return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
}

func getGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Registration{}, err
	}
	resp, err := api.GetGearWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func resolveGearBySN(ctx context.Context, c *gizclaw.Client, sn string) (string, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return "", err
	}
	resp, err := api.ResolveBySNWithResponse(ctx, sn)
	if err != nil {
		return "", err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.PublicKey, nil
	}
	return "", responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func resolveGearByIMEI(ctx context.Context, c *gizclaw.Client, tac, serial string) (string, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return "", err
	}
	resp, err := api.ResolveByIMEIWithResponse(ctx, tac, serial)
	if err != nil {
		return "", err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.PublicKey, nil
	}
	return "", responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func approveGear(ctx context.Context, c *gizclaw.Client, publicKey string, role gearservice.GearRole) (gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Registration{}, err
	}
	resp, err := api.ApproveGearWithResponse(ctx, publicKey, gearservice.ApproveRequest{Role: role})
	if err != nil {
		return gearservice.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func blockGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Registration{}, err
	}
	resp, err := api.BlockGearWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func getGearInfo(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.DeviceInfo, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.DeviceInfo{}, err
	}
	resp, err := api.GetGearInfoWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func getGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Configuration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Configuration{}, err
	}
	resp, err := api.GetGearConfigWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.Configuration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func putGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string, cfg gearservice.Configuration) (gearservice.Configuration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Configuration{}, err
	}
	resp, err := api.PutGearConfigWithResponse(ctx, publicKey, cfg)
	if err != nil {
		return gearservice.Configuration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON404)
}

func getGearRuntime(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Runtime, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Runtime{}, err
	}
	resp, err := api.GetGearRuntimeWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.Runtime{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Runtime{}, responseError(resp.StatusCode(), resp.Body)
}

func listGearsByLabel(ctx context.Context, c *gizclaw.Client, key, value string) ([]gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListByLabelWithResponse(ctx, key, value)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Items, nil
	}
	return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
}

func listGearsByCertification(ctx context.Context, c *gizclaw.Client, pType gearservice.GearCertificationType, authority gearservice.GearCertificationAuthority, id string) ([]gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListByCertificationWithResponse(ctx, pType, authority, id)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Items, nil
	}
	return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
}

func listGearsByFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel gearservice.GearFirmwareChannel) ([]gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListByFirmwareWithResponse(ctx, depot, channel)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200.Items, nil
	}
	return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
}

func deleteGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.Registration{}, err
	}
	resp, err := api.DeleteGearWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func refreshGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.RefreshResult, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.RefreshResult{}, err
	}
	resp, err := api.RefreshGearWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.RefreshResult{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.RefreshResult{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404, resp.JSON409, resp.JSON502)
}

func buildReleaseTar(t *testing.T, release adminservice.DepotRelease, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifest, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("marshal release: %v", err)
	}

	writeTarFile(t, tw, "manifest.json", manifest)
	for name, data := range files {
		writeTarFile(t, tw, name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()

	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("tar write: %v", err)
	}
}

func responseError(status int, body []byte, errs ...interface{}) error {
	for _, errResp := range errs {
		switch e := errResp.(type) {
		case *adminservice.ErrorResponse:
			if e != nil {
				return fmt.Errorf("%s: %s", e.Error.Code, e.Error.Message)
			}
		case *gearservice.ErrorResponse:
			if e != nil {
				return fmt.Errorf("%s: %s", e.Error.Code, e.Error.Message)
			}
		case *serverpublic.ErrorResponse:
			if e != nil {
				return fmt.Errorf("%s: %s", e.Error.Code, e.Error.Message)
			}
		}
	}

	text := strings.TrimSpace(string(body))
	if text != "" {
		return fmt.Errorf("unexpected status %d: %s", status, text)
	}
	if status != 0 {
		return fmt.Errorf("unexpected status %d", status)
	}
	return fmt.Errorf("unexpected empty response")
}

func strPtr(value string) *string {
	return &value
}
