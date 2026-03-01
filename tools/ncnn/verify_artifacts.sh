#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PLATFORM="${1:-linux-amd64}"
PREBUILT_DIR="${ROOT_DIR}/third_party/ncnn/prebuilt/${PLATFORM}"
MODEL_DIR="${ROOT_DIR}/pkg/ncnn/models"

fail() {
	echo "[verify_artifacts] error: $*" >&2
	exit 1
}

check_file_exists() {
	local file="$1"
	[[ -f "${file}" ]] || fail "missing file: ${file}"
}

check_not_lfs_pointer() {
	local file="$1"
	check_file_exists "${file}"
	if grep -q "^version https://git-lfs.github.com/spec/v1" "${file}"; then
		fail "detected Git LFS pointer file: ${file}; run 'git lfs pull' before verify"
	fi
}

for header in c_api.h net.h mat.h option.h platform.h ncnn_export.h layer_type_enum.h layer_shader_type_enum.h; do
	check_file_exists "${PREBUILT_DIR}/include/ncnn/${header}"
done

check_not_lfs_pointer "${PREBUILT_DIR}/lib/libncnn.a"
check_file_exists "${PREBUILT_DIR}/manifest.json"

for model in \
	"speaker_eres2net.ncnn.param" \
	"speaker_eres2net.ncnn.bin" \
	"vad_silero.ncnn.param" \
	"vad_silero.ncnn.bin" \
	"denoise_nsnet2.ncnn.param" \
	"denoise_nsnet2.ncnn.bin"; do
	check_file_exists "${MODEL_DIR}/${model}"
done

check_not_lfs_pointer "${MODEL_DIR}/speaker_eres2net.ncnn.bin"
check_not_lfs_pointer "${MODEL_DIR}/vad_silero.ncnn.bin"
check_not_lfs_pointer "${MODEL_DIR}/denoise_nsnet2.ncnn.bin"

python3 - "${PREBUILT_DIR}/manifest.json" "${PREBUILT_DIR}" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
base_dir = pathlib.Path(sys.argv[2])

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
artifacts = manifest.get("artifacts", [])
if not artifacts:
    raise SystemExit("[verify_artifacts] error: manifest has no artifacts")

for item in artifacts:
    rel = item.get("path")
    expected = item.get("sha256")
    if not rel or not expected:
        raise SystemExit("[verify_artifacts] error: malformed manifest entry")
    path = base_dir / rel
    if not path.is_file():
        raise SystemExit(f"[verify_artifacts] error: missing artifact from manifest: {path}")
    actual = hashlib.sha256(path.read_bytes()).hexdigest()
    if actual != expected:
        raise SystemExit(
            f"[verify_artifacts] error: checksum mismatch for {rel}: expected {expected}, got {actual}"
        )

print(f"[verify_artifacts] checksum verification passed for {len(artifacts)} artifacts")
PY

echo "[verify_artifacts] all checks passed for ${PLATFORM}"
