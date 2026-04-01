package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func (c *Client) ListFirmwares(ctx context.Context) ([]firmware.Depot, error) {
	var out struct {
		Items []firmware.Depot `json:"items"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/firmwares", nil, &out)
	return out.Items, err
}

func (c *Client) GetFirmwareDepot(ctx context.Context, depot string) (firmware.Depot, error) {
	var out firmware.Depot
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/firmwares/"+escapePathSegment(depot), nil, &out)
	return out, err
}

func (c *Client) PutFirmwareInfo(ctx context.Context, depot string, info firmware.DepotInfo) (firmware.Depot, error) {
	var out firmware.Depot
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot), info, &out)
	return out, err
}

func (c *Client) GetFirmwareChannel(ctx context.Context, depot string, channel firmware.Channel) (firmware.DepotRelease, error) {
	var out firmware.DepotRelease
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/firmwares/"+joinEscapedPath(depot, string(channel)), nil, &out)
	return out, err
}

func (c *Client) RollbackFirmware(ctx context.Context, depot string) (firmware.Depot, error) {
	var out firmware.Depot
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot)+":rollback", nil, &out)
	return out, err
}

func (c *Client) ReleaseFirmware(ctx context.Context, depot string) (firmware.Depot, error) {
	var out firmware.Depot
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPut, "/firmwares/"+escapePathSegment(depot)+":release", nil, &out)
	return out, err
}

func (c *Client) UploadFirmware(ctx context.Context, depot string, channel firmware.Channel, tarData []byte) (firmware.DepotRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://giztoy/firmwares/"+joinEscapedPath(depot, string(channel)), bytes.NewReader(tarData))
	if err != nil {
		return firmware.DepotRelease{}, err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := c.HTTPClient(peer.ServiceAdmin).Do(req)
	if err != nil {
		return firmware.DepotRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return firmware.DepotRelease{}, fmt.Errorf("http status %d: %s", resp.StatusCode, string(data))
	}
	var out firmware.DepotRelease
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return firmware.DepotRelease{}, err
	}
	return out, nil
}
