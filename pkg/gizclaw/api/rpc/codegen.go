package rpc

//go:generate go tool oapi-codegen -config=codegen_config.yaml -o generated.go ../../../../api/rpc_types.json

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const MaxFrameSize = 1 << 20 // 1 MiB

const MethodPing = "peer.ping"

type Client struct {
	rw io.ReadWriter

	callMu    sync.Mutex
	closeOnce sync.Once
	closed    atomic.Bool
}

type Server struct {
	ssi StrictServerInterface
}

type StrictServerInterface interface {
	// Ping remote peer and get server time.
	Ping(ctx context.Context, request PingRequest) (*PingResponse, error)
}

var ErrClientClosed = errors.New("rpc: client closed")

func NewClient(rw io.ReadWriter) *Client {
	return &Client{rw: rw}
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
	if c.closed.Load() {
		return nil, ErrClientClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.callMu.Lock()
	defer c.callMu.Unlock()
	if c.closed.Load() {
		return nil, ErrClientClosed
	}

	var deadline time.Time
	var hasDeadline bool
	if deadline, hasDeadline = ctx.Deadline(); hasDeadline {
		// handled below together with cancel-triggered deadline updates
	}
	if conn, ok := c.rw.(interface{ SetDeadline(time.Time) error }); ok {
		if hasDeadline {
			if err := conn.SetDeadline(deadline); err != nil {
				return nil, err
			}
		}
		stopCancel := make(chan struct{})
		defer func() {
			close(stopCancel)
			_ = conn.SetDeadline(time.Time{})
		}()
		go func() {
			select {
			case <-ctx.Done():
				_ = conn.SetDeadline(time.Now())
			case <-stopCancel:
			}
		}()
	}

	if err := WriteRequest(c.rw, req); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if hasDeadline && time.Now().After(deadline) {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	}

	resp, err := ReadResponse(c.rw)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if hasDeadline && time.Now().After(deadline) {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	}
	return resp, nil
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
	var closeErr error
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if closer, ok := c.rw.(io.Closer); ok {
			closeErr = closer.Close()
		}
	})
	return closeErr
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
