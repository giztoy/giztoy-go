# tools/ncnn

Build, package, and verify NCNN prebuilt artifacts for supported targets.

## Scripts

- `build_prebuilt_linux.sh`
  - Downloads and builds upstream NCNN source on Linux.
  - Produces `include/ncnn/*.h` and `lib/libncnn.a` under `.tmp/ncnn-prebuilt/<platform>/`.
  - Uses fixed build flags: `NCNN_VULKAN=OFF`, `NCNN_C_API=ON`.

- `build_prebuilt_darwin.sh`
  - Downloads and builds upstream NCNN source on macOS.
  - Supports `TARGET_ARCH=arm64` and `TARGET_ARCH=amd64`.
  - Produces `include/ncnn/*.h` and `lib/libncnn.a` under `.tmp/ncnn-prebuilt/<platform>/`.

- `package_prebuilt.sh`
  - Copies build outputs into `third_party/ncnn/prebuilt/<platform>/`.
  - Generates `manifest.json` with SHA-256 checksums for all headers and `libncnn.a`.

- `verify_artifacts.sh`
  - Validates required headers, static library, manifest, and embedded model files.
  - Verifies all artifact checksums recorded in `manifest.json`.
  - Detects accidental Git LFS pointer files for model `.bin` and `libncnn.a`.

## Usage

```bash
# Linux amd64 build + package + verify
tools/ncnn/build_prebuilt_linux.sh
tools/ncnn/package_prebuilt.sh linux-amd64
tools/ncnn/verify_artifacts.sh linux-amd64

# Linux arm64 build + package + verify
TARGET_ARCH=arm64 tools/ncnn/build_prebuilt_linux.sh
tools/ncnn/package_prebuilt.sh linux-arm64
tools/ncnn/verify_artifacts.sh linux-arm64

# macOS arm64 build + package + verify
tools/ncnn/build_prebuilt_darwin.sh
tools/ncnn/package_prebuilt.sh darwin-arm64
tools/ncnn/verify_artifacts.sh darwin-arm64

# macOS amd64 build + package + verify (cross-build from arm64 host is supported)
TARGET_ARCH=amd64 tools/ncnn/build_prebuilt_darwin.sh
tools/ncnn/package_prebuilt.sh darwin-amd64
tools/ncnn/verify_artifacts.sh darwin-amd64
```

## Notes

- Default source version: `20260113`.
- Override source metadata when needed:
  - `NCNN_VERSION=<version>`
  - `NCNN_SHA256=<sha256>`
- `pkg/ncnn` links prebuilt static libraries from `third_party/ncnn/prebuilt/<platform>/lib/libncnn.a`.
