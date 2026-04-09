package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	itest "github.com/giztoy/giztoy-go/internal/testutil"
	"github.com/giztoy/giztoy-go/pkg/gears"
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
	result, err := waitForRefreshGearSuccess(admin, deviceResult.Gear.PublicKey)
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
	err = waitForRefreshGearFailure(admin, deviceResult.Gear.PublicKey)
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

func waitForRefreshGearSuccess(admin *Client, publicKey string) (gears.RefreshResult, error) {
	var lastResult gears.RefreshResult
	err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result, err := admin.RefreshGear(ctx, publicKey)
		cancel()
		lastResult = result
		if err == nil && result.Gear.Device.Hardware.Manufacturer == "Acme" {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("refresh gear did not return expected manufacturer, got %q", lastResult.Gear.Device.Hardware.Manufacturer)
	})
	if err != nil {
		return lastResult, err
	}
	return lastResult, nil
}

func waitForRefreshGearFailure(admin *Client, publicKey string) error {
	err := itest.WaitUntil(itest.ReadyTimeout, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := admin.RefreshGear(ctx, publicKey)
		cancel()
		if err != nil && strings.Contains(err.Error(), "DEVICE_REFRESH_FAILED") && !strings.Contains(err.Error(), "DEVICE_OFFLINE") {
			return nil
		}
		if err != nil {
			return err
		}
		return errors.New("refresh gear did not return expected failure")
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = admin.RefreshGear(ctx, publicKey)
	return err
}
