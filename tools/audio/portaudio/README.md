# tools/audio/portaudio

Build, package, and verify PortAudio prebuilt artifacts (`libportaudio.a`) from
the tracked upstream submodule `third_party/audio/portaudio`.

## Scripts

- `build_prebuilt_darwin.sh`
  - Runs on macOS.
  - Builds static `libportaudio.a` from `third_party/audio/portaudio`.
  - Outputs staged files to `.tmp/audio-portaudio-prebuilt/<platform>/`.

- `build_prebuilt_linux.sh`
  - Runs on Linux.
  - Builds static `libportaudio.a` from `third_party/audio/portaudio`.
  - Outputs staged files to `.tmp/audio-portaudio-prebuilt/<platform>/`.

- `package_prebuilt.sh`
  - Copies staged artifacts into `third_party/audio/prebuilt/portaudio/<platform>/`.
  - Generates `manifest.json` with SHA-256 checksums.

- `verify_artifacts.sh`
  - Validates required files and checksum consistency.
  - Detects accidental Git LFS pointer files for `libportaudio.a`.
  - Verifies `manifest.json` schema and artifact checksums.

## Usage

```bash
# macOS arm64 build + package + verify
tools/audio/portaudio/build_prebuilt_darwin.sh
tools/audio/portaudio/package_prebuilt.sh darwin-arm64
tools/audio/portaudio/verify_artifacts.sh darwin-arm64

# macOS amd64 build + package + verify
TARGET_ARCH=amd64 tools/audio/portaudio/build_prebuilt_darwin.sh
tools/audio/portaudio/package_prebuilt.sh darwin-amd64
tools/audio/portaudio/verify_artifacts.sh darwin-amd64

# Linux amd64 build + package + verify
tools/audio/portaudio/build_prebuilt_linux.sh
tools/audio/portaudio/package_prebuilt.sh linux-amd64
tools/audio/portaudio/verify_artifacts.sh linux-amd64

# Linux arm64 build + package + verify (must run on arm64 Linux host)
TARGET_ARCH=arm64 tools/audio/portaudio/build_prebuilt_linux.sh
tools/audio/portaudio/package_prebuilt.sh linux-arm64
tools/audio/portaudio/verify_artifacts.sh linux-arm64
```

## Notes

- The scripts enforce strict shell mode (`set -euo pipefail`) and explicit
  command checks to avoid silent failures.
- `build_prebuilt_linux.sh` intentionally forbids cross-build in this script to
  avoid inconsistent dependency/link behavior across toolchains.
- `pkg/audio/portaudio` native build (cgo + supported platform) links against
  `third_party/audio/prebuilt/portaudio/<platform>/lib/libportaudio.a`.
