#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "[build_prebuilt_linux] error: this script must run on Linux" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
LAME_SRC_DIR="${ROOT_DIR}/third_party/audio/lame"

if [[ ! -f "${LAME_SRC_DIR}/configure" ]]; then
	echo "[build_prebuilt_linux] error: missing lame source at ${LAME_SRC_DIR}" >&2
	echo "[build_prebuilt_linux] hint: run 'git submodule update --init --recursive'" >&2
	exit 1
fi

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_linux] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd tar
need_cmd make
need_cmd gcc
need_cmd ar
need_cmd ranlib
need_cmd git

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
	BUILD_TRIPLET="x86_64-unknown-linux-gnu"
	BUILD_ARCH_NORMALIZED="amd64"
	;;
aarch64 | arm64)
	BUILD_TRIPLET="aarch64-unknown-linux-gnu"
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

WORK_ROOT="${ROOT_DIR}/.tmp/audio-mp3-build/${PLATFORM}"
SRC_BUILD_DIR="${WORK_ROOT}/src"
INSTALL_DIR="${WORK_ROOT}/install"
OUT_ROOT="${ROOT_DIR}/.tmp/audio-mp3-prebuilt/${PLATFORM}"

rm -rf "${WORK_ROOT}" "${OUT_ROOT}"
mkdir -p "${SRC_BUILD_DIR}" "${INSTALL_DIR}" "${OUT_ROOT}"

tar -C "${LAME_SRC_DIR}" --exclude ".git" -cf - . | tar -C "${SRC_BUILD_DIR}" -xf -

PATH_OVERRIDE="/usr/bin:/bin:/usr/sbin:/sbin:${PATH}"
if command -v nproc >/dev/null 2>&1; then
	JOBS="$(nproc)"
else
	JOBS=4
fi
if [[ "${JOBS}" -lt 1 ]]; then
	JOBS=1
fi

(
	cd "${SRC_BUILD_DIR}"
	export PATH="${PATH_OVERRIDE}"
	./configure \
		--build="${BUILD_TRIPLET}" \
		--host="${HOST_TRIPLET}" \
		--disable-shared \
		--enable-static \
		--disable-frontend \
		--disable-decoder \
		--disable-analyzer-hooks \
		--prefix="${INSTALL_DIR}"
	make -j"${JOBS}"
	make install
)

mkdir -p "${OUT_ROOT}/include/lame" "${OUT_ROOT}/lib"
cp "${INSTALL_DIR}/include/lame/lame.h" "${OUT_ROOT}/include/lame/lame.h"
cp "${INSTALL_DIR}/lib/libmp3lame.a" "${OUT_ROOT}/lib/libmp3lame.a"

LAME_COMMIT="$(git -C "${LAME_SRC_DIR}" rev-parse HEAD 2>/dev/null || true)"
LAME_DESCRIBE="$(git -C "${LAME_SRC_DIR}" describe --tags --always 2>/dev/null || true)"

cat >"${OUT_ROOT}/build.env" <<EOF
LAME_SUBMODULE_PATH=third_party/audio/lame
LAME_COMMIT=${LAME_COMMIT}
LAME_DESCRIBE=${LAME_DESCRIBE}
TARGET_PLATFORM=${PLATFORM}
TARGET_ARCH=${TARGET_ARCH}
BUILD_TRIPLET=${BUILD_TRIPLET}
HOST_TRIPLET=${HOST_TRIPLET}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_linux] built mp3 artifacts at ${OUT_ROOT}"
