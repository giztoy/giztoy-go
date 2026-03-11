package httptransport

import (
	"net"
	"time"
)

type streamConn struct {
	net.Conn
}

func wrapStream(conn net.Conn) net.Conn {
	if conn == nil {
		return nil
	}
	return &streamConn{Conn: conn}
}

func (c *streamConn) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}
