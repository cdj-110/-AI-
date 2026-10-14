#!/bin/sh
set -eu
cd /src
export GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc
export CGO_CFLAGS='-I/src/build/iec61850-mms-armv7/arm/include'
export CGO_LDFLAGS='-L/src/build/iec61850-mms-armv7/arm/lib -L/src/build/remote-phase1-armv7/lib -lm'
go build -buildvcs=false -tags 'opcda,iec61850_mms' -trimpath -ldflags '-s -w -extldflags=-Wl,-rpath,/userdata/weikong/lib' -o /out/weikong-gateway-alarm-retain-20260914 ./cmd/gateway
sha256sum /out/weikong-gateway-alarm-retain-20260914
arm-linux-gnueabihf-readelf -d /out/weikong-gateway-alarm-retain-20260914
