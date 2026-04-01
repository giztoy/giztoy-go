package netbench

import (
	"fmt"
	"testing"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/adapters"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/scenarios"
)

// BenchmarkNet_KCP_Throughput benchmarks raw KCP throughput over UDP.
func BenchmarkNet_KCP_Throughput(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				pair, err := adapters.NewKCPPair(loss)
				if err != nil {
					b.Fatalf("new kcp pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				scenarios.BenchmarkConnOneWayThroughput(b, payload, pair.A, pair.B)
				framework.ReportDropMetric(b, pair.Link.LossAB())
			})
		}
	}
}

// BenchmarkNet_KCP_RTT benchmarks raw KCP ping-pong RTT.
func BenchmarkNet_KCP_RTT(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				pair, err := adapters.NewKCPPair(loss)
				if err != nil {
					b.Fatalf("new kcp pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				scenarios.BenchmarkConnPingPongRTT(b, payload, pair.A, pair.B)
				framework.ReportDropMetric(b, pair.Link.LossAB())
			})
		}
	}
}
