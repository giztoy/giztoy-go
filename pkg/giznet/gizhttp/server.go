package gizhttp

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/GizClaw/gizclaw-go/pkg/giznet"
)

type Server struct {
	httpServer *http.Server
	listener   net.Listener
}

func NewServer(conn *giznet.Conn, service uint64, handler http.Handler) *Server {
	listener := NewListener(conn, service)
	return &Server{
		listener: listener,
		httpServer: &http.Server{
			Handler: handler,
			BaseContext: func(_ net.Listener) context.Context {
				return context.Background()
			},
		},
	}
}

func (s *Server) Serve() error {
	err := s.httpServer.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
