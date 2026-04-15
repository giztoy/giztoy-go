package gizclaw

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/firmware"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func TestClientDialAndServeValidation(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		var client *Client
		if err := client.DialAndServe(giznet.PublicKey{}, "127.0.0.1:1"); err == nil || !strings.Contains(err.Error(), "nil client") {
			t.Fatalf("DialAndServe(nil) err = %v", err)
		}
	})

	t.Run("nil key pair", func(t *testing.T) {
		client := &Client{}
		if err := client.DialAndServe(giznet.PublicKey{}, "127.0.0.1:1"); err == nil || !strings.Contains(err.Error(), "nil key pair") {
			t.Fatalf("DialAndServe(nil key pair) err = %v", err)
		}
	})

	t.Run("empty server addr", func(t *testing.T) {
		keyPair, err := giznet.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair error = %v", err)
		}
		client := &Client{KeyPair: keyPair}
		if err := client.DialAndServe(giznet.PublicKey{}, ""); err == nil || !strings.Contains(err.Error(), "empty server addr") {
			t.Fatalf("DialAndServe(empty addr) err = %v", err)
		}
	})

	t.Run("already started", func(t *testing.T) {
		keyPair, err := giznet.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair error = %v", err)
		}
		client := &Client{KeyPair: keyPair, listener: &giznet.Listener{}}
		if err := client.DialAndServe(giznet.PublicKey{}, "127.0.0.1:1"); err == nil || !strings.Contains(err.Error(), "already started") {
			t.Fatalf("DialAndServe(already started) err = %v", err)
		}
	})
}

func TestClientListenAndProxyValidation(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		var client *Client
		if err := client.ListenAndProxy("127.0.0.1:0"); err == nil || !strings.Contains(err.Error(), "nil client") {
			t.Fatalf("ListenAndProxy(nil) err = %v", err)
		}
	})

	t.Run("empty proxy addr", func(t *testing.T) {
		client := &Client{}
		if err := client.ListenAndProxy(""); err == nil || !strings.Contains(err.Error(), "empty proxy addr") {
			t.Fatalf("ListenAndProxy(empty addr) err = %v", err)
		}
	})

	t.Run("disconnected client", func(t *testing.T) {
		client := &Client{}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("net.Listen error = %v", err)
		}
		defer listener.Close()
		if err := client.serveProxyListener(listener); err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("serveProxyListener(disconnected) err = %v", err)
		}
	})
}

func TestClientServePeerPublicValidation(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		var client *Client
		if err := client.servePeerPublic(); err == nil || !strings.Contains(err.Error(), "nil client") {
			t.Fatalf("servePeerPublic(nil) err = %v", err)
		}
	})

	t.Run("disconnected client", func(t *testing.T) {
		client := &Client{}
		if err := client.servePeerPublic(); err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("servePeerPublic(disconnected) err = %v", err)
		}
	})
}

func TestClientProxyMuxRoutesRemoteServices(t *testing.T) {
	client, serverConn, cleanup := newProxyTestPair(t)
	defer cleanup()

	gearServer := &gear.Server{
		Store:           kv.NewMemory(nil),
		BuildCommit:     "test-build",
		ServerPublicKey: "server-pk",
	}
	firmwareServer := &firmware.Server{Store: depotstore.Dir(t.TempDir())}
	service := &Service{
		admin: &adminService{
			FirmwareAdminService: firmwareServer,
			GearsAdminService:    gearServer,
		},
		gear: &gearService{
			FirmwareGearService: firmwareServer,
			GearsGearService:    gearServer,
		},
		public: &serverPublic{
			FirmwareServerPublic: firmwareServer,
			GearsServerPublic:    gearServer,
		},
	}

	go func() { _ = service.serveAdmin(serverConn) }()
	go func() { _ = service.serveGear(serverConn) }()
	go func() { _ = service.servePublic(serverConn) }()

	proxy := httptest.NewServer(client.proxyMux())
	defer proxy.Close()

	resp, body := mustProxyGET(t, proxy.URL+"/admin/firmwares")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/firmwares status = %d body=%s", resp.StatusCode, string(body))
	}

	resp, body = mustProxyGET(t, proxy.URL+"/public/server-info")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /public/server-info status = %d body=%s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), "server-pk") {
		t.Fatalf("GET /public/server-info body = %s", string(body))
	}

	resp, body = mustProxyGET(t, proxy.URL+"/gear/gears")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /gear/gears status = %d body=%s", resp.StatusCode, string(body))
	}

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirect.Get(proxy.URL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin error = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("GET /admin status = %d", resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/admin/" {
		t.Fatalf("GET /admin location = %q", location)
	}
}

func TestClientAccessorsAndConversions(t *testing.T) {
	keyPair, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error = %v", err)
	}

	client := &Client{KeyPair: keyPair, serverPK: keyPair.Public}

	if got := client.ServerPublicKey(); got != keyPair.Public {
		t.Fatalf("ServerPublicKey() = %v, want %v", got, keyPair.Public)
	}

	peerClient, err := client.PeerPublicClient()
	if err != nil {
		t.Fatalf("PeerPublicClient() error = %v", err)
	}
	if peerClient == nil {
		t.Fatal("PeerPublicClient() returned nil client")
	}

	if _, err := client.RPCClient(); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("RPCClient() err = %v", err)
	}

	name := "main"
	device := gearservice.DeviceInfo{
		Sn: func() *string {
			v := "sn-1"
			return &v
		}(),
		Hardware: &gearservice.HardwareInfo{
			Manufacturer:   func() *string { v := "Acme"; return &v }(),
			Model:          func() *string { v := "M1"; return &v }(),
			Depot:          func() *string { v := "demo-main"; return &v }(),
			FirmwareSemver: func() *string { v := "1.2.3"; return &v }(),
			Imeis: &[]gearservice.GearIMEI{{
				Name:   &name,
				Tac:    "12345678",
				Serial: "0000001",
			}},
			Labels: &[]gearservice.GearLabel{{
				Key:   "batch",
				Value: "cn-east",
			}},
		},
	}

	info := gearDeviceToPeerRefreshInfo(device)
	if info.Manufacturer == nil || *info.Manufacturer != "Acme" {
		t.Fatalf("gearDeviceToPeerRefreshInfo() = %+v", info)
	}

	identifiers := gearDeviceToPeerRefreshIdentifiers(device)
	if identifiers.Sn == nil || *identifiers.Sn != "sn-1" {
		t.Fatalf("gearDeviceToPeerRefreshIdentifiers().Sn = %+v", identifiers.Sn)
	}
	if identifiers.Imeis == nil || len(*identifiers.Imeis) != 1 || (*identifiers.Imeis)[0].Tac != "12345678" {
		t.Fatalf("gearDeviceToPeerRefreshIdentifiers().Imeis = %+v", identifiers.Imeis)
	}
	if identifiers.Labels == nil || len(*identifiers.Labels) != 1 || (*identifiers.Labels)[0].Value != "cn-east" {
		t.Fatalf("gearDeviceToPeerRefreshIdentifiers().Labels = %+v", identifiers.Labels)
	}

	version := gearDeviceToPeerRefreshVersion(device)
	if version.Depot == nil || *version.Depot != "demo-main" || version.FirmwareSemver == nil || *version.FirmwareSemver != "1.2.3" {
		t.Fatalf("gearDeviceToPeerRefreshVersion() = %+v", version)
	}

	imei := gearToPeerGearIMEI(gearservice.GearIMEI{Name: &name, Tac: "87654321", Serial: "0000009"})
	if imei.Name == nil || *imei.Name != "main" || imei.Tac != "87654321" || imei.Serial != "0000009" {
		t.Fatalf("gearToPeerGearIMEI() = %+v", imei)
	}

	label := gearToPeerGearLabel(gearservice.GearLabel{Key: "batch", Value: "cn-west"})
	if label.Key != "batch" || label.Value != "cn-west" {
		t.Fatalf("gearToPeerGearLabel() = %+v", label)
	}
}

func newProxyTestPair(t *testing.T) (*Client, *giznet.Conn, func()) {
	t.Helper()

	serverKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(server) error = %v", err)
	}
	clientKey, err := giznet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair(client) error = %v", err)
	}

	serverListener, err := giznet.Listen(serverKey,
		giznet.WithBindAddr("127.0.0.1:0"),
		giznet.WithAllowUnknown(true),
		giznet.WithServiceMuxConfig(giznet.ServiceMuxConfig{
			OnNewService: func(_ giznet.PublicKey, service uint64) bool {
				switch service {
				case ServiceAdmin, ServiceGear, ServiceServerPublic:
					return true
				default:
					return false
				}
			},
		}),
	)
	if err != nil {
		t.Fatalf("giznet.Listen(server) error = %v", err)
	}
	go drainUDP(serverListener.UDP())

	clientListener, err := giznet.Listen(clientKey, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		_ = serverListener.Close()
		t.Fatalf("giznet.Listen(client) error = %v", err)
	}
	go drainUDP(clientListener.UDP())

	connCh := make(chan *giznet.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, acceptErr := serverListener.Accept()
		if acceptErr != nil {
			errCh <- acceptErr
			return
		}
		connCh <- conn
	}()

	clientConn, err := clientListener.Dial(serverKey.Public, serverListener.HostInfo().Addr)
	if err != nil {
		_ = clientListener.Close()
		_ = serverListener.Close()
		t.Fatalf("Dial error = %v", err)
	}

	var serverConn *giznet.Conn
	select {
	case serverConn = <-connCh:
	case acceptErr := <-errCh:
		_ = clientConn.Close()
		_ = clientListener.Close()
		_ = serverListener.Close()
		t.Fatalf("Accept error = %v", acceptErr)
	}

	client := &Client{conn: clientConn}
	cleanup := func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
		_ = clientListener.Close()
		_ = serverListener.Close()
	}
	return client, serverConn, cleanup
}

func mustProxyGET(t *testing.T, url string) (*http.Response, []byte) {
	t.Helper()

	var lastErr error
	var lastStatus int
	var lastBody []byte
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			time.Sleep(20 * time.Millisecond)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			resp.Body = io.NopCloser(strings.NewReader(string(body)))
			return resp, body
		}
		lastStatus = resp.StatusCode
		lastBody = body
		time.Sleep(20 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("GET %s error = %v", url, lastErr)
	}
	t.Fatalf("GET %s status = %d body=%s", url, lastStatus, string(lastBody))
	return nil, nil
}
