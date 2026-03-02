#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "[build_prebuilt_darwin] error: this script must run on macOS" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
SRC_DIR="${ROOT_DIR}/third_party/audio/libogg"

if [[ ! -f "${SRC_DIR}/CMakeLists.txt" ]]; then
	echo "[build_prebuilt_darwin] error: missing libogg source at ${SRC_DIR}" >&2
	echo "[build_prebuilt_darwin] hint: run 'git submodule update --init --recursive third_party/audio/libogg'" >&2
	exit 1
fi

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

WORK_ROOT="${ROOT_DIR}/.tmp/ogg-build/${PLATFORM}"
BUILD_DIR="${WORK_ROOT}/build"
INSTALL_ROOT="${ROOT_DIR}/.tmp/ogg-prebuilt/${PLATFORM}"

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_darwin] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd cmake
need_cmd git
need_cmd sysctl

rm -rf "${BUILD_DIR}" "${INSTALL_ROOT}"
mkdir -p "${BUILD_DIR}" "${INSTALL_ROOT}"

cmake -S "${SRC_DIR}" -B "${BUILD_DIR}" \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_OSX_ARCHITECTURES="${CMAKE_ARCH}" \
	-DBUILD_SHARED_LIBS=OFF \
	-DINSTALL_DOCS=OFF \
	-DINSTALL_PKG_CONFIG_MODULE=OFF \
	-DINSTALL_CMAKE_PACKAGE_MODULE=OFF \
	-DBUILD_TESTING=OFF \
	-DCMAKE_INSTALL_PREFIX="${INSTALL_ROOT}"

JOBS="$(sysctl -n hw.ncpu)"
if [[ "${JOBS}" -lt 1 ]]; then
	JOBS=4
fi

cmake --build "${BUILD_DIR}" -- -j"${JOBS}"
cmake --install "${BUILD_DIR}"

if [[ ! -f "${INSTALL_ROOT}/lib/libogg.a" ]]; then
	echo "[build_prebuilt_darwin] error: missing ${INSTALL_ROOT}/lib/libogg.a" >&2
	exit 1
fi

for header in ogg.h os_types.h config_types.h; do
	if [[ ! -f "${INSTALL_ROOT}/include/ogg/${header}" ]]; then
		echo "[build_prebuilt_darwin] error: missing ${INSTALL_ROOT}/include/ogg/${header}" >&2
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
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_darwin] built ogg artifacts at ${INSTALL_ROOT}"
