package scenarios

import (
	"fmt"
	"net"
	"testing"
)

// StreamOpenFunc opens a new stream on client side.
type StreamOpenFunc func() (net.Conn, error)

// StreamAcceptFunc accepts one incoming stream on server side.
type StreamAcceptFunc func() (net.Conn, error)

// BenchmarkStreamOpenClose benchmarks stream open/close overhead.
func BenchmarkStreamOpenClose(b *testing.B, open StreamOpenFunc, accept StreamAcceptFunc) {
	b.Helper()

	errCh := make(chan error, 1)
	go func() {
		for i := 0; i < b.N; i++ {
			s, err := accept()
			if err != nil {
				errCh <- fmt.Errorf("accept[%d]: %w", i, err)
				return
			}
			_ = s.Close()
		}
		errCh <- nil
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s, err := open()
		if err != nil {
			b.Fatalf("open[%d]: %v", i, err)
		}
		_ = s.Close()
	}
	b.StopTimer()

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
}
