#!/bin/sh
set -eu
daemon=/userdata/weikong/bin/weikong-gateway
config=/userdata/weikong/config/config.local.json
init=/etc/init.d/S99weikong-gateway
backup=/mnt/udisk/.weikong-deploy/alarm-panel-only-backup-20260912
archive=/mnt/udisk/.weikong-deploy/alarm-panel-only-20260912.gz
new=/userdata/weikong/bin/weikong-gateway.alarm-panel-only-new
expected=${1:?new hash}
old_expected=${2:?old hash}
config_expected=${3:?config hash}
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
blkid /dev/sda4 | grep -q 'UUID="a859efdd-6a14-415e-89e5-aa761bc1b46f"'
test "$(hash "$daemon")" = "$old_expected"
test "$(hash "$config")" = "$config_expected"
test ! -e "$new"
gzip -t "$archive"
test "$(gzip -dc "$archive" | sha256sum | cut -d ' ' -f 1)" = "$expected"
test "$(gzip -dc "$backup/weikong-gateway.original.gz" | sha256sum | cut -d ' ' -f 1)" = "$old_expected"
test "$(hash "$backup/config.original.json")" = "$config_expected"
test "$(sqlite3 "$backup/alarm-events.original.db" 'PRAGMA integrity_check;')" = ok
sync
stopped=1
"$init" stop
test -z "$(pidof weikong-gateway 2>/dev/null || true)"
test "$(hash "$config")" = "$config_expected"
replaced=1
# Exact binary has two verified rollback copies before removal; flash cannot hold both.
rm "$daemon"
gzip -dc "$archive" > "$new"
chmod 755 "$new"
test "$(hash "$new")" = "$expected"
env LD_LIBRARY_PATH=/userdata/weikong/lib "$new" -config "$config" -check-config
mv "$new" "$daemon"
sync
"$init" start
attempt=0
while [ "$attempt" -lt 60 ]; do
  if wget -T 2 -qO- http://127.0.0.1:8088/api/readyz | grep -q '"ready":true'; then
    if wget -T 2 -qO- http://127.0.0.1:8088/api/healthz | grep -q '"healthy":true'; then break; fi
  fi
  sleep 1
  attempt=$((attempt+1))
done
test "$attempt" -lt 60
test "$(hash "$daemon")" = "$expected"
test "$(hash "$config")" = "$config_expected"
complete=1
echo DEPLOY_OK
date
wget -qO- http://127.0.0.1:8088/api/healthz
wget -qO- http://127.0.0.1:8088/api/readyz
df -k /userdata /mnt/udisk
