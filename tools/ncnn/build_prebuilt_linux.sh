#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "[build_prebuilt_linux] error: this script must run on Linux" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
NCNN_SRC_DIR="${ROOT_DIR}/third_party/ncnn/upstream"

if [[ ! -f "${NCNN_SRC_DIR}/CMakeLists.txt" ]]; then
	echo "[build_prebuilt_linux] error: missing ncnn source at ${NCNN_SRC_DIR}" >&2
	echo "[build_prebuilt_linux] hint: run 'git submodule update --init --recursive'" >&2
	exit 1
fi

ARCH_INPUT="${TARGET_ARCH:-$(uname -m)}"
case "${ARCH_INPUT}" in
x86_64 | amd64)
	TARGET_ARCH="amd64"
	PLATFORM="linux-amd64"
	HOST_TRIPLET="x86_64-unknown-linux-gnu"
	;;
aarch64 | arm64)
	TARGET_ARCH="arm64"
	PLATFORM="linux-arm64"
	HOST_TRIPLET="aarch64-unknown-linux-gnu"
	;;
*)
	echo "[build_prebuilt_linux] error: unsupported arch ${ARCH_INPUT}" >&2
	exit 1
	;;
esac

BUILD_ARCH_RAW="$(uname -m)"
case "${BUILD_ARCH_RAW}" in
x86_64 | amd64)
	BUILD_ARCH_NORMALIZED="amd64"
	;;
aarch64 | arm64)
	BUILD_ARCH_NORMALIZED="arm64"
	;;
*)
	echo "[build_prebuilt_linux] error: unsupported build arch ${BUILD_ARCH_RAW}" >&2
	exit 1
	;;
esac

if [[ "${TARGET_ARCH}" != "${BUILD_ARCH_NORMALIZED}" ]]; then
	echo "[build_prebuilt_linux] error: cross build is not supported in this script" >&2
	echo "[build_prebuilt_linux]        requested TARGET_ARCH=${TARGET_ARCH}, build host=${BUILD_ARCH_NORMALIZED}" >&2
	exit 1
fi

WORK_ROOT="${ROOT_DIR}/.tmp/ncnn-build/${PLATFORM}"
SRC_BUILD_DIR="${WORK_ROOT}/src"
BUILD_DIR="${WORK_ROOT}/build"
OUT_ROOT="${ROOT_DIR}/.tmp/ncnn-prebuilt/${PLATFORM}"

rm -rf "${WORK_ROOT}" "${OUT_ROOT}"
mkdir -p "${SRC_BUILD_DIR}" "${OUT_ROOT}"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_linux] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd tar
need_cmd cmake
need_cmd make
need_cmd python3
need_cmd git

tar -C "${NCNN_SRC_DIR}" --exclude ".git" -cf - . | tar -C "${SRC_BUILD_DIR}" -xf -

cmake -S "${SRC_BUILD_DIR}" -B "${BUILD_DIR}" \
	-DCMAKE_BUILD_TYPE=Release \
	-DNCNN_VULKAN=OFF \
	-DNCNN_BUILD_TOOLS=OFF \
	-DNCNN_BUILD_EXAMPLES=OFF \
	-DNCNN_BUILD_BENCHMARK=OFF \
	-DNCNN_BUILD_TESTS=OFF \
	-DNCNN_OPENMP=OFF \
	-DNCNN_SIMPLEOMP=ON \
	-DNCNN_C_API=ON \
	-DNCNN_INSTALL_SDK=OFF

if command -v nproc >/dev/null 2>&1; then
	JOBS="$(nproc)"
else
	JOBS=4
fi
if [[ "${JOBS}" -lt 1 ]]; then
	JOBS=1
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
	cp "${SRC_BUILD_DIR}/src/${header}" "${OUT_ROOT}/include/ncnn/${header}"
done

NCNN_COMMIT="$(git -C "${NCNN_SRC_DIR}" rev-parse HEAD 2>/dev/null || true)"
NCNN_DESCRIBE="$(git -C "${NCNN_SRC_DIR}" describe --tags --always 2>/dev/null || true)"
NCNN_VERSION="${NCNN_DESCRIBE}"
if [[ -z "${NCNN_VERSION}" ]]; then
	NCNN_VERSION="unknown"
fi

cat >"${OUT_ROOT}/build.env" <<EOF
NCNN_VERSION=${NCNN_VERSION}
NCNN_SUBMODULE_PATH=third_party/ncnn/upstream
NCNN_COMMIT=${NCNN_COMMIT}
NCNN_DESCRIBE=${NCNN_DESCRIBE}
NCNN_VULKAN=OFF
NCNN_C_API=ON
TARGET_PLATFORM=${PLATFORM}
TARGET_ARCH=${TARGET_ARCH}
HOST_TRIPLET=${HOST_TRIPLET}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_linux] built ncnn artifacts at ${OUT_ROOT}"
