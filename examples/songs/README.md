# examples/songs

This example is used to validate an end-to-end audio chain:

- `pkg/audio/songs` (built-in multi-voice songs)
- `pkg/audio/pcm` mixer (multi-track mixing)
- `pkg/audio/portaudio` (device playback/capture)
- `pkg/audio/codec/mp3` (recording encode / file decode)
- `pkg/audio/codec/ogg` (OGG container read/write)
- Optional: `pkg/audio/codec/opus` encode/decode loopback

## Prerequisites

Playback and recording require PortAudio native runtime support (cgo + supported OS/arch):

```bash
CGO_ENABLED=1 go run ./examples/songs -mode list
```

> OGG interoperability scope: `play-ogg` is currently guaranteed for OGG files
> produced by this example (`record-ogg`). General third-party Ogg Opus files may
> require stricter Opus header/granule semantic handling.

## Common Commands

### 1) List built-in songs

```bash
CGO_ENABLED=1 go run ./examples/songs -mode list
```

### 2) Play a single song

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode play-song \
  -song twinkle_star
```

### 3) Play multi-track mix (overlay multiple songs)

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode play-song \
  -songs twinkle_star,canon
```

### 4) Record microphone to MP3

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode record-mic \
  -timeout 5s \
  -output ./out/mic.mp3
```

### 5) Play an MP3 file

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode play-mp3 \
  -input ./out/mic.mp3
```

### 6) Enable Opus loopback (encode then decode before playback)

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode play-song \
  -song twinkle_star \
  -opus-loopback
```

### 7) Record microphone to OGG (Opus in OGG container)

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode record-ogg \
  -timeout 5s \
  -output-ogg ./out/mic.ogg
```

### 8) Play an OGG file

```bash
CGO_ENABLED=1 go run ./examples/songs \
  -mode play-ogg \
  -input-ogg ./out/mic.ogg
```
