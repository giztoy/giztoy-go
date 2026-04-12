package rpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestClientRoutesResponsesByID(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	client := NewClient(clientSide)
	defer client.Close()

	req1Ch := make(chan *RPCRequest, 1)
	req2Ch := make(chan *RPCRequest, 1)
	serverErrCh := make(chan error, 1)

	go func() {
		req1, err := ReadRequest(serverSide)
		if err != nil {
			serverErrCh <- err
			return
		}
		req1Ch <- req1

		req2, err := ReadRequest(serverSide)
		if err != nil {
			serverErrCh <- err
			return
		}
		req2Ch <- req2

		resp2 := ResultResponse(req2.Id, &PingResponse{ServerTime: serverTimeForID(req2.Id)})
		if err := WriteResponse(serverSide, resp2); err != nil {
			serverErrCh <- err
			return
		}

		resp1 := ResultResponse(req1.Id, &PingResponse{ServerTime: serverTimeForID(req1.Id)})
		if err := WriteResponse(serverSide, resp1); err != nil {
			serverErrCh <- err
			return
		}

		serverErrCh <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type result struct {
		ping *PingResponse
		err  error
	}
	res1Ch := make(chan result, 1)
	res2Ch := make(chan result, 1)

	go func() {
		ping, err := client.Ping(ctx, "req-1")
		res1Ch <- result{ping: ping, err: err}
	}()
	go func() {
		ping, err := client.Ping(ctx, "req-2")
		res2Ch <- result{ping: ping, err: err}
	}()

	res1 := <-res1Ch
	res2 := <-res2Ch

	if res1.err != nil {
		t.Fatalf("Ping(req-1) error: %v", res1.err)
	}
	if res2.err != nil {
		t.Fatalf("Ping(req-2) error: %v", res2.err)
	}
	if res1.ping.ServerTime != serverTimeForID("req-1") {
		t.Fatalf("Ping(req-1) server_time = %d", res1.ping.ServerTime)
	}
	if res2.ping.ServerTime != serverTimeForID("req-2") {
		t.Fatalf("Ping(req-2) server_time = %d", res2.ping.ServerTime)
	}

	if req1 := <-req1Ch; req1.Id == "" {
		t.Fatal("first request missing id")
	} else {
		assertPingRequestHasTimestamp(t, req1)
	}
	if req2 := <-req2Ch; req2.Id == "" {
		t.Fatal("second request missing id")
	} else {
		assertPingRequestHasTimestamp(t, req2)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("server goroutine error: %v", err)
	}
}

func TestClientCallContextTimeoutRemovesPending(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	client := NewClient(clientSide)
	defer client.Close()

	readDone := make(chan *RPCRequest, 1)
	go func() {
		req, _ := ReadRequest(serverSide)
		readDone <- req
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := client.call(ctx, &RPCRequest{
		V:      1,
		Id:     "timeout",
		Method: MethodPing,
		Params: &PingRequest{ClientSendTime: time.Now().UnixMilli()},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Call timeout err = %v, want %v", err, context.DeadlineExceeded)
	}

	if req := <-readDone; req == nil || req.Id != "timeout" {
		t.Fatalf("server received request = %+v", req)
	} else {
		assertPingRequestHasTimestamp(t, req)
	}

	client.pendingMu.Lock()
	defer client.pendingMu.Unlock()
	if client.pending == nil {
		t.Fatal("pending map should still exist")
	}
	if _, ok := client.pending["timeout"]; ok {
		t.Fatal("timed out request should be removed from pending map")
	}
}

func TestServerDispatchesStrictServerInterface(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	impl := &testRPCServer{
		resp: &PingResponse{ServerTime: 456},
	}
	server := NewServer(impl)

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.ServeContext(context.WithValue(context.Background(), rpcTestContextKey{}, "ctx-value"), serverSide)
	}()

	if err := WriteRequest(clientSide, &RPCRequest{
		V:      1,
		Id:     "ping-1",
		Method: MethodPing,
		Params: &PingRequest{ClientSendTime: 123},
	}); err != nil {
		t.Fatalf("WriteRequest error: %v", err)
	}

	resp, err := ReadResponse(clientSide)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("response error = %+v", resp.Error)
	}
	if resp.Result == nil || resp.Result.ServerTime != 456 {
		t.Fatalf("response result = %+v", resp.Result)
	}
	if impl.got.ClientSendTime != 123 {
		t.Fatalf("Ping request client_send_time = %d", impl.got.ClientSendTime)
	}
	if got := impl.ctx.Value(rpcTestContextKey{}); got != "ctx-value" {
		t.Fatalf("context value = %v", got)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("ServeContext error: %v", err)
	}
}

func TestServerReturnsErrorForMissingPingParams(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	defer clientSide.Close()

	server := NewServer(&testRPCServer{resp: &PingResponse{ServerTime: 1}})

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Serve(clientSide)
	}()

	if err := WriteRequest(serverSide, &RPCRequest{
		V:      1,
		Id:     "ping-1",
		Method: MethodPing,
	}); err != nil {
		t.Fatalf("WriteRequest error: %v", err)
	}

	resp, err := ReadResponse(serverSide)
	if err != nil {
		t.Fatalf("ReadResponse error: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("error code = %d", resp.Error.Code)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("Serve error: %v", err)
	}
}

func serverTimeForID(id string) int64 {
	if id == "req-1" {
		return 1
	}
	return 2
}

func assertPingRequestHasTimestamp(t *testing.T, req *RPCRequest) {
	t.Helper()
	if req.Params == nil {
		t.Fatal("ping request params missing")
	}
	if req.Params.ClientSendTime <= 0 {
		t.Fatalf("ping request client_send_time = %d", req.Params.ClientSendTime)
	}
}

type testRPCServer struct {
	ctx  context.Context
	got  PingRequest
	resp *PingResponse
}

func (s *testRPCServer) Ping(ctx context.Context, request PingRequest) (*PingResponse, error) {
	s.ctx = ctx
	s.got = request
	return s.resp, nil
}

type rpcTestContextKey struct{}
