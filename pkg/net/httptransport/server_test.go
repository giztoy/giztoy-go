package httptransport

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/integration/testutil"
	"github.com/giztoy/giztoy-go/pkg/net/core"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
	"github.com/giztoy/giztoy-go/pkg/net/peer"
)

func TestHTTPTransportRoundTrip(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := testutil.NewListenerNode(t, serverKey, core.WithServiceMuxConfig(core.ServiceMuxConfig{
		OnNewService: func(_ noise.PublicKey, service uint64) bool {
			return service == 7
		},
	}))
	defer serverListener.Close()
	clientListener := testutil.NewListenerNode(t, clientKey)
	defer clientListener.Close()
	testutil.ConnectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(serverConn, 7, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte("echo:" + string(payload)))
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	go func() {
		_ = srv.Serve()
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://giztoy/echo", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(clientConn, 7).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "echo:hello" {
		t.Fatalf("body = %q", body)
	}
	if resp.Header.Get("X-Test") != "ok" {
		t.Fatalf("X-Test header = %q", resp.Header.Get("X-Test"))
	}
}

func TestListenerCloseUnblocksAccept(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := testutil.NewListenerNode(t, serverKey)
	defer serverListener.Close()
	clientListener := testutil.NewListenerNode(t, clientKey)
	defer clientListener.Close()
	testutil.ConnectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}

	l := NewListener(serverConn, 9)
	if l.Addr().Network() != "kcp-http" {
		t.Fatalf("Addr().Network() = %q", l.Addr().Network())
	}
	done := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	if err := l.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	select {
	case err := <-done:
		if !IsClosed(err) && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not unblock after Close")
	}
}

func TestAdminServiceFirstRequestCanArriveBeforeServeStarts(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t,
		core.ServiceMuxConfig{},
		core.ServiceMuxConfig{
			OnNewService: func(_ noise.PublicKey, service uint64) bool {
				return service == peer.ServicePublic || service == peer.ServiceAdmin || service == peer.ServiceReverse
			},
		},
	)
	defer cleanup()

	srv := NewServer(serverConn, 1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("admin-ready"))
	}))
	serveDone := make(chan error, 1)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown error: %v", err)
		}
		if err := <-serveDone; err != nil {
			t.Fatalf("Serve error: %v", err)
		}
	}()

	respDone := make(chan struct {
		body string
		err  error
	}, 1)
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://giztoy/admin", nil)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		resp, err := NewClient(clientConn, 1).Do(req)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		respDone <- struct {
			body string
			err  error
		}{body: string(body), err: err}
	}()

	time.Sleep(100 * time.Millisecond)
	go func() {
		serveDone <- srv.Serve()
	}()

	select {
	case result := <-respDone:
		if result.err != nil {
			t.Fatalf("first admin request error: %v", result.err)
		}
		if result.body != "admin-ready" {
			t.Fatalf("body = %q", result.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first admin request timed out")
	}
}

func TestReverseServiceFirstRequestCanArriveBeforeServeStarts(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t,
		core.ServiceMuxConfig{
			OnNewService: func(_ noise.PublicKey, service uint64) bool {
				return service == peer.ServicePublic || service == peer.ServiceReverse
			},
		},
		core.ServiceMuxConfig{},
	)
	defer cleanup()

	srv := NewServer(clientConn, 2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("reverse-ready"))
	}))
	serveDone := make(chan error, 1)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown error: %v", err)
		}
		if err := <-serveDone; err != nil {
			t.Fatalf("Serve error: %v", err)
		}
	}()

	respDone := make(chan struct {
		body string
		err  error
	}, 1)
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://giztoy/reverse", nil)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		resp, err := NewClient(serverConn, 2).Do(req)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		respDone <- struct {
			body string
			err  error
		}{body: string(body), err: err}
	}()

	time.Sleep(100 * time.Millisecond)
	go func() {
		serveDone <- srv.Serve()
	}()

	select {
	case result := <-respDone:
		if result.err != nil {
			t.Fatalf("first reverse request error: %v", result.err)
		}
		if result.body != "reverse-ready" {
			t.Fatalf("body = %q", result.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first reverse request timed out")
	}
}

func TestServerShutdownDrainsActiveRequest(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t,
		core.ServiceMuxConfig{},
		core.ServiceMuxConfig{
			OnNewService: func(_ noise.PublicKey, service uint64) bool {
				return service == 7
			},
		},
	)
	defer cleanup()

	started := make(chan struct{})
	release := make(chan struct{})
	srv := NewServer(serverConn, 7, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Header().Set("Content-Length", "7")
		_, _ = w.Write([]byte("drained"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(10 * time.Millisecond)
	}))
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve()
	}()

	respDone := make(chan struct {
		body string
		err  error
	}, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://giztoy/drain", nil)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		resp, err := NewClient(clientConn, 7).Do(req)
		if err != nil {
			respDone <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		respDone <- struct {
			body string
			err  error
		}{body: string(body), err: err}
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not start")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- srv.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown returned before active request drained: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(release)

	select {
	case result := <-respDone:
		if result.err != nil {
			t.Fatalf("request error: %v", result.err)
		}
		if result.body != "drained" {
			t.Fatalf("body = %q", result.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not finish after release")
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not finish")
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after Shutdown")
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://giztoy/after-shutdown", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(clientConn, 7).Do(req); err == nil {
		t.Fatal("request after Shutdown should fail")
	}
}

func TestRoundTripReadResponseError(t *testing.T) {
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	serverListener := testutil.NewListenerNode(t, serverKey)
	defer serverListener.Close()
	clientListener := testutil.NewListenerNode(t, clientKey)
	defer clientListener.Close()
	testutil.ConnectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		stream, err := serverConn.AcceptService(11)
		if err == nil {
			_ = stream.Close()
		}
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://giztoy/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewClient(clientConn, 11).Do(req); err == nil {
		t.Fatal("RoundTrip should fail when response is not readable")
	}
}

func TestRoundTripHandlesEarlyResponse(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t, core.ServiceMuxConfig{}, core.ServiceMuxConfig{})
	defer cleanup()

	responseStarted := make(chan struct{})
	go func() {
		stream, err := serverConn.AcceptService(13)
		if err != nil {
			return
		}
		defer stream.Close()

		buf := make([]byte, 1)
		window := make([]byte, 0, 4)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				window = append(window, buf[:n]...)
				if len(window) > 4 {
					window = window[len(window)-4:]
				}
				if string(window) == "\r\n\r\n" {
					break
				}
			}
			if err != nil {
				return
			}
		}
		resp := "HTTP/1.1 401 Unauthorized\r\nContent-Length: 6\r\n\r\ndenied"
		close(responseStarted)
		for i := 0; i < len(resp); i++ {
			if _, err := io.WriteString(stream, resp[i:i+1]); err != nil {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	bodyReader, bodyWriter := io.Pipe()
	go func() {
		<-responseStarted
		_ = bodyWriter.CloseWithError(errors.New("client body aborted after early response"))
	}()
	defer func() { _ = bodyWriter.Close() }()

	respDone := make(chan struct {
		status int
		body   string
		err    error
	}, 1)
	go func() {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://giztoy/upload", bodyReader)
		if err != nil {
			respDone <- struct {
				status int
				body   string
				err    error
			}{err: err}
			return
		}
		resp, err := NewClient(clientConn, 13).Do(req)
		if err != nil {
			respDone <- struct {
				status int
				body   string
				err    error
			}{err: err}
			return
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		respDone <- struct {
			status int
			body   string
			err    error
		}{status: resp.StatusCode, body: string(data), err: err}
	}()

	select {
	case <-responseStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("early response did not start")
	}

	select {
	case result := <-respDone:
		if result.err != nil {
			t.Fatalf("early response error: %v", result.err)
		}
		if result.status != http.StatusUnauthorized {
			t.Fatalf("status = %d", result.status)
		}
		if result.body != "denied" {
			t.Fatalf("body = %q", result.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("early response request timed out")
	}
}

func TestRoundTripCloseDoesNotBlockOnStreamingBody(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t, core.ServiceMuxConfig{}, core.ServiceMuxConfig{})
	defer cleanup()

	go func() {
		stream, err := serverConn.AcceptService(14)
		if err != nil {
			return
		}
		defer stream.Close()

		buf := make([]byte, 1)
		window := make([]byte, 0, 4)
		for {
			n, err := stream.Read(buf)
			if n > 0 {
				window = append(window, buf[:n]...)
				if len(window) > 4 {
					window = window[len(window)-4:]
				}
				if string(window) == "\r\n\r\n" {
					break
				}
			}
			if err != nil {
				return
			}
		}
		_, _ = io.WriteString(stream, "HTTP/1.1 401 Unauthorized\r\nContent-Length: 6\r\n\r\ndenied")
		time.Sleep(10 * time.Millisecond)
	}()

	body := &blockingBody{release: make(chan struct{})}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://giztoy/upload", body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(clientConn, 14).Do(req)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- resp.Body.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resp.Body.Close blocked on streaming request body")
	}
	body.releaseRead()
}

func TestRoundTripHonorsRequestContextCancel(t *testing.T) {
	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t, core.ServiceMuxConfig{}, core.ServiceMuxConfig{})
	defer cleanup()

	go func() {
		stream, err := serverConn.AcceptService(15)
		if err != nil {
			return
		}
		defer stream.Close()
		buf := make([]byte, 1)
		_, _ = stream.Read(buf)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://giztoy/hang", nil)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	if _, err := NewClient(clientConn, 15).Do(req); err == nil {
		t.Fatal("request should fail after context cancellation")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("request cancellation took too long")
	}
}

func newHTTPTransportConnPair(t *testing.T, clientCfg, serverCfg core.ServiceMuxConfig) (*peer.Conn, *peer.Conn, func()) {
	t.Helper()

	clientKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := noise.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	clientListener := testutil.NewListenerNode(t, clientKey, core.WithServiceMuxConfig(clientCfg))
	serverListener := testutil.NewListenerNode(t, serverKey, core.WithServiceMuxConfig(serverCfg))
	testutil.ConnectListenerNodes(t, clientListener, clientKey, serverListener, serverKey)

	clientConn, err := clientListener.Peer(serverKey.Public)
	if err != nil {
		t.Fatalf("client Peer error: %v", err)
	}
	serverConn, err := serverListener.Peer(clientKey.Public)
	if err != nil {
		t.Fatalf("server Peer error: %v", err)
	}

	cleanup := func() {
		_ = clientListener.Close()
		_ = serverListener.Close()
	}
	return clientConn, serverConn, cleanup
}

type blockingBody struct {
	release chan struct{}
	once    sync.Once
}

func (b *blockingBody) Read(_ []byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

func (b *blockingBody) Close() error { return nil }

func (b *blockingBody) releaseRead() {
	b.once.Do(func() {
		close(b.release)
	})
}

func TestCustomServiceIDRoundTrip(t *testing.T) {
	const customService uint64 = 42

	clientConn, serverConn, cleanup := newHTTPTransportConnPair(t,
		core.ServiceMuxConfig{},
		core.ServiceMuxConfig{
			OnNewService: func(_ noise.PublicKey, service uint64) bool {
				return service == customService
			},
		},
	)
	defer cleanup()

	srv := NewServer(serverConn, customService, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("custom-ok"))
	}))
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.Serve() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-serveDone
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://giztoy/custom", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewClient(clientConn, customService).Do(req)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "custom-ok" {
		t.Fatalf("body = %q, want %q", body, "custom-ok")
	}
}
