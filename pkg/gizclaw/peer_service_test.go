package gizclaw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/adminservice"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/serverpublic"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/resourcemanager"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/giznet/gizhttp"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

const (
	testReadyTimeout = 10 * time.Second
	testPollInterval = 20 * time.Millisecond
)

func waitUntil(timeout time.Duration, check func() error) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := check(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(testPollInterval)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("condition not satisfied before timeout")
}

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

func TestPeerServiceValidateServices(t *testing.T) {
	tests := []struct {
		name    string
		service *PeerService
		wantErr string
	}{
		{
			name:    "missing admin service",
			service: &PeerService{},
			wantErr: "nil admin service",
		},
		{
			name: "missing gear service",
			service: &PeerService{
				admin: &adminService{},
			},
			wantErr: "nil gear service",
		},
		{
			name: "missing public service",
			service: &PeerService{
				admin: &adminService{},
				gear:  &gearAPIBundle{},
			},
			wantErr: "nil public service",
		},
		{
			name: "complete service bundle",
			service: &PeerService{
				admin:  &adminService{},
				gear:   &gearAPIBundle{},
				public: &serverPublic{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.service.validateServices()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateServices() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateServices() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAdminServiceApplyResourceRequiresBody(t *testing.T) {
	t.Parallel()

	resp, err := (&adminService{}).ApplyResource(context.Background(), adminservice.ApplyResourceRequestObject{})
	if err != nil {
		t.Fatalf("ApplyResource() error = %v", err)
	}
	got, ok := resp.(adminservice.ApplyResource400JSONResponse)
	if !ok {
		t.Fatalf("ApplyResource() response = %T", resp)
	}
	if got.Error.Code != "INVALID_RESOURCE" {
		t.Fatalf("ApplyResource() code = %q", got.Error.Code)
	}
}

func TestAdminServiceResourceMethodsHandleValidationAndManagerErrors(t *testing.T) {
	resource := mustPeerServiceResource(t, `{
		"apiVersion": "gizclaw.admin/v1alpha1",
		"kind": "Credential",
		"metadata": {"name": "minimax-main"},
		"spec": {
			"provider": "minimax",
			"method": "api_key",
			"body": {"api_key": "secret"}
		}
	}`)
	service := &adminService{}

	applyResp, err := service.ApplyResource(context.Background(), adminservice.ApplyResourceRequestObject{JSONBody: &resource})
	if err != nil {
		t.Fatalf("ApplyResource() error = %v", err)
	}
	if got, ok := applyResp.(adminservice.ApplyResource500JSONResponse); !ok || got.Error.Code != "RESOURCE_MANAGER_NOT_CONFIGURED" {
		t.Fatalf("ApplyResource() response = %T %+v", applyResp, applyResp)
	}

	getResp, err := service.GetResource(context.Background(), adminservice.GetResourceRequestObject{
		Kind: apitypes.ResourceKindCredential,
		Name: "minimax-main",
	})
	if err != nil {
		t.Fatalf("GetResource() error = %v", err)
	}
	if got, ok := getResp.(adminservice.GetResource500JSONResponse); !ok || got.Error.Code != "RESOURCE_MANAGER_NOT_CONFIGURED" {
		t.Fatalf("GetResource() response = %T %+v", getResp, getResp)
	}

	putResp, err := service.PutResource(context.Background(), adminservice.PutResourceRequestObject{})
	if err != nil {
		t.Fatalf("PutResource(nil body) error = %v", err)
	}
	if got, ok := putResp.(adminservice.PutResource400JSONResponse); !ok || got.Error.Code != "INVALID_RESOURCE" {
		t.Fatalf("PutResource(nil body) response = %T %+v", putResp, putResp)
	}

	putResp, err = service.PutResource(context.Background(), adminservice.PutResourceRequestObject{
		Kind:     apitypes.ResourceKindWorkspace,
		Name:     "minimax-main",
		JSONBody: &resource,
	})
	if err != nil {
		t.Fatalf("PutResource(path mismatch) error = %v", err)
	}
	if got, ok := putResp.(adminservice.PutResource400JSONResponse); !ok || got.Error.Code != "INVALID_RESOURCE_PATH" {
		t.Fatalf("PutResource(path mismatch) response = %T %+v", putResp, putResp)
	}

	putResp, err = service.PutResource(context.Background(), adminservice.PutResourceRequestObject{
		Kind:     apitypes.ResourceKindCredential,
		Name:     "minimax-main",
		JSONBody: &resource,
	})
	if err != nil {
		t.Fatalf("PutResource(manager error) error = %v", err)
	}
	if got, ok := putResp.(adminservice.PutResource500JSONResponse); !ok || got.Error.Code != "RESOURCE_MANAGER_NOT_CONFIGURED" {
		t.Fatalf("PutResource(manager error) response = %T %+v", putResp, putResp)
	}

	deleteResp, err := service.DeleteResource(context.Background(), adminservice.DeleteResourceRequestObject{
		Kind: apitypes.ResourceKindCredential,
		Name: "minimax-main",
	})
	if err != nil {
		t.Fatalf("DeleteResource() error = %v", err)
	}
	if got, ok := deleteResp.(adminservice.DeleteResource500JSONResponse); !ok || got.Error.Code != "RESOURCE_MANAGER_NOT_CONFIGURED" {
		t.Fatalf("DeleteResource() response = %T %+v", deleteResp, deleteResp)
	}
}

func TestAdminResourceHelpers(t *testing.T) {
	resource := mustPeerServiceResource(t, `{
		"apiVersion": "gizclaw.admin/v1alpha1",
		"kind": "Credential",
		"metadata": {"name": "minimax-main"},
		"spec": {
			"provider": "minimax",
			"method": "api_key",
			"body": {"api_key": "secret"}
		}
	}`)

	if err := validateResourcePathMatch(resource, apitypes.ResourceKindCredential, "minimax-main"); err != nil {
		t.Fatalf("validateResourcePathMatch() error = %v", err)
	}
	if err := validateResourcePathMatch(resource, apitypes.ResourceKindWorkspace, "minimax-main"); err == nil || !strings.Contains(err.Error(), "kind") {
		t.Fatalf("validateResourcePathMatch(kind mismatch) error = %v", err)
	}
	if err := validateResourcePathMatch(resource, apitypes.ResourceKindCredential, "other"); err == nil || !strings.Contains(err.Error(), "metadata.name") {
		t.Fatalf("validateResourcePathMatch(name mismatch) error = %v", err)
	}

	status, body := resourceManagerError(&resourcemanager.Error{StatusCode: http.StatusNotFound, Code: "RESOURCE_NOT_FOUND", Message: "missing"})
	if status != http.StatusNotFound || body.Error.Code != "RESOURCE_NOT_FOUND" {
		t.Fatalf("resourceManagerError(resource error) = %d %+v", status, body)
	}
	status, body = resourceManagerError(errors.New("boom"))
	if status != http.StatusInternalServerError || body.Error.Code != "RESOURCE_MANAGER_ERROR" {
		t.Fatalf("resourceManagerError(generic error) = %d %+v", status, body)
	}
}

func TestResource200JSONResponseSerializesResourceUnion(t *testing.T) {
	resource := mustPeerServiceResource(t, `{
		"apiVersion": "gizclaw.admin/v1alpha1",
		"kind": "Credential",
		"metadata": {"name": "minimax-main"},
		"spec": {
			"provider": "minimax",
			"method": "api_key",
			"body": {"api_key": "secret"}
		}
	}`)
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/resource", func(ctx *fiber.Ctx) error {
		return resource200JSONResponse{Resource: resource}.VisitGetResourceResponse(ctx)
	})

	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	rec := httptest.NewRecorder()
	fiberHTTPHandler(app).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"kind":"Credential"`) {
		t.Fatalf("body = %s", rec.Body.String())
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
	if err := waitUntil(testReadyTimeout, func() error {
		if _, ok := server.manager.ActivePeer(clientKey.Public.String()); !ok {
			return fmt.Errorf("peer not marked online yet")
		}
		gear, loadErr := server.manager.Gears.LoadGear(context.Background(), clientKey.Public.String())
		if loadErr != nil {
			return fmt.Errorf("auto-created gear not ready: %w", loadErr)
		}
		if gear.Role != apitypes.GearRoleUnspecified || gear.Status != apitypes.GearStatusActive {
			return fmt.Errorf("auto-created gear = %+v", gear)
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

func mustPeerServiceResource(t *testing.T, raw string) apitypes.Resource {
	t.Helper()

	var resource apitypes.Resource
	if err := json.Unmarshal([]byte(raw), &resource); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return resource
}
