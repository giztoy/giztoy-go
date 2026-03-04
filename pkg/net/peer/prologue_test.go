package peer

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestRPCRequestEncodeDecodeValid(t *testing.T) {
	req := RPCRequest{
		V:      PrologueVersion,
		ID:     "rpc-1",
		Method: "ping",
		Params: json.RawMessage(`{"x":1}`),
	}

	data, err := EncodeRPCRequest(req)
	if err != nil {
		t.Fatalf("EncodeRPCRequest failed: %v", err)
	}

	decoded, err := DecodeRPCRequest(data)
	if err != nil {
		t.Fatalf("DecodeRPCRequest failed: %v", err)
	}

	if decoded.ID != req.ID || decoded.Method != req.Method || decoded.V != PrologueVersion {
		t.Fatalf("decoded request mismatch: got=%+v want=%+v", decoded, req)
	}
}

func TestRPCRequestDecodeInvalidMissingFields(t *testing.T) {
	if _, err := DecodeRPCRequest([]byte(`{"v":1,"method":"m"}`)); !errors.Is(err, ErrMissingID) {
		t.Fatalf("DecodeRPCRequest(missing id) err=%v, want %v", err, ErrMissingID)
	}
	if _, err := DecodeRPCRequest([]byte(`{"v":1,"id":"   ","method":"m"}`)); !errors.Is(err, ErrMissingID) {
		t.Fatalf("DecodeRPCRequest(blank id) err=%v, want %v", err, ErrMissingID)
	}

	if _, err := DecodeRPCRequest([]byte(`{"v":1,"id":"x"}`)); !errors.Is(err, ErrMissingMethod) {
		t.Fatalf("DecodeRPCRequest(missing method) err=%v, want %v", err, ErrMissingMethod)
	}
	if _, err := DecodeRPCRequest([]byte(`{"v":1,"id":"x","method":" \t "}`)); !errors.Is(err, ErrMissingMethod) {
		t.Fatalf("DecodeRPCRequest(blank method) err=%v, want %v", err, ErrMissingMethod)
	}

	if _, err := DecodeRPCRequest([]byte(`{"v":2,"id":"x","method":"m"}`)); !errors.Is(err, ErrInvalidV) {
		t.Fatalf("DecodeRPCRequest(invalid v) err=%v, want %v", err, ErrInvalidV)
	}
}

func TestPrologueDecodeInvalidJSON(t *testing.T) {
	if _, err := DecodeRPCRequest([]byte(`{"v":1,"id":"x","method":`)); err == nil {
		t.Fatal("DecodeRPCRequest(invalid json) should fail")
	}
	if _, err := DecodeRPCResponse([]byte(`{"v":1,"id":"x","result":`)); err == nil {
		t.Fatal("DecodeRPCResponse(invalid json) should fail")
	}
	if _, err := DecodeEvent([]byte(`{"v":1,"name":`)); err == nil {
		t.Fatal("DecodeEvent(invalid json) should fail")
	}
}

func TestRPCResponseDecodeInvalid(t *testing.T) {
	if _, err := DecodeRPCResponse([]byte(`{"v":1}`)); !errors.Is(err, ErrMissingID) {
		t.Fatalf("DecodeRPCResponse(missing id) err=%v, want %v", err, ErrMissingID)
	}

	if _, err := DecodeRPCResponse([]byte(`{"v":1,"id":"x","error":{"code":1,"message":""}}`)); !errors.Is(err, ErrRPCErrorMessageRequired) {
		t.Fatalf("DecodeRPCResponse(empty error message) err=%v, want %v", err, ErrRPCErrorMessageRequired)
	}
}

func TestRPCResponseEncodeDecodeValid(t *testing.T) {
	resp := RPCResponse{
		V:      PrologueVersion,
		ID:     "rpc-2",
		Result: json.RawMessage(`{"ok":true}`),
	}

	data, err := EncodeRPCResponse(resp)
	if err != nil {
		t.Fatalf("EncodeRPCResponse failed: %v", err)
	}

	decoded, err := DecodeRPCResponse(data)
	if err != nil {
		t.Fatalf("DecodeRPCResponse failed: %v", err)
	}

	if decoded.ID != resp.ID || decoded.V != PrologueVersion {
		t.Fatalf("decoded response mismatch: got=%+v want=%+v", decoded, resp)
	}
}

func TestRPCResponseEncodeDecodeWithErrorPayload(t *testing.T) {
	resp := RPCResponse{
		V:  PrologueVersion,
		ID: "rpc-err-1",
		Error: &RPCError{
			Code:    500,
			Message: "internal error",
			Data:    json.RawMessage(`{"trace":"abc"}`),
		},
	}

	data, err := EncodeRPCResponse(resp)
	if err != nil {
		t.Fatalf("EncodeRPCResponse(error payload) failed: %v", err)
	}

	decoded, err := DecodeRPCResponse(data)
	if err != nil {
		t.Fatalf("DecodeRPCResponse(error payload) failed: %v", err)
	}
	if decoded.Error == nil {
		t.Fatal("decoded response error should not be nil")
	}
	if decoded.Error.Message != resp.Error.Message {
		t.Fatalf("decoded response error message=%q, want %q", decoded.Error.Message, resp.Error.Message)
	}
}

func TestEventEncodeDecodeValid(t *testing.T) {
	evt := Event{
		V:    PrologueVersion,
		Name: "joined",
		Data: json.RawMessage(`{"room":"alpha"}`),
	}

	data, err := EncodeEvent(evt)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}

	if decoded.Name != evt.Name || decoded.V != PrologueVersion {
		t.Fatalf("decoded event mismatch: got=%+v want=%+v", decoded, evt)
	}
}

func TestEventDecodeInvalidMissingField(t *testing.T) {
	if _, err := DecodeEvent([]byte(`{"v":1}`)); !errors.Is(err, ErrMissingName) {
		t.Fatalf("DecodeEvent(missing name) err=%v, want %v", err, ErrMissingName)
	}
	if _, err := DecodeEvent([]byte(`{"v":1,"name":"   "}`)); !errors.Is(err, ErrMissingName) {
		t.Fatalf("DecodeEvent(blank name) err=%v, want %v", err, ErrMissingName)
	}

	if _, err := DecodeEvent([]byte(`{"v":2,"name":"evt"}`)); !errors.Is(err, ErrInvalidV) {
		t.Fatalf("DecodeEvent(invalid v) err=%v, want %v", err, ErrInvalidV)
	}
}
