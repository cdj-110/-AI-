from __future__ import annotations

import argparse
import os
import shlex
import sys
import time
from pathlib import Path

import paramiko


def main() -> int:
    parser = argparse.ArgumentParser(description="Deploy the Python gateway to an Ubuntu host.")
    parser.add_argument("--host", required=True)
    parser.add_argument("--user", default="ubuntu")
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--password-env", default="WK_REMOTE_PASSWORD")
    args = parser.parse_args()

    password = os.environ.get(args.password_env, "")
    if not password:
        raise SystemExit(f"missing password environment variable: {args.password_env}")

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(args.host, username=args.user, password=password, timeout=10)

    def run(command: str, *, sudo: bool = False, timeout: int = 300) -> str:
        remote = f"sudo -S bash -lc {shlex.quote(command)}" if sudo else command
        stdin, stdout, stderr = client.exec_command(remote, get_pty=sudo, timeout=timeout)
        if sudo:
            stdin.write(password + "\n")
            stdin.flush()
        output = stdout.read().decode(errors="replace")
        error = stderr.read().decode(errors="replace")
        status = stdout.channel.recv_exit_status()
        if status:
            raise RuntimeError(f"remote command failed ({status}): {command}\n{output}\n{error}")
        if error.strip():
            output += "\n" + error
        return output

    remote_archive = "/tmp/weikong-gateway-python-arm64-current.tar.gz"
    with client.open_sftp() as sftp:
        sftp.put(str(args.archive.resolve()), remote_archive)

    run(
        """
set -e
stamp=$(date +%Y%m%d-%H%M%S)
if [ -d /opt/weikong-gateway-python ]; then
  mv /opt/weikong-gateway-python "/opt/weikong-gateway-python.backup-$stamp"
fi
mkdir -p /opt/weikong-gateway-python /etc/weikong-gateway-python
tar -xzf /tmp/weikong-gateway-python-arm64-current.tar.gz -C /opt/weikong-gateway-python
if [ ! -f /etc/weikong-gateway-python/config.json ]; then
  if [ -f /etc/weikong/config.local.json ]; then
    cp /etc/weikong/config.local.json /etc/weikong-gateway-python/config.json
  else
    cp /opt/weikong-gateway-python/config.example.json /etc/weikong-gateway-python/config.json
  fi
fi
cp /opt/weikong-gateway-python/weikong-gateway-python.service /etc/systemd/system/
python3 -m venv /opt/weikong-gateway-python/.venv
/opt/weikong-gateway-python/.venv/bin/pip install --upgrade 'pip<25'
/opt/weikong-gateway-python/.venv/bin/pip install -r /opt/weikong-gateway-python/requirements.txt
systemctl daemon-reload
systemctl enable --now weikong-gateway-python
rm -f /tmp/weikong-gateway-python-arm64-current.tar.gz
""",
        sudo=True,
        timeout=600,
    )
    print(run("systemctl --no-pager --full status weikong-gateway-python"))
    for attempt in range(30):
        try:
            print(run("curl -fsS --max-time 5 http://127.0.0.1:8089/login"))
            break
        except RuntimeError:
            if attempt == 29:
                raise
            time.sleep(1)
    client.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
