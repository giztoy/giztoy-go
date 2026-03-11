# Client Commands

这一组文档覆盖本地客户端、设备侧命令，以及管理员通过客户端连接服务端执行的控制面命令：

- `giztoy context`
- `giztoy ping`
- `giztoy admin`
- `giztoy play`

## `giztoy context`

`context` 用来管理“连接哪个服务端”的本地上下文。每个 context 会保存：

- 一个自动生成的设备身份密钥 `identity.key`
- 一个服务端连接配置 `config.yaml`

默认存储目录：

- Linux/macOS: `$XDG_CONFIG_HOME/giztoy` 或 `~/.config/giztoy`
- Windows: `%AppData%/giztoy`

常用子命令：

- `giztoy context create <name> --server <host:port> --pubkey <hex>`
- `giztoy context use <name>`
- `giztoy context list`

示例：

```bash
giztoy context create dev \
  --server 127.0.0.1:9820 \
  --pubkey <server-public-key-hex>

giztoy context use dev
giztoy context list
```

行为说明：

- 第一次 `create` 时会生成当前 context 的身份密钥
- 如果还没有当前 context，第一次创建会自动成为当前 context
- `list` 会用 `*` 标出当前正在使用的 context

## `giztoy ping`

`ping` 走 peer 层连接服务端，用来检查连通性、往返时延和时钟偏差。

常用形式：

- `giztoy ping`
- `giztoy ping --context <name>`

输出包含：

- `Server Time`
- `RTT`
- `Clock Diff`

示例：

```bash
giztoy ping
giztoy ping --context dev
```

注意：

- 不传 `--context` 时，默认读取当前 context
- 如果没有当前 context，会提示先运行 `giztoy context create`

## `giztoy admin`

`admin` 是控制面命令集合，但它本质上仍然是客户端命令：由本地 CLI 通过当前 context 连接到远端服务端，再调用管理接口。

如果没有当前 context，先执行：

```bash
giztoy context create admin \
  --server 127.0.0.1:9820 \
  --pubkey <server-public-key-hex>
giztoy context use admin
```

所有 `admin` 子命令都支持：

```bash
--context <name>
```

用于覆盖默认的当前 context。

### `giztoy admin gears`

用于设备注册状态、配置和运行态管理。

常用命令：

- `giztoy admin gears list`
- `giztoy admin gears get <pubkey>`
- `giztoy admin gears approve <pubkey> <role>`
- `giztoy admin gears block <pubkey>`
- `giztoy admin gears delete <pubkey>`
- `giztoy admin gears refresh <pubkey>`

查询类命令：

- `giztoy admin gears info <pubkey>`
- `giztoy admin gears config <pubkey>`
- `giztoy admin gears runtime <pubkey>`
- `giztoy admin gears ota <pubkey>`

索引检索命令：

- `giztoy admin gears resolve-sn <sn>`
- `giztoy admin gears resolve-imei <tac> <serial>`
- `giztoy admin gears list-by-label <key> <value>`
- `giztoy admin gears list-by-certification <type> <authority> <id>`
- `giztoy admin gears list-by-firmware <depot> <channel>`

配置命令：

- `giztoy admin gears put-config <pubkey> <channel>`

说明：

- `approve` 会把待注册设备切到指定角色
- `block` 会把设备标记为禁用
- `delete` 会清掉设备注册信息
- `refresh` 会从设备侧反向 API 拉取 `info`、`identifiers`、`version`
- `put-config` 当前主要用于更新设备目标固件 channel

示例：

```bash
giztoy admin gears list
giztoy admin gears approve <pubkey> device
giztoy admin gears put-config <pubkey> stable
giztoy admin gears refresh <pubkey>
```

### `giztoy admin firmware`

用于固件仓库、channel 和发布流转管理。

常用命令：

- `giztoy admin firmware list`
- `giztoy admin firmware get <depot>`
- `giztoy admin firmware get-channel <depot> <channel>`
- `giztoy admin firmware put-info <depot> --file <info.json>`
- `giztoy admin firmware upload <depot> <channel> --file <release.tar>`
- `giztoy admin firmware rollback <depot>`
- `giztoy admin firmware release <depot>`

说明：

- `put-info` 写入 depot 的 `info.json`
- `upload` 上传一个 release tar 包到指定 channel
- `release` 按 `testing -> beta -> stable -> rollback` 的顺序推进版本
- `rollback` 把 `rollback` channel 提升回 `stable`

示例：

```bash
giztoy admin firmware put-info demo/main --file ./info.json
giztoy admin firmware upload demo/main testing --file ./release.tar
giztoy admin firmware release demo/main
giztoy admin firmware get-channel demo/main stable
```

## `giztoy play`

`play` 是设备侧命令集合，适合模拟或驱动一个设备和服务端交互。

### `giztoy play register`

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
giztoy play register \
  --token device_default \
  --name demo-device \
  --sn sn-001 \
  --manufacturer Acme \
  --model M1 \
  --depot demo/main \
  --firmware-semver 1.0.0
```

### `giztoy play config`

读取当前设备在服务端侧看到的配置快照。

```bash
giztoy play config
```

### `giztoy play ota`

读取当前设备的 OTA 摘要，通常用于判断是否有可升级固件。

```bash
giztoy play ota
```

### `giztoy play serve`

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
giztoy play serve \
  --name demo-device \
  --manufacturer Acme \
  --model M1 \
  --sn sn-001 \
  --depot demo/main \
  --firmware-semver 1.0.0
```

适用场景：

- 本地模拟一个设备
- 给 `giztoy admin gears refresh` 提供设备信息来源
- 联调注册、配置、OTA、refresh 全链路
