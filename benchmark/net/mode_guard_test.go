package netbench

import (
	"testing"

	"github.com/giztoy/giztoy-go/benchmark/net/internal/framework"
)

func requireScaleMode(b *testing.B) {
	b.Helper()
	if !framework.ManualScale() {
		return
	}
	if !manualTagEnabled || !framework.ManualMode() {
		b.Skipf("scale=%s requires manual mode: set BENCH_NET_MANUAL=1 and run with -tags manual", framework.BenchmarkScale())
	}
}
