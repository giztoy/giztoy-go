package peerpublic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_ = json.NewEncoder(w).Encode(RefreshInfo{Name: ptr("demo")})
		case "/identifiers":
			_ = json.NewEncoder(w).Encode(RefreshIdentifiers{Sn: ptr("sn-1")})
		case "/version":
			_ = json.NewEncoder(w).Encode(RefreshVersion{Depot: ptr("demo"), FirmwareSemver: ptr("1.0.0")})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}
	client, err := NewClient(httpClient)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	info, err := client.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo error: %v", err)
	}
	if info.Name == nil || *info.Name != "demo" {
		t.Fatalf("GetInfo = %+v", info)
	}

	identifiers, err := client.GetIdentifiers(context.Background())
	if err != nil {
		t.Fatalf("GetIdentifiers error: %v", err)
	}
	if identifiers.Sn == nil || *identifiers.Sn != "sn-1" {
		t.Fatalf("GetIdentifiers = %+v", identifiers)
	}

	version, err := client.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion error: %v", err)
	}
	if version.Depot == nil || *version.Depot != "demo" || version.FirmwareSemver == nil || *version.FirmwareSemver != "1.0.0" {
		t.Fatalf("GetVersion = %+v", version)
	}
}

func TestClientErrors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			http.Error(w, "boom", http.StatusBadGateway)
		case "/identifiers":
			_, _ = w.Write([]byte("{"))
		case "/version":
			_ = json.NewEncoder(w).Encode(RefreshVersion{Depot: ptr("demo"), FirmwareSemver: ptr("1.0.0")})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	httpClient := ts.Client()
	httpClient.Transport = rewriteTransport{base: http.DefaultTransport, target: ts.URL}
	client, err := NewClient(httpClient)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}

	_, err = client.GetInfo(context.Background())
	if err == nil {
		t.Fatal("GetInfo should fail on non-2xx")
	}
	clientErr, ok := err.(*ClientError)
	if !ok {
		t.Fatalf("GetInfo error type = %T", err)
	}
	if clientErr.Path != "/info" || clientErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("ClientError = %+v", clientErr)
	}

	if _, err := client.GetIdentifiers(context.Background()); err == nil {
		t.Fatal("GetIdentifiers should fail on invalid json")
	}
	if _, err := client.GetVersion(context.Background()); err != nil {
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

func ptr[T any](v T) *T {
	return &v
}
