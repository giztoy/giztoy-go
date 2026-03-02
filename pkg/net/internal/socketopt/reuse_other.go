//go:build !unix && !windows

package socketopt

import (
	"errors"
	"net"
)

func SetReusePort(_ uintptr) error {
	return errors.New("SO_REUSEPORT not supported on this platform")
}

func ListenUDPReusePort(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", udpAddr)
}
