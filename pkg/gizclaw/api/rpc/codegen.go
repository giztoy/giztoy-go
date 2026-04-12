package rpc

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config=codegen_config.yaml -o generated.go ../../../../api/rpc_types.json

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const MaxFrameSize = 1 << 20 // 1 MiB

const MethodPing = "peer.ping"

type Client struct {
	rw io.ReadWriter

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan callResult

	closeOnce sync.Once
	closedCh  chan struct{}
	closedErr error
}

type Server struct {
	ssi StrictServerInterface
}

type StrictServerInterface interface {
	// Ping remote peer and get server time.
	Ping(ctx context.Context, request PingRequest) (*PingResponse, error)
}

type callResult struct {
	resp *RPCResponse
	err  error
}

var ErrDuplicateRequestID = errors.New("rpc: duplicate request id")
var ErrClientClosed = errors.New("rpc: client closed")

func NewClient(rw io.ReadWriter) *Client {
	c := &Client{
		rw:       rw,
		pending:  make(map[string]chan callResult),
		closedCh: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func NewServer(ssi StrictServerInterface) *Server {
	return &Server{ssi: ssi}
}

func WriteFrame(w io.Writer, data []byte) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(hdr[:])
	if length > MaxFrameSize {
		return nil, fmt.Errorf("rpc: frame too large: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func WriteRequest(w io.Writer, req *RPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return WriteFrame(w, data)
}

func ReadRequest(r io.Reader) (*RPCRequest, error) {
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

func ReadResponse(r io.Reader) (*RPCResponse, error) {
	data, err := ReadFrame(r)
	if err != nil {
		return nil, err
	}
	var resp RPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("rpc: unmarshal response: %w", err)
	}
	return &resp, nil
}

func WriteResponse(w io.Writer, resp *RPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return WriteFrame(w, data)
}

func ErrorResponse(id string, code int, message string) *RPCResponse {
	return &RPCResponse{
		V:  1,
		Id: id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
}

func ResultResponse(id string, result *PingResponse) *RPCResponse {
	return &RPCResponse{
		V:      1,
		Id:     id,
		Result: result,
	}
}

func (c *Client) call(ctx context.Context, req *RPCRequest) (*RPCResponse, error) {
	if req == nil {
		return nil, errors.New("rpc: nil request")
	}
	if req.Id == "" {
		return nil, errors.New("rpc: request id required")
	}

	resultCh := make(chan callResult, 1)

	c.pendingMu.Lock()
	if c.pending == nil {
		c.pendingMu.Unlock()
		return nil, ErrClientClosed
	}
	if _, exists := c.pending[req.Id]; exists {
		c.pendingMu.Unlock()
		return nil, ErrDuplicateRequestID
	}
	c.pending[req.Id] = resultCh
	c.pendingMu.Unlock()

	c.writeMu.Lock()
	err := WriteRequest(c.rw, req)
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(req.Id)
		return nil, err
	}

	select {
	case result := <-resultCh:
		return result.resp, result.err
	case <-ctx.Done():
		c.removePending(req.Id)
		return nil, ctx.Err()
	case <-c.closedCh:
		c.removePending(req.Id)
		if c.closedErr != nil {
			return nil, c.closedErr
		}
		return nil, ErrClientClosed
	}
}

func (c *Client) Ping(ctx context.Context, id string) (*PingResponse, error) {
	resp, err := c.call(ctx, &RPCRequest{
		V:      1,
		Id:     id,
		Method: MethodPing,
		Params: &PingRequest{ClientSendTime: time.Now().UnixMilli()},
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc: %s", resp.Error.Message)
	}
	if resp.Result == nil {
		return nil, fmt.Errorf("rpc: missing ping result")
	}
	return resp.Result, nil
}

func (c *Client) Close() error {
	c.shutdown(ErrClientClosed)
	if closer, ok := c.rw.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (c *Client) readLoop() {
	for {
		resp, err := ReadResponse(c.rw)
		if err != nil {
			c.shutdown(err)
			return
		}
		if resp == nil || resp.Id == "" {
			continue
		}

		c.pendingMu.Lock()
		ch, ok := c.pending[resp.Id]
		if ok {
			delete(c.pending, resp.Id)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- callResult{resp: resp}
		}
	}
}

func (c *Client) removePending(id string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.pending == nil {
		return
	}
	delete(c.pending, id)
}

func (c *Client) shutdown(err error) {
	c.closeOnce.Do(func() {
		if err == nil {
			err = ErrClientClosed
		}
		c.closedErr = err

		c.pendingMu.Lock()
		pending := c.pending
		c.pending = nil
		c.pendingMu.Unlock()

		close(c.closedCh)

		for id, ch := range pending {
			delete(pending, id)
			ch <- callResult{err: err}
		}
	})
}

func (s *Server) Serve(rw io.ReadWriter) error {
	return s.ServeContext(context.Background(), rw)
}

func (s *Server) ServeContext(ctx context.Context, rw io.ReadWriter) error {
	if s == nil || s.ssi == nil {
		return errors.New("rpc: nil server implementation")
	}

	req, err := ReadRequest(rw)
	if err != nil {
		return err
	}

	resp, err := s.dispatch(ctx, req)
	if err != nil {
		return err
	}

	if resp == nil {
		resp = &RPCResponse{V: 1, Id: req.Id}
	}
	if resp.Id == "" {
		resp.Id = req.Id
	}
	if resp.V == 0 {
		resp.V = 1
	}
	return WriteResponse(rw, resp)
}

func (s *Server) dispatch(ctx context.Context, req *RPCRequest) (*RPCResponse, error) {
	switch req.Method {
	case MethodPing:
		if req.Params == nil {
			return ErrorResponse(req.Id, -32602, "missing params"), nil
		}

		response, err := s.ssi.Ping(ctx, *req.Params)
		if err != nil {
			return nil, err
		}
		return ResultResponse(req.Id, response), nil
	default:
		return ErrorResponse(req.Id, -1, fmt.Sprintf("unknown method: %s", req.Method)), nil
	}
}
