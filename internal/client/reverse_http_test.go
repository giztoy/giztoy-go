package client

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/haivivi/giztoy/go/internal/server"
	"github.com/haivivi/giztoy/go/pkg/gears"
)

func TestReverseHTTPRefresh(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	admin := newTestClient(t, srv)
	if _, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, srv)
	deviceResult, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "device"},
		RegistrationToken: "device_default",
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		_ = device.ServeReverseHTTP(ctx, staticProvider{
			info: gears.RefreshInfo{
				Manufacturer: "Acme",
				Model:        "M1",
			},
			identifiers: gears.RefreshIdentifiers{
				SN: "sn-r1",
			},
			version: gears.RefreshVersion{
				Depot:          "demo",
				FirmwareSemVer: "1.2.3",
			},
		})
	}()
	time.Sleep(200 * time.Millisecond)

	result, err := admin.RefreshGear(context.Background(), deviceResult.Gear.PublicKey)
	if err != nil {
		t.Fatalf("RefreshGear error: %v", err)
	}
	if result.Gear.Device.Hardware.Manufacturer != "Acme" {
		t.Fatalf("manufacturer = %q", result.Gear.Device.Hardware.Manufacturer)
	}
}

func TestReverseHTTPRefreshErrorIsNotReportedAsOffline(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	admin := newTestClient(t, srv)
	if _, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, srv)
	deviceResult, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "device"},
		RegistrationToken: "device_default",
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		_ = device.ServeReverseHTTP(ctx, failingReverseProvider{})
	}()
	time.Sleep(200 * time.Millisecond)

	_, err = admin.RefreshGear(context.Background(), deviceResult.Gear.PublicKey)
	if err == nil {
		t.Fatal("RefreshGear should fail when reverse provider fails")
	}
	if !strings.Contains(err.Error(), "DEVICE_REFRESH_FAILED") {
		t.Fatalf("RefreshGear error = %v, want DEVICE_REFRESH_FAILED", err)
	}
	if strings.Contains(err.Error(), "DEVICE_OFFLINE") {
		t.Fatalf("RefreshGear error should not report offline: %v", err)
	}
}

func TestReverseHTTPRefreshWithCustomServiceIDs(t *testing.T) {
	srv, cancel := startTestServerWithConfig(t, server.Config{
		DataDir:          t.TempDir(),
		ListenAddr:       "127.0.0.1:0",
		AdminServiceID:   11,
		ReverseServiceID: 12,
		Gears: server.GearsConfig{
			RegistrationTokens: map[string]gears.RegistrationToken{
				"admin_default":  {Role: gears.GearRoleAdmin},
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
		FirmwareRoot: t.TempDir(),
	})
	defer cancel()

	admin := newTestClient(t, srv)
	if _, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, srv)
	deviceResult, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "device"},
		RegistrationToken: "device_default",
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		_ = device.ServeReverseHTTP(ctx, staticProvider{
			info:        gears.RefreshInfo{Manufacturer: "Acme"},
			identifiers: gears.RefreshIdentifiers{SN: "sn-custom-service"},
			version:     gears.RefreshVersion{Depot: "demo", FirmwareSemVer: "2.0.0"},
		})
	}()
	time.Sleep(200 * time.Millisecond)

	if _, err := admin.GetGear(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGear error with custom service IDs: %v", err)
	}
	result, err := admin.RefreshGear(context.Background(), deviceResult.Gear.PublicKey)
	if err != nil {
		t.Fatalf("RefreshGear error with custom service IDs: %v", err)
	}
	if result.Gear.Device.SN != "sn-custom-service" {
		t.Fatalf("SN = %q", result.Gear.Device.SN)
	}
}

type staticProvider struct {
	info        gears.RefreshInfo
	identifiers gears.RefreshIdentifiers
	version     gears.RefreshVersion
}

func (p staticProvider) Info(context.Context) (gears.RefreshInfo, error) {
	return p.info, nil
}

func (p staticProvider) Identifiers(context.Context) (gears.RefreshIdentifiers, error) {
	return p.identifiers, nil
}

func (p staticProvider) Version(context.Context) (gears.RefreshVersion, error) {
	return p.version, nil
}

type failingReverseProvider struct{}

func (failingReverseProvider) Info(context.Context) (gears.RefreshInfo, error) {
	return gears.RefreshInfo{}, errors.New("provider boom")
}

func (failingReverseProvider) Identifiers(context.Context) (gears.RefreshIdentifiers, error) {
	return gears.RefreshIdentifiers{}, errors.New("provider boom")
}

func (failingReverseProvider) Version(context.Context) (gears.RefreshVersion, error) {
	return gears.RefreshVersion{}, errors.New("provider boom")
}
