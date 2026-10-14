"""Install a dedicated LAN CA on EG100 after backing up its trust bundle."""

from __future__ import annotations

import hashlib
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "apps/gateway-go/deploy/eg100"))
from mysql_live_20260912 import Gateway  # noqa: E402

BUNDLE = "/etc/ssl/certs/ca-certificates.crt"
BACKUP = "/mnt/udisk/weikong-backups/https-lan-20260921/ca-certificates-before-ota.crt"
TEMP = BUNDLE + ".weikong-new"


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> None:
    if len(sys.argv) != 2 or not os.environ.get("WK_EG100_PASSWORD"):
        raise SystemExit("usage: WK_EG100_PASSWORD=... install-eg100-lan-ca.py <ca.crt>")
    ca = Path(sys.argv[1]).read_bytes()
    if not ca.startswith(b"-----BEGIN CERTIFICATE-----") or not ca.endswith(b"-----END CERTIFICATE-----\n"):
        raise ValueError("expected exactly one PEM certificate")
    gateway = Gateway()
    try:
        gateway.shell("grep -q ' /mnt/udisk ' /proc/mounts")
        with gateway.ssh.open_sftp() as sftp:
            with sftp.open(BUNDLE, "rb") as current:
                original = current.read()
            if ca in original:
                print("EG100 already trusts the LAN OTA CA")
                return
            combined = original + (b"" if original.endswith(b"\n") else b"\n") + ca
            gateway.shell("mkdir -p /mnt/udisk/weikong-backups/https-lan-20260921")
            try:
                with sftp.open(BACKUP, "rb") as saved:
                    if saved.read() != original:
                        raise RuntimeError("existing CA backup differs from current trust bundle; stop for manual review")
            except FileNotFoundError:
                gateway.shell(f"cp {BUNDLE} {BACKUP}")
                with sftp.open(BACKUP, "rb") as saved:
                    if saved.read() != original:
                        raise RuntimeError("CA backup verification failed")
            try:
                try:
                    sftp.stat(TEMP)
                except FileNotFoundError:
                    pass
                else:
                    raise RuntimeError("existing CA temporary file requires manual review")
                with sftp.open(TEMP, "wb") as output:
                    output.write(combined)
                sftp.chmod(TEMP, 0o644)
                with sftp.open(TEMP, "rb") as candidate:
                    if digest(candidate.read()) != digest(combined):
                        raise RuntimeError("new trust bundle verification failed")
                gateway.shell(f"mv {TEMP} {BUNDLE}")
            finally:
                try:
                    sftp.remove(TEMP)
                except FileNotFoundError:
                    pass
            with sftp.open(BUNDLE, "rb") as installed:
                if digest(installed.read()) != digest(combined):
                    raise RuntimeError("installed trust bundle verification failed")
            print(f"EG100 LAN CA installed; original trust bundle preserved at {BACKUP}")
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
