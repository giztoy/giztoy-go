package gizclaw

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	apitypes "github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/credential"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/firmware"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/gear"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/mmx"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/workspace"
	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/workspacetemplate"
	"github.com/GizClaw/gizclaw-go/pkg/giznet"
	"github.com/GizClaw/gizclaw-go/pkg/store/depotstore"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

var ErrNilSecurityPolicy = errors.New("gizclaw: nil security policy")
var ErrNilGearStore = errors.New("gizclaw: nil gear store")
var ErrNilDepotStore = errors.New("gizclaw: nil depot store")

type SecurityPolicy interface {
	AllowPeerService(giznet.PublicKey, uint64) bool
}

type AllowAll struct{}

func (AllowAll) AllowPeerService(giznet.PublicKey, uint64) bool {
	return true
}

// Server holds peer transport configuration. Call ListenAndServe or Serve once per
// *giznet.Listener when you need multiple UDP listeners (similar to net/http.Server
// serving multiple listeners). Per-stream protocol handling can be extended later.
//
// Set gear/firmware storage config on the struct, then call ListenAndServe.
// Internal runtime state is built automatically on first serve.
type Server struct {
	KeyPair *giznet.KeyPair

	GearStore          kv.Store
	RegistrationTokens map[string]apitypes.GearRole
	BuildCommit        string
	ServerPublicKey    string
	DepotStore         depotstore.Store

	manager     *Manager
	peerService *PeerService

	mu          sync.Mutex
	listener    *giznet.Listener
	runtimeOnce sync.Once
	runtimeErr  error
}

// ListenAndServe starts a UDP peer listener and blocks serving connections until
// Accept returns an error (for example after Listener.Close). opts are appended
// after default options (AllowUnknown + service mux policy).
func (s *Server) ListenAndServe(keyPair *giznet.KeyPair, opts ...giznet.Option) error {
	if s == nil {
		return errors.New("gizclaw: nil server")
	}
	if keyPair != nil {
		s.KeyPair = keyPair
	}
	if s.KeyPair == nil {
		return errors.New("gizclaw: nil key pair")
	}
	all := append([]giznet.Option{
		giznet.WithAllowUnknown(true),
		giznet.WithServiceMuxConfig(giznet.ServiceMuxConfig{
			OnNewService: s.allowPeerService,
		}),
	}, opts...)
	l, err := giznet.Listen(s.KeyPair, all...)
	if err != nil {
		return err
	}
	if err := s.Serve(l); err != nil {
		_ = l.Close()
		return err
	}
	return nil
}

// Serve accepts peer connections until l.Accept returns an error (for example
// after Listener.Close). Each connection is handled in its own goroutine.
func (s *Server) Serve(l *giznet.Listener) error {
	if s == nil {
		return errors.New("gizclaw: nil server")
	}
	if l == nil {
		return errors.New("gizclaw: nil listener")
	}
	if err := s.initOnce(l.HostInfo().PublicKey.String()); err != nil {
		return err
	}
	s.setListener(l)
	defer s.clearListener(l)
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		svc := s.peerService
		if svc == nil {
			svc = &PeerService{}
		}
		host := &GearPeer{
			Conn:    conn,
			Service: svc,
		}
		go func() {
			_ = host.serve()
		}()
	}
}

// PublicKey returns the configured server public key.
func (s *Server) PublicKey() giznet.PublicKey {
	if s == nil {
		return giznet.PublicKey{}
	}
	if s.KeyPair != nil {
		return s.KeyPair.Public
	}
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	if listener == nil {
		return giznet.PublicKey{}
	}
	return listener.HostInfo().PublicKey
}

// PeerService returns the initialized peer service bundle, or nil before Serve initializes it.
func (s *Server) PeerService() *PeerService {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peerService
}

// Manager returns the initialized peer manager, or nil before Serve initializes it.
func (s *Server) Manager() *Manager {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.manager
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func (s *Server) allowPeerService(pk giznet.PublicKey, service uint64) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return service == ServiceRPC || service == ServiceServerPublic
	}
	return GearsSecurityPolicy{Gears: manager.Gears}.AllowPeerService(pk, service)
}

func (s *Server) setListener(l *giznet.Listener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listener = l
}

func (s *Server) clearListener(l *giznet.Listener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == l {
		s.listener = nil
	}
}

func (s *Server) initOnce(serverPublicKey string) error {
	s.runtimeOnce.Do(func() {
		s.runtimeErr = s.initRuntime(serverPublicKey)
	})
	return s.runtimeErr
}

func (s *Server) initRuntime(serverPublicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.manager != nil && s.peerService != nil {
		return nil
	}
	if s.GearStore == nil {
		return ErrNilGearStore
	}
	if s.DepotStore == nil {
		return ErrNilDepotStore
	}
	if serverPublicKey == "" {
		serverPublicKey = s.ServerPublicKey
	}
	if serverPublicKey == "" && s.KeyPair != nil {
		serverPublicKey = s.KeyPair.Public.String()
	}
	if serverPublicKey == "" {
		return fmt.Errorf("gizclaw: empty server public key")
	}

	gearsServer := &gear.Server{
		Store:              s.GearStore,
		RegistrationTokens: s.RegistrationTokens,
		BuildCommit:        s.BuildCommit,
		ServerPublicKey:    serverPublicKey,
	}
	manager := NewManager(gearsServer)
	gearsServer.PeerManager = manager

	firmwareServer := &firmware.Server{
		Store: s.DepotStore,
		ResolveGearTarget: func(ctx context.Context, publicKey string) (string, firmware.Channel, error) {
			return resolveGearTarget(ctx, gearsServer, publicKey)
		},
	}
	workspaceTemplateServer := &workspacetemplate.Server{Store: s.GearStore}
	workspaceServer := &workspace.Server{Store: s.GearStore}
	credentialServer := &credential.Server{Store: s.GearStore}
	mmxServer := &mmx.Server{Store: s.GearStore}

	s.manager = manager
	s.peerService = &PeerService{
		peerManager: manager,
		admin: &adminService{
			CredentialAdminService:        credentialServer,
			FirmwareAdminService:          firmwareServer,
			GearsAdminService:             gearsServer,
			MiniMaxAdminService:           mmxServer,
			WorkspaceAdminService:         workspaceServer,
			WorkspaceTemplateAdminService: workspaceTemplateServer,
		},
		gear: &gearAPIBundle{
			FirmwareGearService: firmwareServer,
			GearsGearService:    gearsServer,
		},
		public: &serverPublic{
			GearsServerPublic: gearsServer,
		},
	}
	return nil
}

func resolveGearTarget(ctx context.Context, gearsServer *gear.Server, publicKey string) (string, firmware.Channel, error) {
	if gearsServer == nil {
		return "", "", errors.New("gizclaw: gears service not configured")
	}
	gear, err := gearsServer.LoadGear(ctx, publicKey)
	if err != nil {
		return "", "", err
	}
	if gear.Device.Hardware == nil || gear.Device.Hardware.Depot == nil {
		return "", "", errors.New("missing depot")
	}
	if gear.Configuration.Firmware == nil || gear.Configuration.Firmware.Channel == nil {
		return "", "", errors.New("missing channel")
	}
	channel := firmware.Channel(*gear.Configuration.Firmware.Channel)
	if !channel.Valid() {
		return "", "", fmt.Errorf("invalid firmware channel %q", channel)
	}
	return *gear.Device.Hardware.Depot, channel, nil
}
