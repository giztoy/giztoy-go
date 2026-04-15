package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func ConnectFromContext(name string) (*gizclaw.Client, error) {
	c, serverPK, serverAddr, err := DialFromContext(name)
	if err != nil {
		return nil, err
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.DialAndServe(serverPK, serverAddr)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-errCh:
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("gizclaw: client stopped before ready")
		default:
		}
		if err := probeServerPublicReady(c); err == nil {
			return c, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = c.Close()
	return nil, fmt.Errorf("gizclaw: timeout waiting for client readiness")
}

func probeServerPublicReady(c *gizclaw.Client) error {
	if c == nil {
		return fmt.Errorf("gizclaw: nil client")
	}
	if c.PeerConn() == nil {
		return fmt.Errorf("gizclaw: client is not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := GetServerInfo(ctx, c)
	return err
}

func Register(ctx context.Context, c *gizclaw.Client, req serverpublic.RegistrationRequest) (serverpublic.RegistrationResult, error) {
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

func GetConfig(ctx context.Context, c *gizclaw.Client) (serverpublic.Configuration, error) {
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

func GetServerInfo(ctx context.Context, c *gizclaw.Client) (serverpublic.ServerInfo, error) {
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

func GetInfo(ctx context.Context, c *gizclaw.Client) (serverpublic.DeviceInfo, error) {
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

func PutInfo(ctx context.Context, c *gizclaw.Client, info serverpublic.DeviceInfo) (serverpublic.DeviceInfo, error) {
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

func GetRuntime(ctx context.Context, c *gizclaw.Client) (serverpublic.Runtime, error) {
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

func GetRegistration(ctx context.Context, c *gizclaw.Client) (serverpublic.Registration, error) {
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

func GetOTA(ctx context.Context, c *gizclaw.Client) (serverpublic.OTASummary, error) {
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

func DownloadFirmware(ctx context.Context, c *gizclaw.Client, path string) ([]byte, http.Header, error) {
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

func ListFirmwares(ctx context.Context, c *gizclaw.Client) ([]adminservice.Depot, error) {
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

func GetFirmwareDepot(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
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

func GetFirmwareChannel(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel) (adminservice.DepotRelease, error) {
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

func PutFirmwareInfo(ctx context.Context, c *gizclaw.Client, depot string, info adminservice.DepotInfo) (adminservice.Depot, error) {
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

func UploadFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel, data []byte) (adminservice.DepotRelease, error) {
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

func ReleaseFirmware(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
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

func RollbackFirmware(ctx context.Context, c *gizclaw.Client, depot string) (adminservice.Depot, error) {
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

func ListGears(ctx context.Context, c *gizclaw.Client) ([]gearservice.Registration, error) {
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

func GetGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
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

func ResolveGearBySN(ctx context.Context, c *gizclaw.Client, sn string) (string, error) {
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

func ResolveGearByIMEI(ctx context.Context, c *gizclaw.Client, tac, serial string) (string, error) {
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

func ApproveGear(ctx context.Context, c *gizclaw.Client, publicKey string, role gearservice.GearRole) (gearservice.Registration, error) {
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

func BlockGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
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

func GetGearInfo(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.DeviceInfo, error) {
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

func GetGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Configuration, error) {
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

func PutGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string, cfg gearservice.Configuration) (gearservice.Configuration, error) {
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

func GetGearRuntime(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Runtime, error) {
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

func GetGearOTA(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.OTASummary, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return gearservice.OTASummary{}, err
	}
	resp, err := api.GetGearOTAWithResponse(ctx, publicKey)
	if err != nil {
		return gearservice.OTASummary{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return gearservice.OTASummary{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func ListGearsByLabel(ctx context.Context, c *gizclaw.Client, key, value string) ([]gearservice.Registration, error) {
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

func ListGearsByCertification(ctx context.Context, c *gizclaw.Client, pType gearservice.GearCertificationType, authority gearservice.GearCertificationAuthority, id string) ([]gearservice.Registration, error) {
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

func ListGearsByFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel gearservice.GearFirmwareChannel) ([]gearservice.Registration, error) {
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

func DeleteGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.Registration, error) {
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

func RefreshGear(ctx context.Context, c *gizclaw.Client, publicKey string) (gearservice.RefreshResult, error) {
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
