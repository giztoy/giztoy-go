package genx

import (
	"crypto/rand"
	"time"
)

const epoch2025 int64 = 1735689600
const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// NewStreamID generates a short unique stream identifier.
func NewStreamID() string {
	secs := uint32(time.Now().Unix() - epoch2025)
	timePart := base62EncodeUint32(secs)

	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	randomPart := base62Encode(randomBytes)

	return timePart + randomPart
}

func base62EncodeUint32(n uint32) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}

func base62Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var n uint64
	for _, b := range data {
		n = n*256 + uint64(b)
	}

	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}
