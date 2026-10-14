#!/bin/sh
set -eu
vendor_init=/etc/init.d/S99vanxoak_pilot
vendor_bin=/usr/bin/vanxoak_pilot
backup=/userdata/weikong/backups/usb-guard-20260910
test -f "$vendor_init"
test -x "$vendor_bin"
mkdir -p "$backup"
chmod 700 "$backup"
if ! test -f "$backup/S99vanxoak_pilot.original"; then
    cp -p "$vendor_init" "$backup/S99vanxoak_pilot.original"
fi
# The payload is prepared locally and uploaded alongside this script.
test -f /tmp/weikong-vendor-pilot-disabled.init
cp /tmp/weikong-vendor-pilot-disabled.init "$vendor_init"
chmod 755 "$vendor_init"
for vendor_pid in $(pidof vanxoak_pilot 2>/dev/null || true); do
    test "$(readlink /proc/$vendor_pid/exe)" = "$vendor_bin"
    kill -TERM "$vendor_pid"
done
date
echo 'Vendor automatic dialer disabled; original init retained in:'
echo "$backup/S99vanxoak_pilot.original"
