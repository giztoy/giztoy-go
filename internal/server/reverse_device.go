package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/httptransport"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

var ErrDeviceOffline = errors.New("server: device offline")

type reverseDeviceClient struct {
	http *http.Client
}

func newReverseDeviceClient(client *http.Client) *reverseDeviceClient {
	return &reverseDeviceClient{http: client}
}

func (c *reverseDeviceClient) GetInfo(ctx context.Context, _ string) (gears.RefreshInfo, error) {
	var out gears.RefreshInfo
	if err := c.getJSON(ctx, "/info", &out); err != nil {
		return gears.RefreshInfo{}, err
	}
	return out, nil
}

func (c *reverseDeviceClient) GetIdentifiers(ctx context.Context, _ string) (gears.RefreshIdentifiers, error) {
	var out gears.RefreshIdentifiers
	if err := c.getJSON(ctx, "/identifiers", &out); err != nil {
		return gears.RefreshIdentifiers{}, err
	}
	return out, nil
}

func (c *reverseDeviceClient) GetVersion(ctx context.Context, _ string) (gears.RefreshVersion, error) {
	var out gears.RefreshVersion
	if err := c.getJSON(ctx, "/version", &out); err != nil {
		return gears.RefreshVersion{}, err
	}
	return out, nil
}

func (c *reverseDeviceClient) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://giztoy"+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("reverse device http %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Server) refreshGearFromDevice(ctx context.Context, publicKey string) (gears.RefreshResult, error) {
	if _, err := s.gears.Get(ctx, publicKey); err != nil {
		return gears.RefreshResult{}, err
	}
	conn, ok := s.activePeer(publicKey)
	if !ok {
		return gears.RefreshResult{}, ErrDeviceOffline
	}
	client := httptransport.NewClient(conn, peer.ServiceReverse)
	return s.gears.RefreshFromProvider(ctx, publicKey, newReverseDeviceClient(client))
}
