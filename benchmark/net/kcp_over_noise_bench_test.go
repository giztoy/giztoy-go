package netbench

import (
	"fmt"
	"testing"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/adapters"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/scenarios"
)

// BenchmarkNet_KCPOverNoise_Throughput benchmarks reliable transport throughput
// with KCP encapsulated in Noise transport packets.
func BenchmarkNet_KCPOverNoise_Throughput(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				pair, err := adapters.NewKCPNoisePair(loss)
				if err != nil {
					b.Fatalf("new kcp-noise pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				scenarios.BenchmarkConnOneWayThroughput(b, payload, pair.A, pair.B)
				framework.ReportDropMetric(b, pair.Link.LossAB())
			})
		}
	}
}

// BenchmarkNet_KCPOverNoise_RTT benchmarks ping-pong RTT for KCP-over-Noise.
func BenchmarkNet_KCPOverNoise_RTT(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				pair, err := adapters.NewKCPNoisePair(loss)
				if err != nil {
					b.Fatalf("new kcp-noise pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				scenarios.BenchmarkConnPingPongRTT(b, payload, pair.A, pair.B)
				framework.ReportDropMetric(b, pair.Link.LossAB())
			})
		}
	}
}
