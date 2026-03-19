package peer

import (
	"encoding/json"
	"strings"
)

const PrologueVersion = 1

const (
	ServicePublic  uint64 = 0
	ServiceAdmin   uint64 = 1
	ServiceReverse uint64 = 2
)

type RPCRequest struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type RPCResponse struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type Event struct {
	V    int             `json:"v"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

func (r RPCRequest) Validate() error {
	if r.V != PrologueVersion {
		return ErrInvalidV
	}
	if strings.TrimSpace(r.ID) == "" {
		return ErrMissingID
	}
	if strings.TrimSpace(r.Method) == "" {
		return ErrMissingMethod
	}
	return nil
}

func (r RPCResponse) Validate() error {
	if r.V != PrologueVersion {
		return ErrInvalidV
	}
	if strings.TrimSpace(r.ID) == "" {
		return ErrMissingID
	}
	if r.Error != nil && strings.TrimSpace(r.Error.Message) == "" {
		return ErrRPCErrorMessageRequired
	}
	return nil
}

func (e Event) Validate() error {
	if e.V != PrologueVersion {
		return ErrInvalidV
	}
	if strings.TrimSpace(e.Name) == "" {
		return ErrMissingName
	}
	return nil
}

func EncodeRPCRequest(req RPCRequest) ([]byte, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(req)
}

func DecodeRPCRequest(data []byte) (RPCRequest, error) {
	var req RPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return RPCRequest{}, err
	}
	if err := req.Validate(); err != nil {
		return RPCRequest{}, err
	}
	return req, nil
}

func EncodeRPCResponse(resp RPCResponse) ([]byte, error) {
	if err := resp.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(resp)
}

func DecodeRPCResponse(data []byte) (RPCResponse, error) {
	var resp RPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return RPCResponse{}, err
	}
	if err := resp.Validate(); err != nil {
		return RPCResponse{}, err
	}
	return resp, nil
}

func EncodeEvent(evt Event) ([]byte, error) {
	if err := evt.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(evt)
}

func DecodeEvent(data []byte) (Event, error) {
	var evt Event
	if err := json.Unmarshal(data, &evt); err != nil {
		return Event{}, err
	}
	if err := evt.Validate(); err != nil {
		return Event{}, err
	}
	return evt, nil
}
