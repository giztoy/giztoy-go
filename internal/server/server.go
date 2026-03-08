package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/haivivi/giztoy/go/internal/identity"
	"github.com/haivivi/giztoy/go/internal/paths"
	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
	"github.com/haivivi/giztoy/go/pkg/net/peer"
)

// Config holds server startup parameters.
type Config struct {
	DataDir    string
	ListenAddr string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	cfgDir, _ := paths.ConfigDir()
	return Config{
		DataDir:    filepath.Join(cfgDir, "server"),
		ListenAddr: ":9820",
	}
}

// Server is the Giztoy server instance.
type Server struct {
	cfg       Config
	keyPair   *noise.KeyPair
	listener  *peer.Listener
	startTime time.Time
	logger    *log.Logger
}

// New creates a Server, loading or generating its identity key.
func New(cfg Config) (*Server, error) {
	keyPath := filepath.Join(cfg.DataDir, "identity.key")
	kp, err := identity.LoadOrGenerate(keyPath)
	if err != nil {
		return nil, fmt.Errorf("server: identity: %w", err)
	}

	return &Server{
		cfg:     cfg,
		keyPair: kp,
		logger:  log.New(os.Stderr, "[server] ", log.LstdFlags),
	}, nil
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	l, err := peer.Listen(s.keyPair,
		core.WithBindAddr(s.cfg.ListenAddr),
		core.WithAllowUnknown(true),
	)
	if err != nil {
		return fmt.Errorf("server: listen: %w", err)
	}
	s.listener = l
	s.startTime = time.Now()

	info := l.HostInfo()
	s.logger.Printf("public key: %s", s.keyPair.Public)
	s.logger.Printf("listening on %s", info.Addr)

	go s.acceptLoop(ctx)

	<-ctx.Done()
	s.logger.Printf("shutting down")
	return l.Close()
}

// PublicKey returns the server's public key.
func (s *Server) PublicKey() noise.PublicKey {
	return s.keyPair.Public
}

// ListenAddr returns the server's actual listen address.
// Only valid after Run has started.
func (s *Server) ListenAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.HostInfo().Addr.String()
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			s.logger.Printf("accept error: %v", err)
			return
		}
		s.logger.Printf("peer connected: %s", conn.PublicKey().ShortString())
		go s.servePeer(ctx, conn)
	}
}

func (s *Server) servePeer(ctx context.Context, conn *peer.Conn) {
	for {
		stream, err := conn.AcceptService(0)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			s.logger.Printf("accept stream error from %s: %v", conn.PublicKey().ShortString(), err)
			return
		}
		go s.handleStream(stream)
	}
}

func (s *Server) handleStream(stream net.Conn) {
	req, err := ReadRPCRequest(stream)
	if err != nil {
		s.logger.Printf("read rpc: %v", err)
		return
	}

	switch req.Method {
	case "peer.ping":
		s.handlePeerPing(stream, req)
	default:
		resp := &RPCResponse{
			V:  1,
			ID: req.ID,
			Error: &RPCError{
				Code:    -1,
				Message: fmt.Sprintf("unknown method: %s", req.Method),
			},
		}
		_ = WriteRPCResponse(stream, resp)
	}
}

// PingResponse is the response payload for peer.ping.
type PingResponse struct {
	ServerTime int64 `json:"server_time"`
}

func (s *Server) handlePeerPing(stream net.Conn, req *RPCRequest) {
	result := PingResponse{
		ServerTime: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(result)
	if err != nil {
		s.logger.Printf("marshal ping: %v", err)
		return
	}

	resp := &RPCResponse{V: 1, ID: req.ID, Result: data}
	if err := WriteRPCResponse(stream, resp); err != nil {
		s.logger.Printf("write ping: %v", err)
	}
}
