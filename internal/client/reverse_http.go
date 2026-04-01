package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/httptransport"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

type DeviceProvider interface {
	Info(ctx context.Context) (gears.RefreshInfo, error)
	Identifiers(ctx context.Context) (gears.RefreshIdentifiers, error)
	Version(ctx context.Context) (gears.RefreshVersion, error)
}

func (c *Client) ServeReverseHTTP(ctx context.Context, provider DeviceProvider) error {
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
	server := httptransport.NewServer(c.conn, peer.ServiceReverse, mux)
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func writeClientJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
