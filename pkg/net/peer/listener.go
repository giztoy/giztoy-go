package peer

import (
	"sync"
	"time"

	"github.com/haivivi/giztoy/go/pkg/net/core"
	"github.com/haivivi/giztoy/go/pkg/net/noise"
)

type Listener struct {
	mu    sync.Mutex
	udp   *core.UDP
	owned bool

	closeOnce sync.Once
	closedCh  chan struct{}
	known     map[noise.PublicKey]struct{}
}

func Listen(key *noise.KeyPair, opts ...core.Option) (*Listener, error) {
	u, err := core.NewUDP(key, opts...)
	if err != nil {
		return nil, err
	}

	l, err := Wrap(u)
	if err != nil {
		_ = u.Close()
		return nil, err
	}
	l.owned = true
	return l, nil
}

func Wrap(u *core.UDP) (*Listener, error) {
	if u == nil {
		return nil, ErrNilUDP
	}

	l := &Listener{
		udp:      u,
		closedCh: make(chan struct{}),
		known:    make(map[noise.PublicKey]struct{}),
	}

	for p := range u.Peers() {
		if p == nil || p.Info == nil {
			continue
		}
		if p.Info.State == core.PeerStateEstablished {
			l.known[p.Info.PublicKey] = struct{}{}
		}
	}

	return l, nil
}

func (l *Listener) Accept() (*Conn, error) {
	if l == nil {
		return nil, ErrNilListener
	}

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if c := l.pollNewPeer(); c != nil {
			return c, nil
		}

		select {
		case <-l.closedCh:
			return nil, ErrClosed
		case <-ticker.C:
		}
	}
}

func (l *Listener) Peer(pk noise.PublicKey) (*Conn, error) {
	if l == nil {
		return nil, ErrNilListener
	}

	info := l.udp.PeerInfo(pk)
	if info == nil {
		return nil, core.ErrPeerNotFound
	}
	if info.State != core.PeerStateEstablished {
		return nil, core.ErrNoSession
	}

	l.mu.Lock()
	l.known[pk] = struct{}{}
	l.mu.Unlock()

	return &Conn{udp: l.udp, pk: pk}, nil
}

func (l *Listener) Close() error {
	if l == nil {
		return ErrNilListener
	}

	var err error
	l.closeOnce.Do(func() {
		close(l.closedCh)
		if l.owned {
			err = l.udp.Close()
		}
	})

	return err
}

func (l *Listener) pollNewPeer() *Conn {
	l.mu.Lock()
	defer l.mu.Unlock()

	for p := range l.udp.Peers() {
		if p == nil || p.Info == nil {
			continue
		}
		if p.Info.State != core.PeerStateEstablished {
			continue
		}
		if _, ok := l.known[p.Info.PublicKey]; ok {
			continue
		}

		l.known[p.Info.PublicKey] = struct{}{}
		return &Conn{udp: l.udp, pk: p.Info.PublicKey}
	}

	return nil
}
