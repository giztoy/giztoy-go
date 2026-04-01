package httptransport

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

type listener struct {
	conn    *peer.Conn
	service uint64
	closed  atomic.Bool
	once    sync.Once
}

type listenerAddr struct {
	peerPK  string
	service uint64
}

func NewListener(conn *peer.Conn, service uint64) net.Listener {
	return &listener{conn: conn, service: service}
}

func (l *listener) Accept() (net.Conn, error) {
	if l.closed.Load() {
		return nil, net.ErrClosed
	}
	stream, err := l.conn.AcceptService(l.service)
	if err != nil {
		if l.closed.Load() {
			return nil, net.ErrClosed
		}
		return nil, err
	}
	return wrapStream(stream), nil
}

func (l *listener) Close() error {
	l.once.Do(func() {
		l.closed.Store(true)
		if l.conn != nil {
			_ = l.conn.StopAcceptingService(l.service)
		}
	})
	return nil
}

func (l *listener) Addr() net.Addr {
	peerPK := ""
	if l.conn != nil {
		peerPK = l.conn.PublicKey().String()
	}
	return listenerAddr{peerPK: peerPK, service: l.service}
}

func (a listenerAddr) Network() string {
	return "kcp-http"
}

func (a listenerAddr) String() string {
	return fmt.Sprintf("%s/service/%d", a.peerPK, a.service)
}

func IsClosed(err error) bool {
	return errors.Is(err, net.ErrClosed)
}
