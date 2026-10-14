#!/bin/sh
set -eu

: "${WK_TARGET_ARCH:?set WK_TARGET_ARCH to arm64, armv7 or x86_64}"
: "${WK_TOOLCHAIN_ROOT:?set WK_TOOLCHAIN_ROOT to the Buildroot host/toolchain directory}"
: "${WK_SYSROOT:?set WK_SYSROOT to the target sysroot}"

case "${WK_TARGET_ARCH}" in
  arm64|aarch64)
    WK_GOARCH=arm64
    WK_GOARM=
    WK_DEFAULT_PREFIX=aarch64-buildroot-linux-gnu
    ;;
  armv7|arm)
    WK_GOARCH=arm
    WK_GOARM=7
    WK_DEFAULT_PREFIX=arm-buildroot-linux-gnueabihf
    ;;
  x86_64|amd64)
    WK_GOARCH=amd64
    WK_GOARM=
    WK_DEFAULT_PREFIX=x86_64-buildroot-linux-gnu
    ;;
  *)
    echo "unsupported WK_TARGET_ARCH=${WK_TARGET_ARCH}; use arm64, armv7 or x86_64" >&2
    exit 2
    ;;
esac

WK_TOOLCHAIN_PREFIX="${WK_TOOLCHAIN_PREFIX:-${WK_DEFAULT_PREFIX}}"
WK_BUILD_DIR="${WK_BUILD_DIR:-build/wk-opcda-${WK_TARGET_ARCH}}"
WK_STAGE_DIR="${WK_STAGE_DIR:-build/wk-opcda-${WK_TARGET_ARCH}-stage}"

cmake -S native/wk-opcda -B "${WK_BUILD_DIR}" \
  -DCMAKE_TOOLCHAIN_FILE=native/wk-opcda/cmake/buildroot-linux-toolchain.cmake \
  -DCMAKE_BUILD_TYPE=Release
cmake --build "${WK_BUILD_DIR}" --parallel
cmake --install "${WK_BUILD_DIR}" --prefix "${WK_STAGE_DIR}/usr"

GOARM="${WK_GOARM}" CGO_ENABLED=1 GOOS=linux GOARCH="${WK_GOARCH}" \
CC="${WK_TOOLCHAIN_ROOT}/bin/${WK_TOOLCHAIN_PREFIX}-gcc" \
CXX="${WK_TOOLCHAIN_ROOT}/bin/${WK_TOOLCHAIN_PREFIX}-g++" \
CGO_CFLAGS="--sysroot=${WK_SYSROOT} -I${WK_STAGE_DIR}/usr/include" \
CGO_LDFLAGS="--sysroot=${WK_SYSROOT} -L${WK_STAGE_DIR}/usr/lib -Wl,-rpath,/usr/lib" \
go build -tags opcda -o "${WK_BUILD_DIR}/gateway" ./cmd/gateway
