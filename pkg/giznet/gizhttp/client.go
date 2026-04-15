package gizhttp

import (
	"net/http"
	"time"

	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

func NewClient(conn *giznet.Conn, service uint64) *http.Client {
	return &http.Client{
		Transport: NewRoundTripper(conn, service),
		Timeout:   30 * time.Second,
	}
}
