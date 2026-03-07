package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const maxFrameSize = 1 << 20 // 1 MiB

// RPCRequest is the JSON-RPC-like request envelope.
type RPCRequest struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// RPCResponse is the JSON-RPC-like response envelope.
type RPCResponse struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// RPCError is the error field in an RPC response.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WriteFrame writes a length-prefixed frame (u32 LE + payload).
func WriteFrame(w io.Writer, data []byte) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// ReadFrame reads a length-prefixed frame.
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(hdr[:])
	if length > maxFrameSize {
		return nil, fmt.Errorf("rpc: frame too large: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteRPCResponse marshals and writes an RPCResponse as a frame.
func WriteRPCResponse(w io.Writer, resp *RPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteFrame(w, data)
}

// ReadRPCRequest reads and unmarshals an RPCRequest from a frame.
func ReadRPCRequest(r io.Reader) (*RPCRequest, error) {
	data, err := ReadFrame(r)
	if err != nil {
		return nil, err
	}
	var req RPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("rpc: unmarshal request: %w", err)
	}
	return &req, nil
}
