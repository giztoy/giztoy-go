package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/firmware"
)

func TestPublicAndAdminErrorHandlers(t *testing.T) {
	srv, adminPK, devicePK := newHTTPTestServer(t)

	tests := []struct {
		name   string
		pk     string
		target func() http.Handler
		method string
		path   string
		body   []byte
		want   int
	}{
		{name: "server info method not allowed", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/server-info", want: http.StatusMethodNotAllowed},
		{name: "public info invalid json", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPut, path: "/info", body: []byte("{"), want: http.StatusBadRequest},
		{name: "public info missing gear", pk: "missing", target: func() http.Handler { return srv.publicHandler("missing") }, method: http.MethodGet, path: "/info", want: http.StatusNotFound},
		{name: "registration wrong method", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/registration", want: http.StatusMethodNotAllowed},
		{name: "registration missing gear", pk: "missing", target: func() http.Handler { return srv.publicHandler("missing") }, method: http.MethodGet, path: "/registration", want: http.StatusNotFound},
		{name: "runtime wrong method", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/runtime", want: http.StatusMethodNotAllowed},
		{name: "config wrong method", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/config", want: http.StatusMethodNotAllowed},
		{name: "config missing gear", pk: "missing", target: func() http.Handler { return srv.publicHandler("missing") }, method: http.MethodGet, path: "/config", want: http.StatusNotFound},
		{name: "register invalid json", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/register", body: []byte("{"), want: http.StatusBadRequest},
		{name: "register conflict", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/register", body: []byte(`{"device":{"name":"dup"}}`), want: http.StatusConflict},
		{name: "register invalid token", pk: "fresh-device", target: func() http.Handler { return srv.publicHandler("fresh-device") }, method: http.MethodPost, path: "/register", body: []byte(`{"registration_token":"missing","device":{"name":"fresh"}}`), want: http.StatusBadRequest},
		{name: "ota wrong method", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/ota", want: http.StatusMethodNotAllowed},
		{name: "ota missing gear", pk: "missing", target: func() http.Handler { return srv.publicHandler("missing") }, method: http.MethodGet, path: "/ota", want: http.StatusNotFound},
		{name: "download wrong method", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodPost, path: "/download/firmware/firmware.bin", want: http.StatusMethodNotAllowed},
		{name: "download file missing", pk: devicePK, target: func() http.Handler { return srv.publicHandler(devicePK) }, method: http.MethodGet, path: "/download/firmware/missing.bin", want: http.StatusNotFound},
		{name: "admin forbidden for device", pk: devicePK, target: func() http.Handler { return srv.adminHandler(devicePK) }, method: http.MethodGet, path: "/gears", want: http.StatusForbidden},
		{name: "admin unknown caller", pk: "missing", target: func() http.Handler { return srv.adminHandler("missing") }, method: http.MethodGet, path: "/gears", want: http.StatusNotFound},
		{name: "admin gears wrong method", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPost, path: "/gears", want: http.StatusMethodNotAllowed},
		{name: "admin gear invalid approve body", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPost, path: "/gears/" + devicePK + ":approve", body: []byte("{"), want: http.StatusBadRequest},
		{name: "admin gear invalid config json", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/gears/" + devicePK + "/config", body: []byte("{"), want: http.StatusBadRequest},
		{name: "admin gear invalid config", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/gears/" + devicePK + "/config", body: []byte(`{"firmware":{"channel":"bad"}}`), want: http.StatusBadRequest},
		{name: "admin gear config missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/gears/missing/config", body: []byte(`{"firmware":{"channel":"stable"}}`), want: http.StatusNotFound},
		{name: "admin gear missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodGet, path: "/gears/missing", want: http.StatusNotFound},
		{name: "admin gear delete missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodDelete, path: "/gears/missing", want: http.StatusNotFound},
		{name: "admin gear refresh missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPost, path: "/gears/missing:refresh", want: http.StatusNotFound},
		{name: "admin gear invalid route", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodGet, path: "/gears/imei/123", want: http.StatusNotFound},
		{name: "admin gear refresh offline", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPost, path: "/gears/" + devicePK + ":refresh", want: http.StatusConflict},
		{name: "admin firmware list wrong method", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPost, path: "/firmwares", want: http.StatusMethodNotAllowed},
		{name: "admin depot invalid json", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/firmwares/demo", body: []byte("{"), want: http.StatusBadRequest},
		{name: "admin depot info mismatch", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/firmwares/demo", body: []byte(`{"files":[{"path":"other.bin"}]}`), want: http.StatusConflict},
		{name: "admin depot missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodGet, path: "/firmwares/missing", want: http.StatusNotFound},
		{name: "admin channel missing", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodGet, path: "/firmwares/demo/unknown", want: http.StatusNotFound},
		{name: "admin channel invalid upload", pk: adminPK, target: func() http.Handler { return srv.adminHandler(adminPK) }, method: http.MethodPut, path: "/firmwares/demo/stable", body: []byte("bad"), want: http.StatusConflict},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(tt.body))
		rr := httptest.NewRecorder()
		tt.target().ServeHTTP(rr, req)
		if rr.Code != tt.want {
			t.Fatalf("%s: status = %d body=%s", tt.name, rr.Code, rr.Body.String())
		}
	}
}

func TestOTAMissingPolicy(t *testing.T) {
	srv, _, _ := newHTTPTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader([]byte(`{"device":{"name":"no-policy"}}`)))
	rr := httptest.NewRecorder()
	srv.publicHandler("no-policy-pk").ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/ota", nil)
	rr = httptest.NewRecorder()
	srv.publicHandler("no-policy-pk").ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("ota status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestServerPublicKeyAndListenAddr(t *testing.T) {
	srv, err := New(Config{
		DataDir:    t.TempDir(),
		ListenAddr: "127.0.0.1:0",
		ConfigPath: writeTempConfig(t, minimalTestConfig),
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	t.Cleanup(func() { srv.stores.Close() })
	if srv.PublicKey().String() == "" {
		t.Fatal("PublicKey should not be empty")
	}
	if srv.ListenAddr() != "" {
		t.Fatalf("ListenAddr before Run = %q", srv.ListenAddr())
	}
}

func TestAdminFirmwareSwitchErrors(t *testing.T) {
	srv, adminPK, _ := newHTTPTestServer(t)
	handler := srv.adminHandler(adminPK)

	t.Run("release depot missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/firmwares/missing:release", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("release not ready", func(t *testing.T) {
		if err := srv.firmwareUploader.PutInfo("conflict", firmware.DepotInfo{
			Files: []firmware.DepotInfoFile{{Path: "firmware.bin"}},
		}); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/firmwares/conflict:release", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("release internal error", func(t *testing.T) {
		if err := os.WriteFile(srv.firmwareStore.ManifestPath("demo", firmware.ChannelBeta), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/firmwares/demo:release", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rollback not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/firmwares/missing:rollback", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rollback conflict", func(t *testing.T) {
		if err := srv.firmwareUploader.PutInfo("no-rollback", firmware.DepotInfo{
			Files: []firmware.DepotInfoFile{{Path: "firmware.bin"}},
		}); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/firmwares/no-rollback:rollback", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("rollback internal error", func(t *testing.T) {
		if err := os.WriteFile(srv.firmwareStore.ManifestPath("demo", firmware.ChannelRollback), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPut, "/firmwares/demo:rollback", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})
}
