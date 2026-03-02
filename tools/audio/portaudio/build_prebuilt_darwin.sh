#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "[build_prebuilt_darwin] error: this script must run on macOS" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
PORTAUDIO_SRC_DIR="${ROOT_DIR}/third_party/audio/portaudio"

if [[ ! -f "${PORTAUDIO_SRC_DIR}/configure" ]]; then
	echo "[build_prebuilt_darwin] error: missing portaudio source at ${PORTAUDIO_SRC_DIR}" >&2
	echo "[build_prebuilt_darwin] hint: run 'git submodule update --init --recursive third_party/audio/portaudio'" >&2
	exit 1
fi

need_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "[build_prebuilt_darwin] error: missing required command: $1" >&2
		exit 1
	fi
}

need_cmd tar
need_cmd make
need_cmd clang
need_cmd ar
need_cmd ranlib
need_cmd git
need_cmd sysctl

ARCH_INPUT="${TARGET_ARCH:-$(uname -m)}"
case "${ARCH_INPUT}" in
x86_64 | amd64)
	TARGET_ARCH="amd64"
	PLATFORM="darwin-amd64"
	HOST_TRIPLET="x86_64-apple-darwin"
	ARCH_FLAG="x86_64"
	;;
aarch64 | arm64)
	TARGET_ARCH="arm64"
	PLATFORM="darwin-arm64"
	HOST_TRIPLET="arm-apple-darwin"
	ARCH_FLAG="arm64"
	;;
*)
	echo "[build_prebuilt_darwin] error: unsupported arch ${ARCH_INPUT}" >&2
	exit 1
	;;
esac

BUILD_ARCH_RAW="$(uname -m)"
case "${BUILD_ARCH_RAW}" in
x86_64 | amd64)
	BUILD_TRIPLET="x86_64-apple-darwin"
	;;
aarch64 | arm64)
	BUILD_TRIPLET="arm-apple-darwin"
	;;
*)
	echo "[build_prebuilt_darwin] error: unsupported build arch ${BUILD_ARCH_RAW}" >&2
	exit 1
	;;
esac

WORK_ROOT="${ROOT_DIR}/.tmp/audio-portaudio-build/${PLATFORM}"
SRC_BUILD_DIR="${WORK_ROOT}/src"
INSTALL_DIR="${WORK_ROOT}/install"
OUT_ROOT="${ROOT_DIR}/.tmp/audio-portaudio-prebuilt/${PLATFORM}"

rm -rf "${WORK_ROOT}" "${OUT_ROOT}"
mkdir -p "${SRC_BUILD_DIR}" "${INSTALL_DIR}" "${OUT_ROOT}"

tar -C "${PORTAUDIO_SRC_DIR}" --exclude ".git" -cf - . | tar -C "${SRC_BUILD_DIR}" -xf -

PATH_OVERRIDE="/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:${PATH}"
JOBS="$(sysctl -n hw.ncpu 2>/dev/null || true)"
if [[ -z "${JOBS}" || "${JOBS}" -lt 1 ]]; then
	JOBS=4
fi

(
	cd "${SRC_BUILD_DIR}"
	export PATH="${PATH_OVERRIDE}"
	export CC="clang -arch ${ARCH_FLAG}"
	export CFLAGS="-O2 -arch ${ARCH_FLAG}"
	export LDFLAGS="-arch ${ARCH_FLAG}"
	./configure \
		--build="${BUILD_TRIPLET}" \
		--host="${HOST_TRIPLET}" \
		--disable-shared \
		--enable-static \
		--without-jack \
		--without-asihpi \
		--without-oss \
		--prefix="${INSTALL_DIR}"
	make -j"${JOBS}"
	make install
)

mkdir -p "${OUT_ROOT}/include" "${OUT_ROOT}/lib"
cp "${INSTALL_DIR}/include/portaudio.h" "${OUT_ROOT}/include/portaudio.h"
cp "${INSTALL_DIR}/lib/libportaudio.a" "${OUT_ROOT}/lib/libportaudio.a"

PORTAUDIO_COMMIT="$(git -C "${PORTAUDIO_SRC_DIR}" rev-parse HEAD 2>/dev/null || true)"
PORTAUDIO_DESCRIBE="$(git -C "${PORTAUDIO_SRC_DIR}" describe --tags --always 2>/dev/null || true)"

cat >"${OUT_ROOT}/build.env" <<EOF
PORTAUDIO_SUBMODULE_PATH=third_party/audio/portaudio
PORTAUDIO_COMMIT=${PORTAUDIO_COMMIT}
PORTAUDIO_DESCRIBE=${PORTAUDIO_DESCRIBE}
TARGET_PLATFORM=${PLATFORM}
TARGET_ARCH=${TARGET_ARCH}
BUILD_TRIPLET=${BUILD_TRIPLET}
HOST_TRIPLET=${HOST_TRIPLET}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
EOF

echo "[build_prebuilt_darwin] built portaudio artifacts at ${OUT_ROOT}"
