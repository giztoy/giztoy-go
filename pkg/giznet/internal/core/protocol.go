package core

// ProtocolKCP marks KCP-multiplexed traffic inside the encrypted transport
// payload. All other protocol bytes are application-defined direct packets.
const ProtocolKCP byte = 0x00

func IsStreamProtocol(protocol byte) bool {
	switch protocol {
	case ProtocolKCP:
		return true
	default:
		return false
	}
}
