#!/bin/sh
# Fixed-target, low-disk-space deployment for the table width release.
set -eu
wk_daemon=/userdata/weikong/bin/weikong-gateway
wk_config=/userdata/weikong/config/config.local.json
wk_library=/userdata/weikong/lib/libwk_opcda.so.0
wk_init=/etc/init.d/S99weikong-gateway
wk_backup=/media/udisk1/weikong-backups/table-width-20260909
wk_archive=/media/udisk1/.weikong-deploy/table-width-20260909.gz
wk_new=/userdata/weikong/bin/weikong-gateway.table-width-new
wk_expected=332923984203ebc732a48ba68a610c567bc39f9422ec6937905131c4b0ca5829
wk_stopped=0
wk_replaced=0
wk_complete=0

hash() { sha256sum "$1" | cut -d ' ' -f 1; }
finish() {
    wk_result=$?
    trap - EXIT INT TERM
    if [ "$wk_complete" = 0 ] && [ "$wk_stopped" = 1 ]; then
        "$wk_init" stop || true
        if [ "$wk_replaced" = 1 ]; then
            rm -f "$wk_new" "$wk_daemon"
            gzip -dc "$wk_backup/weikong-gateway.gz" > "$wk_new"
            test "$(hash "$wk_new")" = "$wk_old_hash"
            chmod 755 "$wk_new"
            mv "$wk_new" "$wk_daemon"
        fi
        "$wk_init" start
        echo ROLLED_BACK
    fi
    exit "$wk_result"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

grep -q ' /media/udisk1 ' /proc/mounts
test -x "$wk_daemon"
test ! -e "$wk_backup"
test ! -e "$wk_new"
gzip -t "$wk_archive"
test "$(gzip -dc "$wk_archive" | sha256sum | cut -d ' ' -f 1)" = "$wk_expected"
wk_old_hash=$(hash "$wk_daemon")
wk_config_hash=$(hash "$wk_config")
wk_library_hash=$(hash "$wk_library")
mkdir -m 700 "$wk_backup"
gzip -c "$wk_daemon" > "$wk_backup/weikong-gateway.gz"
gzip -t "$wk_backup/weikong-gateway.gz"
test "$(gzip -dc "$wk_backup/weikong-gateway.gz" | sha256sum | cut -d ' ' -f 1)" = "$wk_old_hash"
cp -p "$wk_config" "$wk_backup/config.local.json"
chmod 600 "$wk_backup/config.local.json"
for wk_db in alarm-events collection-incidents; do
    wk_db_path="/userdata/weikong/config/.runtime/$wk_db.db"
    if [ -f "$wk_db_path" ]; then
        sqlite3 -readonly "$wk_db_path" ".backup '$wk_backup/$wk_db.db'"
    fi
done
sync
wk_stopped=1
"$wk_init" stop
test -z "$(pidof weikong-gateway 2>/dev/null || true)"
# The old executable is removed only after its verified USB backup and a full stop.
wk_replaced=1
rm "$wk_daemon"
gzip -dc "$wk_archive" > "$wk_new"
chmod 755 "$wk_new"
test "$(hash "$wk_new")" = "$wk_expected"
env LD_LIBRARY_PATH=/userdata/weikong/lib "$wk_new" -config "$wk_config" -check-config
mv "$wk_new" "$wk_daemon"
sync
"$wk_init" start
wk_attempt=0
while [ "$wk_attempt" -lt 90 ]; do
    if wget -T 2 -qO- http://127.0.0.1:8088/api/readyz | grep -q '"ready":true'; then
        if wget -T 2 -qO- http://127.0.0.1:8088/api/healthz | grep -q '"healthy":true'; then
            break
        fi
    fi
    sleep 1
    wk_attempt=$((wk_attempt+1))
done
test "$wk_attempt" -lt 90
test "$(hash "$wk_daemon")" = "$wk_expected"
test "$(hash "$wk_config")" = "$wk_config_hash"
test "$(hash "$wk_library")" = "$wk_library_hash"
wk_complete=1
echo DEPLOY_OK
date
sha256sum "$wk_daemon" "$wk_config" "$wk_library"
wget -qO- http://127.0.0.1:8088/api/healthz
wget -qO- http://127.0.0.1:8088/api/readyz
df -h /userdata
