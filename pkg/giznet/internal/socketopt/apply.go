package socketopt

import "net"

// Apply applies all configured optimizations to a UDP connection.
// Each optimization is tried independently — failures don't block others.
func Apply(conn *net.UDPConn, cfg Config) *OptimizationReport {
	report := &OptimizationReport{}

	recvBuf := cfg.RecvBufSize
	if recvBuf <= 0 {
		recvBuf = DefaultRecvBufSize
	}
	if err := conn.SetReadBuffer(recvBuf); err != nil {
		report.Entries = append(report.Entries, OptimizationEntry{
			Name: "SO_RCVBUF", Err: err,
		})
	} else {
		actual := GetSocketBufSize(conn, true)
		report.Entries = append(report.Entries, OptimizationEntry{
			Name: "SO_RCVBUF", Applied: true,
			Detail: "SO_RCVBUF=" + itoa(recvBuf) + " (actual=" + itoa(actual) + ")",
		})
	}

	sendBuf := cfg.SendBufSize
	if sendBuf <= 0 {
		sendBuf = DefaultSendBufSize
	}
	if err := conn.SetWriteBuffer(sendBuf); err != nil {
		report.Entries = append(report.Entries, OptimizationEntry{
			Name: "SO_SNDBUF", Err: err,
		})
	} else {
		actual := GetSocketBufSize(conn, false)
		report.Entries = append(report.Entries, OptimizationEntry{
			Name: "SO_SNDBUF", Applied: true,
			Detail: "SO_SNDBUF=" + itoa(sendBuf) + " (actual=" + itoa(actual) + ")",
		})
	}

	applyPlatformOptions(conn, cfg, report)

	return report
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
