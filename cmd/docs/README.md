# `gizclaw` CLI Docs

`gizclaw` 是项目当前的命令行入口，定义在 `cmd/main.go`，实际命令树来自 `cmd/internal/cli`。

按职责可以分成两类：

- `docs/client/`：面向设备侧、普通客户端，以及 admin 控制面客户端使用的命令
- `docs/server/`：面向服务端进程本身的启动与配置

当前根命令结构如下：

```text
gizclaw
├── serve
├── context
├── ping
├── admin
│   ├── gears
│   └── firmware
└── play
    ├── serve
    ├── register
    ├── config
    └── ota
```

阅读建议：

- 如果你要先连上某个 GizClaw 服务端，从 `docs/client/README.md` 的 `context` 开始
- 如果你要启动一个 GizClaw 服务端，从 `docs/server/README.md` 的 `gizclaw serve <workspace>` 开始
- 如果你要做设备管理或固件发布，看 `docs/client/README.md` 的 `admin` 部分
