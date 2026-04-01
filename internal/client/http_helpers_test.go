package client

import (
	"context"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/internal/server"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

func startTestServer(t *testing.T) (*server.Server, context.CancelFunc) {
	t.Helper()
	return startTestServerWithConfig(t, server.Config{
		DataDir:    t.TempDir(),
		ListenAddr: "127.0.0.1:0",
		Gears: server.GearsConfig{
			RegistrationTokens: map[string]gears.RegistrationToken{
				"admin_default":  {Role: gears.GearRoleAdmin},
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		FirmwareRoot: t.TempDir(),
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
	time.Sleep(200 * time.Millisecond)
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
	t.Cleanup(func() { _ = c.Close() })
	return c
}
