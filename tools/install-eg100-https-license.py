"""Preflight and import the signed HTTPS license through EG100's admin API."""

from __future__ import annotations

import hashlib
import json
import os
import shlex
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "apps/gateway-go/deploy/eg100"))
from mysql_live_20260912 import Gateway  # noqa: E402
from deploy_gateway_update import run  # noqa: E402

CONFIG = "/userdata/weikong/config/config.local.json"
BACKUP_DIR = "/mnt/udisk/weikong-backups/https-license-20260921"
EXPECTED_ORIGIN = "https://192.168.1.141:3443"


def read_remote(sftp, path: str) -> bytes:
    with sftp.open(path, "rb") as source:
        return source.read()


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def main() -> None:
    if len(sys.argv) != 2 or not os.environ.get("WK_EG100_PASSWORD") or not os.environ.get("WK_EG100_WEB_PASSWORD"):
        raise SystemExit("EG100 SSH and web credentials and a signed .lic file are required")
    license_file = Path(sys.argv[1])
    new = json.loads(license_file.read_text(encoding="utf-8"))
    if new.get("sn") != "WK20265AF2F5C6" or new.get("licenseSequence") != 9 or new.get("cloudApi") != EXPECTED_ORIGIN or new.get("expiresAt") != "":
        raise ValueError("unexpected HTTPS license identity, sequence, address or lifetime")
    gateway = Gateway()
    try:
        run(gateway.ssh, "grep -q ' /mnt/udisk ' /proc/mounts")
        with gateway.ssh.open_sftp() as sftp:
            config_raw = read_remote(sftp, CONFIG)
            cfg = json.loads(config_raw)
            old_path = cfg["activation"]["file"]
            old_raw = read_remote(sftp, old_path)
            old = json.loads(old_raw)
            for field in ["sn", "hardwareId", "chipUsid", "deviceSecret", "broker", "features"]:
                if new.get(field) != old.get(field):
                    raise ValueError(f"new license unexpectedly changed {field}")
            if old.get("licenseSequence") != 8 or not cfg.get("cloudConnectionEnabled"):
                raise ValueError("EG100 authorization or cloud connection changed; aborting")
            run(gateway.ssh, f"mkdir -p {shlex.quote(BACKUP_DIR)}")
            for source, target, expected in [
                (CONFIG, BACKUP_DIR + "/config-before-https.json", config_raw),
                (old_path, BACKUP_DIR + "/activation-seq8.json", old_raw),
            ]:
                try:
                    sftp.stat(target)
                except FileNotFoundError:
                    run(gateway.ssh, f"cp {shlex.quote(source)} {shlex.quote(target)}")
                else:
                    raise RuntimeError(f"backup target already exists: {target}")
                if digest(read_remote(sftp, target)) != digest(expected):
                    raise RuntimeError("EG100 pre-import backup verification failed")

        gateway.login()
        preview = gateway.api("/api/activation", "POST", {"content": json.dumps(new, ensure_ascii=False)})
        if preview.get("licenseSequence") != 9 or preview.get("sn") != new["sn"]:
            raise RuntimeError("EG100 signed-license preflight did not accept the new version")
        applied = gateway.api("/api/activation", "PUT", {"content": json.dumps(new, ensure_ascii=False)})
        if not applied.get("ok"):
            raise RuntimeError("EG100 did not confirm the import")
        print("EG100 signed HTTPS license accepted; waiting for the scheduled service restart", flush=True)
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            time.sleep(3)
            try:
                with gateway.ssh.open_sftp() as sftp:
                    current_cfg = json.loads(read_remote(sftp, CONFIG))
                    active = json.loads(read_remote(sftp, current_cfg["activation"]["file"]))
                if active.get("licenseSequence") != 9 or active.get("cloudApi") != EXPECTED_ORIGIN:
                    continue
                health = gateway.api("/api/healthz")
                if health.get("healthy") and health.get("pointCount") == 6:
                    print(json.dumps({"sn": active["sn"], "licenseSequence": 9, "cloudApi": active["cloudApi"], "cloudConnectionEnabled": current_cfg.get("cloudConnectionEnabled"), "healthy": True, "pointCount": health["pointCount"], "backupDirectory": BACKUP_DIR}), flush=True)
                    return
            except Exception:
                continue
        raise TimeoutError("license import completed but EG100 post-restart health was not confirmed")
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
