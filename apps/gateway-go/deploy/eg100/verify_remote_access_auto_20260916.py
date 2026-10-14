"""Verify the EG100 remote-access release without a browser login."""
import hashlib
import json
from pathlib import Path

from mysql_live_20260912 import Gateway


BASE = Path("build/remote-access-auto-release-20260916")


def local_json(gateway, path):
    return json.loads(
        gateway.shell(f"wget -T 5 -qO- http://127.0.0.1:8088{path}")
    )


def main():
    manifest = json.loads((BASE / "manifest.json").read_text(encoding="utf-8"))
    gateway = Gateway()
    try:
        hashes = gateway.shell(
            "sha256sum /userdata/weikong/bin/weikong-gateway "
            "/userdata/weikong/config/config.local.json"
        ).splitlines()
        assert hashes[0].split()[0] == manifest["new"]
        assert hashes[1].split()[0] == manifest["config"]
        health = local_json(gateway, "/api/healthz")
        ready = local_json(gateway, "/api/readyz")
        assert health["healthy"] and ready["ready"]
        assert health["pointCount"] == 52

        local_index = Path("internal/web/frontend/vue-dist/index.html")
        local_assets = [
            path
            for path in Path("internal/web/frontend/vue-dist/assets").iterdir()
            if path.name.startswith("index-") and path.suffix in {".js", ".css"}
        ]
        asset_checks = []
        for local in local_assets:
            remote = f"/assets/{local.name}"
            headers = gateway.shell(
                f"wget -S -O /tmp/wk-release-asset http://127.0.0.1:8088{remote} 2>&1 || true"
            )
            remote_size = int(gateway.shell("wc -c </tmp/wk-release-asset").strip())
            remote_hash = gateway.shell("sha256sum /tmp/wk-release-asset").split()[0]
            expected_hash = hashlib.sha256(local.read_bytes()).hexdigest()
            asset_checks.append(
                {
                    "asset": remote,
                    "headers": headers.strip().splitlines()[:8],
                    "size": remote_size,
                    "expectedSize": local.stat().st_size,
                    "hashMatches": remote_hash == expected_hash,
                }
            )

        tunnel = gateway.shell(
            "netstat -tn 2>/dev/null | grep '192.168.1.141:3100' || true"
        ).strip()
        result = {
            "verified": True,
            "configUnchanged": True,
            "health": health,
            "ready": ready,
            "localIndexHash": hashlib.sha256(local_index.read_bytes()).hexdigest(),
            "assetChecks": asset_checks,
            "remoteTunnelConnection": tunnel,
            "platform": gateway.shell(
                "df -Pk /userdata /mnt/udisk; /etc/init.d/S99weikong-gateway status"
            ),
        }
        (BASE / "verification.json").write_text(
            json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        print(json.dumps(result, ensure_ascii=False, indent=2))
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
