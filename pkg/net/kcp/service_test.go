package kcp

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// serviceMuxPair creates a connected pair of ServiceMux instances.
func serviceMuxPair() (client, server *ServiceMux) {
	var clientMux, serverMux *ServiceMux

	clientMux = NewServiceMux(ServiceMuxConfig{
		IsClient: true,
		Output: func(service uint64, data []byte) error {
			return serverMux.Input(service, data)
		},
	})
	serverMux = NewServiceMux(ServiceMuxConfig{
		IsClient: false,
		Output: func(service uint64, data []byte) error {
			return clientMux.Input(service, data)
		},
	})

	return clientMux, serverMux
}

func readExactWithTimeout(t *testing.T, r io.Reader, n int, timeout time.Duration) []byte {
	t.Helper()

	errCh := make(chan error, 1)
	buf := make([]byte, n)
	go func() {
		_, err := io.ReadFull(r, buf)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("ReadFull failed: %v", err)
		}
		return buf
	case <-time.After(timeout):
		t.Fatalf("ReadFull timeout after %s", timeout)
		return nil
	}
}

func TestServiceMux_OpenWriteThenAccept(t *testing.T) {
	client, server := serviceMuxPair()
	defer client.Close()
	defer server.Close()

	stream, err := client.OpenStream(1)
	if err != nil {
		t.Fatalf("client OpenStream failed: %v", err)
	}
	defer stream.Close()

	msg := []byte("hello-direct-kcp")
	writeErr := make(chan error, 1)
	go func() {
		_, err := stream.Write(msg)
		writeErr <- err
	}()

	accepted, service, err := server.AcceptStream()
	if err != nil {
		t.Fatalf("server AcceptStream failed: %v", err)
	}
	defer accepted.Close()

	if service != 1 {
		t.Fatalf("accepted service=%d, want 1", service)
	}

	if got := readExactWithTimeout(t, accepted, len(msg), 5*time.Second); !bytes.Equal(got, msg) {
		t.Fatalf("server recv mismatch: got=%q want=%q", got, msg)
	}

	select {
	case err := <-writeErr:
		if err != nil {
			t.Fatalf("client write failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client write did not complete")
	}
}

func TestServiceMux_BidirectionalDataPath(t *testing.T) {
	client, server := serviceMuxPair()
	defer client.Close()
	defer server.Close()

	clientStream, err := client.OpenStream(0)
	if err != nil {
		t.Fatalf("client OpenStream failed: %v", err)
	}
	defer clientStream.Close()

	request := []byte("req")
	if _, err := clientStream.Write(request); err != nil {
		t.Fatalf("client write req failed: %v", err)
	}

	serverStream, svc, err := server.AcceptStream()
	if err != nil {
		t.Fatalf("server AcceptStream failed: %v", err)
	}
	defer serverStream.Close()
	if svc != 0 {
		t.Fatalf("accepted service=%d, want 0", svc)
	}

	if got := readExactWithTimeout(t, serverStream, len(request), 5*time.Second); !bytes.Equal(got, request) {
		t.Fatalf("server recv req mismatch: got=%q want=%q", got, request)
	}

	response := []byte("resp")
	if _, err := serverStream.Write(response); err != nil {
		t.Fatalf("server write resp failed: %v", err)
	}

	if got := readExactWithTimeout(t, clientStream, len(response), 5*time.Second); !bytes.Equal(got, response) {
		t.Fatalf("client recv resp mismatch: got=%q want=%q", got, response)
	}
}

func TestServiceMux_AcceptStreamOn(t *testing.T) {
	client, server := serviceMuxPair()
	defer client.Close()
	defer server.Close()

	const svc uint64 = 7
	clientStream, err := client.OpenStream(svc)
	if err != nil {
		t.Fatalf("client OpenStream failed: %v", err)
	}
	defer clientStream.Close()

	msg := []byte("accept-on-service")
	if _, err := clientStream.Write(msg); err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	serverStream, err := server.AcceptStreamOn(svc)
	if err != nil {
		t.Fatalf("server AcceptStreamOn failed: %v", err)
	}
	defer serverStream.Close()

	if got := readExactWithTimeout(t, serverStream, len(msg), 5*time.Second); !bytes.Equal(got, msg) {
		t.Fatalf("server recv mismatch: got=%q want=%q", got, msg)
	}
}

func TestServiceMux_AcceptStream_NoDuplicateReturn(t *testing.T) {
	client, server := serviceMuxPair()
	defer client.Close()
	defer server.Close()

	stream1, err := client.OpenStream(1)
	if err != nil {
		t.Fatalf("client OpenStream(1) failed: %v", err)
	}
	defer stream1.Close()

	if _, err := stream1.Write([]byte("first")); err != nil {
		t.Fatalf("stream1.Write failed: %v", err)
	}

	accepted1, svc1, err := server.AcceptStream()
	if err != nil {
		t.Fatalf("first AcceptStream failed: %v", err)
	}
	defer accepted1.Close()
	if svc1 != 1 {
		t.Fatalf("first accepted service=%d, want 1", svc1)
	}
	_ = readExactWithTimeout(t, accepted1, len("first"), 5*time.Second)

	accept2Ch := make(chan struct {
		conn    net.Conn
		service uint64
		err     error
	}, 1)
	go func() {
		c, s, err := server.AcceptStream()
		accept2Ch <- struct {
			conn    net.Conn
			service uint64
			err     error
		}{conn: c, service: s, err: err}
	}()

	select {
	case r := <-accept2Ch:
		if r.conn != nil {
			_ = r.conn.Close()
		}
		t.Fatalf("second AcceptStream returned early: service=%d err=%v", r.service, r.err)
	case <-time.After(200 * time.Millisecond):
		// expected: second accept should block until new inbound activity
	}

	stream2, err := client.OpenStream(2)
	if err != nil {
		t.Fatalf("client OpenStream(2) failed: %v", err)
	}
	defer stream2.Close()

	if _, err := stream2.Write([]byte("second")); err != nil {
		t.Fatalf("stream2.Write failed: %v", err)
	}

	select {
	case r := <-accept2Ch:
		if r.err != nil {
			t.Fatalf("second AcceptStream err=%v", r.err)
		}
		defer r.conn.Close()
		if r.service != 2 {
			t.Fatalf("second accepted service=%d, want 2", r.service)
		}
		_ = readExactWithTimeout(t, r.conn, len("second"), 5*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("second AcceptStream did not unblock on new inbound activity")
	}
}

func TestServiceMux_AcceptStreamOn_AnnouncedPathReturnsWrappedConn(t *testing.T) {
	client, server := serviceMuxPair()
	defer client.Close()
	defer server.Close()

	stream, err := client.OpenStream(3)
	if err != nil {
		t.Fatalf("client OpenStream failed: %v", err)
	}
	defer stream.Close()

	first := []byte("hello")
	if _, err := stream.Write(first); err != nil {
		t.Fatalf("stream.Write(first) failed: %v", err)
	}

	accepted, err := server.AcceptStreamOn(3)
	if err != nil {
		t.Fatalf("AcceptStreamOn failed: %v", err)
	}
	if got := readExactWithTimeout(t, accepted, len(first), 5*time.Second); !bytes.Equal(got, first) {
		t.Fatalf("first payload mismatch: got=%q want=%q", got, first)
	}

	// Close() should be no-op on wrapped direct stream.
	if err := accepted.Close(); err != nil {
		t.Fatalf("accepted.Close failed: %v", err)
	}

	second := []byte("world")
	if _, err := stream.Write(second); err != nil {
		t.Fatalf("stream.Write(second) failed: %v", err)
	}
	if got := readExactWithTimeout(t, accepted, len(second), 5*time.Second); !bytes.Equal(got, second) {
		t.Fatalf("second payload mismatch after Close no-op: got=%q want=%q", got, second)
	}
}

func TestServiceMux_RejectService(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{
		OnNewService: func(service uint64) bool {
			return service != 99
		},
	})
	defer mux.Close()

	err := mux.Input(99, []byte{serviceFrameData, 0x01, 0x02, 0x03})
	if !errors.Is(err, ErrServiceRejected) {
		t.Fatalf("Input(rejected service) err=%v, want %v", err, ErrServiceRejected)
	}
}

func TestServiceMux_OutputErrorObservable(t *testing.T) {
	injected := errors.New("injected output error")

	var callbackCount atomic.Uint64
	var callbackService atomic.Uint64

	mux := NewServiceMux(ServiceMuxConfig{
		Output: func(service uint64, data []byte) error {
			_ = service
			_ = data
			return injected
		},
		OnOutputError: func(service uint64, err error) {
			if !errors.Is(err, injected) {
				t.Errorf("OnOutputError err=%v, want injected", err)
			}
			callbackService.Store(service)
			callbackCount.Add(1)
		},
	})
	defer mux.Close()

	stream, err := mux.OpenStream(0)
	if err != nil {
		t.Fatalf("OpenStream failed: %v", err)
	}
	defer stream.Close()

	_ = stream.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	_, _ = stream.Write([]byte("trigger-output"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mux.OutputErrorCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := mux.OutputErrorCount(); got == 0 {
		t.Fatalf("OutputErrorCount=%d, want > 0", got)
	}
	if got := callbackCount.Load(); got == 0 {
		t.Fatalf("OnOutputError callback count=%d, want > 0", got)
	}
	if got := callbackService.Load(); got != 0 {
		t.Fatalf("OnOutputError service=%d, want 0", got)
	}
}

func TestServiceMux_CloseUnblocksAccept(t *testing.T) {
	_, server := serviceMuxPair()

	done := make(chan error, 1)
	go func() {
		_, _, err := server.AcceptStream()
		done <- err
	}()

	time.Sleep(100 * time.Millisecond)
	_ = server.Close()

	select {
	case err := <-done:
		if !errors.Is(err, ErrServiceMuxClosed) {
			t.Fatalf("AcceptStream err=%v, want %v", err, ErrServiceMuxClosed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AcceptStream did not unblock after Close")
	}
}

func TestServiceMux_CloseConcurrentWithInput_NoPanic(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{
		Output: func(service uint64, data []byte) error {
			_ = service
			_ = data
			return nil
		},
	})

	const workers = 8
	const loops = 256

	start := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < loops; i++ {
				_ = mux.Input(uint64(i%2), []byte{0x00, 0x01, 0x02, 0x03})
			}
		}()
	}

	close(start)
	time.Sleep(2 * time.Millisecond)
	_ = mux.Close()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// no panic / no hang
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent Input + Close did not complete in time")
	}
}

func TestServiceMux_AcceptStream_CloseDominatesQueuedResult(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{})

	peerSide, localSide := net.Pipe()
	defer peerSide.Close()
	defer localSide.Close()

	// 预置一个可消费的 accept 结果。
	mux.acceptCh <- acceptResult{conn: peerSide, service: 7}

	if err := mux.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	conn, service, err := mux.AcceptStream()
	if !errors.Is(err, ErrServiceMuxClosed) {
		t.Fatalf("AcceptStream err=%v, want %v", err, ErrServiceMuxClosed)
	}
	if conn != nil {
		t.Fatalf("AcceptStream conn=%v, want nil", conn)
	}
	if service != 0 {
		t.Fatalf("AcceptStream service=%d, want 0", service)
	}
}

func TestServiceMux_AcceptStreamOn_CloseDominatesAnnouncedPath(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{})

	entry, err := mux.getOrCreateService(11)
	if err != nil {
		t.Fatalf("getOrCreateService failed: %v", err)
	}
	defer entry.conn.Close()

	entry.announced.Store(true)

	mux.servicesMu.Lock()
	resultCh := make(chan struct {
		conn net.Conn
		err  error
	}, 1)
	go func() {
		conn, err := mux.AcceptStreamOn(11)
		resultCh <- struct {
			conn net.Conn
			err  error
		}{conn: conn, err: err}
	}()

	time.Sleep(20 * time.Millisecond)
	mux.closed.Store(true)
	close(mux.closeCh)
	mux.servicesMu.Unlock()

	select {
	case r := <-resultCh:
		if !errors.Is(r.err, ErrServiceMuxClosed) {
			t.Fatalf("AcceptStreamOn announced path err=%v, want %v", r.err, ErrServiceMuxClosed)
		}
		if r.conn != nil {
			t.Fatalf("AcceptStreamOn announced path conn=%v, want nil", r.conn)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AcceptStreamOn announced path did not return")
	}
}

func TestServiceMux_AcceptStreamOn_CloseDominatesReadyPath(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{})

	entry, err := mux.getOrCreateService(12)
	if err != nil {
		t.Fatalf("getOrCreateService failed: %v", err)
	}
	defer entry.conn.Close()

	entry.announced.Store(false)
	entry.readyOnce.Do(func() { close(entry.readyCh) })

	mux.servicesMu.Lock()
	resultCh := make(chan struct {
		conn net.Conn
		err  error
	}, 1)
	go func() {
		conn, err := mux.AcceptStreamOn(12)
		resultCh <- struct {
			conn net.Conn
			err  error
		}{conn: conn, err: err}
	}()

	time.Sleep(20 * time.Millisecond)
	mux.closed.Store(true)
	close(mux.closeCh)
	mux.servicesMu.Unlock()

	select {
	case r := <-resultCh:
		if !errors.Is(r.err, ErrServiceMuxClosed) {
			t.Fatalf("AcceptStreamOn ready path err=%v, want %v", r.err, ErrServiceMuxClosed)
		}
		if r.conn != nil {
			t.Fatalf("AcceptStreamOn ready path conn=%v, want nil", r.conn)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("AcceptStreamOn ready path did not return")
	}
}

func TestServiceMux_OpenAfterConnCloseRecreates(t *testing.T) {
	mux := NewServiceMux(ServiceMuxConfig{})
	defer mux.Close()

	first, err := mux.OpenStream(0)
	if err != nil {
		t.Fatalf("first OpenStream failed: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	second, err := mux.OpenStream(0)
	if err != nil {
		t.Fatalf("second OpenStream failed: %v", err)
	}

	if first == second {
		t.Fatal("expected recreated stream instance, got same net.Conn")
	}
}

var _ net.Conn = (*KCPConn)(nil)
