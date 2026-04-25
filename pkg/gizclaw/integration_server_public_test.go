package gizclaw_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"testing"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func TestIntegrationServerPublicRegisterAndReadBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("nested firmware path validation is only supported in Linux runtime")
	}

	ts := startTestServer(t)
	device := newTestClient(t, ts)
	if device.PeerConn() == nil {
		t.Fatal("PeerConn returned nil")
	}

	result, err := register(context.Background(), device, serverpublic.RegistrationRequest{
		Device: apitypes.DeviceInfo{
			Name: strPtr("demo-device"),
			Sn:   strPtr("sn-001"),
			Hardware: &apitypes.HardwareInfo{
				Manufacturer: strPtr("Acme"),
				Model:        strPtr("M1"),
			},
		},
		RegistrationToken: strPtr("device_default"),
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if result.Gear.PublicKey == "" {
		t.Fatal("empty public key after register")
	}

	info, err := getInfo(context.Background(), device)
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	if info.Name == nil || *info.Name != "demo-device" {
		t.Fatalf("device name = %+v", info.Name)
	}

	registration, err := getRegistration(context.Background(), device)
	if err != nil {
		t.Fatalf("GetRegistration error: %v", err)
	}
	if registration.Role != apitypes.GearRoleDevice {
		t.Fatalf("role = %q", registration.Role)
	}

	if _, err := getServerInfo(context.Background(), device); err != nil {
		t.Fatalf("GetServerInfo error: %v", err)
	}
	if _, err := putInfo(context.Background(), device, apitypes.DeviceInfo{
		Name: strPtr("demo-device-2"),
		Sn:   strPtr("sn-002"),
		Hardware: &apitypes.HardwareInfo{
			Depot: strPtr("demo-main"),
		},
	}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	if _, err := getRuntime(context.Background(), device); err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	if _, err := getConfig(context.Background(), device); err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            apitypes.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}
	if _, err := putFirmwareInfo(context.Background(), admin, "demo-main", apitypes.DepotInfo{
		Files: &[]apitypes.DepotInfoFile{{Path: "bundles/firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutFirmwareInfo error: %v", err)
	}

	payload := []byte("public-firmware")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildReleaseTar(t, apitypes.DepotRelease{
		FirmwareSemver: "1.0.0",
		Channel:        strPtr("stable"),
		Files: &[]apitypes.DepotFile{{
			Path:   "bundles/firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"bundles/firmware.bin": payload})
	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("stable"), tarData); err != nil {
		t.Fatalf("UploadFirmware error: %v", err)
	}
	if _, err := putGearConfig(context.Background(), admin, result.Gear.PublicKey, apitypes.Configuration{
		Firmware: &apitypes.FirmwareConfig{Channel: func() *apitypes.GearFirmwareChannel {
			ch := apitypes.GearFirmwareChannel("stable")
			return &ch
		}()},
	}); err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}

	if _, err := getOTA(context.Background(), device); err != nil {
		t.Fatalf("GetOTA error: %v", err)
	}
	data, headers, err := downloadFirmware(context.Background(), device, "bundles/firmware.bin")
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
