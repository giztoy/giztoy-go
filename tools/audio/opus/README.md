# tools/audio/opus

Build, package, and verify libopus prebuilt artifacts for supported targets.

## Scripts

- `build_prebuilt_linux.sh`
  - Builds static `libopus.a` from `third_party/audio/libopus` on Linux.
  - Supports `TARGET_ARCH=amd64|arm64` (native build expected).
  - Installs outputs into `.tmp/opus-prebuilt/<platform>/`.

- `build_prebuilt_darwin.sh`
  - Builds static `libopus.a` from `third_party/audio/libopus` on macOS.
  - Supports `TARGET_ARCH=arm64|amd64` (`amd64` can be cross-built on Apple Silicon).
  - Installs outputs into `.tmp/opus-prebuilt/<platform>/`.

- `package_prebuilt.sh`
  - Copies build outputs into `third_party/audio/prebuilt/libopus/<platform>/`.
  - Generates `manifest.json` with SHA-256 checksums for headers and `libopus.a`.

- `verify_artifacts.sh`
  - Verifies required headers, static library, manifest, and checksum integrity.
  - Detects accidental Git LFS pointer file for `libopus.a`.

## Usage

```bash
# macOS arm64 build + package + verify
tools/audio/opus/build_prebuilt_darwin.sh
tools/audio/opus/package_prebuilt.sh darwin-arm64
tools/audio/opus/verify_artifacts.sh darwin-arm64

# macOS amd64 build (from arm64 host) + package + verify
TARGET_ARCH=amd64 tools/audio/opus/build_prebuilt_darwin.sh
tools/audio/opus/package_prebuilt.sh darwin-amd64
tools/audio/opus/verify_artifacts.sh darwin-amd64

# Linux amd64 build + package + verify
tools/audio/opus/build_prebuilt_linux.sh
tools/audio/opus/package_prebuilt.sh linux-amd64
tools/audio/opus/verify_artifacts.sh linux-amd64

# Linux arm64 build + package + verify
TARGET_ARCH=arm64 tools/audio/opus/build_prebuilt_linux.sh
tools/audio/opus/package_prebuilt.sh linux-arm64
tools/audio/opus/verify_artifacts.sh linux-arm64
```

## Notes

- Source of truth is the `third_party/audio/libopus` submodule revision.
- `pkg/audio/codec/opus` links prebuilt static libraries from `third_party/audio/prebuilt/libopus/<platform>/lib/libopus.a`.
