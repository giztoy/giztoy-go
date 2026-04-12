package gizclaw

const (
	ProtocolEvent       byte = 0x03
	ProtocolStampedOpus byte = 0x10
)

const (
	ServiceRPC          uint64 = 0x00
	ServiceServerPublic uint64 = 0x01
	ServicePeerPublic   uint64 = 0x02
	ServiceAdmin        uint64 = 0x10
	ServiceGear         uint64 = 0x11
)
