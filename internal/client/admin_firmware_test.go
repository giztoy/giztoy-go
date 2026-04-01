package client

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
)

func TestAdminFirmwareLifecycle(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	admin := newTestClient(t, srv)
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
	tarData := buildClientReleaseTar(t, firmware.DepotRelease{
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
	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelBeta, buildClientReleaseTar(t, firmware.DepotRelease{
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
	if _, err := admin.UploadFirmware(context.Background(), "demo/main", firmware.ChannelTesting, buildClientReleaseTar(t, firmware.DepotRelease{
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

func buildClientReleaseTar(t *testing.T, release firmware.DepotRelease, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifest, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("json marshal release: %v", err)
	}
	writeClientTarFile(t, tw, "manifest.json", manifest)
	for name, data := range files {
		writeClientTarFile(t, tw, name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func writeClientTarFile(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("tar write: %v", err)
	}
}
