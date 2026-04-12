# Net Benchmark Guide

This directory contains benchmark suites for `pkg/giznet` protocol stacks:

1. `KCP` (raw KCP over UDP)
2. `Noise` transport
3. `KCP over Noise`

---

## Directory Layout

```text
pkg/giznet/internal/benchmark/
├── *_bench_test.go
└── internal/
    ├── framework/   # UDP harness, matrix config, metrics
    ├── adapters/    # Protocol stack adapters
    └── scenarios/   # Reusable benchmark scenarios
```

---

## Run Benchmarks

```bash
# Run all benchmarks directly
go test ./pkg/giznet/internal/benchmark -run '^$' -bench . -benchmem

# Run one family only
go test ./pkg/giznet/internal/benchmark -run '^$' -bench 'BenchmarkNet_KCP_' -benchmem
```

---

## Packet-Loss Testing Modes

### A) In-process loss model (fast and reproducible)

```bash
BENCH_NET_LOSS_RATES=0,0.01,0.05 go test ./pkg/giznet/internal/benchmark -run '^$' -bench 'BenchmarkNet_KCPOverNoise_Throughput' -benchmem
```

Notes:

- `BENCH_NET_LOSS_RATES` accepts either decimals (`0.01`) or percentages (`1`).
- The in-process model uses deterministic pseudo-random drops for repeatability.

### B) System-level netem (recommended for realistic stress)

Use `pkg/giznet/internal/benchmark/run.sh` with system traffic shaping:

- Linux: `tc netem`
- macOS: `pfctl + dnctl`

```bash
./pkg/giznet/internal/benchmark/run.sh --profile wifi --scale smoke
```

> Recommendation: when system-level netem is enabled, keep `BENCH_NET_LOSS_RATES=0` to avoid double-loss modeling.

---

## Fixed Profile Runner

`run.sh` supports fixed profiles: `clean / wifi / mobile / bad / all`.

```bash
# Quick smoke validation
./pkg/giznet/internal/benchmark/run.sh --profile clean --scale smoke --no-system-netem

# Full matrix run and publish to FULL_RESULT.md (manual mode)
./pkg/giznet/internal/benchmark/run.sh --profile all --scale full --manual --publish-result

# Smoke matrix run and publish to SMOKE_RESULT.md
./pkg/giznet/internal/benchmark/run.sh --profile all --scale smoke --no-system-netem --publish-result
```

Important:

- `smoke` can be used in normal CI/local default flow.
- `full` requires manual mode: `--manual` (which uses `-tags manual` and `BENCH_NET_MANUAL=1`).
- Minimal smoke datapath checks are implemented as regular tests (`smoke_test.go`) and run under `go test ./...`.

---

## Key Parameters


| Variable                      | Meaning                                | Default                           |
| ----------------------------- | -------------------------------------- | --------------------------------- |
| `BENCH_NET_SCALE`             | Matrix scale (`smoke`, `full`)         | `smoke`                           |
| `BENCH_NET_PAYLOAD_SIZES`     | Payload sizes in bytes                 | smoke:`1024`, full:`64,1024,4096` |
| `BENCH_NET_LOSS_RATES`        | In-process loss rates                  | `0`                               |
| `BENCH_NET_PARALLEL_STREAMS`  | Stream parallelism                     | smoke:`1,8`, full:`1,8,32`        |
| `BENCH_NET_MAX_TOTAL_STREAMS` | Safety cap for total composite streams | `100000`                          |
| `BENCH_NET_UDP_BASE_PORT`     | UDP base port (A=base, B=base+1)       | `41000`                           |


---

## Result Files

Published benchmark reports default to files under `pkg/giznet/internal/benchmark/`, and report tables use a short `Key` column (`R01`, `R02`, ...), while full benchmark names are placed in the final `Details` column for readability.