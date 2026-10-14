#!/bin/sh
set -eu
daemon=/userdata/weikong/bin/weikong-gateway
config=/userdata/weikong/config/config.local.json
init=/etc/init.d/S99weikong-gateway
backup=${1:?backup directory}
archive=${2:?release archive}
expected=${3:?new hash}
old_expected=${4:?old hash}
config_expected=${5:?config hash}
new=/userdata/weikong/bin/weikong-gateway.remote-config-new
hash() { sha256sum "$1" | cut -d ' ' -f 1; }
stopped=0
replaced=0
complete=0
finish() {
  result=$?
  trap - EXIT INT TERM
  if [ "$complete" = 0 ] && [ "$stopped" = 1 ]; then
    "$init" stop || true
    if [ "$replaced" = 1 ]; then
      rm -f "$new" "$daemon"
      gzip -dc "$backup/weikong-gateway.original.gz" > "$new"
      test "$(hash "$new")" = "$old_expected"
      chmod 755 "$new"
      mv "$new" "$daemon"
    fi
    "$init" start
    echo ROLLED_BACK
  fi
  exit "$result"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts
test "$(hash "$daemon")" = "$old_expected"
test "$(hash "$config")" = "$config_expected"
test "$(gzip -dc "$archive" | sha256sum | cut -d ' ' -f 1)" = "$expected"
test "$(gzip -dc "$backup/weikong-gateway.original.gz" | sha256sum | cut -d ' ' -f 1)" = "$old_expected"
test "$(hash "$backup/config.original.json")" = "$config_expected"
test ! -e "$new"
stopped=1
"$init" stop
test -z "$(pidof weikong-gateway 2>/dev/null || true)"
for source in /userdata/weikong/config/.runtime/alarm-events.db /userdata/weikong/config/.runtime/collection-incidents.db /mnt/udisk/.weikong/history/telemetry-history.db /mnt/udisk/.remote-storage/eg100-pg-integration.db /mnt/udisk/.remote-storage/eg100-mysql-integration.db /mnt/udisk/.remote-storage/eg100-sqlserver-integration.db; do
  test -f "$source" || continue
  name=$(basename "$source")
  cp -p "$source" "$backup/$name"
  if [ -f "$source-wal" ]; then cp -p "$source-wal" "$backup/$name-wal"; fi
  chmod 600 "$backup/$name"*
  test "$(sqlite3 "$backup/$name" 'PRAGMA quick_check;')" = ok
  echo "BACKUP_VERIFIED $name"
done
sync
replaced=1
rm "$daemon"
gzip -dc "$archive" > "$new"
chmod 755 "$new"
test "$(hash "$new")" = "$expected"
env LD_LIBRARY_PATH=/userdata/weikong/lib "$new" -config "$config" -check-config
mv "$new" "$daemon"
sync
"$init" start
attempt=0
while [ "$attempt" -lt 120 ]; do
  if wget -T 2 -qO- http://127.0.0.1:8088/api/readyz | grep -q '"ready":true'; then
    if wget -T 2 -qO- http://127.0.0.1:8088/api/healthz | grep -q '"healthy":true'; then break; fi
  fi
  sleep 1
  attempt=$((attempt+1))
done
test "$attempt" -lt 120
test "$(hash "$daemon")" = "$expected"
test "$(hash "$config")" = "$config_expected"
complete=1
echo DEPLOY_OK
wget -qO- http://127.0.0.1:8088/api/healthz
df -k /userdata /mnt/udisk
