#!/bin/sh
set -eu

ROOT=/mnt/udisk/.weikong/python
APP="$ROOT/current"
PYTHON="$ROOT/runtime/python/bin/python3"
CONFIG=/userdata/weikong/config/config.local.json

cd "$APP"
export WEIKONG_APP_DIR="$ROOT"
export WEIKONG_DISK_BYTES="$(($(du -sk "$ROOT" | cut -f1) * 1024))"
export PYTHONUNBUFFERED=1
exec "$PYTHON" -m gateway.main --config "$CONFIG" --listen 0.0.0.0:8088 >>/var/log/weikong-gateway-python.log 2>&1
