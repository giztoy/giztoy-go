# tools/audio/mp3

Build, package, and verify MP3 prebuilt artifacts (`libmp3lame.a`) from the
tracked upstream submodule `third_party/audio/lame`.

## Scripts

- `build_prebuilt_darwin.sh`
  - Runs on macOS.
  - Builds `libmp3lame.a` from `third_party/audio/lame`.
  - Outputs staged files to `.tmp/audio-mp3-prebuilt/<platform>/`.

- `build_prebuilt_linux.sh`
  - Runs on Linux.
  - Builds `libmp3lame.a` from `third_party/audio/lame`.
  - Outputs staged files to `.tmp/audio-mp3-prebuilt/<platform>/`.

- `package_prebuilt.sh`
  - Copies staged artifacts into `third_party/audio/prebuilt/lame/<platform>/`.
  - Generates `manifest.json` with SHA-256 checksums.

- `verify_artifacts.sh`
  - Validates required files and checksum consistency.
  - Detects accidental Git LFS pointer files for `libmp3lame.a`.
  - Verifies `manifest.json`.

## Usage

```bash
# macOS arm64 build + package + verify
tools/audio/mp3/build_prebuilt_darwin.sh
tools/audio/mp3/package_prebuilt.sh darwin-arm64
tools/audio/mp3/verify_artifacts.sh darwin-arm64

# macOS amd64 build + package + verify
TARGET_ARCH=amd64 tools/audio/mp3/build_prebuilt_darwin.sh
tools/audio/mp3/package_prebuilt.sh darwin-amd64
tools/audio/mp3/verify_artifacts.sh darwin-amd64

# Linux amd64 build + package + verify
tools/audio/mp3/build_prebuilt_linux.sh
tools/audio/mp3/package_prebuilt.sh linux-amd64
tools/audio/mp3/verify_artifacts.sh linux-amd64

# Linux arm64 build + package + verify (must run on an arm64 Linux host)
TARGET_ARCH=arm64 tools/audio/mp3/build_prebuilt_linux.sh
tools/audio/mp3/package_prebuilt.sh linux-arm64
tools/audio/mp3/verify_artifacts.sh linux-arm64
```

## Notes

- The scripts intentionally prepend `/usr/bin:/bin:/usr/sbin:/sbin` to `PATH`
  while running `configure`/`make`, to avoid non-standard toolchain wrappers
  breaking autoconf checks.
- `build_prebuilt_linux.sh` does not support cross-build: when
  `TARGET_ARCH=arm64`, you must execute the script on an arm64 Linux machine.
- `pkg/audio/codec/mp3` links against
  `third_party/audio/prebuilt/lame/<platform>/lib/libmp3lame.a` on supported cgo
  platforms.
