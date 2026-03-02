#!/usr/bin/env bash

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "[build_prebuilt_linux] error: this script must run on Linux" >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
PORTAUDIO_SRC_DIR="${ROOT_DIR}/third_party/audio/portaudio"

if [[ ! -f "${PORTAUDIO_SRC_DIR}/configure" ]]; then
	echo "[build_prebuilt_linux] error: missing portaudio source at ${PORTAUDIO_SRC_DIR}" >&2
	echo "[build_prebuilt_linux] hint: run 'git submodule update --init --recursive third_party/audio/portaudio'" >&2
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

WORK_ROOT="${ROOT_DIR}/.tmp/audio-portaudio-build/${PLATFORM}"
SRC_BUILD_DIR="${WORK_ROOT}/src"
INSTALL_DIR="${WORK_ROOT}/install"
OUT_ROOT="${ROOT_DIR}/.tmp/audio-portaudio-prebuilt/${PLATFORM}"

rm -rf "${WORK_ROOT}" "${OUT_ROOT}"
mkdir -p "${SRC_BUILD_DIR}" "${INSTALL_DIR}" "${OUT_ROOT}"

tar -C "${PORTAUDIO_SRC_DIR}" --exclude ".git" -cf - . | tar -C "${SRC_BUILD_DIR}" -xf -

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

echo "[build_prebuilt_linux] built portaudio artifacts at ${OUT_ROOT}"
