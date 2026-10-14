#!/bin/sh
set -eu
cd /src

: "${WK_ACTIVATION_PUBLIC_KEY:?WK_ACTIVATION_PUBLIC_KEY is required for a production gateway build}"
: "${WK_ACTIVATION_TRUSTED_PUBLIC_KEYS:?WK_ACTIVATION_TRUSTED_PUBLIC_KEYS key ring is required for v2 licenses}"

export CGO_ENABLED=1
go test ./internal/remoteaccess ./internal/config

export GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc
export CGO_CFLAGS='-I/src/build/iec61850-mms-armv7/arm/include'
export CGO_LDFLAGS='-L/src/build/iec61850-mms-armv7/arm/lib -L/src/build/remote-phase1-armv7/lib -lm'

app_version="${WK_APP_VERSION:-V0.7}"
build_time="${WK_BUILD_TIME:-}"

out=/src/build/remote-access-auto-release-20260916
mkdir -p "$out"
go build -buildvcs=false -tags 'opcda,iec61850_mms,rjgt102' -trimpath \
  -ldflags "-s -w -X weikong-iot-platform/apps/gateway-go/internal/buildmeta.Version=${app_version} -X weikong-iot-platform/apps/gateway-go/internal/buildmeta.BuildTime=${build_time} -X weikong-iot-platform/apps/gateway-go/internal/config.ActivationBindingRequired=true -X weikong-iot-platform/apps/gateway-go/internal/config.ActivationSignatureRequired=true -X weikong-iot-platform/apps/gateway-go/internal/config.SecureChipRequired=true -X weikong-iot-platform/apps/gateway-go/internal/config.ActivationSignaturePublicKey=${WK_ACTIVATION_PUBLIC_KEY} -X 'weikong-iot-platform/apps/gateway-go/internal/config.ActivationTrustedPublicKeys=${WK_ACTIVATION_TRUSTED_PUBLIC_KEYS}' -X 'weikong-iot-platform/apps/gateway-go/internal/ota.TrustedPublicKeys=${WK_OTA_TRUSTED_PUBLIC_KEYS:-}' -extldflags=-Wl,-rpath,/userdata/weikong/lib" \
  -o "$out/weikong-gateway" ./cmd/gateway

sha256sum "$out/weikong-gateway"
arm-linux-gnueabihf-readelf -h "$out/weikong-gateway" | grep -E 'Class:|Machine:'
