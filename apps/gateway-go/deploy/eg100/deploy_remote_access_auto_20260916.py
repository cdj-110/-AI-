"""Back up and deploy the simplified cloud/remote-access release to EG100."""
import gzip
import hashlib
import json
import re
import time
from pathlib import Path

from mysql_live_20260912 import Gateway


BASE = Path("build/remote-access-auto-release-20260916")
PAYLOAD = BASE / "weikong-gateway"


def save(name, value):
    BASE.mkdir(parents=True, exist_ok=True)
    (BASE / f"{name}.json").write_text(
        json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(name, json.dumps(value, ensure_ascii=False), flush=True)


def local_json(gateway, path):
    return json.loads(
        gateway.shell(f"wget -T 5 -qO- http://127.0.0.1:8088{path}")
    )


def main():
    payload = PAYLOAD.read_bytes()
    new_hash = hashlib.sha256(payload).hexdigest()
    gateway = Gateway()
    try:
        health_before = local_json(gateway, "/api/healthz")
        old_hash = gateway.shell(
            "sha256sum /userdata/weikong/bin/weikong-gateway"
        ).split()[0]
        with gateway.ssh.open_sftp() as sftp:
            with sftp.open("/userdata/weikong/config/config.local.json") as source:
                config_hash = hashlib.sha256(source.read()).hexdigest()
            old_size = sftp.stat("/userdata/weikong/bin/weikong-gateway").st_size

            flash_free = int(
                gateway.shell("df -Pk /userdata").splitlines()[-1].split()[3]
            ) * 1024
            usb_free = int(
                gateway.shell("df -Pk /mnt/udisk").splitlines()[-1].split()[3]
            ) * 1024
            if flash_free + old_size - len(payload) < 4 * 1024 * 1024:
                raise RuntimeError("EG100 flash headroom would fall below 4 MB")
            if usb_free < 512 * 1024 * 1024:
                raise RuntimeError("EG100 USB backup space is below 512 MB")

            stamp = time.strftime("%Y%m%d-%H%M%S")
            backup = f"/mnt/udisk/.weikong-deploy/remote-access-auto-backup-{stamp}"
            archive = f"/mnt/udisk/.weikong-deploy/remote-access-auto-{stamp}.gz"
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
            deploy_script = Path(
                "deploy/eg100/deploy_remote_config_20260915.sh"
            ).read_bytes()
            with sftp.open(f"{backup}/deploy.sh", "wb") as target:
                target.write(deploy_script)

        save(
            "manifest",
            {
                "new": new_hash,
                "old": old_hash,
                "config": config_hash,
                "backup": backup,
                "archive": archive,
                "size": len(payload),
            },
        )
        save("health-before", health_before)

        output = gateway.shell(
            f"sh {backup}/deploy.sh {backup} {archive} {new_hash} {old_hash} {config_hash} "
            f"> {backup}/deploy.log 2>&1; result=$?; cat {backup}/deploy.log; exit $result",
            timeout=600,
        )
        save("deployment", output)

        health_after = local_json(gateway, "/api/healthz")
        ready_after = local_json(gateway, "/api/readyz")
        if not health_after["healthy"] or not ready_after["ready"]:
            raise RuntimeError("EG100 did not become healthy and ready after deployment")
        if health_after["pointCount"] != health_before["pointCount"]:
            raise RuntimeError("Point count changed during deployment")
        if (
            gateway.shell("sha256sum /userdata/weikong/config/config.local.json").split()[0]
            != config_hash
        ):
            raise RuntimeError("Gateway configuration changed during deployment")
        if (
            gateway.shell("sha256sum /userdata/weikong/bin/weikong-gateway").split()[0]
            != new_hash
        ):
            raise RuntimeError("Deployed binary hash does not match release")

        index = Path("internal/web/frontend/vue-dist/index.html").read_text(
            encoding="utf-8"
        )
        assets = re.findall(r'(?:src|href)="(/assets/[^\"]+)"', index)
        if len(assets) < 2:
            raise RuntimeError("Deployed page did not reference expected assets")

        time.sleep(5)
        tunnel = gateway.shell(
            "netstat -tn 2>/dev/null | grep '192.168.1.141:3100' || true"
        ).strip()
        save(
            "verification",
            {
                "verified": True,
                "configUnchanged": True,
                "health": health_after,
                "ready": ready_after,
                "assets": assets,
                "remoteTunnelConnection": tunnel,
                "platform": gateway.shell(
                    "df -Pk /userdata /mnt/udisk; /etc/init.d/S99weikong-gateway status"
                ),
            },
        )
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
