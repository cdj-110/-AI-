#!/bin/sh
set -eu

SOURCE_DIR="${1:-/tmp/weikong-opcda-v11-20260825-1536}"
APP_DIR=/userdata/weikong
DAEMON="$APP_DIR/bin/weikong-gateway"
LIBRARY="$APP_DIR/lib/libwk_opcda.so.0"
CONFIG="$APP_DIR/config/config.local.json"
INIT_SCRIPT=/etc/init.d/S99weikong-gateway
BACKUP_DIR="$APP_DIR/backups/opcda-v11-20260825"
BACKUP="$BACKUP_DIR/weikong-gateway-pre-opcda-v11.gz"
TEMP_BACKUP=/tmp/weikong-gateway-pre-opcda-v11.gz
OLD_HASH=4cccb24019a4da7db4dc708217a25521e2084a7016a5d6a090e72effe20dacca
NEW_HASH=5a4886cde51f575be9c97a7da3abeb583d577b525342b78a6e383b2c850d451e
LIBRARY_HASH=780c966ba2e935d3c58b5d4bdfb888b6a3002dcc66dd397b4c186171169e6c05

hash_of() {
    sha256sum "$1" | cut -d ' ' -f 1
}

wait_for_health() {
    count=0
    while [ "$count" -lt 15 ]; do
        if wget -qO /tmp/weikong-production-health.json \
            http://127.0.0.1:8088/api/healthz; then
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    return 1
}

rollback() {
    echo "Production health check failed; restoring previous gateway"
    "$INIT_SCRIPT" stop || true
    gzip -dc "$TEMP_BACKUP" >"$DAEMON.rollback"
    chmod 755 "$DAEMON.rollback"
    if [ "$(hash_of "$DAEMON.rollback")" != "$OLD_HASH" ]; then
        echo "Rollback binary hash mismatch" >&2
        return 1
    fi
    mv "$DAEMON.rollback" "$DAEMON"
    "$INIT_SCRIPT" start
    wait_for_health
}

test -x "$DAEMON"
test -f "$CONFIG"
test -x "$INIT_SCRIPT"
test -f "$SOURCE_DIR/bin/weikong-gateway"
test -f "$SOURCE_DIR/lib/libwk_opcda.so.0"

if [ "$(hash_of "$DAEMON")" != "$OLD_HASH" ]; then
    echo "Current production gateway is not the expected 20260824 build" >&2
    exit 2
fi
if [ "$(hash_of "$SOURCE_DIR/bin/weikong-gateway")" != "$NEW_HASH" ]; then
    echo "New gateway hash mismatch" >&2
    exit 3
fi
if [ "$(hash_of "$SOURCE_DIR/lib/libwk_opcda.so.0")" != "$LIBRARY_HASH" ]; then
    echo "OPC DA library hash mismatch" >&2
    exit 4
fi

gzip -c "$DAEMON" >"$TEMP_BACKUP"
gzip -t "$TEMP_BACKUP"

mkdir -p "$APP_DIR/lib" "$BACKUP_DIR"
cp "$SOURCE_DIR/lib/libwk_opcda.so.0" "$LIBRARY.new"
chmod 755 "$LIBRARY.new"
cp "$SOURCE_DIR/bin/weikong-gateway" "$DAEMON.opcda-v11.new"
chmod 755 "$DAEMON.opcda-v11.new"

test "$(hash_of "$LIBRARY.new")" = "$LIBRARY_HASH"
test "$(hash_of "$DAEMON.opcda-v11.new")" = "$NEW_HASH"
env LD_LIBRARY_PATH="$SOURCE_DIR/lib" \
    "$DAEMON.opcda-v11.new" -config "$CONFIG" -check-config

if [ -f "$SOURCE_DIR/data/gateway.pid" ]; then
    sidecar_pid="$(cat "$SOURCE_DIR/data/gateway.pid" 2>/dev/null || true)"
    if [ -n "$sidecar_pid" ] && kill -0 "$sidecar_pid" 2>/dev/null &&
       tr '\000' ' ' <"/proc/$sidecar_pid/cmdline" | grep -Fq "$SOURCE_DIR"; then
        kill -TERM "$sidecar_pid"
        count=0
        while kill -0 "$sidecar_pid" 2>/dev/null && [ "$count" -lt 10 ]; do
            sleep 1
            count=$((count + 1))
        done
        if kill -0 "$sidecar_pid" 2>/dev/null; then
            kill -KILL "$sidecar_pid"
        fi
    fi
fi

mv "$LIBRARY.new" "$LIBRARY"
mv "$DAEMON.opcda-v11.new" "$DAEMON"

if ! "$INIT_SCRIPT" restart || ! wait_for_health; then
    rollback
    exit 20
fi

if [ ! -f "$BACKUP" ]; then
    cp "$TEMP_BACKUP" "$BACKUP.tmp"
    gzip -t "$BACKUP.tmp"
    mv "$BACKUP.tmp" "$BACKUP"
fi
rm -f "$TEMP_BACKUP"

echo "DEPLOY_OK"
sha256sum "$DAEMON" "$LIBRARY" "$BACKUP"
cat /tmp/weikong-production-health.json
echo
"$INIT_SCRIPT" status
ps w | grep '[w]eikong-gateway'
df -k /userdata
