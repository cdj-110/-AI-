#!/usr/bin/env python3
"""Safely update only the EG100 Go gateway while preserving config and OPC DA library."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import os
import shlex
import shutil
import tempfile
from pathlib import Path

import paramiko


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="192.168.1.214")
    parser.add_argument("--user", default="root")
    parser.add_argument("--gateway", required=True, type=Path)
    parser.add_argument("--backup-label", required=True)
    parser.add_argument("--usb-root", default="/mnt/udisk")
    parser.add_argument("--low-space", action="store_true", help="Back up to USB, then replace the old binary before expanding the new one")
    return parser.parse_args()


def run(client: paramiko.SSHClient, command: str, timeout: int = 120) -> str:
    _, stdout, stderr = client.exec_command(command, timeout=timeout)
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status = stdout.channel.recv_exit_status()
    if status != 0:
        raise RuntimeError(error.strip() or output.strip())
    return output


def upload_over_ssh(client: paramiko.SSHClient, source: Path, destination: str) -> None:
    transport = client.get_transport()
    if transport is None:
        raise RuntimeError("SSH transport is unavailable")
    partial = destination + ".part"
    run(client, f"rm -f {shlex.quote(partial)}")
    channel = transport.open_session(
        window_size=64 * 1024 * 1024,
        max_packet_size=1024 * 1024,
        timeout=20,
    )
    channel.settimeout(300)
    try:
        channel.exec_command(f"cat > {shlex.quote(partial)}")
        with source.open("rb") as stream:
            while chunk := stream.read(1024 * 1024):
                channel.sendall(chunk)
        channel.shutdown_write()
        status = channel.recv_exit_status()
        error = b""
        while channel.recv_stderr_ready():
            error += channel.recv_stderr(65536)
        if status != 0:
            raise RuntimeError(error.decode("utf-8", "replace").strip() or "SSH upload failed")
    finally:
        channel.close()
    expected_size = source.stat().st_size
    actual_size = int(run(client, f"wc -c < {shlex.quote(partial)}").strip())
    if actual_size != expected_size:
        run(client, f"rm -f {shlex.quote(partial)}")
        raise RuntimeError(f"SSH upload size mismatch: expected {expected_size}, got {actual_size}")
    run(client, f"mv {shlex.quote(partial)} {shlex.quote(destination)}")


def main() -> int:
    args = parse_args()
    password = os.environ.get("WK_EG100_PASSWORD", "")
    if not password:
        raise RuntimeError("WK_EG100_PASSWORD is required")
    gateway = args.gateway.resolve()
    if not gateway.is_file():
        raise FileNotFoundError(gateway)
    expected_hash = hashlib.sha256(gateway.read_bytes()).hexdigest()
    safe_label = "".join(
        character if character.isalnum() or character in "-_" else "-"
        for character in args.backup_label
    ).strip("-")
    if not safe_label:
        raise RuntimeError("backup label is empty after normalization")

    usb_root = args.usb_root.rstrip("/")
    if usb_root not in {"/mnt/udisk", "/media/udisk1"}:
        raise RuntimeError("--usb-root must be /mnt/udisk or /media/udisk1")
    remote_stage_dir = f"{usb_root}/.weikong-deploy"
    # EG100 has less than 100 MiB RAM and /tmp is tmpfs, while the USB volume
    # is mounted noexec. Keep the expanded executable beside the live binary,
    # validate it there, then atomically rename it into place.
    remote_temp = f"/userdata/weikong/bin/weikong-gateway-{safe_label}.new"
    remote_archive = f"{remote_stage_dir}/weikong-gateway-{safe_label}.gz"
    archive_path: Path | None = None
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    try:
        with tempfile.NamedTemporaryFile(prefix="weikong-gateway-", suffix=".gz", delete=False) as archive:
            archive_path = Path(archive.name)
            with gateway.open("rb") as source, gzip.GzipFile(fileobj=archive, mode="wb", mtime=0) as compressed:
                shutil.copyfileobj(source, compressed, length=1024 * 1024)

        client.connect(args.host, username=args.user, password=password, timeout=15)
        run(
            client,
            f"grep -q ' {shlex.quote(usb_root)} ' /proc/mounts && mkdir -p {shlex.quote(remote_stage_dir)}",
        )
        # The EG100 SFTP server stalls on multi-megabyte uploads. Stream the
        # archive over the authenticated SSH channel and verify its byte count.
        upload_over_ssh(client, archive_path, remote_archive)

        command = f"""
set -eu
wk_daemon=/userdata/weikong/bin/weikong-gateway
wk_library_dir=/userdata/weikong/lib
wk_config=/userdata/weikong/config/config.local.json
wk_init=/etc/init.d/S99weikong-gateway
wk_temp={shlex.quote(remote_temp)}
wk_archive={shlex.quote(remote_archive)}
wk_expected={shlex.quote(expected_hash)}
wk_usb_root={shlex.quote(usb_root)}
wk_low_space={'1' if args.low_space else '0'}
wk_backup_dir="$wk_usb_root/weikong-backups/{shlex.quote(safe_label)}"
wk_backup="$wk_backup_dir/weikong-gateway-before-update.gz"
wk_alarm_dir=/userdata/weikong/config/.runtime
wk_alarm_backup_dir="$wk_backup_dir/alarm-history"
wk_history_dir=/userdata/weikong/data
wk_history_backup_dir="$wk_backup_dir/history-storage"

test -x "$wk_daemon"
test -f "$wk_config"
test -f "$wk_library_dir/libwk_opcda.so.0"
grep -q " $wk_usb_root " /proc/mounts
gzip -t "$wk_archive"
if [ "$wk_low_space" = 0 ]; then
    gzip -dc "$wk_archive" >"$wk_temp"
    chmod 755 "$wk_temp"
    test "$(sha256sum "$wk_temp" | cut -d ' ' -f 1)" = "$wk_expected"
fi

mkdir -p "$wk_backup_dir"
gzip -c "$wk_daemon" >"$wk_backup.tmp"
gzip -t "$wk_backup.tmp"
mv "$wk_backup.tmp" "$wk_backup"

backup_alarm_history() {{
    mkdir -p "$wk_alarm_backup_dir"
    for wk_alarm_name in alarm-events.jsonl alarm-events.jsonl.migrated alarm-events.db alarm-events.db-wal alarm-events.db-shm; do
        if [ -f "$wk_alarm_dir/$wk_alarm_name" ]; then
            gzip -c "$wk_alarm_dir/$wk_alarm_name" >"$wk_alarm_backup_dir/$wk_alarm_name.gz.tmp" || return 1
            gzip -t "$wk_alarm_backup_dir/$wk_alarm_name.gz.tmp" || return 1
            mv "$wk_alarm_backup_dir/$wk_alarm_name.gz.tmp" "$wk_alarm_backup_dir/$wk_alarm_name.gz" || return 1
        fi
    done
}}

restore_alarm_history() {{
    mkdir -p "$wk_alarm_dir"
    rm -f "$wk_alarm_dir/alarm-events.jsonl" "$wk_alarm_dir/alarm-events.jsonl.migrated" \
        "$wk_alarm_dir/alarm-events.db" "$wk_alarm_dir/alarm-events.db-wal" "$wk_alarm_dir/alarm-events.db-shm"
    for wk_alarm_name in alarm-events.jsonl alarm-events.jsonl.migrated alarm-events.db alarm-events.db-wal alarm-events.db-shm; do
        if [ -f "$wk_alarm_backup_dir/$wk_alarm_name.gz" ]; then
            gzip -dc "$wk_alarm_backup_dir/$wk_alarm_name.gz" >"$wk_alarm_dir/$wk_alarm_name"
        fi
    done
}}

backup_history_storage() {{
    mkdir -p "$wk_history_backup_dir"
    for wk_history_name in telemetry-history.db telemetry-history.db-wal telemetry-history.db-shm; do
        if [ -f "$wk_history_dir/$wk_history_name" ]; then
            gzip -c "$wk_history_dir/$wk_history_name" >"$wk_history_backup_dir/$wk_history_name.gz.tmp" || return 1
            gzip -t "$wk_history_backup_dir/$wk_history_name.gz.tmp" || return 1
            mv "$wk_history_backup_dir/$wk_history_name.gz.tmp" "$wk_history_backup_dir/$wk_history_name.gz" || return 1
        fi
    done
}}

restore_history_storage() {{
    mkdir -p "$wk_history_dir"
    rm -f "$wk_history_dir/telemetry-history.db" "$wk_history_dir/telemetry-history.db-wal" \
        "$wk_history_dir/telemetry-history.db-shm"
    for wk_history_name in telemetry-history.db telemetry-history.db-wal telemetry-history.db-shm; do
        if [ -f "$wk_history_backup_dir/$wk_history_name.gz" ]; then
            gzip -dc "$wk_history_backup_dir/$wk_history_name.gz" >"$wk_history_dir/$wk_history_name"
        fi
    done
}}

release_page_cache() {{
    sync
    if [ -w /proc/sys/vm/drop_caches ]; then
        echo 3 >/proc/sys/vm/drop_caches
    fi
}}

rollback() {{
    "$wk_init" stop || true
    gzip -dc "$wk_backup" >"$wk_daemon"
    chmod 755 "$wk_daemon"
    restore_alarm_history
    restore_history_storage
    release_page_cache
    "$wk_init" start
}}

"$wk_init" stop
if ! backup_alarm_history; then
    "$wk_init" start
    exit 19
fi
if ! backup_history_storage; then
    "$wk_init" start
    exit 17
fi
if [ "$wk_low_space" = 1 ]; then
    # The rootfs cannot hold both ELF files at once. A verified USB backup is
    # already present; any failed write, hash or config check restores it.
    rm -f "$wk_daemon"
    if ! gzip -dc "$wk_archive" >"$wk_daemon" || ! chmod 755 "$wk_daemon"; then
        rollback
        exit 23
    fi
    if [ "$(sha256sum "$wk_daemon" | cut -d ' ' -f 1)" != "$wk_expected" ]; then
        rollback
        exit 24
    fi
    if ! env LD_LIBRARY_PATH="$wk_library_dir" "$wk_daemon" -config "$wk_config" -check-config; then
        rollback
        exit 18
    fi
else
    if ! env LD_LIBRARY_PATH="$wk_library_dir" "$wk_temp" -config "$wk_config" -check-config; then
        "$wk_init" start
        exit 18
    fi
    if ! mv "$wk_temp" "$wk_daemon" || ! chmod 755 "$wk_daemon"; then
        rollback
        exit 20
    fi
fi
rm -f "$wk_temp" "$wk_archive"
release_page_cache
if ! "$wk_init" start; then
    rollback
    exit 21
fi

wk_attempt=0
# Some field configurations open several protocol/database resources before the
# management listener becomes ready. Allow up to three minutes so a healthy
# gateway is not rolled back merely because startup takes longer than 20 s.
while [ "$wk_attempt" -lt 180 ]; do
    # The management listener can run as either HTTP or HTTPS. The EG100
    # BusyBox wget has no TLS support, so first use the full HTTP health check
    # and then fall back to verifying that the HTTPS listener accepts a local
    # TCP connection. This avoids rolling back a healthy TLS deployment.
    if wget -qO /tmp/weikong-health.json http://127.0.0.1:8088/api/healthz; then
        break
    fi
    if {{ exec 3<>/dev/tcp/127.0.0.1/8088; }} 2>/dev/null; then
        exec 3>&-
        echo '{{"healthy":true,"transport":"tls-tcp","detail":"HTTPS listener accepted local TCP connection"}}' > /tmp/weikong-health.json
        break
    fi
    sleep 1
    wk_attempt=$((wk_attempt + 1))
done
if [ "$wk_attempt" -ge 180 ]; then
    rollback
    exit 22
fi

echo DEPLOY_OK
sha256sum "$wk_daemon" "$wk_library_dir/libwk_opcda.so.0" "$wk_backup"
find "$wk_alarm_dir" -maxdepth 1 -name 'alarm-events*' -exec ls -l {{}} \\;
cat /tmp/weikong-health.json
echo
"$wk_init" status
"""
        print(run(client, command, timeout=360))
    finally:
        try:
            sftp = client.open_sftp()
            try:
                sftp.remove(remote_temp)
            except FileNotFoundError:
                pass
            try:
                sftp.remove(remote_archive)
            except FileNotFoundError:
                pass
            finally:
                sftp.close()
        finally:
            client.close()
            if archive_path is not None:
                archive_path.unlink(missing_ok=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
