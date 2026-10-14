#!/bin/sh
# Run from the gateway-go root in a Linux Go image with armhf GCC and make.
set -eu
export PATH=/usr/local/go/bin:$PATH
wk_root=$(pwd)
wk_out="$wk_root/build/iec61850-mms-armv7"
wk_source="$wk_out/source"
wk_revision=a13961110b8238d2d8ea577c1fb7592ba3017ad8
test "$(git -C "$wk_source" rev-parse HEAD)" = "$wk_revision"
mkdir -p "$wk_out/native" "$wk_out/arm"
make -C "$wk_source" -j4 TARGET=POSIX LIB_OBJS_DIR="$wk_out/native-objects" CFLAGS='-O2 -g0 -fPIC' INSTALL_PREFIX="$wk_out/native" install
CGO_ENABLED=1 CC=gcc CGO_CFLAGS="-I$wk_out/native/include" CGO_LDFLAGS="-L$wk_out/native/lib -lm" \
  go test -tags iec61850_mms ./internal/web ./internal/collector ./internal/forward -count=1
make -C "$wk_source" -j4 TARGET=LINUX-ARM LIB_OBJS_DIR="$wk_out/arm-objects" CFLAGS='-O2 -g0 -fPIC -march=armv7-a -mfpu=vfpv3-d16 -mfloat-abi=hard' INSTALL_PREFIX="$wk_out/arm" install
export GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc
export CGO_CFLAGS="-I$wk_out/arm/include"
export CGO_LDFLAGS="-L$wk_out/arm/lib -L$wk_root/build/remote-phase1-armv7/lib -lm"
go build -buildvcs=false -tags 'opcda,iec61850_mms' -trimpath \
  -ldflags '-s -w -extldflags=-Wl,-rpath,/userdata/weikong/lib' \
  -o "$wk_out/weikong-gateway" ./cmd/gateway
go version -m "$wk_out/weikong-gateway"
arm-linux-gnueabihf-readelf -d "$wk_out/weikong-gateway"
sha256sum "$wk_out/weikong-gateway"
