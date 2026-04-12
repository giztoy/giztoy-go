package gizclaw_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

func TestPublicRegisterAndReadBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("nested firmware path validation is only supported in Linux runtime")
	}

	ts := startTestServer(t)

	device := newTestClient(t, ts)
	if device.PeerConn() == nil {
		t.Fatal("PeerConn returned nil")
	}
	result, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device: gears.DeviceInfo{
			Name: "demo-device",
			SN:   "sn-001",
			Hardware: gears.HardwareInfo{
				Manufacturer: "Acme",
				Model:        "M1",
			},
		},
		RegistrationToken: "device_default",
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if result.Gear.PublicKey == "" {
		t.Fatal("empty public key after register")
	}

	info, err := device.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	if info.Name != "demo-device" {
		t.Fatalf("device name = %q", info.Name)
	}

	registration, err := device.GetRegistration(context.Background())
	if err != nil {
		t.Fatalf("GetRegistration error: %v", err)
	}
	if registration.Role != gears.GearRoleDevice {
		t.Fatalf("role = %q", registration.Role)
	}

	if _, err := device.GetServerInfo(context.Background()); err != nil {
		t.Fatalf("GetServerInfo error: %v", err)
	}
	if _, err := device.PutInfo(context.Background(), gears.DeviceInfo{
		Name: "demo-device-2",
		SN:   "sn-002",
		Hardware: gears.HardwareInfo{
			Depot: "demo/main",
		},
	}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	if _, err := device.GetRuntime(context.Background()); err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	if _, err := device.GetConfig(context.Background()); err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}

	admin := newTestClient(t, ts)
	if _, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}
	if _, err := admin.PutFirmwareInfo(context.Background(), "demo/main", firmware.DepotInfo{
		Files: []firmware.DepotInfoFile{{Path: "bundles/firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutFirmwareInfo error: %v", err)
	}

	payload := []byte("public-firmware")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(firmware.ChannelStable),
		Files: []firmware.DepotFile{{
			Path:   "bundles/firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"bundles/firmware.bin": payload})
	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelStable, tarData); err != nil {
		t.Fatalf("UploadFirmware error: %v", err)
	}
	if _, err := admin.PutGearConfig(context.Background(), result.Gear.PublicKey, gears.Configuration{
		Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}

	if _, err := device.GetOTA(context.Background()); err != nil {
		t.Fatalf("GetOTA error: %v", err)
	}
	data, headers, err := device.DownloadFirmware(context.Background(), "bundles/firmware.bin")
	if err != nil {
		t.Fatalf("DownloadFirmware error: %v", err)
	}
	if string(data) != string(payload) {
		t.Fatalf("DownloadFirmware payload = %q", data)
	}
	if headers.Get("X-Checksum-SHA256") == "" {
		t.Fatal("missing checksum header")
	}
}
