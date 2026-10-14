"""Small password-authenticated SSH/SFTP helper for lab gateway deployment.

The password is read from WK_SSH_PASSWORD and is never written to disk or
printed. This helper deliberately exposes only command execution and file
upload; backup/rollback decisions stay visible in the caller's shell commands.
"""

from __future__ import annotations

import argparse
import os
import sys

import paramiko


def connect(host: str, user: str, timeout: int) -> paramiko.SSHClient:
    password = os.environ.get("WK_SSH_PASSWORD", "")
    if not password:
        raise RuntimeError("WK_SSH_PASSWORD is required")
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(
        host,
        username=user,
        password=password,
        timeout=timeout,
        auth_timeout=timeout,
        banner_timeout=timeout,
        allow_agent=False,
        look_for_keys=False,
    )
    return client


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("operation", choices=("exec", "put"))
    parser.add_argument("host")
    parser.add_argument("user")
    parser.add_argument("source")
    parser.add_argument("destination", nargs="?")
    parser.add_argument("--timeout", type=int, default=10)
    args = parser.parse_args()

    client = connect(args.host, args.user, args.timeout)
    try:
        if args.operation == "put":
            if not args.destination:
                parser.error("put requires local source and remote destination")
            with client.open_sftp() as sftp:
                sftp.put(args.source, args.destination)
            print(f"uploaded {args.source} -> {args.host}:{args.destination}")
            return 0

        _, stdout, stderr = client.exec_command(args.source, timeout=max(args.timeout, 30))
        out = stdout.read()
        err = stderr.read()
        if out:
            sys.stdout.buffer.write(out)
        if err:
            sys.stderr.buffer.write(err)
        return stdout.channel.recv_exit_status()
    finally:
        client.close()


if __name__ == "__main__":
    raise SystemExit(main())
