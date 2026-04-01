package client

import (
	"context"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func (c *Client) ListGears(ctx context.Context) ([]gears.Registration, error) {
	var out struct {
		Items []gears.Registration `json:"items"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears", nil, &out)
	return out.Items, err
}

func (c *Client) GetGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gears.Registration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/"+escapePathSegment(publicKey), nil, &out)
	return out, err
}

func (c *Client) ResolveGearBySN(ctx context.Context, sn string) (string, error) {
	var out struct {
		PublicKey string `json:"public_key"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/sn/"+escapePathSegment(sn), nil, &out)
	return out.PublicKey, err
}

func (c *Client) ResolveGearByIMEI(ctx context.Context, tac, serial string) (string, error) {
	var out struct {
		PublicKey string `json:"public_key"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/imei/"+joinEscapedPath(tac, serial), nil, &out)
	return out.PublicKey, err
}

func (c *Client) ListGearsByLabel(ctx context.Context, key, value string) ([]gears.Registration, error) {
	var out struct {
		Items []gears.Registration `json:"items"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/label/"+joinEscapedPath(key, value), nil, &out)
	return out.Items, err
}

func (c *Client) ListGearsByCertification(ctx context.Context, certType gears.GearCertificationType, authority gears.GearCertificationAuthority, id string) ([]gears.Registration, error) {
	var out struct {
		Items []gears.Registration `json:"items"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/certification/"+joinEscapedPath(string(certType), string(authority), id), nil, &out)
	return out.Items, err
}

func (c *Client) ListGearsByFirmware(ctx context.Context, depot string, channel gears.GearFirmwareChannel) ([]gears.Registration, error) {
	var out struct {
		Items []gears.Registration `json:"items"`
	}
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/firmware/"+joinEscapedPath(depot, string(channel)), nil, &out)
	return out.Items, err
}

func (c *Client) GetGearInfo(ctx context.Context, publicKey string) (gears.DeviceInfo, error) {
	var out gears.DeviceInfo
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/info", nil, &out)
	return out, err
}

func (c *Client) GetGearConfig(ctx context.Context, publicKey string) (gears.Configuration, error) {
	var out gears.Configuration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/config", nil, &out)
	return out, err
}

func (c *Client) PutGearConfig(ctx context.Context, publicKey string, cfg gears.Configuration) (gears.Configuration, error) {
	var out gears.Configuration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPut, "/gears/"+escapePathSegment(publicKey)+"/config", cfg, &out)
	return out, err
}

func (c *Client) GetGearRuntime(ctx context.Context, publicKey string) (gears.Runtime, error) {
	var out gears.Runtime
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/runtime", nil, &out)
	return out, err
}

func (c *Client) GetGearOTA(ctx context.Context, publicKey string) (firmware.OTASummary, error) {
	var out firmware.OTASummary
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodGet, "/gears/"+escapePathSegment(publicKey)+"/ota", nil, &out)
	return out, err
}

func (c *Client) ApproveGear(ctx context.Context, publicKey string, role gears.GearRole) (gears.Registration, error) {
	var out gears.Registration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":approve", map[string]any{"role": role}, &out)
	return out, err
}

func (c *Client) BlockGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gears.Registration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":block", nil, &out)
	return out, err
}

func (c *Client) DeleteGear(ctx context.Context, publicKey string) (gears.Registration, error) {
	var out gears.Registration
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodDelete, "/gears/"+escapePathSegment(publicKey), nil, &out)
	return out, err
}

func (c *Client) RefreshGear(ctx context.Context, publicKey string) (gears.RefreshResult, error) {
	var out gears.RefreshResult
	err := c.doJSON(ctx, peer.ServiceAdmin, http.MethodPost, "/gears/"+escapePathSegment(publicKey)+":refresh", nil, &out)
	return out, err
}
