package core

// Protocol field values carried inside the encrypted transport payload.
const (
	// ProtocolRPC intentionally uses a high-bit value to keep RPC stream frames
	// distinct from the low-byte direct packet protocols handled by ServiceMux.
	ProtocolRPC   byte = 0x81
	ProtocolEVENT byte = 0x03
	ProtocolOPUS  byte = 0x10
)

// IsFoundationProtocol reports whether protocol is part of the
// currently implemented protocol whitelist.
func IsFoundationProtocol(protocol byte) bool {
	switch protocol {
	case ProtocolRPC, ProtocolEVENT, ProtocolOPUS:
		return true
	default:
		return false
	}
}
