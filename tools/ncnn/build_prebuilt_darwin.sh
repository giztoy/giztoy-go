#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "[build_prebuilt_darwin] error: this script must run on macOS" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

NCNN_VERSION="${NCNN_VERSION:-20260113}"
NCNN_SHA256="${NCNN_SHA256:-53696039ee8ba5c8db6446bdf12a576b8d7f7b0c33bb6749f94688bddf5a3d5c}"
NCNN_URL="https://github.com/Tencent/ncnn/releases/download/${NCNN_VERSION}/ncnn-${NCNN_VERSION}-full-source.zip"

ARCH_INPUT="${TARGET_ARCH:-$(uname -m)}"
case "${ARCH_INPUT}" in
x86_64 | amd64)
	PLATFORM="darwin-amd64"
	CMAKE_ARCH="x86_64"
	;;
aarch64 | arm64)
	PLATFORM="darwin-arm64"
	CMAKE_ARCH="arm64"
	;;
*)
	echo "[build_prebuilt_darwin] error: unsupported arch ${ARCH_INPUT}" >&2
	exit 1
	;;
esac

WORK_ROOT="${ROOT_DIR}/.tmp/ncnn-build/${PLATFORM}"
OUT_ROOT="${ROOT_DIR}/.tmp/ncnn-prebuilt/${PLATFORM}"

SRC_ZIP="${WORK_ROOT}/ncnn-${NCNN_VERSION}.zip"
SRC_DIR="${WORK_ROOT}"
BUILD_DIR="${WORK_ROOT}/build"

mkdir -p "${WORK_ROOT}" "${OUT_ROOT}"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_darwin] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd curl
need_cmd cmake
need_cmd make
need_cmd unzip
need_cmd python3
need_cmd shasum

sha256_file() {
	shasum -a 256 "$1" | awk '{print $1}'
}

if [[ ! -f "${SRC_ZIP}" ]]; then
	echo "[build_prebuilt_darwin] downloading ${NCNN_URL}"
	curl --fail --location --silent --show-error "${NCNN_URL}" --output "${SRC_ZIP}"
fi

ACTUAL_SHA="$(sha256_file "${SRC_ZIP}")"
if [[ "${ACTUAL_SHA}" != "${NCNN_SHA256}" ]]; then
	echo "[build_prebuilt_darwin] error: sha256 mismatch for ${SRC_ZIP}" >&2
	echo "  expected: ${NCNN_SHA256}" >&2
	echo "  actual:   ${ACTUAL_SHA}" >&2
	exit 1
fi

rm -rf "${BUILD_DIR}" "${OUT_ROOT}"
shopt -s dotglob nullglob
for path in "${WORK_ROOT}"/*; do
	if [[ "${path}" == "${SRC_ZIP}" ]]; then
		continue
	fi
	rm -rf "${path}"
done
shopt -u dotglob nullglob
mkdir -p "${OUT_ROOT}"
unzip -q "${SRC_ZIP}" -d "${WORK_ROOT}"

cmake -S "${SRC_DIR}" -B "${BUILD_DIR}" \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_OSX_ARCHITECTURES="${CMAKE_ARCH}" \
	-DNCNN_VULKAN=OFF \
	-DNCNN_BUILD_TOOLS=OFF \
	-DNCNN_BUILD_EXAMPLES=OFF \
	-DNCNN_BUILD_BENCHMARK=OFF \
	-DNCNN_BUILD_TESTS=OFF \
	-DNCNN_OPENMP=OFF \
	-DNCNN_SIMPLEOMP=ON \
	-DNCNN_C_API=ON \
	-DNCNN_INSTALL_SDK=OFF

JOBS="$(sysctl -n hw.ncpu)"
if [[ "${JOBS}" -lt 1 ]]; then
	JOBS=4
fi

cmake --build "${BUILD_DIR}" -- -j"${JOBS}"

mkdir -p "${OUT_ROOT}/include/ncnn" "${OUT_ROOT}/lib"
cp "${BUILD_DIR}/src/libncnn.a" "${OUT_ROOT}/lib/libncnn.a"

cp "${BUILD_DIR}/src/platform.h" "${OUT_ROOT}/include/ncnn/platform.h"
cp "${BUILD_DIR}/src/ncnn_export.h" "${OUT_ROOT}/include/ncnn/ncnn_export.h"
cp "${BUILD_DIR}/src/layer_type_enum.h" "${OUT_ROOT}/include/ncnn/layer_type_enum.h"
cp "${BUILD_DIR}/src/layer_shader_type_enum.h" "${OUT_ROOT}/include/ncnn/layer_shader_type_enum.h"

for header in \
	c_api.h net.h mat.h blob.h layer.h option.h allocator.h paramdict.h datareader.h \
	modelbin.h cpu.h pipelinecache.h gpu.h command.h pipeline.h simplestl.h simpleocv.h \
	simplemath.h simpleomp.h simplevk.h expression.h layer_type.h layer_shader_type.h \
	benchmark.h vulkan_header_fix.h; do
	cp "${SRC_DIR}/src/${header}" "${OUT_ROOT}/include/ncnn/${header}"
done

cat >"${OUT_ROOT}/build.env" <<EOF
NCNN_VERSION=${NCNN_VERSION}
NCNN_SHA256=${NCNN_SHA256}
NCNN_VULKAN=OFF
NCNN_C_API=ON
TARGET_PLATFORM=${PLATFORM}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_darwin] built ncnn artifacts at ${OUT_ROOT}"
