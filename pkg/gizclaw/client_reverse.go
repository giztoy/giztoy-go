package gizclaw

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/giznet/gizhttp"
)

type PeerPublicProvider interface {
	Info(ctx context.Context) (gears.RefreshInfo, error)
	Identifiers(ctx context.Context) (gears.RefreshIdentifiers, error)
	Version(ctx context.Context) (gears.RefreshVersion, error)
}

type DeviceProvider = PeerPublicProvider

func (c *Client) ServePeerPublic(ctx context.Context, provider PeerPublicProvider) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		info, err := provider.Info(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeClientJSON(w, info)
	})
	mux.HandleFunc("/identifiers", func(w http.ResponseWriter, r *http.Request) {
		out, err := provider.Identifiers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeClientJSON(w, out)
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		out, err := provider.Version(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeClientJSON(w, out)
	})

	server := gizhttp.NewServer(c.conn, ServicePeerPublic, mux)
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (c *Client) ServeReverseHTTP(ctx context.Context, provider PeerPublicProvider) error {
	return c.ServePeerPublic(ctx, provider)
}

func writeClientJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
