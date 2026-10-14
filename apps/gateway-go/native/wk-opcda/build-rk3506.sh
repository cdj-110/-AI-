#!/bin/sh
set -eu

: "${RK3506_TOOLCHAIN_ROOT:?set RK3506_TOOLCHAIN_ROOT to the Buildroot host/toolchain directory}"
: "${RK3506_SYSROOT:?set RK3506_SYSROOT to the RK3506 target sysroot}"

WK_TARGET_ARCH=armv7 \
WK_TOOLCHAIN_ROOT="${RK3506_TOOLCHAIN_ROOT}" \
WK_SYSROOT="${RK3506_SYSROOT}" \
WK_TOOLCHAIN_PREFIX="${RK3506_TOOLCHAIN_PREFIX:-arm-buildroot-linux-gnueabihf}" \
WK_BUILD_DIR="${OPCDA_BUILD_DIR:-build/wk-opcda-rk3506}" \
WK_STAGE_DIR="${OPCDA_STAGE_DIR:-build/wk-opcda-rk3506-stage}" \
./native/wk-opcda/build-linux.sh
