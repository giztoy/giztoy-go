package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/haivivi/giztoy/go/pkg/firmware"
	"github.com/haivivi/giztoy/go/pkg/gears"
)

const (
	publicServiceID         = 0
	defaultAdminServiceID   = 1
	defaultReverseServiceID = 2
)

func (c *Client) GetServerInfo(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/server-info", nil, &out)
	return out, err
}

func (c *Client) GetInfo(ctx context.Context) (gears.DeviceInfo, error) {
	var out gears.DeviceInfo
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/info", nil, &out)
	return out, err
}

func (c *Client) PutInfo(ctx context.Context, info gears.DeviceInfo) (gears.DeviceInfo, error) {
	var out gears.DeviceInfo
	err := c.doJSON(ctx, publicServiceID, http.MethodPut, "/info", info, &out)
	return out, err
}

func (c *Client) GetRegistration(ctx context.Context) (gears.Registration, error) {
	var out gears.Registration
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/registration", nil, &out)
	return out, err
}

func (c *Client) GetRuntime(ctx context.Context) (gears.Runtime, error) {
	var out gears.Runtime
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/runtime", nil, &out)
	return out, err
}

func (c *Client) GetConfig(ctx context.Context) (gears.Configuration, error) {
	var out gears.Configuration
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/config", nil, &out)
	return out, err
}

func (c *Client) Register(ctx context.Context, req gears.RegistrationRequest) (gears.RegistrationResult, error) {
	var out gears.RegistrationResult
	err := c.doJSON(ctx, publicServiceID, http.MethodPost, "/register", req, &out)
	return out, err
}

func (c *Client) GetOTA(ctx context.Context) (firmware.OTASummary, error) {
	var out firmware.OTASummary
	err := c.doJSON(ctx, publicServiceID, http.MethodGet, "/ota", nil, &out)
	return out, err
}

func (c *Client) DownloadFirmware(ctx context.Context, path string) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://giztoy/download/firmware/"+escapePathSegment(path), nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.HTTPClient(publicServiceID).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("public http status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	return data, resp.Header, err
}

func (c *Client) doJSON(ctx context.Context, service uint64, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://giztoy"+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTPClient(service).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(data))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
