#!/bin/sh
# Fixed-target release. Original data is also backed up off-device before use.
set -eu
daemon=/userdata/weikong/bin/weikong-gateway
config=/userdata/weikong/config/config.local.json
candidate=/userdata/weikong/config/config.usb-guard-new.json
init=/etc/init.d/S99weikong-gateway
backup=/userdata/weikong/backups/usb-guard-20260910
archive=/mnt/udisk/.weikong-deploy/usb-guard-20260910.gz
new=/userdata/weikong/bin/weikong-gateway.usb-guard-new
expected=51301bb7fbad2e2d8e6841aeff7fa5b94d6ac598de95f1e376decba451d54ae6
old_expected=a695dcf49dffea9a9f49882099f3f9c1c71f5f6d5a890a59d612d32ab6085ab8
config_expected=${1:?original config hash required}
stopped=0
replaced=0
config_replaced=0
complete=0
hash() { sha256sum "$1" | cut -d ' ' -f 1; }
finish() {
    result=$?
    trap - EXIT INT TERM
    if [ "$complete" = 0 ] && [ "$stopped" = 1 ]; then
        "$init" stop || true
        if [ "$replaced" = 1 ]; then
            # Exact files only; original executable has a verified local backup.
            rm -f "$new" "$daemon"
            gzip -dc "$backup/weikong-gateway.original.gz" > "$new"
            test "$(hash "$new")" = "$old_expected"
            chmod 755 "$new"
            mv "$new" "$daemon"
        fi
        if [ "$config_replaced" = 1 ]; then
            cp -p "$backup/config.original.json" "$config"
        fi
        "$init" start
        echo ROLLED_BACK
    fi
    exit "$result"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

test -z "$(pidof vanxoak_pilot 2>/dev/null || true)"
grep -q '^/dev/sda4 /mnt/udisk ext4 ' /proc/mounts
# This board's BusyBox blkid ignores util-linux formatting switches.
blkid /dev/sda4 | grep -q 'UUID="a859efdd-6a14-415e-89e5-aa761bc1b46f"'
test "$(hash "$daemon")" = "$old_expected"
test "$(hash "$config")" = "$config_expected"
test -f "$candidate"
test ! -e "$new"
test ! -e "$backup/weikong-gateway.original.gz"
test ! -e "$backup/config.original.json"
gzip -t "$archive"
test "$(gzip -dc "$archive" | sha256sum | cut -d ' ' -f 1)" = "$expected"
# Keep the rollback executable on internal flash, not on the suspect USB bus.
gzip -c "$daemon" > "$backup/weikong-gateway.original.gz"
gzip -t "$backup/weikong-gateway.original.gz"
test "$(gzip -dc "$backup/weikong-gateway.original.gz" | sha256sum | cut -d ' ' -f 1)" = "$old_expected"
cp -p "$config" "$backup/config.original.json"
chmod 600 "$backup/weikong-gateway.original.gz" "$backup/config.original.json" "$candidate"
test "$(hash "$config")" = "$config_expected"
sync
stopped=1
"$init" stop
test -z "$(pidof weikong-gateway 2>/dev/null || true)"
test "$(hash "$config")" = "$config_expected"
replaced=1
rm "$daemon"
gzip -dc "$archive" > "$new"
chmod 755 "$new"
test "$(hash "$new")" = "$expected"
env LD_LIBRARY_PATH=/userdata/weikong/lib "$new" -config "$candidate" -check-config
mv "$new" "$daemon"
config_replaced=1
mv "$candidate" "$config"
sync
"$init" start
attempt=0
while [ "$attempt" -lt 90 ]; do
    if wget -T 2 -qO- http://127.0.0.1:8088/api/readyz | grep -q '"ready":true'; then
        if wget -T 2 -qO- http://127.0.0.1:8088/api/healthz | grep -q '"healthy":true'; then break; fi
    fi
    sleep 1
    attempt=$((attempt+1))
done
test "$attempt" -lt 90
test "$(hash "$daemon")" = "$expected"
complete=1
echo DEPLOY_OK
date
wget -qO- http://127.0.0.1:8088/api/healthz
wget -qO- http://127.0.0.1:8088/api/readyz
df -h /userdata /mnt/udisk
