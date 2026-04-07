package device_test

import (
	"context"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func TestDeviceRegisterConfigPingFlow(t *testing.T) {
	dataDir := t.TempDir()
	srv, err := server.New(server.Config{
		DataDir:    dataDir,
		ListenAddr: "127.0.0.1:0",
		Stores: map[string]stores.Config{
			"mem": {Kind: stores.KindKeyValue, Backend: "memory"},
			"fw":  {Kind: stores.KindFS, Backend: "filesystem", Dir: "firmware"},
		},
		Gears: server.GearsConfig{
			Store: "mem",
			RegistrationTokens: map[string]gears.RegistrationToken{
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		Depots: server.DepotsConfig{Store: "fw"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = srv.Run(ctx)
	}()
	waitForTestServerReady(t, srv)

	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(key, srv.ListenAddr(), srv.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	waitForClientPublicReady(t, c)
	defer c.Close()

	if _, err := c.Register(context.Background(), gears.RegistrationRequest{
		Device: gears.DeviceInfo{
			Name: "device-1",
			Hardware: gears.HardwareInfo{
				Manufacturer: "Acme",
				Model:        "M1",
			},
		},
		RegistrationToken: "device_default",
	}); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	cfg, err := c.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	if cfg.Firmware.Channel != "" {
		t.Fatalf("unexpected firmware channel: %q", cfg.Firmware.Channel)
	}

	if _, err := c.Ping(); err != nil {
		t.Fatalf("Ping error: %v", err)
	}
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
		c, err := client.Dial(key, addr, srv.PublicKey())
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

func waitForClientPublicReady(t *testing.T, c *client.Client) {
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

func probeClientPublicReady(c *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := c.GetServerInfo(ctx)
	return err
}
