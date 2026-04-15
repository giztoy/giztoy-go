# Client Commands

这一组文档覆盖本地客户端、设备侧命令，以及管理员通过客户端连接服务端执行的控制面命令：

- `gizclaw context`
- `gizclaw ping`
- `gizclaw admin`
- `gizclaw play`

## `gizclaw context`

`context` 用来管理“连接哪个服务端”的本地上下文。每个 context 会保存：

- 一个自动生成的设备身份密钥 `identity.key`
- 一个服务端连接配置 `config.yaml`

默认存储目录：

- Linux/macOS: `$XDG_CONFIG_HOME/gizclaw` 或 `~/.config/gizclaw`
- Windows: `%AppData%/gizclaw`

常用子命令：

- `gizclaw context create <name> --server <host:port> --pubkey <hex>`
- `gizclaw context use <name>`
- `gizclaw context list`

示例：

```bash
gizclaw context create dev \
  --server 127.0.0.1:9820 \
  --pubkey <server-public-key-hex>

gizclaw context use dev
gizclaw context list
```

行为说明：

- 第一次 `create` 时会生成当前 context 的身份密钥
- 如果还没有当前 context，第一次创建会自动成为当前 context
- `list` 会用 `*` 标出当前正在使用的 context

## `gizclaw ping`

`ping` 走 peer 层连接服务端，用来检查连通性、往返时延和时钟偏差。

常用形式：

- `gizclaw ping`
- `gizclaw ping --context <name>`

输出包含：

- `Server Time`
- `RTT`
- `Clock Diff`

示例：

```bash
gizclaw ping
gizclaw ping --context dev
```

注意：

- 不传 `--context` 时，默认读取当前 context
- 如果没有当前 context，会提示先运行 `gizclaw context create`

## `gizclaw admin`

`admin` 是控制面命令集合，但它本质上仍然是客户端命令：由本地 CLI 通过当前 context 连接到远端服务端，再调用管理接口。

如果没有当前 context，先执行：

```bash
gizclaw context create admin \
  --server 127.0.0.1:9820 \
  --pubkey <server-public-key-hex>
gizclaw context use admin
```

所有 `admin` 子命令都支持：

```bash
--context <name>
```

用于覆盖默认的当前 context。

### `gizclaw admin gears`

用于设备注册状态、配置和运行态管理。

常用命令：

- `gizclaw admin gears list`
- `gizclaw admin gears get <pubkey>`
- `gizclaw admin gears approve <pubkey> <role>`
- `gizclaw admin gears block <pubkey>`
- `gizclaw admin gears delete <pubkey>`
- `gizclaw admin gears refresh <pubkey>`

查询类命令：

- `gizclaw admin gears info <pubkey>`
- `gizclaw admin gears config <pubkey>`
- `gizclaw admin gears runtime <pubkey>`
- `gizclaw admin gears ota <pubkey>`

索引检索命令：

- `gizclaw admin gears resolve-sn <sn>`
- `gizclaw admin gears resolve-imei <tac> <serial>`
- `gizclaw admin gears list-by-label <key> <value>`
- `gizclaw admin gears list-by-certification <type> <authority> <id>`
- `gizclaw admin gears list-by-firmware <depot> <channel>`

配置命令：

- `gizclaw admin gears put-config <pubkey> <channel>`

说明：

- `approve` 会把待注册设备切到指定角色
- `block` 会把设备标记为禁用
- `delete` 会清掉设备注册信息
- `refresh` 会从设备侧反向 API 拉取 `info`、`identifiers`、`version`
- `put-config` 当前主要用于更新设备目标固件 channel

示例：

```bash
gizclaw admin gears list
gizclaw admin gears approve <pubkey> device
gizclaw admin gears put-config <pubkey> stable
gizclaw admin gears refresh <pubkey>
```

### `gizclaw admin firmware`

用于固件仓库、channel 和发布流转管理。

常用命令：

- `gizclaw admin firmware list`
- `gizclaw admin firmware get <depot>`
- `gizclaw admin firmware get-channel <depot> <channel>`
- `gizclaw admin firmware put-info <depot> --file <info.json>`
- `gizclaw admin firmware upload <depot> <channel> --file <release.tar>`
- `gizclaw admin firmware rollback <depot>`
- `gizclaw admin firmware release <depot>`

说明：

- `put-info` 写入 depot 的 `info.json`
- `upload` 上传一个 release tar 包到指定 channel
- `release` 按 `testing -> beta -> stable -> rollback` 的顺序推进版本
- `rollback` 把 `rollback` channel 提升回 `stable`

示例：

```bash
gizclaw admin firmware put-info demo/main --file ./info.json
gizclaw admin firmware upload demo/main testing --file ./release.tar
gizclaw admin firmware release demo/main
gizclaw admin firmware get-channel demo/main stable
```

## `gizclaw play`

`play` 是设备侧命令集合，适合模拟或驱动一个设备和服务端交互。

### `gizclaw play register`

向服务端发起设备注册。

常用参数：

- `--context <name>`
- `--token <registration-token>`
- `--name <device-name>`
- `--sn <serial-number>`
- `--manufacturer <vendor>`
- `--model <model>`
- `--hardware-revision <rev>`
- `--depot <firmware-depot>`
- `--firmware-semver <semver>`

示例：

```bash
gizclaw play register \
  --token device_default \
  --name demo-device \
  --sn sn-001 \
  --manufacturer Acme \
  --model M1 \
  --depot demo/main \
  --firmware-semver 1.0.0
```

### `gizclaw play config`

读取当前设备在服务端侧看到的配置快照。

```bash
gizclaw play config
```

### `gizclaw play ota`

读取当前设备的 OTA 摘要，通常用于判断是否有可升级固件。

```bash
gizclaw play ota
```

### `gizclaw play serve`

在当前连接上启动一个设备侧的反向 HTTP provider，供服务端执行 `refresh` 时访问。

常用参数：

- `--name`
- `--manufacturer`
- `--model`
- `--hardware-revision`
- `--sn`
- `--depot`
- `--firmware-semver`

示例：

```bash
gizclaw play serve \
  --name demo-device \
  --manufacturer Acme \
  --model M1 \
  --sn sn-001 \
  --depot demo/main \
  --firmware-semver 1.0.0
```

适用场景：

- 本地模拟一个设备
- 给 `gizclaw admin gears refresh` 提供设备信息来源
- 联调注册、配置、OTA、refresh 全链路

