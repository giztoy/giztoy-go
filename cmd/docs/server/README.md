# Server Commands

## `gizclaw serve`

服务端启动方式只有一种：

```bash
gizclaw serve <dir>
```

例如：

```bash
gizclaw serve ~/data
gizclaw serve ./workspace/server-a
```

`gizclaw serve <dir>` 的心智模型就是：把 `<dir>` 当成服务端工作目录来启动。它总是在前台运行。启动时默认读取：

```text
./config.yaml
```

配置字段说明已经单独放到这里：

- `cmd/docs/server/config.yaml`
- [config.yaml](./config.yaml)

推荐直接照着这份带注释的示例来写你自己的 `<dir>/config.yaml`。

其中 `config.yaml` 里出现的路径字段，如果写相对路径，就按这个工作目录解析；如果写绝对路径，就直接使用该绝对路径。

一个最小的工作目录通常长这样：

```text
<dir>/
├── config.yaml
├── identity.key
├── serve.pid
└── firmware/
```

说明：

- `config.yaml`：服务端配置文件
- `identity.key`：服务端身份密钥，不存在时自动生成
- `serve.pid`：当前服务进程 pid；前台运行和 system service 启动都会用它做互斥检查
- `firmware/`：默认固件目录；只有在 `depots.store` 没显式改到别处时才会用到

准备好 `config.yaml` 之后，直接启动：

```bash
gizclaw serve ~/data
```

行为约定：

- 如果 `serve.pid` 已存在，启动会报错
- `-f`：先终止旧进程，再启动新的进程

如果你更希望交给系统服务管理器，而不是直接手动运行 `serve`，也可以使用：

```bash
gizclaw service install <workspace>
gizclaw service status
gizclaw service start
gizclaw service stop
gizclaw service restart
gizclaw service uninstall
```

当前 `service` 子命令通过系统 service manager 托管，实际启动的仍然是：

```bash
gizclaw serve <workspace>
```

约定：

- `install <workspace>` 只负责安装服务定义
- 如果重复 `install`，会报错并提示先执行 `gizclaw service uninstall`
- `status` / `start` / `stop` / `restart` 不需要再传 workspace
- `uninstall` 会先自动 `stop`，再删除已安装的服务定义

如果你要通过 CLI 连接服务端做设备管理或固件发布，请看 `../client/README.md` 中的 `admin` 部分。