#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

PLATFORM="${1:-darwin-arm64}"
SRC_DIR="${2:-${ROOT_DIR}/.tmp/audio-mp3-prebuilt/${PLATFORM}}"
DST_DIR="${ROOT_DIR}/third_party/audio/prebuilt/${PLATFORM}"
BUILD_META_SRC="${SRC_DIR}/build.mp3.env"
if [[ ! -f "${BUILD_META_SRC}" && -f "${SRC_DIR}/build.env" ]]; then
	BUILD_META_SRC="${SRC_DIR}/build.env"
fi

if [[ ! -d "${SRC_DIR}" ]]; then
	echo "[package_prebuilt] error: source directory not found: ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/include/lame/lame.h" ]]; then
	echo "[package_prebuilt] error: missing include/lame/lame.h in ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/lib/libmp3lame.a" ]]; then
	echo "[package_prebuilt] error: missing lib/libmp3lame.a in ${SRC_DIR}" >&2
	exit 1
fi

mkdir -p "${DST_DIR}/include/lame" "${DST_DIR}/lib"

cp "${SRC_DIR}/include/lame/lame.h" "${DST_DIR}/include/lame/lame.h"
cp "${SRC_DIR}/lib/libmp3lame.a" "${DST_DIR}/lib/libmp3lame.a"

if [[ -f "${BUILD_META_SRC}" ]]; then
	cp "${BUILD_META_SRC}" "${DST_DIR}/build.mp3.env"
fi

python3 - "${DST_DIR}" "${PLATFORM}" <<'PY'
import hashlib
import json
import pathlib
import sys
import time

dst_dir = pathlib.Path(sys.argv[1])
platform = sys.argv[2]

build_env = {}
build_env_path = dst_dir / "build.mp3.env"
if not build_env_path.exists():
    fallback = dst_dir / "build.env"
    if fallback.exists():
        build_env_path = fallback
if build_env_path.exists():
    for line in build_env_path.read_text(encoding="utf-8").splitlines():
        if "=" not in line:
            continue
        key, val = line.split("=", 1)
        build_env[key] = val

artifacts = []
for rel in ["include/lame/lame.h", "lib/libmp3lame.a"]:
    path = dst_dir / rel
    artifacts.append(
        {
            "path": rel,
            "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
        }
    )

manifest = {
    "schema_version": 1,
    "platform": platform,
    "lame": {
        "submodule_path": build_env.get("LAME_SUBMODULE_PATH", "third_party/audio/lame"),
        "commit": build_env.get("LAME_COMMIT", "unknown"),
        "describe": build_env.get("LAME_DESCRIBE", "unknown"),
    },
    "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "artifacts": artifacts,
}

(dst_dir / "manifest.mp3.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
print(f"[package_prebuilt] wrote manifest with {len(artifacts)} artifacts")
PY

echo "[package_prebuilt] packaged ${PLATFORM} artifacts into ${DST_DIR}"
