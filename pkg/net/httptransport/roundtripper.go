package httptransport

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

type RoundTripper struct {
	conn    *peer.Conn
	service uint64
}

func NewRoundTripper(conn *peer.Conn, service uint64) *RoundTripper {
	return &RoundTripper{conn: conn, service: service}
}

func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	stream, err := rt.conn.OpenService(rt.service)
	if err != nil {
		return nil, fmt.Errorf("httptransport: open service %d: %w", rt.service, err)
	}

	stopCancel := watchRequestContext(req, stream)
	writeReq, bodyCtrl := cloneRequestForWrite(req)
	go func() {
		_ = writeReq.Write(stream)
	}()

	resp, err := http.ReadResponse(bufio.NewReader(stream), req)
	if err != nil {
		stopCancel()
		bodyCtrl.abort()
		_ = stream.Close()
		return nil, fmt.Errorf("httptransport: read response: %w", err)
	}
	bodyCtrl.abort()
	_ = stream.SetWriteDeadline(time.Now())
	resp.Body = &readCloser{
		ReadCloser: resp.Body,
		closeFn: func() error {
			stopCancel()
			bodyCtrl.abort()
			return stream.Close()
		},
	}
	return resp, nil
}

type requestBodyController struct {
	body io.Closer
	once sync.Once
}

func cloneRequestForWrite(req *http.Request) (*http.Request, *requestBodyController) {
	clone := req.Clone(req.Context())
	ctrl := &requestBodyController{}
	if req.Body != nil {
		clone.Body = req.Body
		ctrl.body = req.Body
	}
	return clone, ctrl
}

func (c *requestBodyController) abort() {
	c.once.Do(func() {
		if c.body != nil {
			_ = c.body.Close()
		}
	})
}

func watchRequestContext(req *http.Request, stream net.Conn) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case <-req.Context().Done():
			_ = stream.SetDeadline(time.Now())
			_ = stream.Close()
		case <-done:
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

type readCloser struct {
	io.ReadCloser
	closeFn func() error
}

func (r *readCloser) Close() error {
	err1 := r.ReadCloser.Close()
	err2 := r.closeFn()
	if err1 != nil {
		return err1
	}
	return err2
}
