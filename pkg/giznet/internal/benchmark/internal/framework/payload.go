package framework

import "sync"

var payloadCache sync.Map // map[int][]byte

// Payload returns a deterministic payload of given size.
// The underlying buffer is cached and must be treated as read-only.
func Payload(size int) []byte {
	if size <= 0 {
		return nil
	}
	if v, ok := payloadCache.Load(size); ok {
		return v.([]byte)
	}
	b := make([]byte, size)
	for i := range b {
		b[i] = byte((i*31 + 17) & 0xff)
	}
	payloadCache.Store(size, b)
	return b
}
