package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestWriteReadFrame(t *testing.T) {
	payload := []byte(`{"hello":"world"}`)
	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatalf("WriteFrame err=%v", err)
	}

	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame err=%v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("ReadFrame got=%q, want=%q", got, payload)
	}
}

func TestReadFrameEmpty(t *testing.T) {
	_, err := ReadFrame(&bytes.Buffer{})
	if err == nil {
		t.Fatal("ReadFrame(empty) should fail")
	}
}

func TestReadFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], maxFrameSize+1)
	buf.Write(hdr[:])
	_, err := ReadFrame(&buf)
	if err == nil {
		t.Fatal("ReadFrame(too large) should fail")
	}
}

func TestWriteReadRPCResponse(t *testing.T) {
	var buf bytes.Buffer
	resp := &RPCResponse{V: 1, ID: "r1", Result: []byte(`{"ok":true}`)}
	if err := WriteRPCResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}

	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var got RPCResponse
	if err := json.Unmarshal(frame, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "r1" {
		t.Fatalf("ID=%q", got.ID)
	}
}

func TestReadRPCRequest(t *testing.T) {
	var buf bytes.Buffer
	req := RPCRequest{V: 1, ID: "q1", Method: "admin.status"}
	data, _ := json.Marshal(req)
	if err := WriteFrame(&buf, data); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRPCRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRPCRequest err=%v", err)
	}
	if got.Method != "admin.status" {
		t.Fatalf("Method=%q", got.Method)
	}
}

func TestReadRPCRequestBadJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, []byte("not json")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRPCRequest(&buf); err == nil {
		t.Fatal("ReadRPCRequest(bad json) should fail")
	}
}

func TestRPCResponseWithError(t *testing.T) {
	var buf bytes.Buffer
	resp := &RPCResponse{
		V:  1,
		ID: "e1",
		Error: &RPCError{Code: -1, Message: "unknown method"},
	}
	if err := WriteRPCResponse(&buf, resp); err != nil {
		t.Fatal(err)
	}

	frame, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var got RPCResponse
	if err := json.Unmarshal(frame, &got); err != nil {
		t.Fatal(err)
	}
	if got.Error == nil {
		t.Fatal("expected error in response")
	}
	if got.Error.Code != -1 {
		t.Fatalf("Error.Code=%d", got.Error.Code)
	}
}
