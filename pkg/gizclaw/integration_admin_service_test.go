package gizclaw_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
)

func TestIntegrationAdminServiceFirmwareLifecycle(t *testing.T) {
	ts := startTestServer(t)

	admin := newTestClient(t, ts)
	if _, err := register(context.Background(), admin, serverpublic.RegistrationRequest{
		Device:            serverpublic.DeviceInfo{Name: strPtr("admin")},
		RegistrationToken: strPtr("admin_default"),
	}); err != nil {
		t.Fatalf("admin register error: %v", err)
	}

	if _, err := putFirmwareInfo(context.Background(), admin, "demo-main", adminservice.DepotInfo{
		Files: &[]adminservice.DepotInfoFile{{Path: "firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutFirmwareInfo error: %v", err)
	}

	payload := []byte("firmware-v1")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildReleaseTar(t, adminservice.DepotRelease{
		FirmwareSemver: "1.0.0",
		Channel:        strPtr("stable"),
		Files: &[]adminservice.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})

	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("stable"), tarData); err != nil {
		t.Fatalf("UploadFirmware error: %v", err)
	}
	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("beta"), buildReleaseTar(t, adminservice.DepotRelease{
		FirmwareSemver: "1.1.0",
		Channel:        strPtr("beta"),
		Files: &[]adminservice.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware beta error: %v", err)
	}
	if _, err := uploadFirmware(context.Background(), admin, "demo-main", adminservice.Channel("testing"), buildReleaseTar(t, adminservice.DepotRelease{
		FirmwareSemver: "1.2.0",
		Channel:        strPtr("testing"),
		Files: &[]adminservice.DepotFile{{
			Path:   "firmware.bin",
			Sha256: hex.EncodeToString(sum256[:]),
			Md5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})); err != nil {
		t.Fatalf("UploadFirmware testing error: %v", err)
	}

	depot, err := getFirmwareDepot(context.Background(), admin, "demo-main")
	if err != nil {
		t.Fatalf("GetFirmwareDepot error: %v", err)
	}
	if depot.Stable.FirmwareSemver != "1.0.0" {
		t.Fatalf("stable semver = %q", depot.Stable.FirmwareSemver)
	}
	items, err := listFirmwares(context.Background(), admin)
	if err != nil {
		t.Fatalf("ListFirmwares error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "demo-main" {
		t.Fatalf("ListFirmwares = %+v", items)
	}
	if release, err := getFirmwareChannel(context.Background(), admin, "demo-main", adminservice.Channel("stable")); err != nil || release.FirmwareSemver != "1.0.0" {
		t.Fatalf("GetFirmwareChannel = %+v, %v", release, err)
	}
	if _, err := releaseFirmware(context.Background(), admin, "demo-main"); err != nil {
		t.Fatalf("ReleaseFirmware error: %v", err)
	}
	if _, err := rollbackFirmware(context.Background(), admin, "demo-main"); err != nil {
		t.Fatalf("RollbackFirmware error: %v", err)
	}
}
