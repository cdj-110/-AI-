"""Small deployment helper for password-authenticated EG100 test units."""

from __future__ import annotations

import argparse
import os
from pathlib import Path

import paramiko


def connect(host: str, password: str) -> paramiko.SSHClient:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, username="root", password=password, timeout=8)
    return client


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("host")
    parser.add_argument("--password", default=os.environ.get("EG100_PASSWORD", ""))
    parser.add_argument("--command")
    parser.add_argument("--upload", nargs=2, metavar=("LOCAL", "REMOTE"))
    parser.add_argument("--resume-upload", action="store_true")
    args = parser.parse_args()
    if not args.password:
        raise SystemExit("set EG100_PASSWORD or pass --password")

    client = connect(args.host, args.password)
    try:
        if args.upload:
            local, remote = args.upload
            with client.open_sftp() as sftp:
                local_path = Path(local).resolve()
                offset = 0
                if args.resume_upload:
                    try:
                        offset = sftp.stat(remote).st_size
                    except OSError:
                        offset = 0
                if offset > local_path.stat().st_size:
                    offset = 0
                with local_path.open("rb") as source, sftp.open(remote, "ab" if offset else "wb") as target:
                    source.seek(offset)
                    while chunk := source.read(256 * 1024):
                        target.write(chunk)
                    target.flush()
        if args.command:
            _, stdout, stderr = client.exec_command(args.command, timeout=600)
            output = stdout.read().decode("utf-8", "replace")
            error = stderr.read().decode("utf-8", "replace")
            if output:
                print(output, end="")
            if error:
                print(error, end="")
            status = stdout.channel.recv_exit_status()
            if status:
                raise SystemExit(status)
    finally:
        client.close()


if __name__ == "__main__":
    main()
