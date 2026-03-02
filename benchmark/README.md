# Benchmark 入口

本目录包含基准测试脚本与场景。

- `run.sh`：跨平台（Linux/macOS）运行入口，可注入系统级丢包/延迟。
- `net/`：网络协议栈 benchmark（KCP / Noise / KCP over Noise / Yamux over KCP over Noise）。

快速开始：

```bash
./benchmark/run.sh --loss 0 --delay 0
```

更多说明见：`benchmark/net/README.md`
