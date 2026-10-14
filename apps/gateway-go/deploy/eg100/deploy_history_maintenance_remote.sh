#!/bin/sh
set -eu

expected_hash="$1"
backup_label="$2"
source_binary="/media/udisk1/.weikong-deploy/gateway-history-maintenance-armv7"
staged_binary="/tmp/gateway-history-maintenance-armv7"
daemon="/userdata/weikong/bin/weikong-gateway"
library_dir="/userdata/weikong/lib"
config="/userdata/weikong/config/config.local.json"
init_script="/etc/init.d/S99weikong-gateway"
backup_dir="/media/udisk1/weikong-backups/$backup_label"
alarm_dir="/userdata/weikong/config/.runtime"
history_dir="/userdata/weikong/data"

test -x "$daemon"
test -f "$config"
test -f "$library_dir/libwk_opcda.so.0"
test -f "$source_binary"
grep -q ' /media/udisk1 ' /proc/mounts
test "$(sha256sum "$source_binary" | cut -d ' ' -f 1)" = "$expected_hash"
cp "$source_binary" "$staged_binary"
chmod 755 "$staged_binary"
test "$(sha256sum "$staged_binary" | cut -d ' ' -f 1)" = "$expected_hash"

mkdir -p "$backup_dir/alarm-history" "$backup_dir/history-storage"
config_hash_before="$(sha256sum "$config" | cut -d ' ' -f 1)"
gzip -c "$daemon" >"$backup_dir/weikong-gateway-before-update.gz.tmp"
gzip -t "$backup_dir/weikong-gateway-before-update.gz.tmp"
mv "$backup_dir/weikong-gateway-before-update.gz.tmp" "$backup_dir/weikong-gateway-before-update.gz"

backup_file() {
    source_file="$1"
    target_file="$2"
    if [ -f "$source_file" ]; then
        gzip -c "$source_file" >"$target_file.tmp"
        gzip -t "$target_file.tmp"
        mv "$target_file.tmp" "$target_file"
    fi
}

restore_file() {
    source_file="$1"
    target_file="$2"
    if [ -f "$source_file" ]; then
        gzip -dc "$source_file" >"$target_file"
    fi
}

restore_runtime_data() {
    rm -f "$alarm_dir/alarm-events.jsonl" "$alarm_dir/alarm-events.jsonl.migrated" \
        "$alarm_dir/alarm-events.db" "$alarm_dir/alarm-events.db-wal" "$alarm_dir/alarm-events.db-shm"
    for name in alarm-events.jsonl alarm-events.jsonl.migrated alarm-events.db alarm-events.db-wal alarm-events.db-shm; do
        restore_file "$backup_dir/alarm-history/$name.gz" "$alarm_dir/$name"
    done
    rm -f "$history_dir/telemetry-history.db" "$history_dir/telemetry-history.db-wal" \
        "$history_dir/telemetry-history.db-shm"
    for name in telemetry-history.db telemetry-history.db-wal telemetry-history.db-shm; do
        restore_file "$backup_dir/history-storage/$name.gz" "$history_dir/$name"
    done
}

rollback() {
    "$init_script" stop || true
    gzip -dc "$backup_dir/weikong-gateway-before-update.gz" >"$daemon"
    chmod 755 "$daemon"
    restore_runtime_data
    "$init_script" start
}

"$init_script" stop
for name in alarm-events.jsonl alarm-events.jsonl.migrated alarm-events.db alarm-events.db-wal alarm-events.db-shm; do
    backup_file "$alarm_dir/$name" "$backup_dir/alarm-history/$name.gz"
done
for name in telemetry-history.db telemetry-history.db-wal telemetry-history.db-shm; do
    backup_file "$history_dir/$name" "$backup_dir/history-storage/$name.gz"
done

if ! env LD_LIBRARY_PATH="$library_dir" "$staged_binary" -config "$config" -check-config; then
    "$init_script" start
    exit 18
fi
if ! cp "$staged_binary" "$daemon" || ! chmod 755 "$daemon"; then
    rollback
    exit 20
fi
if ! "$init_script" start; then
    rollback
    exit 21
fi

attempt=0
while [ "$attempt" -lt 25 ]; do
    if wget -qO /tmp/weikong-health.json http://127.0.0.1:8088/api/healthz; then
        break
    fi
    sleep 1
    attempt=$((attempt + 1))
done
if [ "$attempt" -ge 25 ]; then
    rollback
    exit 22
fi

config_hash_after="$(sha256sum "$config" | cut -d ' ' -f 1)"
if [ "$config_hash_before" != "$config_hash_after" ]; then
    rollback
    exit 23
fi
test "$(sha256sum "$daemon" | cut -d ' ' -f 1)" = "$expected_hash"
rm -f "$staged_binary"

echo DEPLOY_OK
echo "CONFIG_SHA256=$config_hash_after"
echo "BACKUP_DIR=$backup_dir"
sha256sum "$daemon" "$library_dir/libwk_opcda.so.0" "$backup_dir/weikong-gateway-before-update.gz"
cat /tmp/weikong-health.json
echo
"$init_script" status
