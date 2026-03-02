# tools/audio/ogg

Build, package, and verify libogg prebuilt artifacts for supported targets.

## Scripts

- `build_prebuilt_linux.sh`
  - Builds static `libogg.a` from `third_party/audio/libogg` on Linux.
  - Supports `TARGET_ARCH=amd64|arm64`。
  - **不支持跨架构构建**：脚本会校验 `TARGET_ARCH` 与当前 Linux host 架构一致；
    例如 `TARGET_ARCH=arm64` 需要在 arm64 Linux 主机执行。
  - 输出到 `.tmp/ogg-prebuilt/<platform>/`。

- `build_prebuilt_darwin.sh`
  - Builds static `libogg.a` from `third_party/audio/libogg` on macOS.
  - Supports `TARGET_ARCH=arm64|amd64`（Apple Silicon 可交叉到 `amd64`）。
  - 输出到 `.tmp/ogg-prebuilt/<platform>/`。

- `package_prebuilt.sh`
  - 将 staging 产物复制到 `third_party/audio/prebuilt/libogg/<platform>/`。
  - 生成带 SHA-256 校验的 `manifest.json`。

- `verify_artifacts.sh`
  - 校验必需文件、`manifest.json` 和 checksum 一致性。
  - 检测 `libogg.a` 是否误提交为 Git LFS pointer。

## Usage

```bash
# macOS arm64 build + package + verify
tools/audio/ogg/build_prebuilt_darwin.sh
tools/audio/ogg/package_prebuilt.sh darwin-arm64
tools/audio/ogg/verify_artifacts.sh darwin-arm64

# macOS amd64 build (from arm64 host) + package + verify
TARGET_ARCH=amd64 tools/audio/ogg/build_prebuilt_darwin.sh
tools/audio/ogg/package_prebuilt.sh darwin-amd64
tools/audio/ogg/verify_artifacts.sh darwin-amd64

# Linux amd64 build + package + verify
tools/audio/ogg/build_prebuilt_linux.sh
tools/audio/ogg/package_prebuilt.sh linux-amd64
tools/audio/ogg/verify_artifacts.sh linux-amd64

# Linux arm64 build + package + verify
# (must run on an arm64 Linux host)
TARGET_ARCH=arm64 tools/audio/ogg/build_prebuilt_linux.sh
tools/audio/ogg/package_prebuilt.sh linux-arm64
tools/audio/ogg/verify_artifacts.sh linux-arm64
```

## Notes

- Source of truth 是 `third_party/audio/libogg` submodule revision。
- `pkg/audio/codec/ogg` 在支持平台的 cgo 构建下，默认从
  `third_party/audio/prebuilt/libogg/<platform>/` 链接静态库。
