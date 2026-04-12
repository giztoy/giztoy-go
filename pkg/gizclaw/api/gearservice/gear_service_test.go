package gearservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/kv"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func TestGearClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gears":
			_ = json.NewEncoder(w).Encode(RegistrationList{
				Items: []Registration{{PublicKey: "gear-pk"}},
			})
		case "/gears/gear-pk/runtime":
			_ = json.NewEncoder(w).Encode(Runtime{
				Online: true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}

	client, err := NewGearClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewGearClient error: %v", err)
	}

	items, err := client.ListGears(context.Background())
	if err != nil {
		t.Fatalf("ListGears error: %v", err)
	}
	if len(items.Items) != 1 || items.Items[0].PublicKey != "gear-pk" {
		t.Fatalf("ListGears = %+v", items)
	}

	runtime, err := client.GetGearRuntime(context.Background(), "gear-pk")
	if err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	if !runtime.Online {
		t.Fatalf("GetGearRuntime = %+v", runtime)
	}
}

func TestGearClientErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gears/missing":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error: ErrorPayload{
					Code:    "GEAR_NOT_FOUND",
					Message: "missing",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}

	client, err := NewGearClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewGearClient error: %v", err)
	}

	_, err = client.GetGear(context.Background(), "missing")
	if err == nil {
		t.Fatal("GetGear should fail")
	}
	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("GetGear error type = %T", err)
	}
	if clientErr.Path != "/gears/{publicKey}" || clientErr.StatusCode != http.StatusNotFound {
		t.Fatalf("ClientError = %+v", clientErr)
	}
	if clientErr.Payload == nil || clientErr.Payload.Error.Code != "GEAR_NOT_FOUND" {
		t.Fatalf("ClientError payload = %+v", clientErr.Payload)
	}
}

func TestGearServerGetGearRuntime(t *testing.T) {
	server := &GearServer{
		Gears:   newTestGearService(t),
		Manager: stubPeerManager{runtime: gears.Runtime{Online: true, LastAddr: "127.0.0.1:9000"}},
	}

	resp, err := server.GetGearRuntime(context.Background(), GetGearRuntimeRequestObject{PublicKey: "gear-pk"})
	if err != nil {
		t.Fatalf("GetGearRuntime error: %v", err)
	}
	okResp, ok := resp.(GetGearRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetGearRuntime response type = %T", resp)
	}
	if !okResp.Online || okResp.LastAddr == nil || *okResp.LastAddr != "127.0.0.1:9000" {
		t.Fatalf("GetGearRuntime response = %+v", okResp)
	}
}

func TestGearServerRefreshGearMapsOffline(t *testing.T) {
	server := &GearServer{
		Gears:   newTestGearService(t),
		Manager: stubPeerManager{online: false, err: errors.New("offline")},
	}

	resp, err := server.RefreshGear(context.Background(), RefreshGearRequestObject{PublicKey: "gear-pk"})
	if err != nil {
		t.Fatalf("RefreshGear error: %v", err)
	}
	errResp, ok := resp.(RefreshGear409JSONResponse)
	if !ok {
		t.Fatalf("RefreshGear response type = %T", resp)
	}
	if errResp.Error.Code != "DEVICE_OFFLINE" {
		t.Fatalf("RefreshGear error code = %q", errResp.Error.Code)
	}
}

func TestRefreshGear500JSONResponse(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(ctx *fiber.Ctx) error {
		return refreshGear500JSONResponse(internalGearError("boom")).VisitRefreshGearResponse(ctx)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	adaptor.FiberApp(app).ServeHTTP(rec, req)

	var payload ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if payload.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("payload = %+v", payload)
	}
}

func newTestGearService(t *testing.T) *gears.Service {
	t.Helper()

	service := gears.NewService(gears.NewStore(kv.NewMemory(nil)), nil)
	_, err := service.Register(context.Background(), gears.RegistrationRequest{
		PublicKey: "gear-pk",
		Device: gears.DeviceInfo{
			SN: "sn-1",
			Hardware: gears.HardwareInfo{
				Depot: "demo",
			},
		},
	})
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	return service
}

type stubPeerManager struct {
	runtime gears.Runtime
	result  gears.RefreshResult
	online  bool
	err     error
}

func (s stubPeerManager) PeerRuntime(context.Context, string) gears.Runtime {
	return s.runtime
}

func (s stubPeerManager) RefreshDevice(context.Context, string) (gears.RefreshResult, bool, error) {
	return s.result, s.online, s.err
}

type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	rewritten, err := http.NewRequest(req.Method, t.target+req.URL.Path, nil)
	if err != nil {
		return nil, err
	}
	clone.URL = rewritten.URL
	return t.base.RoundTrip(clone)
}
