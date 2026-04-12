package gizclaw_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

func TestAdminGearsLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	adminResult, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	})
	if err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	device := newTestClient(t, ts)
	deviceResult, err := device.Register(context.Background(), gears.RegistrationRequest{
		Device: gears.DeviceInfo{
			Name: "device",
			SN:   "sn/1",
			Hardware: gears.HardwareInfo{
				Depot:  "demo/main",
				IMEIs:  []gears.GearIMEI{{Name: "main", TAC: "12345678", Serial: "0000001"}},
				Labels: []gears.GearLabel{{Key: "batch", Value: "cn/east"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}

	items, err := admin.ListGears(context.Background())
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("ListGears returned %d items", len(items))
	}

	if _, err := admin.ApproveGear(context.Background(), deviceResult.Gear.PublicKey, gears.GearRoleDevice); err != nil {
		t.Fatalf("ApproveGear error: %v", err)
	}
	if _, err := admin.GetGear(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGear error: %v", err)
	}
	if publicKey, err := admin.ResolveGearBySN(context.Background(), "sn/1"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearBySN = %q, %v", publicKey, err)
	}
	if publicKey, err := admin.ResolveGearByIMEI(context.Background(), "12345678", "0000001"); err != nil || publicKey != deviceResult.Gear.PublicKey {
		t.Fatalf("ResolveGearByIMEI = %q, %v", publicKey, err)
	}
	if _, err := admin.PutGearConfig(context.Background(), deviceResult.Gear.PublicKey, gears.Configuration{
		Certifications: []gears.GearCertification{{
			Type:      gears.GearCertificationTypeCertification,
			Authority: gears.GearCertificationAuthorityCE,
			ID:        "ce/001",
		}},
		Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutGearConfig error: %v", err)
	}
	if _, err := admin.GetGearInfo(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearInfo error: %v", err)
	}
	if _, err := admin.GetGearConfig(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearConfig error: %v", err)
	}
	if _, err := admin.GetGearRuntime(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	if _, err := admin.ListGearsByLabel(context.Background(), "batch", "cn/east"); err != nil {
		t.Fatalf("ListGearsByLabel error: %v", err)
	}
	if _, err := admin.ListGearsByCertification(context.Background(), gears.GearCertificationTypeCertification, gears.GearCertificationAuthorityCE, "ce/001"); err != nil {
		t.Fatalf("ListGearsByCertification error: %v", err)
	}
	if _, err := admin.ListGearsByFirmware(context.Background(), "demo/main", gears.GearFirmwareChannelStable); err != nil {
		t.Fatalf("ListGearsByFirmware error: %v", err)
	}
	if _, err := admin.BlockGear(context.Background(), deviceResult.Gear.PublicKey); err != nil {
		t.Fatalf("BlockGear error: %v", err)
	}
	if _, err := admin.DeleteGear(context.Background(), adminResult.Gear.PublicKey); err != nil {
		t.Fatalf("DeleteGear error: %v", err)
	}
}

func TestAdminFirmwareLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := admin.Register(context.Background(), gears.RegistrationRequest{
		Device:            gears.DeviceInfo{Name: "admin"},
		RegistrationToken: "admin_default",
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	if _, err := admin.PutFirmwareInfo(context.Background(), "demo/main", firmware.DepotInfo{
		Files: []firmware.DepotInfoFile{{Path: "firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutFirmwareInfo error: %v", err)
	}

	payload := []byte("firmware-v1")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(firmware.ChannelStable),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})

	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelStable, tarData); err != nil {
		t.Fatalf("UploadFirmware error: %v", err)
	}
	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelBeta, buildReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.1.0",
		Channel:        string(firmware.ChannelBeta),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware beta error: %v", err)
	}
	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelTesting, buildReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.2.0",
		Channel:        string(firmware.ChannelTesting),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware testing error: %v", err)
	}

	depot, err := admin.GetFirmwareDepot(context.Background(), "demo/main")
	if err != nil {
		t.Fatalf("GetFirmwareDepot error: %v", err)
	}
	if depot.Stable.FirmwareSemVer != "1.0.0" {
		t.Fatalf("stable semver = %q", depot.Stable.FirmwareSemVer)
	}
	items, err := admin.ListFirmwares(context.Background())
	if err != nil {
		t.Fatalf("ListFirmwares error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo/main" {
		t.Fatalf("ListFirmwares = %+v", items)
	}
	if release, err := admin.GetFirmwareChannel(context.Background(), "demo/main", firmware.ChannelStable); err != nil || release.FirmwareSemVer != "1.0.0" {
		t.Fatalf("GetFirmwareChannel = %+v, %v", release, err)
	}
	if _, err := admin.ReleaseFirmware(context.Background(), "demo/main"); err != nil {
		t.Fatalf("ReleaseFirmware error: %v", err)
	}
	if _, err := admin.RollbackFirmware(context.Background(), "demo/main"); err != nil {
		t.Fatalf("RollbackFirmware error: %v", err)
	}
}
