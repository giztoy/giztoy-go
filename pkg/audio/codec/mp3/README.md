# pkg/audio/codec/mp3

MP3 codec package for giztoy-go audio pipeline.

## Features

- MP3 encode (PCM16LE -> MP3) via LAME (`libmp3lame.a`)
- MP3 decode (MP3 -> PCM16LE) via pure Go decoder
- Stream helpers: `EncodePCMStream` and `DecodeFull`

## Build/link strategy

- On supported cgo platforms (`linux|darwin` + `amd64|arm64`), encoder links
  prebuilt static library from:
  - `third_party/audio/prebuilt/<platform>/include/lame/lame.h`
  - `third_party/audio/prebuilt/<platform>/lib/libmp3lame.a`
- On unsupported platforms, encoder APIs return explicit
  `unsupported platform` errors.

## Quick test

```bash
go test ./pkg/audio/codec/mp3 -v
```
