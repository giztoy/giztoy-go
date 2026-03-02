# pkg/audio/codec/ogg

OGG container package for giztoy-go audio pipeline.

## Features

- OGG page encode/decode（含 CRC 校验）
- 逻辑包与物理页互转（支持跨页 continuation）
- 流式读写：`StreamWriter` / `StreamReader`

## Build/link strategy

- 在支持 cgo 的平台（`linux|darwin` + `amd64|arm64`）下，构建会默认从仓内
  prebuilt 路径引入 libogg 头文件与静态库：
  - `third_party/audio/prebuilt/libogg/<platform>/include`
  - `third_party/audio/prebuilt/libogg/<platform>/lib/libogg.a`
- 不依赖 `pkg-config`。

## Quick test

```bash
go test ./pkg/audio/codec/ogg -v
```
