#!/usr/bin/env python3
"""Atomically configure the EG100 outbound remote-management tunnel."""

from __future__ import annotations

import argparse
import json
import os
import shlex
from datetime import datetime, timezone

import paramiko


CONFIG_PATH = "/userdata/weikong/config/config.local.json"
BACKUP_ROOT = "/media/udisk1/weikong-backups/remote-access"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="192.168.1.214")
    parser.add_argument("--user", default="root")
    parser.add_argument("--endpoint", required=True)
    parser.add_argument("--local-url", default="http://127.0.0.1:8088")
    parser.add_argument("--local-insecure-skip-verify", action="store_true")
    parser.add_argument("--disable", action="store_true")
    return parser.parse_args()


def run(client: paramiko.SSHClient, command: str) -> str:
    _, stdout, stderr = client.exec_command(command, timeout=30)
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status = stdout.channel.recv_exit_status()
    if status != 0:
        raise RuntimeError(error.strip() or output.strip())
    return output.strip()


def main() -> int:
    args = parse_args()
    password = os.environ.get("WK_EG100_PASSWORD", "")
    if not password:
        raise RuntimeError("WK_EG100_PASSWORD is required")
    if not args.endpoint.startswith(("ws://", "wss://")):
        raise ValueError("endpoint must use ws:// or wss://")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(args.host, username=args.user, password=password, timeout=15)
    try:
        sftp = client.open_sftp()
        try:
            with sftp.open(CONFIG_PATH, "r") as source:
                config = json.loads(source.read().decode("utf-8"))
            # Deliberately omit username/password: the gateway reuses its bound
            # factory activation credential, avoiding another long-lived secret.
            config["remoteAccess"] = {
                "enabled": not args.disable,
                "endpoint": args.endpoint,
                "localUrl": args.local_url,
                "insecureSkipVerify": False,
                "localInsecureSkipVerify": args.local_insecure_skip_verify,
            }
            payload = json.dumps(config, ensure_ascii=False, indent=2).encode("utf-8") + b"\n"
            temporary = CONFIG_PATH + ".remote-access.new"
            with sftp.open(temporary, "w") as target:
                target.write(payload)
        finally:
            sftp.close()

        timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        backup = f"{BACKUP_ROOT}/config.local.before-{timestamp}.json"
        print(run(client, f"""
set -eu
test -f {shlex.quote(CONFIG_PATH)}
grep -q ' /media/udisk1 ' /proc/mounts
mkdir -p {shlex.quote(BACKUP_ROOT)}
cp {shlex.quote(CONFIG_PATH)} {shlex.quote(backup)}
chmod 600 {shlex.quote(backup)} {shlex.quote(CONFIG_PATH + '.remote-access.new')}
mv {shlex.quote(CONFIG_PATH + '.remote-access.new')} {shlex.quote(CONFIG_PATH)}
echo CONFIGURED
echo {shlex.quote(backup)}
"""))
    finally:
        client.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
