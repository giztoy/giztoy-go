package gizclaw_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
)

func TestIntegrationPeerPublicRefresh(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("admin")},
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, ts)
	deviceResult, err := register(context.Background(), device, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("gear")},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	device.Device = apitypes.DeviceInfo{
		Hardware: &apitypes.HardwareInfo{
			Manufacturer:   strPtr("Acme"),
			Model:          strPtr("M1"),
			Depot:          strPtr("demo"),
			FirmwareSemver: strPtr("1.2.3"),
		},
		Sn: strPtr("sn-r1"),
	}

	result, err := waitForRefreshGearSuccess(admin, deviceResult.Gear.PublicKey)
	if err != nil {
		t.Fatalf("RefreshGear error: %v", err)
	}
	if result.Gear.Device.Hardware == nil || result.Gear.Device.Hardware.Manufacturer == nil || *result.Gear.Device.Hardware.Manufacturer != "Acme" {
		t.Fatalf("manufacturer = %+v", result.Gear.Device.Hardware)
	}
}

func TestIntegrationPeerPublicRefreshReportsOfflineWhenDeviceDisconnected(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("admin")},
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, ts)
	deviceResult, err := register(context.Background(), device, gearservice.RegistrationRequest{
		Device: apitypes.DeviceInfo{Name: strPtr("gear")},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}
	if err := device.Close(); err != nil {
		t.Fatalf("device close error: %v", err)
	}

	err = waitForRefreshGearFailure(admin, deviceResult.Gear.PublicKey)
	if err == nil {
		t.Fatal("RefreshGear should fail when peer disconnects")
	}
	if !isOfflineRefreshError(err) {
		t.Fatalf("RefreshGear error = %v, want offline-equivalent error", err)
	}
}

func waitForRefreshGearSuccess(admin *gizclaw.Client, publicKey string) (adminservice.RefreshResult, error) {
	var lastResult adminservice.RefreshResult
	err := waitUntil(testReadyTimeout, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result, err := refreshGear(ctx, admin, publicKey)
		cancel()
		lastResult = result
		if err == nil &&
			result.Gear.Device.Hardware != nil &&
			result.Gear.Device.Hardware.Manufacturer != nil &&
			*result.Gear.Device.Hardware.Manufacturer == "Acme" {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("refresh gear did not return expected manufacturer, got %+v", lastResult.Gear.Device.Hardware)
	})
	if err != nil {
		return lastResult, err
	}
	return lastResult, nil
}

func waitForRefreshGearFailure(admin *gizclaw.Client, publicKey string) error {
	var offlineErr error
	err := waitUntil(testReadyTimeout, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := refreshGear(ctx, admin, publicKey)
		cancel()
		if isOfflineRefreshError(err) {
			offlineErr = err
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
	_, err = refreshGear(ctx, admin, publicKey)
	if isOfflineRefreshError(err) {
		return err
	}
	if offlineErr != nil {
		return offlineErr
	}
	return err
}

func isOfflineRefreshError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "DEVICE_OFFLINE") || strings.Contains(msg, "conn closed")
}
