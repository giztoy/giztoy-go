package kcp

import thirdpartynet "github.com/giztoy/giztoy-go/third_party/net"

// KCP re-exports the third-party KCP binding type.
type KCP = thirdpartynet.KCP

// NewKCP creates a new KCP instance with the given conversation ID.
func NewKCP(conv uint32, output func([]byte)) *KCP {
	return thirdpartynet.NewKCP(conv, output)
}

// GetConv extracts the conversation ID from a KCP packet.
func GetConv(data []byte) uint32 {
	return thirdpartynet.GetConv(data)
}
