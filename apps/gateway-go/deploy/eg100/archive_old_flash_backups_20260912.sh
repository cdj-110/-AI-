#!/bin/sh
# Recoverable cleanup: relocate only two inspected, immutable old binaries.
set -eu
umask 077
dest=/mnt/udisk/.weikong-deploy/flash-backup-archive-20260912
grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts
blkid /dev/sda4 | grep -q 'UUID="a859efdd-6a14-415e-89e5-aa761bc1b46f"'
test ! -e "$dest"
mkdir -m 700 "$dest"
df -Pk /userdata /mnt/udisk
for label in dlt-select-20260910 usb-guard-20260910; do
  source=/userdata/weikong/backups/$label/weikong-gateway.original.gz
  target=$dest/$label.gateway.original.gz
  test "$(readlink -f "$source")" = "$source"
  test -f "$source"
  gzip -t "$source"
  cp -p "$source" "$target"
  chmod 600 "$target"
  sync
  test "$(sha256sum "$source" | cut -d ' ' -f 1)" = "$(sha256sum "$target" | cut -d ' ' -f 1)"
  gzip -t "$target"
  sha256sum "$source" "$target"
  # Exact inspected source only; the verified archive remains recoverable.
  rm -f "$source"
done
sync
echo ARCHIVE_COMPLETE
df -Pk /userdata /mnt/udisk
wget -T 3 -qO- http://127.0.0.1:8088/api/healthz
