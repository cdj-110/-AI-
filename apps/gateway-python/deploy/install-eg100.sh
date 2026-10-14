#!/bin/sh
set -eu

ROOT=/mnt/udisk/.weikong/python
RUNTIME_NEW="$ROOT/runtime.new"
STAGING="$ROOT/staging"
PYTHON="$RUNTIME_NEW/python/bin/python3"
SITE="$RUNTIME_NEW/python/lib/python3.10/site-packages"

mkdir -p "$SITE"
for wheel in "$STAGING"/wheels/*.whl; do
  unzip -oq "$wheel" -d "$SITE"
done

"$PYTHON" -c 'import fastapi, uvicorn, pymodbus, paho.mqtt.client, wsproto; print(fastapi.__version__, uvicorn.__version__, pymodbus.__version__)'
cd "$STAGING/current"
PYTHONPATH=. "$PYTHON" -m gateway.main --config /userdata/weikong/config/config.local.json --check-config

rm -rf "$ROOT/runtime" "$ROOT/current"
mv "$RUNTIME_NEW" "$ROOT/runtime"
mv "$STAGING/current" "$ROOT/current"
chmod 755 "$ROOT/current/run-eg100.sh"
echo "EG100 Python application installed"
