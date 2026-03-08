package scenarios

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ServiceStreamOpenFunc opens a stream on a specific service (KCP lane).
type ServiceStreamOpenFunc func(service uint64) (net.Conn, error)

// ServiceStreamAcceptFunc accepts one incoming stream on a specific service.
type ServiceStreamAcceptFunc func(service uint64) (net.Conn, error)

// BenchmarkServiceCompositeAggregateThroughput benchmarks aggregate throughput
// under composite concurrency:
//
//	totalStreams = kcpServices * yamuxPerService
//
// i.e. multiple KCP services in parallel, with multiple yamux streams per service.
func BenchmarkServiceCompositeAggregateThroughput(
	b *testing.B,
	payload []byte,
	kcpServices int,
	yamuxPerService int,
	open ServiceStreamOpenFunc,
	accept ServiceStreamAcceptFunc,
) {
	b.Helper()
	if kcpServices <= 0 || yamuxPerService <= 0 {
		b.Fatalf("kcpServices=%d yamuxPerService=%d must be > 0", kcpServices, yamuxPerService)
	}

	totalStreams := kcpServices * yamuxPerService
	clientStreams := make([]net.Conn, 0, totalStreams)
	serverStreams := make([]net.Conn, 0, totalStreams)

	for svc := 0; svc < kcpServices; svc++ {
		for st := 0; st < yamuxPerService; st++ {
			c, err := open(uint64(svc))
			if err != nil {
				b.Fatalf("open service=%d stream=%d failed: %v", svc, st, err)
			}
			s, err := accept(uint64(svc))
			if err != nil {
				_ = c.Close()
				b.Fatalf("accept service=%d stream=%d failed: %v", svc, st, err)
			}
			clientStreams = append(clientStreams, c)
			serverStreams = append(serverStreams, s)
		}
	}
	defer func() {
		for _, s := range clientStreams {
			_ = s.Close()
		}
		for _, s := range serverStreams {
			_ = s.Close()
		}
	}()

	var recvOps atomic.Int64
	errCh := make(chan error, totalStreams)
	var wg sync.WaitGroup
	for i := 0; i < totalStreams; i++ {
		wg.Add(1)
		go func(streamIdx int, s net.Conn) {
			defer wg.Done()
			buf := make([]byte, len(payload))
			for {
				if _, err := io.ReadFull(s, buf); err != nil {
					if err == io.EOF || err == io.ErrUnexpectedEOF {
						return
					}
					errCh <- fmt.Errorf("server stream[%d] read: %w", streamIdx, err)
					return
				}
				recvOps.Add(1)
			}
		}(i, serverStreams[i])
	}

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := clientStreams[i%totalStreams]
		if _, err := s.Write(payload); err != nil {
			b.Fatalf("write[%d] failed: %v", i, err)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	for recvOps.Load() < int64(b.N) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := recvOps.Load(); got < int64(b.N) {
		b.Fatalf("delivered ops=%d, want >= %d", got, b.N)
	}
	b.StopTimer()

	for _, s := range clientStreams {
		_ = s.Close()
	}
	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		b.Fatal(err)
	}

	b.ReportMetric(float64(kcpServices), "kcp_services")
	b.ReportMetric(float64(yamuxPerService), "yamux_per_kcp")
	b.ReportMetric(float64(totalStreams), "total_streams")
}
