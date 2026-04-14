package core

// ProtocolKCP marks KCP-multiplexed traffic inside the encrypted transport
// payload. All other protocol bytes are application-defined direct packets.
const ProtocolKCP byte = 0x00
