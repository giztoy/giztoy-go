package giznet

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/GizClaw/gizclaw-go/pkg/giznet/internal/core"
)

type Listener struct {
	mu sync.Mutex

	udp       *core.UDP
	closeOnce sync.Once
	closedCh  chan struct{}
	closed    atomic.Bool
	known     map[PublicKey]struct{}
	events    chan PeerEvent
}

func Listen(key *KeyPair, opts ...Option) (*Listener, error) {
	l := &Listener{
		closedCh: make(chan struct{}),
		known:    make(map[PublicKey]struct{}),
		events:   make(chan PeerEvent, 64),
	}

	// Append our handler last so it always wins if the caller accidentally
	// passes WithOnPeerEvent (last-write-wins in the options slice).
	allOpts := append(opts, core.WithOnPeerEvent(l.onPeerEvent))
	u, err := core.NewUDP(key, allOpts...)
	if err != nil {
		return nil, err
	}
	l.udp = u

	return l, nil
}

func (l *Listener) onPeerEvent(ev PeerEvent) bool {
	if l.closed.Load() {
		return false
	}
	select {
	case l.events <- ev:
		return true
	default:
		return false
	}
}

func (l *Listener) Accept() (*Conn, error) {
	if l == nil {
		return nil, ErrNilListener
	}

	for {
		select {
		case <-l.closedCh:
			return nil, ErrClosed
		case ev, ok := <-l.events:
			if !ok {
				return nil, ErrClosed
			}
			if ev.State != core.PeerStateEstablished {
				continue
			}
			l.mu.Lock()
			if _, dup := l.known[ev.PublicKey]; dup {
				l.mu.Unlock()
				continue
			}
			l.known[ev.PublicKey] = struct{}{}
			l.mu.Unlock()
			return &Conn{udp: l.udp, pk: ev.PublicKey, listener: l}, nil
		}
	}
}

// PeerEvents returns the channel that receives peer state change events.
// The channel is buffered (cap 64); slow consumers miss events.
func (l *Listener) PeerEvents() <-chan PeerEvent {
	return l.events
}

func (l *Listener) Peer(pk PublicKey) (*Conn, error) {
	if l == nil {
		return nil, ErrNilListener
	}

	info := l.udp.PeerInfo(pk)
	if info == nil {
		return nil, ErrPeerNotFound
	}
	if info.State != core.PeerStateEstablished {
		return nil, ErrNoSession
	}

	return &Conn{udp: l.udp, pk: pk, listener: l}, nil
}

// release removes a peer from the known set so it can be re-accepted.
func (l *Listener) release(pk PublicKey) {
	if l == nil {
		return
	}
	l.mu.Lock()
	delete(l.known, pk)
	l.mu.Unlock()
}

func (l *Listener) SetPeerEndpoint(pk PublicKey, endpoint *net.UDPAddr) {
	l.udp.SetPeerEndpoint(pk, endpoint)
}

func (l *Listener) Connect(pk PublicKey) error {
	return l.udp.Connect(pk)
}

// Dial sets the peer endpoint, performs a synchronous Noise IK handshake,
// and returns an established Conn.
func (l *Listener) Dial(pk PublicKey, addr *net.UDPAddr) (*Conn, error) {
	l.mu.Lock()
	l.known[pk] = struct{}{}
	l.mu.Unlock()

	l.SetPeerEndpoint(pk, addr)
	if err := l.Connect(pk); err != nil {
		l.release(pk)
		return nil, err
	}
	return &Conn{udp: l.udp, pk: pk, listener: l}, nil
}

func (l *Listener) UDP() *UDP {
	return l.udp
}

func (l *Listener) HostInfo() *HostInfo {
	return l.udp.HostInfo()
}

func (l *Listener) Close() error {
	if l == nil {
		return ErrNilListener
	}

	var err error
	l.closeOnce.Do(func() {
		close(l.closedCh)
		l.closed.Store(true)
		// Close UDP first so no more onPeerEvent callbacks can fire,
		// then close the events channel safely.
		err = l.udp.Close()
		close(l.events)
	})

	return err
}
