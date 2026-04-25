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

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
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

func GetConfig(ctx context.Context, c *gizclaw.Client) (apitypes.Configuration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.Configuration{}, err
	}
	resp, err := api.GetConfigWithResponse(ctx)
	if err != nil {
		return apitypes.Configuration{}, err
	}
	if resp.JSON200 != nil {
		cfg := *resp.JSON200
		if cfg.Firmware == nil {
			cfg.Firmware = &apitypes.FirmwareConfig{}
		}
		return cfg, nil
	}
	return apitypes.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func GetServerInfo(ctx context.Context, c *gizclaw.Client) (apitypes.ServerInfo, error) {
	api, err := c.ServerPublicClient()
	if err != nil {
		return apitypes.ServerInfo{}, err
	}
	resp, err := api.GetServerInfoWithResponse(ctx)
	if err != nil {
		return apitypes.ServerInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.ServerInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func GetInfo(ctx context.Context, c *gizclaw.Client) (apitypes.DeviceInfo, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	resp, err := api.GetInfoWithResponse(ctx)
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func PutInfo(ctx context.Context, c *gizclaw.Client, info apitypes.DeviceInfo) (apitypes.DeviceInfo, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	resp, err := api.PutInfoWithResponse(ctx, info)
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON404)
}

func GetRuntime(ctx context.Context, c *gizclaw.Client) (apitypes.Runtime, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.Runtime{}, err
	}
	resp, err := api.GetRuntimeWithResponse(ctx)
	if err != nil {
		return apitypes.Runtime{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Runtime{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func GetRegistration(ctx context.Context, c *gizclaw.Client) (apitypes.Registration, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.Registration{}, err
	}
	resp, err := api.GetRegistrationWithResponse(ctx)
	if err != nil {
		return apitypes.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func GetOTA(ctx context.Context, c *gizclaw.Client) (apitypes.OTASummary, error) {
	api, err := c.GearServiceClient()
	if err != nil {
		return apitypes.OTASummary{}, err
	}
	resp, err := api.GetOTAWithResponse(ctx)
	if err != nil {
		return apitypes.OTASummary{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.OTASummary{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func DownloadFirmware(ctx context.Context, c *gizclaw.Client, path string) ([]byte, http.Header, error) {
	api, err := c.GearServiceClient()
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

func ListFirmwares(ctx context.Context, c *gizclaw.Client) ([]apitypes.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	resp, err := api.ListDepotsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
	}
	return resp.JSON200.Items, nil
}

func GetFirmwareDepot(ctx context.Context, c *gizclaw.Client, depot string) (apitypes.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Depot{}, err
	}
	resp, err := api.GetDepotWithResponse(ctx, depot)
	if err != nil {
		return apitypes.Depot{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Depot{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func GetFirmwareChannel(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel) (apitypes.DepotRelease, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.DepotRelease{}, err
	}
	resp, err := api.GetChannelWithResponse(ctx, depot, channel)
	if err != nil {
		return apitypes.DepotRelease{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.DepotRelease{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func PutFirmwareInfo(ctx context.Context, c *gizclaw.Client, depot string, info apitypes.DepotInfo) (apitypes.Depot, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Depot{}, err
	}
	resp, err := api.PutDepotInfoWithResponse(ctx, depot, info)
	if err != nil {
		return apitypes.Depot{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Depot{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON409, resp.JSON500)
}

func UploadFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel adminservice.Channel, data []byte) (apitypes.DepotRelease, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.DepotRelease{}, err
	}
	resp, err := api.PutChannelWithBodyWithResponse(ctx, depot, channel, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return apitypes.DepotRelease{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.DepotRelease{}, responseError(resp.StatusCode(), resp.Body, resp.JSON409)
}

func ReleaseFirmware(ctx context.Context, c *gizclaw.Client, depot string) (apitypes.Depot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://gizclaw/depots/"+url.PathEscape(depot)+"/@release", nil)
	if err != nil {
		return apitypes.Depot{}, err
	}
	resp, err := c.HTTPClient(gizclaw.ServiceAdmin).Do(req)
	if err != nil {
		return apitypes.Depot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apitypes.Depot{}, err
	}
	if resp.StatusCode == http.StatusOK {
		var out apitypes.Depot
		if err := json.Unmarshal(body, &out); err != nil {
			return apitypes.Depot{}, err
		}
		return out, nil
	}
	return apitypes.Depot{}, responseError(resp.StatusCode, body)
}

func RollbackFirmware(ctx context.Context, c *gizclaw.Client, depot string) (apitypes.Depot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://gizclaw/depots/"+url.PathEscape(depot)+"/@rollback", nil)
	if err != nil {
		return apitypes.Depot{}, err
	}
	resp, err := c.HTTPClient(gizclaw.ServiceAdmin).Do(req)
	if err != nil {
		return apitypes.Depot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apitypes.Depot{}, err
	}
	if resp.StatusCode == http.StatusOK {
		var out apitypes.Depot
		if err := json.Unmarshal(body, &out); err != nil {
			return apitypes.Depot{}, err
		}
		return out, nil
	}
	return apitypes.Depot{}, responseError(resp.StatusCode, body)
}

type pagedItems[T any] struct {
	HasNext    bool
	Items      []T
	NextCursor *string
}

func collectAllPages[T any](
	fetchPage func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[T], error),
) ([]T, error) {
	limit := adminservice.Limit(200)
	var cursor *adminservice.Cursor
	items := make([]T, 0)
	for {
		page, err := fetchPage(cursor, &limit)
		if err != nil {
			return nil, err
		}
		items = append(items, page.Items...)
		if !page.HasNext || page.NextCursor == nil || *page.NextCursor == "" {
			return items, nil
		}
		next := adminservice.Cursor(*page.NextCursor)
		cursor = &next
	}
}

func ListGears(ctx context.Context, c *gizclaw.Client) ([]apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	return collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[apitypes.Registration], error) {
		resp, err := api.ListGearsWithResponse(ctx, &adminservice.ListGearsParams{
			Cursor: cursor,
			Limit:  limit,
		})
		if err != nil {
			return pagedItems[apitypes.Registration]{}, err
		}
		if resp.JSON200 == nil {
			return pagedItems[apitypes.Registration]{}, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
		}
		return pagedItems[apitypes.Registration]{
			HasNext:    resp.JSON200.HasNext,
			Items:      resp.JSON200.Items,
			NextCursor: resp.JSON200.NextCursor,
		}, nil
	})
}

func GetGear(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Registration{}, err
	}
	resp, err := api.GetGearWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func ResolveGearBySN(ctx context.Context, c *gizclaw.Client, sn string) (string, error) {
	api, err := c.ServerAdminClient()
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
	api, err := c.ServerAdminClient()
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

func ApproveGear(ctx context.Context, c *gizclaw.Client, publicKey string, role apitypes.GearRole) (apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Registration{}, err
	}
	resp, err := api.ApproveGearWithResponse(ctx, publicKey, adminservice.ApproveRequest{Role: role})
	if err != nil {
		return apitypes.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400)
}

func BlockGear(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Registration{}, err
	}
	resp, err := api.BlockGearWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func GetGearInfo(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.DeviceInfo, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	resp, err := api.GetGearInfoWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.DeviceInfo{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.DeviceInfo{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func GetGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.Configuration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Configuration{}, err
	}
	resp, err := api.GetGearConfigWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.Configuration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func PutGearConfig(ctx context.Context, c *gizclaw.Client, publicKey string, cfg apitypes.Configuration) (apitypes.Configuration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Configuration{}, err
	}
	resp, err := api.PutGearConfigWithResponse(ctx, publicKey, cfg)
	if err != nil {
		return apitypes.Configuration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Configuration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON400, resp.JSON404)
}

func GetGearRuntime(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.Runtime, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Runtime{}, err
	}
	resp, err := api.GetGearRuntimeWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.Runtime{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Runtime{}, responseError(resp.StatusCode(), resp.Body)
}

func GetGearOTA(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.OTASummary, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.OTASummary{}, err
	}
	resp, err := api.GetGearOTAWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.OTASummary{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.OTASummary{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func ListGearsByLabel(ctx context.Context, c *gizclaw.Client, key, value string) ([]apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	return collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[apitypes.Registration], error) {
		resp, err := api.ListByLabelWithResponse(ctx, key, value, &adminservice.ListByLabelParams{
			Cursor: cursor,
			Limit:  limit,
		})
		if err != nil {
			return pagedItems[apitypes.Registration]{}, err
		}
		if resp.JSON200 == nil {
			return pagedItems[apitypes.Registration]{}, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
		}
		return pagedItems[apitypes.Registration]{
			HasNext:    resp.JSON200.HasNext,
			Items:      resp.JSON200.Items,
			NextCursor: resp.JSON200.NextCursor,
		}, nil
	})
}

func ListGearsByCertification(ctx context.Context, c *gizclaw.Client, pType apitypes.GearCertificationType, authority apitypes.GearCertificationAuthority, id string) ([]apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	return collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[apitypes.Registration], error) {
		resp, err := api.ListByCertificationWithResponse(ctx, pType, authority, id, &adminservice.ListByCertificationParams{
			Cursor: cursor,
			Limit:  limit,
		})
		if err != nil {
			return pagedItems[apitypes.Registration]{}, err
		}
		if resp.JSON200 == nil {
			return pagedItems[apitypes.Registration]{}, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
		}
		return pagedItems[apitypes.Registration]{
			HasNext:    resp.JSON200.HasNext,
			Items:      resp.JSON200.Items,
			NextCursor: resp.JSON200.NextCursor,
		}, nil
	})
}

func ListGearsByFirmware(ctx context.Context, c *gizclaw.Client, depot string, channel apitypes.GearFirmwareChannel) ([]apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return nil, err
	}
	return collectAllPages(func(cursor *adminservice.Cursor, limit *adminservice.Limit) (pagedItems[apitypes.Registration], error) {
		resp, err := api.ListByFirmwareWithResponse(ctx, depot, channel, &adminservice.ListByFirmwareParams{
			Cursor: cursor,
			Limit:  limit,
		})
		if err != nil {
			return pagedItems[apitypes.Registration]{}, err
		}
		if resp.JSON200 == nil {
			return pagedItems[apitypes.Registration]{}, responseError(resp.StatusCode(), resp.Body, resp.JSON500)
		}
		return pagedItems[apitypes.Registration]{
			HasNext:    resp.JSON200.HasNext,
			Items:      resp.JSON200.Items,
			NextCursor: resp.JSON200.NextCursor,
		}, nil
	})
}

func DeleteGear(ctx context.Context, c *gizclaw.Client, publicKey string) (apitypes.Registration, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return apitypes.Registration{}, err
	}
	resp, err := api.DeleteGearWithResponse(ctx, publicKey)
	if err != nil {
		return apitypes.Registration{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return apitypes.Registration{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404)
}

func RefreshGear(ctx context.Context, c *gizclaw.Client, publicKey string) (adminservice.RefreshResult, error) {
	api, err := c.ServerAdminClient()
	if err != nil {
		return adminservice.RefreshResult{}, err
	}
	resp, err := api.RefreshGearWithResponse(ctx, publicKey)
	if err != nil {
		return adminservice.RefreshResult{}, err
	}
	if resp.JSON200 != nil {
		return *resp.JSON200, nil
	}
	return adminservice.RefreshResult{}, responseError(resp.StatusCode(), resp.Body, resp.JSON404, resp.JSON409, resp.JSON502)
}

func responseError(status int, body []byte, errs ...interface{}) error {
	for _, errResp := range errs {
		switch e := errResp.(type) {
		case *apitypes.ErrorResponse:
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
