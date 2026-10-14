"""Inspect EG100 web-login state without exposing password hashes or secrets."""
import json
from pathlib import Path

from mysql_live_20260912 import Gateway


def main():
    gateway = Gateway()
    try:
        with gateway.ssh.open_sftp() as sftp:
            with sftp.open("/userdata/weikong/config/config.local.json") as source:
                config = json.loads(source.read().decode("utf-8"))
            audit_path = "/userdata/weikong/config/.runtime/security-audit.jsonl"
            try:
                with sftp.open(audit_path) as source:
                    lines = source.read().decode("utf-8", errors="replace").splitlines()
            except OSError:
                lines = []

        web = config.get("web") or {}
        security = config.get("security") or {}
        users = [
            {
                "username": item.get("username"),
                "displayName": item.get("displayName"),
                "roleKey": item.get("roleKey"),
                "enabled": item.get("enabled", True),
                "mustChangePassword": item.get("mustChangePassword", False),
                "mfaEnabled": item.get("mfaEnabled", False),
            }
            for item in security.get("users", [])
        ]
        events = []
        for line in lines[-200:]:
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("path") == "/login":
                events.append(
                    {
                        key: event.get(key)
                        for key in (
                            "timestamp",
                            "username",
                            "remoteIp",
                            "status",
                            "result",
                            "detail",
                        )
                    }
                )
        result = {
            "legacyWebUsername": web.get("username"),
            "securityUsers": users,
            "recentLoginEvents": events[-20:],
        }
        output = Path("build/remote-access-auto-release-20260916/login-diagnosis.json")
        output.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
        print(json.dumps(result, ensure_ascii=False, indent=2))
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    main()
