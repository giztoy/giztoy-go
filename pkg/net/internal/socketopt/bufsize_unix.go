//go:build unix

package socketopt

import (
	"net"
	"syscall"
)

// GetSocketBufSize reads the actual socket buffer size via getsockopt.
func GetSocketBufSize(conn *net.UDPConn, recv bool) int {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0
	}
	opt := syscall.SO_SNDBUF
	if recv {
		opt = syscall.SO_RCVBUF
	}
	var val int
	raw.Control(func(fd uintptr) {
		val, _ = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, opt)
	})
	return val
}
