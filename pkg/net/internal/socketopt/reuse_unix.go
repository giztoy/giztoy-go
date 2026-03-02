//go:build unix

package socketopt

import (
	"context"
	"net"
	"syscall"
)

// ListenUDPReusePort creates a UDP socket with SO_REUSEPORT set.
func ListenUDPReusePort(addr string) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				err = SetReusePort(fd)
			})
			return err
		},
	}
	pc, err := lc.ListenPacket(context.Background(), "udp", addr)
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
}
