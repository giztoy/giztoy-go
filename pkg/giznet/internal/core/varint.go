package core

import "errors"

// ErrVarintTruncated is returned when a varint is truncated.
var ErrVarintTruncated = errors.New("core: varint truncated")

// ErrVarintOverflow is returned when a varint exceeds 64 bits.
var ErrVarintOverflow = errors.New("core: varint overflow")

// MaxVarintLen is the maximum number of bytes in a varint-encoded uint64.
const MaxVarintLen = 10

// EncodeVarint encodes v as a protobuf-style varint into buf.
// buf must be at least MaxVarintLen bytes to hold any uint64.
func EncodeVarint(buf []byte, v uint64) int {
	i := 0
	for v >= 0x80 {
		buf[i] = byte(v) | 0x80
		v >>= 7
		i++
	}
	buf[i] = byte(v)
	return i + 1
}

// AppendVarint appends a varint-encoded uint64 to dst.
func AppendVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

// DecodeVarint decodes a varint from buf.
func DecodeVarint(buf []byte) (uint64, int, error) {
	var v uint64
	for i, b := range buf {
		if i >= MaxVarintLen {
			return 0, 0, ErrVarintOverflow
		}
		v |= uint64(b&0x7F) << (7 * uint(i))
		if b < 0x80 {
			return v, i + 1, nil
		}
	}
	return 0, 0, ErrVarintTruncated
}

// VarintLen returns the number of bytes needed to encode v as a varint.
func VarintLen(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}
