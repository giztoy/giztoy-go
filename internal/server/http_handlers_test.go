package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/firmware"
	"github.com/haivivi/giztoy/go/pkg/gears"
)

func TestPublicAndAdminHandlers(t *testing.T) {
	srv, adminPK, devicePK := newHTTPTestServer(t)

	t.Run("public server info", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/server-info", nil)
		rr := httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("public info and registration", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/info", nil)
		rr := httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/registration", nil)
		rr = httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/runtime", nil)
		rr = httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/config", nil)
		rr = httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		cfgBody, _ := json.Marshal(gears.DeviceInfo{
			Name: "device-updated",
			SN:   "sn-device",
			Hardware: gears.HardwareInfo{
				Depot:  "demo",
				IMEIs:  []gears.GearIMEI{{Name: "main", TAC: "12345678", Serial: "0000001"}},
				Labels: []gears.GearLabel{{Key: "batch", Value: "b1"}},
			},
		})
		req = httptest.NewRequest(http.MethodPut, "/info", bytes.NewReader(cfgBody))
		rr = httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("put info status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("public ota and download", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ota", nil)
		rr := httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/download/firmware/firmware.bin", nil)
		rr = httptest.NewRecorder()
		srv.publicHandler(devicePK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
		if rr.Header().Get("X-Checksum-SHA256") == "" {
			t.Fatal("missing checksum header")
		}
	})

	t.Run("public register new device", func(t *testing.T) {
		body, _ := json.Marshal(gears.RegistrationRequest{
			Device: gears.DeviceInfo{Name: "new-device"},
		})
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.publicHandler("new-device-pk").ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("admin gears list and get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/gears", nil)
		rr := httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/"+devicePK, nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/sn/sn-device", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/imei/12345678/0000001", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/label/batch/b1", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/certification/certification/ce/ce-001", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/gears/firmware/demo/stable", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("admin gears config and state changes", func(t *testing.T) {
		cfgBody, _ := json.Marshal(gears.Configuration{
			Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelStable},
		})
		req := httptest.NewRequest(http.MethodPut, "/gears/"+devicePK+"/config", bytes.NewReader(cfgBody))
		rr := httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		for _, path := range []string{
			"/gears/" + devicePK + "/info",
			"/gears/" + devicePK + "/config",
			"/gears/" + devicePK + "/runtime",
			"/gears/" + devicePK + "/ota",
		} {
			req = httptest.NewRequest(http.MethodGet, path, nil)
			rr = httptest.NewRecorder()
			srv.adminHandler(adminPK).ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s status = %d body=%s", path, rr.Code, rr.Body.String())
			}
		}

		req = httptest.NewRequest(http.MethodPost, "/gears/"+devicePK+":block", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/gears/"+devicePK, nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("delete status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("admin firmware list and get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/firmwares", nil)
		rr := httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/firmwares/demo", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/firmwares/demo/stable", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodPut, "/firmwares/demo:release", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("release status = %d body=%s", rr.Code, rr.Body.String())
		}

		req = httptest.NewRequest(http.MethodPut, "/firmwares/demo:rollback", nil)
		rr = httptest.NewRecorder()
		srv.adminHandler(adminPK).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("rollback status = %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func newHTTPTestServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	srv, err := New(Config{
		DataDir:    t.TempDir(),
		ListenAddr: "127.0.0.1:0",
		ConfigPath: writeTempConfig(t, `
stores:
  mem:
    kind: keyvalue
    backend: memory
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: mem
depots:
  store: fw
`),
		Gears: GearsConfig{
			RegistrationTokens: map[string]gears.RegistrationToken{
				"admin_default":  {Role: gears.GearRoleAdmin},
				"device_default": {Role: gears.GearRoleDevice},
			},
		},
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	t.Cleanup(func() { srv.stores.Close() })

	adminResult, err := srv.gears.Register(context.Background(), gears.RegistrationRequest{
		PublicKey:         "admin-pk",
		RegistrationToken: "admin_default",
		Device:            gears.DeviceInfo{Name: "admin"},
	})
	if err != nil {
		t.Fatalf("admin register error: %v", err)
	}
	deviceResult, err := srv.gears.Register(context.Background(), gears.RegistrationRequest{
		PublicKey:         "device-pk",
		RegistrationToken: "device_default",
		Device: gears.DeviceInfo{
			Name: "device",
			SN:   "sn-device",
			Hardware: gears.HardwareInfo{
				Depot:  "demo",
				IMEIs:  []gears.GearIMEI{{Name: "main", TAC: "12345678", Serial: "0000001"}},
				Labels: []gears.GearLabel{{Key: "batch", Value: "b1"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("device register error: %v", err)
	}
	if _, err := srv.gears.PutConfig(context.Background(), "device-pk", gears.Configuration{
		Certifications: []gears.GearCertification{{
			Type:      gears.GearCertificationTypeCertification,
			Authority: gears.GearCertificationAuthorityCE,
			ID:        "ce-001",
		}},
		Firmware: gears.FirmwareConfig{Channel: gears.GearFirmwareChannelStable},
	}); err != nil {
		t.Fatalf("PutConfig error: %v", err)
	}
	srv.markPeerOnline("device-pk", nil)

	if err := srv.firmwareUploader.PutInfo("demo", firmware.DepotInfo{
		Files: []firmware.DepotInfoFile{{Path: "firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	payload := []byte("fw")
	sum256 := sha256.Sum256(payload)
	sumMD5 := md5.Sum(payload)
	tarData := buildServerReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.0.0",
		Channel:        string(firmware.ChannelStable),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload})
	if _, err := srv.firmwareUploader.UploadTar("demo", firmware.ChannelStable, bytes.NewReader(tarData)); err != nil {
		t.Fatalf("UploadTar error: %v", err)
	}
	if _, err := srv.firmwareUploader.UploadTar("demo", firmware.ChannelBeta, bytes.NewReader(buildServerReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.1.0",
		Channel:        string(firmware.ChannelBeta),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload}))); err != nil {
		t.Fatalf("UploadTar beta error: %v", err)
	}
	if _, err := srv.firmwareUploader.UploadTar("demo", firmware.ChannelTesting, bytes.NewReader(buildServerReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "1.2.0",
		Channel:        string(firmware.ChannelTesting),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload}))); err != nil {
		t.Fatalf("UploadTar testing error: %v", err)
	}
	if _, err := srv.firmwareUploader.UploadTar("demo", firmware.ChannelRollback, bytes.NewReader(buildServerReleaseTar(t, firmware.DepotRelease{
		FirmwareSemVer: "0.9.0",
		Channel:        string(firmware.ChannelRollback),
		Files: []firmware.DepotFile{{
			Path:   "firmware.bin",
			SHA256: hex.EncodeToString(sum256[:]),
			MD5:    hex.EncodeToString(sumMD5[:]),
		}},
	}, map[string][]byte{"firmware.bin": payload}))); err != nil {
		t.Fatalf("UploadTar rollback error: %v", err)
	}

	return srv, adminResult.Gear.PublicKey, deviceResult.Gear.PublicKey
}

func buildServerReleaseTar(t *testing.T, release firmware.DepotRelease, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	manifest, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("json marshal release: %v", err)
	}
	writeServerTarFile(t, tw, "manifest.json", manifest)
	for name, data := range files {
		writeServerTarFile(t, tw, name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func writeServerTarFile(t *testing.T, tw *tar.Writer, name string, data []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("tar write: %v", err)
	}
}
