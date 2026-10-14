#!/bin/sh
set -eu
cd /src
export CGO_ENABLED=1
go test ./internal/cloud ./internal/config ./internal/app
export GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc
export CGO_CFLAGS='-I/src/build/iec61850-mms-armv7/arm/include'
export CGO_LDFLAGS='-L/src/build/iec61850-mms-armv7/arm/lib -L/src/build/remote-phase1-armv7/lib -lm'
mkdir -p /src/build/remote-config-release-20260915
go build -buildvcs=false -tags 'opcda,iec61850_mms' -trimpath -ldflags '-s -w -extldflags=-Wl,-rpath,/userdata/weikong/lib' -o /src/build/remote-config-release-20260915/weikong-gateway-remote-config-20260915 ./cmd/gateway
sha256sum /src/build/remote-config-release-20260915/weikong-gateway-remote-config-20260915
arm-linux-gnueabihf-readelf -h /src/build/remote-config-release-20260915/weikong-gateway-remote-config-20260915
