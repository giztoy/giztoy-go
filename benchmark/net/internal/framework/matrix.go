package framework

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	envPayloadSizes = "BENCH_NET_PAYLOAD_SIZES"
	envLossRates    = "BENCH_NET_LOSS_RATES"
	envParallels    = "BENCH_NET_PARALLEL_STREAMS"
	envScale        = "BENCH_NET_SCALE"
	envMaxTotal     = "BENCH_NET_MAX_TOTAL_STREAMS"
)

const (
	// ScaleSmoke is intended for quick sanity runs.
	ScaleSmoke = "smoke"
	// ScaleFull is intended for complete benchmark matrix runs.
	ScaleFull = "full"
)

const envManual = "BENCH_NET_MANUAL"

// BenchmarkScale returns benchmark scale mode from env:
// - smoke: tiny matrix for fast sanity runs
// - full:  default complete matrix
// Invalid/empty values fallback to smoke.
func BenchmarkScale() string {
	s := strings.ToLower(strings.TrimSpace(os.Getenv(envScale)))
	switch s {
	case ScaleSmoke, ScaleFull:
		return s
	default:
		return ScaleSmoke
	}
}

// ManualMode returns true when manual benchmark mode is enabled.
func ManualMode() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envManual)))
	return v == "1" || v == "true" || v == "yes"
}

// ManualScale returns true when current scale requires manual mode.
func ManualScale() bool {
	s := BenchmarkScale()
	return s == ScaleFull
}

// PayloadSizes returns benchmark payload sizes (bytes).
//
// Env override example:
//
//	BENCH_NET_PAYLOAD_SIZES=64,1024,4096
func PayloadSizes() []int {
	defaults := []int{64, 1024, 4096}
	switch BenchmarkScale() {
	case ScaleSmoke:
		defaults = []int{1024}
	}
	return parseIntList(envPayloadSizes, defaults)
}

// LossRates returns benchmark loss rates in [0,1].
//
// Env override examples:
//
//	BENCH_NET_LOSS_RATES=0,0.01,0.05
//	BENCH_NET_LOSS_RATES=0,1,5      # interpreted as percent
func LossRates() []float64 {
	vals := parseFloatList(envLossRates, []float64{0})
	out := make([]float64, 0, len(vals))
	for _, v := range vals {
		if v > 1 {
			v = v / 100.0
		}
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		out = append(out, v)
	}
	return uniqueFloatSorted(out)
}

// ParallelStreams returns parallel stream counts for multiplexing benchmarks.
//
// Env override example:
//
//	BENCH_NET_PARALLEL_STREAMS=1,8,32
func ParallelStreams() []int {
	defaults := []int{1, 8, 32}
	switch BenchmarkScale() {
	case ScaleSmoke:
		defaults = []int{1, 8}
	}
	return parseIntList(envParallels, defaults)
}

// MaxTotalStreams returns the safety limit for total opened streams.
//
// Env override example:
//
//	BENCH_NET_MAX_TOTAL_STREAMS=1000000
func MaxTotalStreams() int {
	raw := strings.TrimSpace(os.Getenv(envMaxTotal))
	if raw == "" {
		return 100000
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 100000
	}
	return v
}

// LossLabel returns a short label (e.g. "1.0pct").
func LossLabel(rate float64) string {
	return fmt.Sprintf("%.1fpct", rate*100)
}

func parseIntList(env string, defaults []int) []int {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return append([]int(nil), defaults...)
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v <= 0 {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return append([]int(nil), defaults...)
	}
	sort.Ints(out)
	return uniqueIntSorted(out)
}

func parseFloatList(env string, defaults []float64) []float64 {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return append([]float64(nil), defaults...)
	}
	parts := strings.Split(raw, ",")
	out := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return append([]float64(nil), defaults...)
	}
	sort.Float64s(out)
	return out
}

func uniqueIntSorted(in []int) []int {
	if len(in) == 0 {
		return in
	}
	out := make([]int, 0, len(in))
	last := in[0] - 1
	for _, v := range in {
		if v == last {
			continue
		}
		out = append(out, v)
		last = v
	}
	return out
}

func uniqueFloatSorted(in []float64) []float64 {
	if len(in) == 0 {
		return in
	}
	out := make([]float64, 0, len(in))
	const eps = 1e-9
	last := in[0] - 1
	for _, v := range in {
		if len(out) > 0 {
			if d := v - last; d < eps && d > -eps {
				continue
			}
		}
		out = append(out, v)
		last = v
	}
	return out
}
