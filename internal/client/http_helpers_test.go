package client

import (
	"context"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func startTestServer(t *testing.T) (*server.Server, context.CancelFunc) {
	t.Helper()
	dataDir := t.TempDir()
	return startTestServerWithConfig(t, server.Config{
		DataDir:    dataDir,
		ListenAddr: "127.0.0.1:0",
		Stores: map[string]stores.Config{
			"mem": {Kind: stores.KindKeyValue, Backend: "memory"},
			"fw":  {Kind: stores.KindFS, Backend: "filesystem", Dir: "firmware"},
		},
		Gears: server.GearsConfig{
			Store: "mem",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"admin_default":  {Role: gears.GearRoleAdmin},
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		Depots: server.DepotsConfig{Store: "fw"},
	})
}

func startTestServerWithConfig(t *testing.T, cfg server.Config) (*server.Server, context.CancelFunc) {
	t.Helper()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = srv.Run(ctx)
	}()
	waitForTestServerReady(t, srv)
	return srv, cancel
}

func newTestClient(t *testing.T, srv *server.Server) *Client {
	t.Helper()
	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	c, err := Dial(key, srv.ListenAddr(), srv.PublicKey())
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	waitForClientPublicReady(t, c)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func waitForTestServerReady(t *testing.T, srv *server.Server) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		addr := srv.ListenAddr()
		if addr == "" {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		key, err := noise.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair(ready check): %v", err)
		}
		c, err := Dial(key, addr, srv.PublicKey())
		if err == nil {
			infoErr := probeClientPublicReady(c)
			_ = c.Close()
			if infoErr == nil {
				return
			}
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("test server did not become ready")
}

func waitForClientPublicReady(t *testing.T, c *Client) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := probeClientPublicReady(c); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("test client public service did not become ready")
}

func probeClientPublicReady(c *Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := c.GetServerInfo(ctx)
	return err
}
