package scenarios

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
)

// BenchmarkConnOneWayThroughput benchmarks one-way reliable stream throughput.
func BenchmarkConnOneWayThroughput(b *testing.B, payload []byte, writer io.Writer, reader io.Reader) {
	b.Helper()

	errCh := make(chan error, 1)
	go func() {
		buf := make([]byte, len(payload))
		for i := 0; i < b.N; i++ {
			if _, err := io.ReadFull(reader, buf); err != nil {
				errCh <- fmt.Errorf("recv[%d] failed: %w", i, err)
				return
			}
			if !bytes.Equal(buf, payload) {
				errCh <- fmt.Errorf("recv[%d] payload mismatch", i)
				return
			}
		}
		errCh <- nil
	}()

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := writer.Write(payload); err != nil {
			b.Fatalf("send[%d] failed: %v", i, err)
		}
	}

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
}

// DatagramSendFunc sends one datagram payload.
type DatagramSendFunc func([]byte) error

// DatagramRecvFunc receives one datagram payload with timeout.
type DatagramRecvFunc func(timeout time.Duration) ([]byte, error)

// BenchmarkDatagramOneWayThroughput benchmarks one-way datagram throughput.
//
// For lossy environments, this benchmark is best-effort: dropped packets do not
// fail the benchmark. Delivery ratio is reported as an extra metric.
func BenchmarkDatagramOneWayThroughput(b *testing.B, payload []byte, send DatagramSendFunc, recv DatagramRecvFunc) {
	b.Helper()

	var recvCount atomic.Uint64
	stop := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		for {
			select {
			case <-stop:
				errCh <- nil
				return
			default:
			}

			pkt, err := recv(20 * time.Millisecond)
			if err != nil {
				continue
			}
			if len(pkt) != len(payload) {
				errCh <- fmt.Errorf("recv payload len=%d, want=%d", len(pkt), len(payload))
				return
			}
			recvCount.Add(1)
		}
	}()

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := send(payload); err != nil {
			b.Fatalf("send[%d] failed: %v", i, err)
		}
	}

	// Allow in-flight datagrams to arrive before stopping receiver loop.
	target := uint64(b.N)
	deadline := time.Now().Add(2 * time.Second)
	var (
		last   = recvCount.Load()
		stable int
	)
	for time.Now().Before(deadline) {
		cur := recvCount.Load()
		if cur >= target {
			break
		}
		if cur == last {
			stable++
		} else {
			stable = 0
			last = cur
		}
		if stable >= 5 && cur > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(stop)
	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
	b.StopTimer()

	deliveryPct := 0.0
	if b.N > 0 {
		deliveryPct = float64(recvCount.Load()) * 100.0 / float64(b.N)
	}
	b.ReportMetric(deliveryPct, "delivery_pct")
	framework.ReportThroughputMBps(b, recvCount.Load()*uint64(len(payload)), b.Elapsed())
}

// DatagramRecvBlockingFunc receives one datagram payload without timeout control.
type DatagramRecvBlockingFunc func() ([]byte, error)

// BenchmarkDatagramOneWayThroughputStrict benchmarks one-way datagram throughput
// for loss-free paths where every packet must be delivered.
func BenchmarkDatagramOneWayThroughputStrict(
	b *testing.B,
	payload []byte,
	send DatagramSendFunc,
	recv DatagramRecvBlockingFunc,
) {
	b.Helper()

	errCh := make(chan error, 1)
	go func() {
		for i := 0; i < b.N; i++ {
			pkt, err := recv()
			if err != nil {
				errCh <- fmt.Errorf("recv[%d] failed: %w", i, err)
				return
			}
			if !bytes.Equal(pkt, payload) {
				errCh <- fmt.Errorf("recv[%d] payload mismatch", i)
				return
			}
		}
		errCh <- nil
	}()

	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := send(payload); err != nil {
			b.Fatalf("send[%d] failed: %v", i, err)
		}
	}

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
	b.StopTimer()

	b.ReportMetric(100.0, "delivery_pct")
	framework.ReportThroughputMBps(b, uint64(b.N*len(payload)), b.Elapsed())
}
