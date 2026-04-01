package device_test

import (
	"context"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/client"
	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func TestDeviceRegisterConfigPingFlow(t *testing.T) {
	srv, err := server.New(server.Config{
		DataDir:    t.TempDir(),
		ListenAddr: "127.0.0.1:0",
		Gears: server.GearsConfig{
			RegistrationTokens: map[string]gears.RegistrationToken{
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		FirmwareRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = srv.Run(ctx)
	}()
	time.Sleep(200 * time.Millisecond)

	key, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.Dial(key, srv.ListenAddr(), srv.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
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
