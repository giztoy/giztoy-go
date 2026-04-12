package adminservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/giztoy/giztoy-go/pkg/firmware"
)

func TestClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/firmwares":
			_ = json.NewEncoder(w).Encode(DepotList{
				Items: []Depot{{Name: "demo"}},
			})
		case "/firmwares/demo/stable":
			_ = json.NewEncoder(w).Encode(DepotRelease{
				Channel:        stringPtr("stable"),
				FirmwareSemver: "1.0.0",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}

	client, err := NewClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	depots, err := client.ListDepots(context.Background())
	if err != nil {
		t.Fatalf("ListDepots error: %v", err)
	}
	if len(depots.Items) != 1 || depots.Items[0].Name != "demo" {
		t.Fatalf("ListDepots = %+v", depots)
	}

	channel, err := client.GetChannel(context.Background(), "demo", Stable)
	if err != nil {
		t.Fatalf("GetChannel error: %v", err)
	}
	if channel.FirmwareSemver != "1.0.0" {
		t.Fatalf("GetChannel firmware semver = %q", channel.FirmwareSemver)
	}
}

func TestClientErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/firmwares":
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error: ErrorPayload{
					Code:    "DIRECTORY_SCAN_FAILED",
					Message: "boom",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}

	client, err := NewClient("http://gizclaw", WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.ListDepots(context.Background())
	if err == nil {
		t.Fatal("ListDepots should fail")
	}
	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("ListDepots error type = %T", err)
	}
	if clientErr.Path != "/firmwares" || clientErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("ClientError = %+v", clientErr)
	}
	if clientErr.Payload == nil || clientErr.Payload.Error.Code != "DIRECTORY_SCAN_FAILED" {
		t.Fatalf("ClientError payload = %+v", clientErr.Payload)
	}
}

func TestServerListDepotsAndValidation(t *testing.T) {
	server := newTestAdminServer(t)

	resp, err := server.ListDepots(context.Background(), ListDepotsRequestObject{})
	if err != nil {
		t.Fatalf("ListDepots error: %v", err)
	}
	listResp, ok := resp.(ListDepots200JSONResponse)
	if !ok {
		t.Fatalf("ListDepots response type = %T", resp)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].Name != "demo" {
		t.Fatalf("ListDepots response = %+v", listResp)
	}

	putResp, err := server.PutDepotInfo(context.Background(), PutDepotInfoRequestObject{Depot: "demo"})
	if err != nil {
		t.Fatalf("PutDepotInfo error: %v", err)
	}
	errResp, ok := putResp.(PutDepotInfo400JSONResponse)
	if !ok {
		t.Fatalf("PutDepotInfo response type = %T", putResp)
	}
	if errResp.Error.Code != "INVALID_JSON" {
		t.Fatalf("PutDepotInfo error code = %q", errResp.Error.Code)
	}
}

func TestServerReleaseDepotMapsNotFound(t *testing.T) {
	root := t.TempDir()
	store := firmware.NewStore(root)
	scanner := firmware.NewScanner(store)
	switcher := firmware.NewSwitcher(store, scanner)
	server := &Server{FirmwareSwitcher: switcher}

	resp, err := server.ReleaseDepot(context.Background(), ReleaseDepotRequestObject{Depot: "missing"})
	if err != nil {
		t.Fatalf("ReleaseDepot error: %v", err)
	}
	errResp, ok := resp.(ReleaseDepot404JSONResponse)
	if !ok {
		t.Fatalf("ReleaseDepot response type = %T", resp)
	}
	if errResp.Error.Code != "DEPOT_NOT_FOUND" {
		t.Fatalf("ReleaseDepot error code = %q", errResp.Error.Code)
	}
}

func newTestAdminServer(t *testing.T) *Server {
	t.Helper()

	root := t.TempDir()
	store := firmware.NewStore(root)
	scanner := firmware.NewScanner(store)
	uploader := firmware.NewUploader(store, scanner)
	switcher := firmware.NewSwitcher(store, scanner)
	if err := uploader.PutInfo("demo", firmware.DepotInfo{
		Files: []firmware.DepotInfoFile{{Path: "firmware.bin"}},
	}); err != nil {
		t.Fatalf("PutInfo error: %v", err)
	}
	return &Server{
		FirmwareScanner:  scanner,
		FirmwareUploader: uploader,
		FirmwareSwitcher: switcher,
	}
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
