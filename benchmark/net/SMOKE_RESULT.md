# Net Benchmark Report (20260302-100259)

## 1) Test Conditions

- Script: `./benchmark/run.sh`
- Scale: `smoke`
- Bench filter: `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$`
- Benchtime: `100ms`
- Count: `1`
- Manual mode: `0`
- Go tags: `<none>`
- System-level netem: `0` (1=enabled, 0=disabled)
- UDP base ports: `41000` and `41001`

| Profile | loss | delay | internal-loss |
|---|---:|---:|---:|
| clean | 0% | 0ms | 0.00% |

## 2) Machine Info

- Run timestamp: 2026-03-02 10:03:01 +0800
- Go version: go version go1.26.0 darwin/arm64
- OS: Darwin 25.3.0
- Arch: arm64
- macOS version: 26.3
- Model: Mac16,5
- CPU: Apple M4 Max
- Logical cores: 16
- Memory bytes: 68719476736

## 3) Benchmark Summary

Key column intentionally uses short IDs (Rxx). Full benchmark case is in the last **Details** column.

### Profile: clean

| Key | MB/s | ns/op | drop% | delivery% | allocs/op | Details |
|---|---:|---:|---:|---:|---:|---|
| R01 | 55.69 | 18389 | - | - | 41 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100` |

## 4) Raw Outputs

### Profile: clean

<details>
<summary>Expand raw output: clean</summary>

```text
# profile=clean
# scale=smoke
# manual=0 tags=<none>
# system-netem=0 loss=0% delay=0ms
# internal-loss=0 base-port=41000
# bench=BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$ benchtime=100ms count=1
goos: darwin
goarch: arm64
pkg: github.com/vibing/giztoy-go/benchmark/net
cpu: Apple M4 Max
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100-16         	    5877	     18389 ns/op	  55.69 MB/s	        10.00 kcp_services	       100.0 total_streams	        10.00 yamux_per_kcp	   16366 B/op	      41 allocs/op
PASS
ok  	github.com/vibing/giztoy-go/benchmark/net	1.410s
```
</details>

