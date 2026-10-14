"""Back up and deploy the remote-config reliability release to EG100."""
import gzip
import hashlib
import json
import time
from pathlib import Path

from mysql_live_20260912 import Gateway


BASE = Path("build/remote-config-release-20260915")
PAYLOAD = BASE / "weikong-gateway-remote-config-20260915"


def save(name, value):
    BASE.mkdir(parents=True, exist_ok=True)
    (BASE / f"{name}.json").write_text(
        json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(name, json.dumps(value, ensure_ascii=False), flush=True)


def main():
    payload = PAYLOAD.read_bytes()
    new_hash = hashlib.sha256(payload).hexdigest()
    gateway = Gateway()
    try:
        gateway.login()
        health_before = gateway.api("/api/healthz")
        storage_before = gateway.api("/api/remote-storage/status")
        old_hash = gateway.shell("sha256sum /userdata/weikong/bin/weikong-gateway").split()[0]
        with gateway.ssh.open_sftp() as sftp:
            with sftp.open("/userdata/weikong/config/config.local.json") as source:
                config_hash = hashlib.sha256(source.read()).hexdigest()
            stamp = time.strftime("%Y%m%d-%H%M%S")
            backup = f"/mnt/udisk/.weikong-deploy/remote-config-backup-{stamp}"
            archive = f"/mnt/udisk/.weikong-deploy/remote-config-{stamp}.gz"
            gateway.shell(
                "grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && "
                f"test ! -e {backup} && mkdir -m 700 {backup} && "
                f"gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && "
                f"cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && "
                f"chmod 600 {backup}/* && sync"
            )
            compressed = gzip.compress(payload, mtime=0)
            with sftp.open(archive, "wb") as target:
                for offset in range(0, len(compressed), 32768):
                    target.write(compressed[offset : offset + 32768])
            script = Path("deploy/eg100/deploy_remote_config_20260915.sh").read_bytes()
            with sftp.open(f"{backup}/deploy.sh", "wb") as target:
                target.write(script)
        save("manifest", {
            "new": new_hash, "old": old_hash, "config": config_hash,
            "backup": backup, "archive": archive, "size": len(payload),
        })
        save("health-before", health_before)
        save("storage-before", storage_before)
        output = gateway.shell(
            f"sh {backup}/deploy.sh {backup} {archive} {new_hash} {old_hash} {config_hash} > {backup}/deploy.log 2>&1; "
            f"result=$?; cat {backup}/deploy.log; exit $result",
            timeout=600,
        )
        save("deployment", output)
        # Web sessions are intentionally invalidated when the gateway process restarts.
        gateway.cookies.clear()
        gateway.login()
        health_after = gateway.api("/api/healthz")
        assert health_after["healthy"] and gateway.api("/api/readyz")["ready"]
        assert health_after["pointCount"] == health_before["pointCount"]
        assert gateway.shell("sha256sum /userdata/weikong/config/config.local.json").split()[0] == config_hash
        assert gateway.shell("sha256sum /userdata/weikong/bin/weikong-gateway").split()[0] == new_hash
        time.sleep(8)
        storage_after = gateway.api("/api/remote-storage/status")
        assert all(item["cacheReady"] for item in storage_after["statuses"])
        save("verification", {
            "verified": True,
            "configUnchanged": True,
            "health": health_after,
            "storage": storage_after,
            "platform": gateway.shell("df -Pk /userdata /mnt/udisk; /etc/init.d/S99weikong-gateway status"),
        })
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
