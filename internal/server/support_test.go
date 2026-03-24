package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/haivivi/giztoy/go/pkg/gears"
)

const minimalTestConfig = `
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
`

func writeTempConfig(t *testing.T, yml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigHelpersAndLoad(t *testing.T) {
	cfgPath := writeTempConfig(t, `
listen: 127.0.0.1:9999
stores:
  mem:
    kind: keyvalue
    backend: memory
  depots:
    kind: filestore
    backend: filesystem
    dir: depots
gears:
  store: mem
depots:
  store: depots
`)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9999" {
		t.Fatalf("listen = %q", cfg.ListenAddr)
	}
	if cfg.Gears.Store != "mem" || cfg.Depots.Store != "depots" {
		t.Fatalf("store refs = %+v", cfg)
	}

	runtimeCfg := Config{
		DataDir: t.TempDir(),
		Gears:   cfg.Gears,
		Depots:  cfg.Depots,
	}
	if got := runtimeCfg.effectiveListenAddr(); got != ":9820" {
		t.Fatalf("effectiveListenAddr = %q", got)
	}
	if err := runtimeCfg.validate(); err != nil {
		t.Fatalf("validate error: %v", err)
	}

	// New() with config path exercises full Registry store creation.
	srvFromConfig, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: cfgPath,
	})
	if err != nil {
		t.Fatalf("New with config path error: %v", err)
	}
	srvFromConfig.stores.Close()
	if srvFromConfig.cfg.ListenAddr != "127.0.0.1:9999" {
		t.Fatalf("listen from config = %q", srvFromConfig.cfg.ListenAddr)
	}
	srvWithOverride, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: cfgPath,
		ListenAddr: "127.0.0.1:7777",
	})
	if err != nil {
		t.Fatalf("New with listen override error: %v", err)
	}
	srvWithOverride.stores.Close()
	if srvWithOverride.cfg.ListenAddr != "127.0.0.1:7777" {
		t.Fatalf("listen override = %q", srvWithOverride.cfg.ListenAddr)
	}
	srvWithRuntimeGears, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: cfgPath,
		Gears: GearsConfig{
			RegistrationTokens: map[string]gears.RegistrationToken{
				"runtime": {Role: gears.GearRoleAdmin},
			},
		},
	})
	if err != nil {
		t.Fatalf("New with runtime gears override error: %v", err)
	}
	srvWithRuntimeGears.stores.Close()
	if srvWithRuntimeGears.cfg.Gears.Store != "mem" {
		t.Fatalf("gears store merge = %q", srvWithRuntimeGears.cfg.Gears.Store)
	}
	if _, ok := srvWithRuntimeGears.cfg.Gears.RegistrationTokens["runtime"]; !ok {
		t.Fatalf("runtime registration tokens lost: %+v", srvWithRuntimeGears.cfg.Gears.RegistrationTokens)
	}
	srvWithRuntimeDepots, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: cfgPath,
		Depots:     DepotsConfig{Store: "depots"},
	})
	if err != nil {
		t.Fatalf("New with runtime depots override error: %v", err)
	}
	srvWithRuntimeDepots.stores.Close()
	if srvWithRuntimeDepots.cfg.Depots.Store != "depots" {
		t.Fatalf("depots store merge = %q", srvWithRuntimeDepots.cfg.Depots.Store)
	}

	// --- validate ---
	if err := (Config{}).validate(); err == nil {
		t.Fatal("validate should fail for empty DataDir")
	}
	if err := (Config{DataDir: t.TempDir()}).validate(); err == nil {
		t.Fatal("validate should fail for empty gears.store")
	}
	if err := (Config{DataDir: t.TempDir(), Gears: GearsConfig{Store: "x"}}).validate(); err == nil {
		t.Fatal("validate should fail for empty depots.store")
	}

	// --- New(): store-level errors via Registry ---
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  bad:
    kind: keyvalue
    backend: memory
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: bad
depots:
  store: bad
`),
	}); err == nil {
		t.Fatal("New should fail when depots references non-filestore kind")
	}
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  bg-nodir:
    kind: keyvalue
    backend: badger
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: bg-nodir
depots:
  store: fw
`),
	}); err == nil {
		t.Fatal("New should fail for badger store without dir")
	}
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  unsup:
    kind: keyvalue
    backend: redis
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: unsup
depots:
  store: fw
`),
	}); err == nil {
		t.Fatal("New should fail for unsupported backend")
	}
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  mem:
    kind: keyvalue
    backend: memory
  dep-nodir:
    kind: filestore
    backend: filesystem
gears:
  store: mem
depots:
  store: dep-nodir
`),
	}); err == nil {
		t.Fatal("New should fail for filestore without dir")
	}
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: missing
depots:
  store: fw
`),
	}); err == nil {
		t.Fatal("New should fail for missing gears store ref")
	}

	// --- New(): badger + filestore happy path ---
	srv, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  bg:
    kind: keyvalue
    backend: badger
    dir: gears-data
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
gears:
  store: bg
depots:
  store: fw
`),
	})
	if err != nil {
		t.Fatalf("New with badger+filesystem: %v", err)
	}
	srv.stores.Close()

	// --- New(): missing gears.store ---
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  fw:
    kind: filestore
    backend: filesystem
    dir: firmware
depots:
  store: fw
`),
	}); err == nil {
		t.Fatal("New should fail when gears.store is missing")
	}

	// --- New(): missing depots.store ---
	if _, err := New(Config{
		DataDir:    t.TempDir(),
		ConfigPath: writeTempConfig(t, `
stores:
  mem:
    kind: keyvalue
    backend: memory
gears:
  store: mem
`),
	}); err == nil {
		t.Fatal("New should fail when depots.store is missing")
	}

	// --- effectiveListenAddr ---
	if got := (Config{ListenAddr: ":8080"}).effectiveListenAddr(); got != ":8080" {
		t.Fatalf("effectiveListenAddr = %q, want :8080", got)
	}

	// --- LoadConfig missing file ---
	if _, err := LoadConfig("/nonexistent/path/config.yaml"); err == nil {
		t.Fatal("LoadConfig should fail for missing file")
	}
}

func TestSingleConnListenerAndServiceAddr(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer clientSide.Close()

	l := &singleConnListener{conn: serverSide}
	conn, err := l.Accept()
	if err != nil {
		t.Fatalf("Accept error: %v", err)
	}
	if conn == nil {
		t.Fatal("nil conn")
	}
	if _, err := l.Accept(); err == nil {
		t.Fatal("second Accept should fail")
	}
	if l.Addr().String() != "service0" {
		t.Fatalf("addr = %q", l.Addr().String())
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if serviceAddr("x").Network() != "x" || serviceAddr("x").String() != "x" {
		t.Fatal("serviceAddr methods mismatch")
	}
}

func TestReverseClientAndRefreshOffline(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			writeJSON(w, http.StatusOK, gears.RefreshInfo{Name: "demo"})
		case "/identifiers":
			writeJSON(w, http.StatusOK, gears.RefreshIdentifiers{SN: "sn-1"})
		case "/version":
			writeJSON(w, http.StatusOK, gears.RefreshVersion{Depot: "demo", FirmwareSemVer: "1.0.0"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}
	client := newReverseDeviceClient(httpClient)
	if _, err := client.GetInfo(context.Background(), ""); err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	if _, err := client.GetIdentifiers(context.Background(), ""); err != nil {
		t.Fatalf("GetIdentifiers error: %v", err)
	}
	if _, err := client.GetVersion(context.Background(), ""); err != nil {
		t.Fatalf("GetVersion error: %v", err)
	}

	srv, _, _ := newHTTPTestServer(t)
	if _, err := srv.refreshGearFromDevice(context.Background(), "missing"); err == nil {
		t.Fatal("refreshGearFromDevice for missing gear should fail")
	} else if err != gears.ErrGearNotFound {
		t.Fatalf("refreshGearFromDevice missing err = %v", err)
	}
	if _, err := srv.refreshGearFromDevice(context.Background(), "device-pk"); err == nil {
		t.Fatal("refreshGearFromDevice for offline peer should fail")
	} else if err != ErrDeviceOffline {
		t.Fatalf("refreshGearFromDevice offline err = %v", err)
	}
}

func TestReverseClientErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			http.Error(w, "boom", http.StatusBadGateway)
		case "/identifiers":
			_, _ = w.Write([]byte("{"))
		case "/version":
			writeJSON(w, http.StatusOK, gears.RefreshVersion{Depot: "demo", FirmwareSemVer: "1.0.0"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}
	client := newReverseDeviceClient(httpClient)
	if _, err := client.GetInfo(context.Background(), ""); err == nil {
		t.Fatal("GetInfo should fail on non-2xx")
	}
	if _, err := client.GetIdentifiers(context.Background(), ""); err == nil {
		t.Fatal("GetIdentifiers should fail on invalid json")
	}
	if _, err := client.GetVersion(context.Background(), ""); err != nil {
		t.Fatalf("GetVersion error: %v", err)
	}
}

type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	parsed, err := http.NewRequest(req.Method, t.target+req.URL.Path, nil)
	if err != nil {
		return nil, err
	}
	clone.URL = parsed.URL
	return t.base.RoundTrip(clone)
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusBadRequest, "INVALID_PARAMS", "bad")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
	var body ErrorBody
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Error.Code != "INVALID_PARAMS" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

func TestServeSingleHTTPConn(t *testing.T) {
	srv, _, _ := newHTTPTestServer(t)
	serverSide, clientSide := net.Pipe()
	done := make(chan struct{})
	go func() {
		srv.serveSingleHTTPConn("device-pk", serverSide, srv.publicHandler("device-pk"))
		close(done)
	}()
	req, _ := http.NewRequest(http.MethodGet, "http://giztoy/server-info", nil)
	if err := req.Write(clientSide); err != nil {
		t.Fatalf("request write error: %v", err)
	}
	if _, err := http.ReadResponse(bufio.NewReader(clientSide), req); err != nil {
		t.Fatalf("response read error: %v", err)
	}
	_ = clientSide.Close()
	<-done
}
