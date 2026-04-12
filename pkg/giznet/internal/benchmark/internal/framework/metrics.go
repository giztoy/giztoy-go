package framework

import (
	"testing"
	"time"
)

// ReportDropMetric reports simulated packet drop ratio for a benchmark.
func ReportDropMetric(b *testing.B, model *LossModel) {
	if model == nil {
		return
	}
	attempt := model.Attempt()
	if attempt == 0 {
		b.ReportMetric(0, "drop_pct")
		return
	}
	dropPct := float64(model.Dropped()) * 100.0 / float64(attempt)
	b.ReportMetric(dropPct, "drop_pct")
}

// ReportThroughputMBps reports MB/s by bytes and duration.
func ReportThroughputMBps(b *testing.B, bytes uint64, dur time.Duration) {
	if dur <= 0 {
		return
	}
	mbps := float64(bytes) / dur.Seconds() / (1024 * 1024)
	b.ReportMetric(mbps, "MB/s")
}
