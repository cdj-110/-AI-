#!/bin/sh
set -eu
cd /src
export CGO_ENABLED=1
go test ./internal/state ./internal/web -run 'TestCollectionOverview' -count=1
export GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc
export CGO_CFLAGS='-I/src/build/iec61850-mms-armv7/arm/include'
export CGO_LDFLAGS='-L/src/build/iec61850-mms-armv7/arm/lib -L/src/build/remote-phase1-armv7/lib -lm'
go build -buildvcs=false -tags 'opcda,iec61850_mms' -trimpath -ldflags '-s -w -extldflags=-Wl,-rpath,/userdata/weikong/lib' -o build/iec61850-mms-armv7/weikong-gateway-topology-20260912 ./cmd/gateway
sha256sum build/iec61850-mms-armv7/weikong-gateway-topology-20260912
arm-linux-gnueabihf-readelf -d build/iec61850-mms-armv7/weikong-gateway-topology-20260912
