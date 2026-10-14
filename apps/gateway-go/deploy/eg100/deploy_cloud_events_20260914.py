"""Deploy the cloud alarm-event release to EG100 with verified rollback."""
import gzip
import hashlib
import http.client
import json
import re
import sys
import time
from pathlib import Path

from mysql_live_20260912 import Gateway


BASE = Path("build/cloud-events-release-20260914")
PAYLOAD = Path("build/weikong-gateway-cloud-events-20260914")
OLD = "91ab8b13850a92e7f6e3bce38f40fd796614484b3e17f53da4e486a63fa0c087"
NEW = "1b34354d9007405910a724f156e773066f2b02b7aae4162fb0e487864ecb2d74"
BACKUP = "/mnt/udisk/.weikong-deploy/cloud-events-backup-20260914"
ARCHIVE = "/mnt/udisk/.weikong-deploy/cloud-events-20260914.gz"


def save(name, value):
    BASE.mkdir(parents=True, exist_ok=True)
    (BASE / f"{name}.json").write_text(
        json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(name, json.dumps(value, ensure_ascii=False), flush=True)


def raw(gateway, path):
    connection = http.client.HTTPConnection("127.0.0.1", 8088, timeout=25)
    connection.sock = gateway.ssh.get_transport().open_channel(
        "direct-tcpip", ("127.0.0.1", 8088), ("127.0.0.1", 0), timeout=25
    )
    connection.sock.settimeout(25)
    try:
        cookie = "; ".join(key + "=" + value.value for key, value in gateway.cookies.items())
        connection.request("GET", path, headers={"Cookie": cookie})
        response = connection.getresponse()
        assert response.status == 200, (path, response.status)
        return response.read()
    finally:
        connection.close()


def deploy(gateway):
    payload = PAYLOAD.read_bytes()
    assert hashlib.sha256(payload).hexdigest() == NEW
    save("health-before", gateway.api("/api/healthz"))
    save("storage-before", gateway.api("/api/remote-storage/status"))
    current = gateway.shell("sha256sum /userdata/weikong/bin/weikong-gateway").split()[0]
    assert current == OLD, f"unexpected current release: {current}"
    gateway.shell(
        "grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && "
        "blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'"
    )

    with gateway.ssh.open_sftp() as sftp:
        flash_free = int(gateway.shell("df -Pk /userdata").splitlines()[-1].split()[3]) * 1024
        usb_free = int(gateway.shell("df -Pk /mnt/udisk").splitlines()[-1].split()[3]) * 1024
        old_size = sftp.stat("/userdata/weikong/bin/weikong-gateway").st_size
        assert flash_free + old_size - len(payload) > 4 * 1024 * 1024
        assert usb_free > 1024 * 1024 * 1024
        with sftp.open("/userdata/weikong/config/config.local.json") as config_file:
            config_hash = hashlib.sha256(config_file.read()).hexdigest()

        gateway.shell(
            f"test ! -e {BACKUP} && mkdir -m 700 {BACKUP} && "
            f"gzip -c /userdata/weikong/bin/weikong-gateway > {BACKUP}/weikong-gateway.original.gz && "
            f"cp -p /userdata/weikong/config/config.local.json {BACKUP}/config.original.json && "
            f"chmod 600 {BACKUP}/* && sync"
        )
        local_backup = BASE / "gateway-original.gz"
        sftp.get(f"{BACKUP}/weikong-gateway.original.gz", str(local_backup))
        assert hashlib.sha256(gzip.decompress(local_backup.read_bytes())).hexdigest() == OLD
        sftp.get(f"{BACKUP}/config.original.json", str(BASE / "config.original.json"))
        assert hashlib.sha256((BASE / "config.original.json").read_bytes()).hexdigest() == config_hash

        compressed = gzip.compress(payload, mtime=0)
        with sftp.open(ARCHIVE, "wb") as target:
            for offset in range(0, len(compressed), 32768):
                target.write(compressed[offset : offset + 32768])

        script = Path("deploy/eg100/deploy_mysql_fix_20260912.sh").read_text(encoding="utf-8")
        script = script.replace(
            "/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912", BACKUP
        ).replace(
            "/mnt/udisk/.weikong-deploy/mysql-fix-20260912.gz", ARCHIVE
        ).replace(
            "weikong-gateway.mysql-new", "weikong-gateway.cloud-events-new"
        )
        start = script.index("cp -p /userdata/weikong/config/.runtime/alarm-events.db")
        end = script.index("\nsync\necho BACKUPS_VERIFIED", start)
        databases = r'''
for source in /userdata/weikong/config/.runtime/alarm-events.db /mnt/udisk/.weikong/history/telemetry-history.db /mnt/udisk/.remote-storage/eg100-pg-integration.db /mnt/udisk/.remote-storage/eg100-mysql-integration.db /mnt/udisk/.remote-storage/eg100-sqlserver-integration.db; do
  test -f "$source" || continue
  name=$(basename "$source")
  cp -p "$source" "$backup/$name"
  if [ -f "$source-wal" ]; then cp -p "$source-wal" "$backup/$name-wal"; fi
  chmod 600 "$backup/$name"*
  test "$(sqlite3 "$backup/$name" 'PRAGMA quick_check;')" = ok
  echo "BACKUP_VERIFIED $name"
done
'''
        script = script[:start] + databases + script[end:]
        script = script.replace('"$attempt" -lt 60', '"$attempt" -lt 120')
        with sftp.open(f"{BACKUP}/deploy.sh", "wb") as target:
            target.write(script.encode())

    save(
        "manifest",
        {"new": NEW, "old": OLD, "config": config_hash, "backup": BACKUP, "size": len(payload)},
    )
    result = gateway.shell(
        f"sh {BACKUP}/deploy.sh {NEW} {OLD} {config_hash} > {BACKUP}/deploy.log 2>&1; "
        f"result=$?; cat {BACKUP}/deploy.log; exit $result",
        timeout=600,
    )
    save("deployment", result)


def verify(gateway):
    manifest = json.loads((BASE / "manifest.json").read_text(encoding="utf-8"))
    hashes = gateway.shell(
        "sha256sum /userdata/weikong/bin/weikong-gateway /userdata/weikong/config/config.local.json"
    ).splitlines()
    assert hashes[0].split()[0] == NEW
    assert hashes[1].split()[0] == manifest["config"]
    first = gateway.api("/api/healthz")
    assert first["healthy"] and gateway.api("/api/readyz")["ready"]
    before = json.loads((BASE / "health-before.json").read_text(encoding="utf-8"))
    assert first["pointCount"] == before["pointCount"]

    index = raw(gateway, "/").decode()
    assets = re.findall(r'(?:src|href)="(/assets/[^\"]+)"', index)
    assert len(assets) >= 2
    for path in assets:
        local = Path("internal/web/frontend/vue-dist") / path.lstrip("/")
        assert hashlib.sha256(local.read_bytes()).digest() == hashlib.sha256(raw(gateway, path)).digest()

    initial_storage = gateway.api("/api/remote-storage/status")["statuses"]
    time.sleep(12)
    last = gateway.api("/api/healthz")
    assert last["healthy"] and last["lastCollectAt"] != first["lastCollectAt"]
    statuses = gateway.api("/api/remote-storage/status")["statuses"]
    assert all(status["cacheReady"] for status in statuses)
    save(
        "verification",
        {
            "verified": True,
            "configUnchanged": True,
            "health": last,
            "assets": assets,
            "storage": statuses,
            "storageStatusCount": len(initial_storage),
            "platform": gateway.shell(
                "df -Pk /userdata /mnt/udisk; /etc/init.d/S99weikong-gateway status"
            ),
        },
    )


if __name__ == "__main__":
    BASE.mkdir(parents=True, exist_ok=True)
    gateway = Gateway()
    try:
        gateway.login()
        if sys.argv[1] == "deploy":
            deploy(gateway)
        elif sys.argv[1] == "verify":
            verify(gateway)
        else:
            raise ValueError("choose deploy or verify")
    finally:
        gateway.ssh.close()
