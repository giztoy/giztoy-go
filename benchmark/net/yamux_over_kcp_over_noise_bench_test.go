package netbench

import (
	"fmt"
	"net"
	"testing"

	"github.com/haivivi/giztoy/go/benchmark/net/internal/adapters"
	"github.com/haivivi/giztoy/go/benchmark/net/internal/framework"
	"github.com/haivivi/giztoy/go/benchmark/net/internal/scenarios"
)

// BenchmarkNet_YamuxOverKCPOverNoise_StreamOpenClose benchmarks stream lifecycle cost.
func BenchmarkNet_YamuxOverKCPOverNoise_StreamOpenClose(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, loss := range framework.LossRates() {
		name := fmt.Sprintf("loss=%s", framework.LossLabel(loss))
		b.Run(name, func(b *testing.B) {
			pair, err := adapters.NewYamuxKCPNoisePair(loss)
			if err != nil {
				b.Fatalf("new yamux-kcp-noise pair failed: %v", err)
			}
			defer func() { _ = pair.Close() }()

			scenarios.BenchmarkStreamOpenClose(
				b,
				func() (net.Conn, error) {
					return pair.Client.OpenStream(0)
				},
				func() (net.Conn, error) {
					c, _, err := pair.Server.AcceptStream()
					return c, err
				},
			)
			framework.ReportDropMetric(b, pair.Link.LossAB())
		})
	}
}

// BenchmarkNet_YamuxOverKCPOverNoise_AggregateThroughput benchmarks aggregate
// throughput across multiple concurrent streams.
func BenchmarkNet_YamuxOverKCPOverNoise_AggregateThroughput(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, parallel := range framework.ParallelStreams() {
			for _, loss := range framework.LossRates() {
				name := fmt.Sprintf("size=%d/parallel=%d/loss=%s", size, parallel, framework.LossLabel(loss))
				b.Run(name, func(b *testing.B) {
					pair, err := adapters.NewYamuxKCPNoisePair(loss)
					if err != nil {
						b.Fatalf("new yamux-kcp-noise pair failed: %v", err)
					}
					defer func() { _ = pair.Close() }()

					scenarios.BenchmarkStreamAggregateThroughput(
						b,
						payload,
						parallel,
						func() (net.Conn, error) {
							return pair.Client.OpenStream(0)
						},
						func() (net.Conn, error) {
							c, _, err := pair.Server.AcceptStream()
							return c, err
						},
					)
					framework.ReportDropMetric(b, pair.Link.LossAB())
				})
			}
		}
	}
}

// BenchmarkNet_YamuxOverKCPOverNoise_RPCStyle benchmarks RPC-like req/resp
// multiplexing on multiple streams.
func BenchmarkNet_YamuxOverKCPOverNoise_RPCStyle(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		req := framework.Payload(size)
		for _, parallel := range framework.ParallelStreams() {
			for _, loss := range framework.LossRates() {
				name := fmt.Sprintf("size=%d/parallel=%d/loss=%s", size, parallel, framework.LossLabel(loss))
				b.Run(name, func(b *testing.B) {
					pair, err := adapters.NewYamuxKCPNoisePair(loss)
					if err != nil {
						b.Fatalf("new yamux-kcp-noise pair failed: %v", err)
					}
					defer func() { _ = pair.Close() }()

					scenarios.BenchmarkRPCStyleMultiplexing(
						b,
						req,
						parallel,
						func() (net.Conn, error) {
							return pair.Client.OpenStream(0)
						},
						func() (net.Conn, error) {
							c, _, err := pair.Server.AcceptStream()
							return c, err
						},
					)
					framework.ReportDropMetric(b, pair.Link.LossAB())
				})
			}
		}
	}
}

// BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput benchmarks
// composite concurrency:
// - multiple KCP services in parallel
// - multiple yamux streams per KCP service
//
// Level L means: L KCP services × L yamux streams per service.
func BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	payload := framework.Payload(1024)
	maxTotal := framework.MaxTotalStreams()
	kcpLevels := framework.KCPServiceLevels()
	yamuxLevels := framework.YamuxPerKCPLevels()

	for _, kcpServices := range kcpLevels {
		for _, yamuxPerKCP := range yamuxLevels {
			totalStreams := kcpServices * yamuxPerKCP
			for _, loss := range framework.LossRates() {
				name := fmt.Sprintf("kcp=%d/yamux=%d/total=%d/loss=%s", kcpServices, yamuxPerKCP, totalStreams, framework.LossLabel(loss))
				b.Run(name, func(b *testing.B) {
					if totalStreams > maxTotal {
						b.Skipf("total streams=%d exceeds BENCH_NET_MAX_TOTAL_STREAMS=%d", totalStreams, maxTotal)
					}

					pair, err := adapters.NewYamuxKCPNoisePair(loss)
					if err != nil {
						b.Fatalf("new yamux-kcp-noise pair failed: %v", err)
					}
					defer func() { _ = pair.Close() }()

					scenarios.BenchmarkServiceCompositeAggregateThroughput(
						b,
						payload,
						kcpServices,
						yamuxPerKCP,
						func(service uint64) (net.Conn, error) {
							return pair.Client.OpenStream(service)
						},
						func() (net.Conn, uint64, error) {
							return pair.Server.AcceptStream()
						},
					)
					framework.ReportDropMetric(b, pair.Link.LossAB())
				})
			}
		}
	}
}
