package netbench

import (
	"fmt"
	"testing"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/adapters"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
	"github.com/giztoy/giztoy-go/benchmark/net/internal/scenarios"
	"github.com/giztoy/giztoy-go/pkg/net/noise"
)

const maxNoiseDatagramPayload = 4096

// BenchmarkNet_Noise_Handshake benchmarks full IK handshake + session split.
func BenchmarkNet_Noise_Handshake(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for i := 0; i < b.N; i++ {
		initiatorStatic, err := noise.GenerateKeyPair()
		if err != nil {
			b.Fatalf("generate initiator key: %v", err)
		}
		responderStatic, err := noise.GenerateKeyPair()
		if err != nil {
			b.Fatalf("generate responder key: %v", err)
		}

		initiator, err := noise.NewHandshakeState(noise.Config{
			Pattern:      noise.PatternIK,
			Initiator:    true,
			LocalStatic:  initiatorStatic,
			RemoteStatic: &responderStatic.Public,
		})
		if err != nil {
			b.Fatalf("new initiator hs: %v", err)
		}
		responder, err := noise.NewHandshakeState(noise.Config{
			Pattern:     noise.PatternIK,
			Initiator:   false,
			LocalStatic: responderStatic,
		})
		if err != nil {
			b.Fatalf("new responder hs: %v", err)
		}

		msg1, err := initiator.WriteMessage(nil)
		if err != nil {
			b.Fatalf("initiator write msg1: %v", err)
		}
		if _, err := responder.ReadMessage(msg1); err != nil {
			b.Fatalf("responder read msg1: %v", err)
		}

		msg2, err := responder.WriteMessage(nil)
		if err != nil {
			b.Fatalf("responder write msg2: %v", err)
		}
		if _, err := initiator.ReadMessage(msg2); err != nil {
			b.Fatalf("initiator read msg2: %v", err)
		}

		if _, _, err := initiator.Split(); err != nil {
			b.Fatalf("initiator split: %v", err)
		}
		if _, _, err := responder.Split(); err != nil {
			b.Fatalf("responder split: %v", err)
		}
	}
}

// BenchmarkNet_Noise_TransportThroughput benchmarks Noise datagram throughput.
func BenchmarkNet_Noise_TransportThroughput(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				if size > maxNoiseDatagramPayload {
					b.Skipf("raw noise datagram payload > %d may hit MTU/EMSGSIZE", maxNoiseDatagramPayload)
				}
				pair, err := adapters.NewNoisePair(loss)
				if err != nil {
					b.Fatalf("new noise pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				if loss == 0 {
					scenarios.BenchmarkDatagramOneWayThroughputStrict(
						b,
						payload,
						pair.SendAToB,
						pair.RecvOnBBlocking,
					)
				} else {
					scenarios.BenchmarkDatagramOneWayThroughput(
						b,
						payload,
						pair.SendAToB,
						pair.RecvOnB,
					)
				}
				framework.ReportDropMetric(b, pair.Link.LossAB())
			})
		}
	}
}

// BenchmarkNet_Noise_RTT benchmarks Noise datagram ping-pong RTT.
func BenchmarkNet_Noise_RTT(b *testing.B) {
	b.ReportAllocs()
	requireScaleMode(b)
	for _, size := range framework.PayloadSizes() {
		payload := framework.Payload(size)
		for _, loss := range framework.LossRates() {
			name := fmt.Sprintf("size=%d/loss=%s", size, framework.LossLabel(loss))
			b.Run(name, func(b *testing.B) {
				if size > maxNoiseDatagramPayload {
					b.Skipf("raw noise datagram payload > %d may hit MTU/EMSGSIZE", maxNoiseDatagramPayload)
				}
				if loss > 0 {
					b.Skip("raw datagram RTT under loss is non-deterministic")
				}
				pair, err := adapters.NewNoisePair(loss)
				if err != nil {
					b.Fatalf("new noise pair failed: %v", err)
				}
				defer func() { _ = pair.Close() }()

				scenarios.BenchmarkDatagramPingPongRTT(
					b,
					payload,
					pair.SendAToB,
					pair.RecvOnA,
					pair.SendBToA,
					pair.RecvOnB,
				)
			})
		}
	}
}
