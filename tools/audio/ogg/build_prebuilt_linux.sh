#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "[build_prebuilt_linux] error: this script must run on Linux" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SRC_DIR="${ROOT_DIR}/third_party/audio/libogg"

if [[ ! -f "${SRC_DIR}/CMakeLists.txt" ]]; then
	echo "[build_prebuilt_linux] error: missing libogg source at ${SRC_DIR}" >&2
	echo "[build_prebuilt_linux] hint: run 'git submodule update --init --recursive third_party/audio/libogg'" >&2
	exit 1
fi

ARCH_INPUT="${TARGET_ARCH:-$(uname -m)}"
case "${ARCH_INPUT}" in
x86_64 | amd64)
	TARGET_ARCH_NORMALIZED="amd64"
	PLATFORM="linux-amd64"
	;;
aarch64 | arm64)
	TARGET_ARCH_NORMALIZED="arm64"
	PLATFORM="linux-arm64"
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

if [[ "${TARGET_ARCH_NORMALIZED}" != "${BUILD_ARCH_NORMALIZED}" ]]; then
	echo "[build_prebuilt_linux] error: cross build is not supported in this script" >&2
	echo "[build_prebuilt_linux]        requested TARGET_ARCH=${TARGET_ARCH_NORMALIZED}, build host=${BUILD_ARCH_NORMALIZED}" >&2
	exit 1
fi

WORK_ROOT="${ROOT_DIR}/.tmp/ogg-build/${PLATFORM}"
BUILD_DIR="${WORK_ROOT}/build"
INSTALL_ROOT="${ROOT_DIR}/.tmp/ogg-prebuilt/${PLATFORM}"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_linux] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd cmake
need_cmd git

rm -rf "${BUILD_DIR}" "${INSTALL_ROOT}"
mkdir -p "${BUILD_DIR}" "${INSTALL_ROOT}"

cmake -S "${SRC_DIR}" -B "${BUILD_DIR}" \
	-DCMAKE_BUILD_TYPE=Release \
	-DBUILD_SHARED_LIBS=OFF \
	-DINSTALL_DOCS=OFF \
	-DINSTALL_PKG_CONFIG_MODULE=OFF \
	-DINSTALL_CMAKE_PACKAGE_MODULE=OFF \
	-DBUILD_TESTING=OFF \
	-DCMAKE_INSTALL_PREFIX="${INSTALL_ROOT}"

if command -v nproc >/dev/null 2>&1; then
	JOBS="$(nproc)"
else
	JOBS=4
fi
if [[ "${JOBS}" -lt 1 ]]; then
	JOBS=1
fi

cmake --build "${BUILD_DIR}" -- -j"${JOBS}"
cmake --install "${BUILD_DIR}"

if [[ ! -f "${INSTALL_ROOT}/lib/libogg.a" ]]; then
	echo "[build_prebuilt_linux] error: missing ${INSTALL_ROOT}/lib/libogg.a" >&2
	exit 1
fi

for header in ogg.h os_types.h config_types.h; do
	if [[ ! -f "${INSTALL_ROOT}/include/ogg/${header}" ]]; then
		echo "[build_prebuilt_linux] error: missing ${INSTALL_ROOT}/include/ogg/${header}" >&2
		exit 1
	fi
done

LIBOGG_SOURCE_REV="$(git -C "${SRC_DIR}" rev-parse HEAD)"
LIBOGG_SOURCE_TAG="$(git -C "${SRC_DIR}" describe --tags --always)"

cat >"${INSTALL_ROOT}/build.env" <<EOF
LIBOGG_SOURCE_TAG=${LIBOGG_SOURCE_TAG}
LIBOGG_SOURCE_REV=${LIBOGG_SOURCE_REV}
BUILD_SHARED_LIBS=OFF
INSTALL_DOCS=OFF
INSTALL_PKG_CONFIG_MODULE=OFF
INSTALL_CMAKE_PACKAGE_MODULE=OFF
BUILD_TESTING=OFF
TARGET_PLATFORM=${PLATFORM}
TARGET_ARCH=${TARGET_ARCH_NORMALIZED}
BUILD_ARCH=${BUILD_ARCH_NORMALIZED}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_linux] built ogg artifacts at ${INSTALL_ROOT}"
