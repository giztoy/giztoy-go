//go:build !unix && !windows

package socketopt

import "net"

func GetSocketBufSize(_ *net.UDPConn, _ bool) int { return 0 }
