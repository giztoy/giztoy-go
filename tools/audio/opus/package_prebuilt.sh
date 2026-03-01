#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

PLATFORM="${1:-darwin-arm64}"
SRC_DIR="${2:-${ROOT_DIR}/.tmp/opus-prebuilt/${PLATFORM}}"
DST_DIR="${ROOT_DIR}/third_party/audio/prebuilt/libopus/${PLATFORM}"

if [[ ! -d "${SRC_DIR}" ]]; then
	echo "[package_prebuilt] error: source directory not found: ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -d "${SRC_DIR}/include/opus" ]]; then
	echo "[package_prebuilt] error: missing include/opus in ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/lib/libopus.a" ]]; then
	echo "[package_prebuilt] error: missing lib/libopus.a in ${SRC_DIR}" >&2
	exit 1
fi

rm -rf "${DST_DIR}"
mkdir -p "${DST_DIR}"
cp -R "${SRC_DIR}/include" "${DST_DIR}/include"
mkdir -p "${DST_DIR}/lib"
cp "${SRC_DIR}/lib/libopus.a" "${DST_DIR}/lib/libopus.a"

if [[ -f "${SRC_DIR}/build.env" ]]; then
	cp "${SRC_DIR}/build.env" "${DST_DIR}/build.env"
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
for path in sorted((dst_dir / "include" / "opus").glob("*.h")):
    rel = path.relative_to(dst_dir).as_posix()
    sha = hashlib.sha256(path.read_bytes()).hexdigest()
    artifacts.append({"path": rel, "sha256": sha})

lib_path = dst_dir / "lib" / "libopus.a"
artifacts.append(
    {
        "path": "lib/libopus.a",
        "sha256": hashlib.sha256(lib_path.read_bytes()).hexdigest(),
    }
)

manifest = {
    "schema_version": 1,
    "platform": platform,
    "opus_source_tag": build_env.get("OPUS_SOURCE_TAG", "unknown"),
    "opus_source_rev": build_env.get("OPUS_SOURCE_REV", "unknown"),
    "build": {
        "OPUS_BUILD_SHARED_LIBRARY": build_env.get("OPUS_BUILD_SHARED_LIBRARY", "OFF"),
        "OPUS_BUILD_PROGRAMS": build_env.get("OPUS_BUILD_PROGRAMS", "OFF"),
        "OPUS_BUILD_TESTING": build_env.get("OPUS_BUILD_TESTING", "OFF"),
    },
    "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "artifacts": artifacts,
}

(dst_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
print(f"[package_prebuilt] wrote manifest with {len(artifacts)} artifacts")
PY

echo "[package_prebuilt] packaged ${PLATFORM} artifacts into ${DST_DIR}"
