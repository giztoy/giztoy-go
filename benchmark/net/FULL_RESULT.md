# Net Benchmark Report (20260302-103320)

## 1) Test Conditions

- Script: `./benchmark/run.sh`
- Scale: `full`
- Bench filter: `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$`
- Benchtime: `100ms`
- Count: `1`
- Manual mode: `1`
- Go tags: `manual`
- System-level netem: `0` (1=enabled, 0=disabled)
- UDP base ports: `41000` and `41001`

| Profile | loss | delay | internal-loss |
|---|---:|---:|---:|
| clean | 0% | 0ms | 0.00% |
| wifi | 1% | 20ms | 1.00% |
| mobile | 5% | 60ms | 5.00% |
| bad | 10% | 120ms | 10.00% |

## 2) Machine Info

- Run timestamp: 2026-03-02 10:35:12 +0800
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
| R01 | 48.23 | 21230 | 0 | - | 42 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=0.0pct` |
| R02 | 48.36 | 21177 | 0 | - | 42 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=0.0pct` |
| R03 | 50.37 | 20331 | 0 | - | 44 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=0.0pct` |

### Profile: wifi

| Key | MB/s | ns/op | drop% | delivery% | allocs/op | Details |
|---|---:|---:|---:|---:|---:|---|
| R01 | 36.34 | 28180 | 0.9748 | - | 40 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=1.0pct` |
| R02 | 34.54 | 29644 | 1.003 | - | 42 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=1.0pct` |
| R03 | 28.01 | 36565 | 0.9615 | - | 46 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=1.0pct` |

### Profile: mobile

| Key | MB/s | ns/op | drop% | delivery% | allocs/op | Details |
|---|---:|---:|---:|---:|---:|---|
| R01 | 2.26 | 453366 | 5.380 | - | 45 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=5.0pct` |
| R02 | 26.38 | 38815 | 5.004 | - | 43 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=5.0pct` |
| R03 | 14.32 | 71519 | 5.013 | - | 51 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=5.0pct` |

### Profile: bad

| Key | MB/s | ns/op | drop% | delivery% | allocs/op | Details |
|---|---:|---:|---:|---:|---:|---|
| R01 | 5.13 | 199552 | 10.31 | - | 43 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=10.0pct` |
| R02 | 4.09 | 250225 | 10.23 | - | 48 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=10.0pct` |
| R03 | 0.16 | 6247526 | 10.30 | - | 486 | `BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=10.0pct` |

## 4) Raw Outputs

### Profile: clean

<details>
<summary>Expand raw output: clean</summary>

```text
# profile=clean
# scale=full
# manual=1 tags=manual
# system-netem=0 loss=0% delay=0ms
# internal-loss=0 base-port=41000
# bench=BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$ benchtime=100ms count=1
goos: darwin
goarch: arm64
pkg: github.com/vibing/giztoy-go/benchmark/net
cpu: Apple M4 Max
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=0.0pct-16         	    5524	     21230 ns/op	  48.23 MB/s	         0 drop_pct	        10.00 kcp_services	       100.0 total_streams	        10.00 yamux_per_kcp	   16061 B/op	      42 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=0.0pct-16       	    5425	     21177 ns/op	  48.36 MB/s	         0 drop_pct	        10.00 kcp_services	      1000 total_streams	       100.0 yamux_per_kcp	   15776 B/op	      42 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=0.0pct-16       	    5877	     20331 ns/op	  50.37 MB/s	         0 drop_pct	       100.0 kcp_services	      1000 total_streams	        10.00 yamux_per_kcp	   15365 B/op	      44 allocs/op
PASS
ok  	github.com/vibing/giztoy-go/benchmark/net	3.481s
```
</details>

### Profile: wifi

<details>
<summary>Expand raw output: wifi</summary>

```text
# profile=wifi
# scale=full
# manual=1 tags=manual
# system-netem=0 loss=1% delay=20ms
# internal-loss=0.01 base-port=41000
# bench=BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$ benchtime=100ms count=1
goos: darwin
goarch: arm64
pkg: github.com/vibing/giztoy-go/benchmark/net
cpu: Apple M4 Max
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=1.0pct-16         	    3879	     28180 ns/op	  36.34 MB/s	         0.9748 drop_pct	        10.00 kcp_services	       100.0 total_streams	        10.00 yamux_per_kcp	   18470 B/op	      40 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=1.0pct-16       	    4560	     29644 ns/op	  34.54 MB/s	         1.003 drop_pct	        10.00 kcp_services	      1000 total_streams	       100.0 yamux_per_kcp	   17352 B/op	      42 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=1.0pct-16       	    3072	     36565 ns/op	  28.01 MB/s	         0.9615 drop_pct	       100.0 kcp_services	      1000 total_streams	        10.00 yamux_per_kcp	   16970 B/op	      46 allocs/op
PASS
ok  	github.com/vibing/giztoy-go/benchmark/net	7.124s
```
</details>

### Profile: mobile

<details>
<summary>Expand raw output: mobile</summary>

```text
# profile=mobile
# scale=full
# manual=1 tags=manual
# system-netem=0 loss=5% delay=60ms
# internal-loss=0.05 base-port=41000
# bench=BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$ benchtime=100ms count=1
goos: darwin
goarch: arm64
pkg: github.com/vibing/giztoy-go/benchmark/net
cpu: Apple M4 Max
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=5.0pct-16         	     339	    453366 ns/op	   2.26 MB/s	         5.380 drop_pct	        10.00 kcp_services	       100.0 total_streams	        10.00 yamux_per_kcp	   23719 B/op	      45 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=5.0pct-16       	    2666	     38815 ns/op	  26.38 MB/s	         5.004 drop_pct	        10.00 kcp_services	      1000 total_streams	       100.0 yamux_per_kcp	   19376 B/op	      43 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=5.0pct-16       	    1453	     71519 ns/op	  14.32 MB/s	         5.013 drop_pct	       100.0 kcp_services	      1000 total_streams	        10.00 yamux_per_kcp	   19505 B/op	      51 allocs/op
PASS
ok  	github.com/vibing/giztoy-go/benchmark/net	46.255s
```
</details>

### Profile: bad

<details>
<summary>Expand raw output: bad</summary>

```text
# profile=bad
# scale=full
# manual=1 tags=manual
# system-netem=0 loss=10% delay=120ms
# internal-loss=0.10 base-port=41000
# bench=BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput$ benchtime=100ms count=1
goos: darwin
goarch: arm64
pkg: github.com/vibing/giztoy-go/benchmark/net
cpu: Apple M4 Max
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=10/total=100/loss=10.0pct-16         	     560	    199552 ns/op	   5.13 MB/s	        10.31 drop_pct	        10.00 kcp_services	       100.0 total_streams	        10.00 yamux_per_kcp	   20594 B/op	      43 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=10/yamux=100/total=1000/loss=10.0pct-16       	     572	    250225 ns/op	   4.09 MB/s	        10.23 drop_pct	        10.00 kcp_services	      1000 total_streams	       100.0 yamux_per_kcp	   23577 B/op	      48 allocs/op
BenchmarkNet_YamuxOverKCPOverNoise_MultiKCPAggregateThroughput/kcp=100/yamux=10/total=1000/loss=10.0pct-16       	      22	   6247526 ns/op	   0.16 MB/s	        10.30 drop_pct	       100.0 kcp_services	      1000 total_streams	        10.00 yamux_per_kcp	   91984 B/op	     486 allocs/op
PASS
ok  	github.com/vibing/giztoy-go/benchmark/net	52.182s
```
</details>

