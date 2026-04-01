package httptransport

import (
	"net/http"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func NewClient(conn *peer.Conn, service uint64) *http.Client {
	return &http.Client{
		Transport: NewRoundTripper(conn, service),
		Timeout:   30 * time.Second,
	}
}
