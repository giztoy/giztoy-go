package gizclaw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/integration/testutil"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/giznet/gizhttp"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func TestPublicFiberAdapterServerInfo(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(ctx *fiber.Ctx) error {
		base := ctx.UserContext()
		if base == nil {
			base = context.Background()
		}
		ctx.SetUserContext(serverpublic.WithCallerPublicKey(base, "gear-pk"))
		return ctx.Next()
	})
	serverpublic.RegisterHandlers(app, serverpublic.NewStrictHandler(&serverPublic{
		GearsServerPublic: &gear.Server{
			BuildCommit:     "test-build",
			ServerPublicKey: "server-pk",
		},
	}, nil))

	req := httptest.NewRequest(http.MethodGet, "/server-info", nil)
	rec := httptest.NewRecorder()
	adaptor.FiberApp(app).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPeerServicePublicRoundTrip(t *testing.T) {
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
				return service == ServiceServerPublic
			},
		}),
	)
	if err != nil {
		t.Fatalf("giznet.Listen(server) error = %v", err)
	}
	defer serverListener.Close()
	go drainUDP(serverListener.UDP())

	clientListener, err := giznet.Listen(clientKey, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("giznet.Listen(client) error = %v", err)
	}
	defer clientListener.Close()
	go drainUDP(clientListener.UDP())

	connCh := make(chan *giznet.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := serverListener.Accept()
		if err != nil {
			errCh <- err
			return
		}
		connCh <- conn
	}()

	conn, err := clientListener.Dial(serverKey.Public, serverListener.HostInfo().Addr)
	if err != nil {
		t.Fatalf("Dial error = %v", err)
	}
	defer conn.Close()

	var serverConn *giznet.Conn
	select {
	case serverConn = <-connCh:
	case err := <-errCh:
		t.Fatalf("Accept error = %v", err)
	}
	defer serverConn.Close()

	gearsServer := &gear.Server{
		BuildCommit:     "test-build",
		ServerPublicKey: serverKey.Public.String(),
	}
	service := &PeerService{
		peerManager: NewManager(gearsServer),
		public: &serverPublic{
			GearsServerPublic: gearsServer,
		},
	}
	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- service.servePublic(serverConn)
	}()

	client := &http.Client{Transport: gizhttp.NewRoundTripper(conn, ServiceServerPublic)}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://gizclaw/server-info", nil)
	if err != nil {
		t.Fatalf("http.NewRequest error = %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		select {
		case serveErr := <-serveErrCh:
			t.Fatalf("client.Do error = %v; servePublic error = %v", err, serveErr)
		default:
		}
		t.Fatalf("client.Do error = %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.StatusCode, string(body))
	}
}

func TestPeerServiceServeConnRequiresHandlers(t *testing.T) {
	service := &PeerService{}

	err := service.ServeConn(&giznet.Conn{})
	if err == nil {
		t.Fatal("ServeConn should fail when handlers are missing")
	}
	if err.Error() != "gizclaw: nil admin service" {
		t.Fatalf("ServeConn error = %v", err)
	}
}

func TestIntegrationPeerServiceServeConnClientCloseUnblocksAndMarksPeerOffline(t *testing.T) {
	const closeTimeout = 2 * time.Second

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
				case ServiceAdmin, ServiceGear, ServiceServerPublic, ServiceRPC:
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
	defer serverListener.Close()
	go drainUDP(serverListener.UDP())

	clientListener, err := giznet.Listen(clientKey, giznet.WithBindAddr("127.0.0.1:0"), giznet.WithAllowUnknown(true))
	if err != nil {
		t.Fatalf("giznet.Listen(client) error = %v", err)
	}
	defer clientListener.Close()
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
		t.Fatalf("Dial error = %v", err)
	}
	defer clientConn.Close()

	var serverConn *giznet.Conn
	select {
	case serverConn = <-connCh:
	case acceptErr := <-errCh:
		t.Fatalf("Accept error = %v", acceptErr)
	case <-time.After(5 * time.Second):
		t.Fatal("Accept timeout")
	}
	defer serverConn.Close()

	server := &Server{
		KeyPair:         serverKey,
		GearStore:       mustBadgerInMemory(t, nil),
		DepotStore:      depotstore.Dir(t.TempDir()),
		BuildCommit:     "test-build",
		ServerPublicKey: serverKey.Public.String(),
	}
	if err := server.initRuntime(serverKey.Public.String()); err != nil {
		t.Fatalf("initRuntime error = %v", err)
	}

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.peerService.ServeConn(serverConn)
	}()

	client := &http.Client{
		Transport: gizhttp.NewRoundTripper(clientConn, ServiceServerPublic),
		Timeout:   time.Second,
	}
	if err := testutil.WaitUntil(testutil.ReadyTimeout, func() error {
		if _, ok := server.manager.ActivePeer(clientKey.Public.String()); !ok {
			return fmt.Errorf("peer not marked online yet")
		}

		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://gizclaw/server-info", nil)
		if reqErr != nil {
			return reqErr
		}
		resp, doErr := client.Do(req)
		if doErr != nil {
			select {
			case serveErr := <-serveErrCh:
				return fmt.Errorf("ServeConn exited before ready: %w", serveErr)
			default:
			}
			return doErr
		}
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server-info status = %d body=%s", resp.StatusCode, string(body))
		}
		return nil
	}); err != nil {
		t.Fatalf("ServeConn did not become ready: %v", err)
	}

	start := time.Now()
	if err := clientConn.Close(); err != nil {
		t.Fatalf("clientConn.Close error = %v", err)
	}
	if err := clientListener.Close(); err != nil {
		t.Fatalf("clientListener.Close error = %v", err)
	}

	select {
	case serveErr := <-serveErrCh:
		if serveErr != nil {
			t.Fatalf("ServeConn error after client close = %v", serveErr)
		}
	case <-time.After(closeTimeout):
		t.Fatalf("ServeConn did not exit within %v after client close", closeTimeout)
	}

	if took := time.Since(start); took > closeTimeout {
		t.Fatalf("ServeConn close path took %v, want <= %v", took, closeTimeout)
	}

	if _, ok := server.manager.ActivePeer(clientKey.Public.String()); ok {
		t.Fatal("peer should be removed after client close")
	}
	if runtime := server.manager.PeerRuntime(context.Background(), clientKey.Public.String()); runtime.Online || !runtime.LastSeenAt.IsZero() {
		t.Fatalf("peer runtime after client close = %+v", runtime)
	}
}

func TestFiberHTTPHandlerHidesPanicDetail(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/panic", func(*fiber.Ctx) error {
		panic("secret-panic-detail")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	fiberHTTPHandler(app).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func drainUDP(u *giznet.UDP) {
	buf := make([]byte, 65535)
	for {
		if _, _, err := u.ReadFrom(buf); err != nil {
			return
		}
	}
}
