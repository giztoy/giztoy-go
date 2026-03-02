#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

PLATFORM="${1:-darwin-arm64}"
SRC_DIR="${2:-${ROOT_DIR}/.tmp/audio-portaudio-prebuilt/${PLATFORM}}"
DST_DIR="${ROOT_DIR}/third_party/audio/prebuilt/portaudio/${PLATFORM}"
BUILD_META_SRC="${SRC_DIR}/build.env"

if [[ ! -d "${SRC_DIR}" ]]; then
	echo "[package_prebuilt] error: source directory not found: ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/include/portaudio.h" ]]; then
	echo "[package_prebuilt] error: missing include/portaudio.h in ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/lib/libportaudio.a" ]]; then
	echo "[package_prebuilt] error: missing lib/libportaudio.a in ${SRC_DIR}" >&2
	exit 1
fi

rm -rf "${DST_DIR}"
mkdir -p "${DST_DIR}/include" "${DST_DIR}/lib"

cp "${SRC_DIR}/include/portaudio.h" "${DST_DIR}/include/portaudio.h"
cp "${SRC_DIR}/lib/libportaudio.a" "${DST_DIR}/lib/libportaudio.a"

if [[ -f "${BUILD_META_SRC}" ]]; then
	cp "${BUILD_META_SRC}" "${DST_DIR}/build.env"
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
build_env_path = dst_dir / "build.env"
if build_env_path.exists():
    for line in build_env_path.read_text(encoding="utf-8").splitlines():
        if "=" not in line:
            continue
        key, val = line.split("=", 1)
        build_env[key] = val

artifacts = []
for rel in ["include/portaudio.h", "lib/libportaudio.a"]:
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
    "portaudio": {
        "submodule_path": build_env.get("PORTAUDIO_SUBMODULE_PATH", "third_party/audio/portaudio"),
        "commit": build_env.get("PORTAUDIO_COMMIT", "unknown"),
        "describe": build_env.get("PORTAUDIO_DESCRIBE", "unknown"),
    },
    "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "artifacts": artifacts,
}

(dst_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
print(f"[package_prebuilt] wrote manifest with {len(artifacts)} artifacts")
PY

echo "[package_prebuilt] packaged ${PLATFORM} artifacts into ${DST_DIR}"
