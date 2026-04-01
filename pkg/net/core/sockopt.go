package core

import (
	"net"

	"github.com/giztoy/giztoy-go/pkg/net/internal/socketopt"
)

const (
	DefaultRecvBufSize = socketopt.DefaultRecvBufSize
	DefaultSendBufSize = socketopt.DefaultSendBufSize
	DefaultBusyPollUS  = socketopt.DefaultBusyPollUS
	DefaultGSOSegment  = socketopt.DefaultGSOSegment
	DefaultBatchSize   = socketopt.DefaultBatchSize
)

// SocketConfig holds configuration for UDP socket optimizations.
type SocketConfig = socketopt.Config

// OptimizationEntry records the result of a single socket optimization attempt.
type OptimizationEntry = socketopt.OptimizationEntry

// OptimizationReport collects the results of all optimization attempts.
type OptimizationReport = socketopt.OptimizationReport

func DefaultSocketConfig() SocketConfig { return socketopt.DefaultConfig() }
func FullSocketConfig() SocketConfig    { return socketopt.FullConfig() }

// ApplySocketOptions applies all configured optimizations to a UDP connection.
// Each optimization is tried independently — failures don't block others.
func ApplySocketOptions(conn *net.UDPConn, cfg SocketConfig) *OptimizationReport {
	return socketopt.Apply(conn, cfg)
}

// ListenUDPReusePort creates a UDP socket with SO_REUSEPORT set.
func ListenUDPReusePort(addr string) (*net.UDPConn, error) {
	return socketopt.ListenUDPReusePort(addr)
}

// SetReusePort sets SO_REUSEPORT on a raw fd before bind.
func SetReusePort(fd uintptr) error {
	return socketopt.SetReusePort(fd)
}

// getSocketBufSize reads the actual socket buffer size via getsockopt.
func getSocketBufSize(conn *net.UDPConn, recv bool) int {
	return socketopt.GetSocketBufSize(conn, recv)
}

// batchConn wraps a UDPConn for batch I/O using recvmmsg/sendmmsg.
type batchConn = socketopt.BatchConn

func newBatchConn(conn *net.UDPConn, batchSize int) *batchConn {
	return socketopt.NewBatchConn(conn, batchSize)
}
