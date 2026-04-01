package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/giztoy/giztoy-go/internal/identity"
	"github.com/giztoy/giztoy-go/internal/paths"
	"github.com/giztoy/giztoy-go/internal/stores"
	"github.com/giztoy/giztoy-go/pkg/firmware"
	"github.com/giztoy/giztoy-go/pkg/gears"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/httptransport"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

// Config holds server startup parameters.
type Config struct {
	DataDir    string
	ListenAddr string
	ConfigPath string
	Stores     map[string]stores.Config
	Gears      GearsConfig
	Depots     DepotsConfig
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	cfgDir, _ := paths.ConfigDir()
	return Config{
		DataDir:    filepath.Join(cfgDir, "server"),
		ListenAddr: ":9820",
	}
}

type activePeer struct {
	conn       *peer.Conn
	lastSeenAt int64
	online     bool
}

// Server is the Giztoy server instance.
type Server struct {
	cfg       Config
	stores    *stores.Stores
	keyPair   *noise.KeyPair
	listener  *peer.Listener
	startTime time.Time
	logger    *log.Logger
	gears     *gears.Service

	firmwareStore    *firmware.Store
	firmwareScanner  *firmware.Scanner
	firmwareUploader *firmware.Uploader
	firmwareSwitcher *firmware.Switcher
	firmwareOTA      *firmware.OTAService

	activePeersMu sync.RWMutex
	activePeers   map[string]*activePeer
}

// New creates a Server, loading or generating its identity key.
func New(cfg Config) (*Server, error) {
	if cfg.ConfigPath != "" {
		fileCfg, err := LoadConfig(cfg.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("server: load config: %w", err)
		}
		cfg = mergeFileConfig(cfg, fileCfg)
	}
	defaults := DefaultConfig()
	if cfg.DataDir == "" {
		cfg.DataDir = defaults.DataDir
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaults.ListenAddr
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	keyPath := filepath.Join(cfg.DataDir, "identity.key")
	kp, err := identity.LoadOrGenerate(keyPath)
	if err != nil {
		return nil, fmt.Errorf("server: identity: %w", err)
	}

	ss, err := stores.New(cfg.DataDir, cfg.Stores)
	if err != nil {
		return nil, fmt.Errorf("server: stores: %w", err)
	}
	storesOK := false
	defer func() {
		if !storesOK {
			ss.Close()
		}
	}()

	gearsKV, err := ss.KV(cfg.Gears.Store)
	if err != nil {
		return nil, fmt.Errorf("server: gears store: %w", err)
	}
	gearStore := gears.NewStore(gearsKV)
	gearService := gears.NewService(gearStore, cfg.Gears.RegistrationTokens)

	fwStore, err := ss.FS(cfg.Depots.Store)
	if err != nil {
		return nil, fmt.Errorf("server: firmware store: %w", err)
	}
	if err := os.MkdirAll(fwStore.Root(), 0o755); err != nil {
		return nil, fmt.Errorf("server: firmware dir: %w", err)
	}
	fwScanner := firmware.NewScanner(fwStore)

	storesOK = true
	return &Server{
		cfg:              cfg,
		stores:           ss,
		keyPair:          kp,
		logger:           log.New(os.Stderr, "[server] ", log.LstdFlags),
		gears:            gearService,
		firmwareStore:    fwStore,
		firmwareScanner:  fwScanner,
		firmwareUploader: firmware.NewUploader(fwStore, fwScanner),
		firmwareSwitcher: firmware.NewSwitcher(fwStore, fwScanner),
		firmwareOTA:      firmware.NewOTAService(fwStore, fwScanner),
		activePeers:      make(map[string]*activePeer),
	}, nil
}

// Run starts the server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	l, err := peer.Listen(s.keyPair,
		core.WithBindAddr(s.cfg.ListenAddr),
		core.WithAllowUnknown(true),
		core.WithServiceMuxConfig(core.ServiceMuxConfig{
			OnNewService: s.allowPeerService,
		}),
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
	if err := l.Close(); err != nil {
		s.logger.Printf("listener close: %v", err)
	}
	if err := s.stores.Close(); err != nil {
		s.logger.Printf("stores close: %v", err)
	}
	return nil
}

func (s *Server) allowPeerService(_ noise.PublicKey, service uint64) bool {
	switch service {
	case peer.ServicePublic, peer.ServiceAdmin, peer.ServiceReverse:
		return true
	default:
		return false
	}
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
	defer conn.Close()

	publicKey := conn.PublicKey().String()
	s.markPeerOnline(publicKey, conn)
	defer s.markPeerOffline(publicKey, conn)

	go func() {
		if err := s.serveAdminHTTP(ctx, conn); err != nil && !errors.Is(err, net.ErrClosed) {
			s.logger.Printf("admin http error from %s: %v", conn.PublicKey().ShortString(), err)
		}
	}()

	for {
		stream, err := conn.AcceptService(peer.ServicePublic)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			s.logger.Printf("accept stream error from %s: %v", conn.PublicKey().ShortString(), err)
			return
		}
		s.touchPeer(publicKey, conn)
		go s.handleService0Stream(conn, stream)
	}
}

func (s *Server) handleService0Stream(conn *peer.Conn, stream net.Conn) {
	pk := conn.PublicKey().String()
	peeked := newPeekConn(stream)
	if looksLikeHTTP(peeked.reader) {
		s.serveSingleHTTPConn(pk, peeked, s.publicHandler(pk))
		return
	}
	s.handleStream(peeked)
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

func (s *Server) serveSingleHTTPConn(publicKey string, conn net.Conn, handler http.Handler) {
	listener := &singleConnListener{conn: conn}
	server := &http.Server{
		Handler: handler,
		BaseContext: func(_ net.Listener) context.Context {
			return withCallerPublicKey(context.Background(), publicKey)
		},
	}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		s.logger.Printf("public http serve error: %v", err)
	}
}

func (s *Server) serveAdminHTTP(ctx context.Context, conn *peer.Conn) error {
	server := httptransport.NewServer(conn, peer.ServiceAdmin, s.adminHandler(conn.PublicKey().String()))
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return server.Serve()
}

func (s *Server) markPeerOnline(publicKey string, conn *peer.Conn) {
	s.activePeersMu.Lock()
	defer s.activePeersMu.Unlock()
	s.activePeers[publicKey] = &activePeer{
		conn:       conn,
		lastSeenAt: time.Now().UnixMilli(),
		online:     true,
	}
}

func (s *Server) touchPeer(publicKey string, conn *peer.Conn) {
	s.activePeersMu.Lock()
	defer s.activePeersMu.Unlock()
	state, ok := s.activePeers[publicKey]
	if !ok || state.conn != conn {
		return
	}
	state.lastSeenAt = time.Now().UnixMilli()
}

func (s *Server) markPeerOffline(publicKey string, conn *peer.Conn) {
	s.activePeersMu.Lock()
	defer s.activePeersMu.Unlock()
	state, ok := s.activePeers[publicKey]
	if !ok || state.conn != conn {
		return
	}
	delete(s.activePeers, publicKey)
}

func (s *Server) peerRuntime(publicKey string) gears.Runtime {
	s.activePeersMu.RLock()
	defer s.activePeersMu.RUnlock()
	state, ok := s.activePeers[publicKey]
	if !ok {
		return gears.Runtime{}
	}
	return gears.Runtime{
		Online:     state.online,
		LastSeenAt: state.lastSeenAt,
	}
}

func (s *Server) activePeer(publicKey string) (*peer.Conn, bool) {
	s.activePeersMu.RLock()
	defer s.activePeersMu.RUnlock()
	state, ok := s.activePeers[publicKey]
	if !ok || !state.online || state.conn == nil {
		return nil, false
	}
	return state.conn, true
}

type peekConn struct {
	net.Conn
	reader *bufio.Reader
}

func newPeekConn(conn net.Conn) *peekConn {
	return &peekConn{Conn: conn, reader: bufio.NewReader(conn)}
}

func (c *peekConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func looksLikeHTTP(r *bufio.Reader) bool {
	peek, err := r.Peek(8)
	if err != nil && len(peek) == 0 {
		return false
	}
	methods := []string{"GET ", "PUT ", "POST ", "HEAD ", "PATCH ", "DELETE ", "OPTIONS "}
	upper := strings.ToUpper(string(peek))
	for _, method := range methods {
		if strings.HasPrefix(upper, method) {
			return true
		}
	}
	return false
}

type singleConnListener struct {
	conn net.Conn
	once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var conn net.Conn
	l.once.Do(func() {
		conn = l.conn
		l.conn = nil
	})
	if conn == nil {
		return nil, net.ErrClosed
	}
	return conn, nil
}

func (l *singleConnListener) Close() error {
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	return serviceAddr("service0")
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

type serviceAddr string

func (a serviceAddr) Network() string { return string(a) }
func (a serviceAddr) String() string  { return string(a) }
