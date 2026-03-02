package socketopt

const (
	DefaultRecvBufSize = 4 * 1024 * 1024 // 4MB
	DefaultSendBufSize = 4 * 1024 * 1024 // 4MB
	DefaultBusyPollUS  = 50              // 50μs busy-poll duration
	DefaultGSOSegment  = 1400            // MTU-sized GSO segments
	DefaultBatchSize   = 64              // recvmmsg/sendmmsg batch size
)

// Config holds configuration for UDP socket optimizations.
// All fields are optional; zero values trigger sensible defaults.
type Config struct {
	RecvBufSize int  // SO_RCVBUF in bytes (0 → DefaultRecvBufSize)
	SendBufSize int  // SO_SNDBUF in bytes (0 → DefaultSendBufSize)
	BusyPollUS  int  // SO_BUSY_POLL in μs (Linux, 0 = disabled)
	GRO         bool // UDP_GRO receive coalescing (Linux 4.18+)
	GSO         bool // UDP_SEGMENT send segmentation (Linux 4.18+)
}

// DefaultConfig returns recommended defaults for high-throughput use.
func DefaultConfig() Config {
	return Config{
		RecvBufSize: DefaultRecvBufSize,
		SendBufSize: DefaultSendBufSize,
	}
}

// FullConfig returns a config with all optimizations enabled.
func FullConfig() Config {
	return Config{
		RecvBufSize: DefaultRecvBufSize,
		SendBufSize: DefaultSendBufSize,
		BusyPollUS:  DefaultBusyPollUS,
		GRO:         true,
		GSO:         true,
	}
}
