#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PLATFORM="${1:-linux-amd64}"
SRC_DIR="${2:-${ROOT_DIR}/.tmp/ncnn-prebuilt/${PLATFORM}}"
DST_DIR="${ROOT_DIR}/third_party/ncnn/prebuilt/${PLATFORM}"

if [[ ! -d "${SRC_DIR}" ]]; then
	echo "[package_prebuilt] error: source directory not found: ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -d "${SRC_DIR}/include/ncnn" ]]; then
	echo "[package_prebuilt] error: missing include/ncnn in ${SRC_DIR}" >&2
	exit 1
fi

if [[ ! -f "${SRC_DIR}/lib/libncnn.a" ]]; then
	echo "[package_prebuilt] error: missing lib/libncnn.a in ${SRC_DIR}" >&2
	exit 1
fi

rm -rf "${DST_DIR}"
mkdir -p "${DST_DIR}"
cp -R "${SRC_DIR}/include" "${DST_DIR}/include"
mkdir -p "${DST_DIR}/lib"
cp "${SRC_DIR}/lib/libncnn.a" "${DST_DIR}/lib/libncnn.a"

if [[ -f "${SRC_DIR}/build.env" ]]; then
	cp "${SRC_DIR}/build.env" "${DST_DIR}/build.env"
fi

python3 - "${DST_DIR}" "${PLATFORM}" <<'PY'
import hashlib
import json
import pathlib
import time
import sys

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
for path in sorted((dst_dir / "include" / "ncnn").glob("*.h")):
    rel = path.relative_to(dst_dir).as_posix()
    sha = hashlib.sha256(path.read_bytes()).hexdigest()
    artifacts.append({"path": rel, "sha256": sha})

lib_path = dst_dir / "lib" / "libncnn.a"
artifacts.append(
    {
        "path": "lib/libncnn.a",
        "sha256": hashlib.sha256(lib_path.read_bytes()).hexdigest(),
    }
)

manifest = {
    "schema_version": 1,
    "platform": platform,
    "ncnn_version": build_env.get("NCNN_VERSION", "unknown"),
    "build": {
        "NCNN_VULKAN": build_env.get("NCNN_VULKAN", "OFF"),
        "NCNN_C_API": build_env.get("NCNN_C_API", "ON"),
    },
    "generated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
    "artifacts": artifacts,
}

(dst_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
print(f"[package_prebuilt] wrote manifest with {len(artifacts)} artifacts")
PY

echo "[package_prebuilt] packaged ${PLATFORM} artifacts into ${DST_DIR}"
