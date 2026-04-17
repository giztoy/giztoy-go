package core

const (
	// ProtocolKCP marks KCP-multiplexed traffic inside the encrypted transport
	// payload.
	ProtocolKCP byte = 0x00
	// ProtocolConnCtrl carries internal connection-control messages.
	ProtocolConnCtrl byte = 0xFF
)

var closeCtrlPayload = []byte{0x01}
