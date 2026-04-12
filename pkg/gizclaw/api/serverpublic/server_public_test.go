package serverpublic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/kv"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func TestPublicClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server-info":
			_ = json.NewEncoder(w).Encode(ServerInfo{
				BuildCommit: "abc123",
			})
		case "/download/firmware/fw.bin":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("X-Checksum-MD5", "md5sum")
			w.Header().Set("X-Checksum-SHA256", "sha256sum")
			_, _ = w.Write([]byte("firmware"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}

	client, err := NewPublicClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewPublicClient error: %v", err)
	}

	info, err := client.GetServerInfo(context.Background())
	if err != nil {
		t.Fatalf("GetServerInfo error: %v", err)
	}
	if info.BuildCommit != "abc123" {
		t.Fatalf("GetServerInfo = %+v", info)
	}

	file, err := client.DownloadFirmware(context.Background(), "fw.bin")
	if err != nil {
		t.Fatalf("DownloadFirmware error: %v", err)
	}
	if string(file.Body) != "firmware" || file.XChecksumMD5 != "md5sum" || file.XChecksumSHA256 != "sha256sum" {
		t.Fatalf("DownloadFirmware = %+v", file)
	}
}

func TestPublicClientErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config":
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

	client, err := NewPublicClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewPublicClient error: %v", err)
	}

	_, err = client.GetConfig(context.Background())
	if err == nil {
		t.Fatal("GetConfig should fail")
	}
	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("GetConfig error type = %T", err)
	}
	if clientErr.Path != "/config" || clientErr.StatusCode != http.StatusNotFound {
		t.Fatalf("ClientError = %+v", clientErr)
	}
	if clientErr.Payload == nil || clientErr.Payload.Error.Code != "GEAR_NOT_FOUND" {
		t.Fatalf("ClientError payload = %+v", clientErr.Payload)
	}
}

func TestPublicServerRegisterGearAndConflict(t *testing.T) {
	server := &PublicServer{
		Gears: newTestGearService(),
	}
	ctx := WithCallerPublicKey(context.Background(), "gear-pk")
	body := RegisterGearJSONRequestBody{
		Device: DeviceInfo{
			Sn: ptr("sn-1"),
			Hardware: &HardwareInfo{
				Depot: ptr("demo"),
			},
		},
	}

	resp, err := server.RegisterGear(ctx, RegisterGearRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("RegisterGear error: %v", err)
	}
	okResp, ok := resp.(RegisterGear200JSONResponse)
	if !ok {
		t.Fatalf("RegisterGear response type = %T", resp)
	}
	if okResp.Gear.PublicKey != "gear-pk" || okResp.Registration.PublicKey != "gear-pk" {
		t.Fatalf("RegisterGear response = %+v", okResp)
	}

	resp, err = server.RegisterGear(ctx, RegisterGearRequestObject{Body: &body})
	if err != nil {
		t.Fatalf("RegisterGear duplicate error: %v", err)
	}
	conflictResp, ok := resp.(RegisterGear409JSONResponse)
	if !ok {
		t.Fatalf("RegisterGear duplicate response type = %T", resp)
	}
	if conflictResp.Error.Code != "GEAR_ALREADY_EXISTS" {
		t.Fatalf("RegisterGear duplicate error code = %q", conflictResp.Error.Code)
	}
}

func TestPublicServerRuntimeAndServerInfo(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	server := &PublicServer{
		BuildCommit:     "commit-1",
		ServerPublicKey: "server-pk",
		PeerServer:      stubRuntimeProvider{runtime: gears.Runtime{Online: true, LastSeenAt: now.UnixMilli(), LastAddr: "127.0.0.1:9000"}},
		Now:             func() time.Time { return now },
	}
	ctx := WithCallerPublicKey(context.Background(), "gear-pk")

	resp, err := server.GetRuntime(ctx, GetRuntimeRequestObject{})
	if err != nil {
		t.Fatalf("GetRuntime error: %v", err)
	}
	runtimeResp, ok := resp.(GetRuntime200JSONResponse)
	if !ok {
		t.Fatalf("GetRuntime response type = %T", resp)
	}
	if !runtimeResp.Online || runtimeResp.LastAddr == nil || *runtimeResp.LastAddr != "127.0.0.1:9000" {
		t.Fatalf("GetRuntime response = %+v", runtimeResp)
	}

	infoObj, err := server.GetServerInfo(context.Background(), GetServerInfoRequestObject{})
	if err != nil {
		t.Fatalf("GetServerInfo error: %v", err)
	}
	infoResp, ok := infoObj.(GetServerInfo200JSONResponse)
	if !ok {
		t.Fatalf("GetServerInfo response type = %T", infoObj)
	}
	if infoResp.BuildCommit != "commit-1" || infoResp.PublicKey != "server-pk" || infoResp.ServerTime != now.UnixMilli() {
		t.Fatalf("GetServerInfo response = %+v", infoResp)
	}
}

func TestGetInfo500JSONResponse(t *testing.T) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(ctx *fiber.Ctx) error {
		return getInfo500JSONResponse(internalPublicError("boom")).VisitGetInfoResponse(ctx)
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

func newTestGearService() *gears.Service {
	return gears.NewService(gears.NewStore(kv.NewMemory(nil)), nil)
}

type stubRuntimeProvider struct {
	runtime gears.Runtime
}

func (s stubRuntimeProvider) PeerRuntime(context.Context, string) gears.Runtime {
	return s.runtime
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

func ptr[T any](v T) *T {
	return &v
}
