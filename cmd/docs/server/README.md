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

`gizclaw serve <dir>` 的心智模型就是：把 `<dir>` 当成服务端工作目录来启动。启动时默认读取：

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
└── firmware/
```

说明：

- `config.yaml`：服务端配置文件
- `identity.key`：服务端身份密钥，不存在时自动生成
- `firmware/`：默认固件目录；只有在 `depots.store` 没显式改到别处时才会用到

准备好 `config.yaml` 之后，直接启动：

```bash
gizclaw serve ~/data
```

如果你要通过 CLI 连接服务端做设备管理或固件发布，请看 `../client/README.md` 中的 `admin` 部分。