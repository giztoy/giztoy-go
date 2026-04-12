package gizclaw

import (
	"errors"
	"github.com/giztoy/giztoy-go/pkg/giznet"
	"net"
	"sync"
)

var ErrNilSecurityPolicy = errors.New("gizclaw: nil security policy")

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
// Set KeyPair, SecurityPolicy, and pass giznet.WithBindAddr (and any other giznet.Option)
// to ListenAndServe. Manager is optional state shared with refresh / runtime helpers.
type Server struct {
	KeyPair    *giznet.KeyPair
	Manager    *Manager
	PeerServer *PeerServer

	// SecurityPolicy is required.
	SecurityPolicy SecurityPolicy

	// Cleanup runs once when Close is called.
	Cleanup func() error

	mu          sync.Mutex
	listener    *giznet.Listener
	cleanupOnce sync.Once
	cleanupErr  error
}

// ListenAndServe starts a UDP peer listener and blocks serving connections until
// Accept returns an error (for example after Listener.Close). opts are appended
// after default options (AllowUnknown + service mux policy).
func (s *Server) ListenAndServe(opts ...giznet.Option) error {
	if s == nil {
		return errors.New("gizclaw: nil server")
	}
	if s.KeyPair == nil {
		return errors.New("gizclaw: nil key pair")
	}
	if s.SecurityPolicy == nil {
		return ErrNilSecurityPolicy
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
	return s.Serve(l)
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
		peerServer := s.PeerServer
		if peerServer == nil {
			peerServer = &PeerServer{}
		}
		go func() {
			_ = peerServer.Serve(conn)
		}()
	}
}

// PublicKey returns the configured server public key.
func (s *Server) PublicKey() giznet.PublicKey {
	if s == nil || s.KeyPair == nil {
		return giznet.PublicKey{}
	}
	return s.KeyPair.Public
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var err error

	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	if listener != nil {
		err = listener.Close()
	}
	s.cleanupOnce.Do(func() {
		if s.Cleanup != nil {
			s.cleanupErr = s.Cleanup()
		}
	})
	if s.cleanupErr != nil {
		return errors.Join(err, s.cleanupErr)
	}
	return err
}

func (s *Server) allowPeerService(pk giznet.PublicKey, service uint64) bool {
	if s == nil || s.SecurityPolicy == nil {
		return false
	}
	return s.SecurityPolicy.AllowPeerService(pk, service)
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
