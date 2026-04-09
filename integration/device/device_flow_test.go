package device_test

import (
	"context"
	"testing"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/internal/stores"
	itest "github.com/giztoy/giztoy-go/internal/testutil"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func TestDeviceRegisterConfigPingFlow(t *testing.T) {
	dataDir := t.TempDir()
	listenAddr := itest.AllocateUDPAddr(t)
	srv, err := server.New(server.Config{
		DataDir:    dataDir,
		ListenAddr: listenAddr,
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
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Run(ctx)
	}()
	waitForTestServerReady(t, srv, listenAddr, errCh)

	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(key, listenAddr, srv.PublicKey())
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

func waitForTestServerReady(t *testing.T, srv *server.Server, addr string, errCh <-chan error) {
	t.Helper()

	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		select {
		case err := <-errCh:
			return err
		default:
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
				return nil
			}
			return infoErr
		}
		return err
	}); err != nil {
		t.Fatalf("test server did not become ready: %v", err)
	}
}

func waitForClientPublicReady(t *testing.T, c *client.Client) {
	t.Helper()

	if err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		return probeClientPublicReady(c)
	}); err != nil {
		t.Fatalf("test client public service did not become ready: %v", err)
	}
}

func probeClientPublicReady(c *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), itest.ProbeTimeout)
	defer cancel()
	_, err := c.GetServerInfo(ctx)
	return err
}
