package core

import "errors"

// ErrPayloadTooShort indicates a missing protocol byte.
var ErrPayloadTooShort = errors.New("net: payload too short")

// EncodePayload encodes protocol + payload into a single payload.
//
// Wire format: protocol(1B) | payload(N)
func EncodePayload(protocol byte, payload []byte) []byte {
	result := make([]byte, 1+len(payload))
	result[0] = protocol
	copy(result[1:], payload)
	return result
}

// DecodePayload decodes protocol + payload from a payload.
//
// Wire format: protocol(1B) | payload(N)
func DecodePayload(data []byte) (protocol byte, payload []byte, err error) {
	if len(data) < 1 {
		return 0, nil, ErrPayloadTooShort
	}
	protocol = data[0]
	return protocol, data[1:], nil
}
