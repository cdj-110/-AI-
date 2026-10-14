from __future__ import annotations

import argparse
import json
from pathlib import Path


def build_config(point_count: int, listen: str, device_address: str) -> dict:
    points = []
    width = max(5, len(str(point_count)))
    for index in range(point_count):
        metric = f"py-P{index + 1:0{width}d}"
        points.append({
            "name": f"eth0_Modbus采集_设备1@{metric}",
            "metric": metric,
            "function": 3,
            "register": index,
            "quantity": 1,
            "dataType": "uint16",
            "byteOrder": "ABCD",
            "scale": 1,
            "offset": 0,
            "unit": "",
            "decimals": 0,
            "enabled": True,
        })
    return {
        "gatewayKey": f"python-load-{point_count}",
        "collectIntervalSeconds": 1,
        "collectIntervalMilliseconds": 0,
        "activation": {"enabled": False},
        "mqtt": {"enabled": False, "broker": "", "clientId": "", "username": "", "password": ""},
        "mqttChannels": [],
        "resources": [{
            "resourceKey": "eth0",
            "name": "网口1",
            "type": "network",
            "enabled": True,
            "network": {
                "name": "网口1",
                "interface": "eth0",
                "mode": "dhcp",
                "enabled": True,
            },
        }],
        "serialPorts": [],
        "networkPorts": [{
            "name": "网口1",
            "interface": "eth0",
            "mode": "dhcp",
            "enabled": True,
        }],
        "channels": [{
            "channelKey": "eth0-modbus-tcp",
            "resourceKey": "eth0",
            "name": "Modbus采集",
            "type": "network",
            "role": "collect",
            "protocol": "modbus-tcp",
            "collectIntervalSeconds": 1,
            "enabled": True,
        }],
        "devices": [{
            "deviceKey": "python-load-device-1",
            "channelKey": "eth0-modbus-tcp",
            "name": "设备1",
            "protocol": "modbus-tcp",
            "address": device_address,
            "slaveId": 1,
            "timeoutSeconds": 2,
            "enabled": True,
            "points": points,
        }],
        "edgeComputing": {"groups": []},
        "offlineCache": {"enabled": False, "maxSizeMB": 16, "storagePath": ""},
        "historyStorage": {"enabled": False, "maxSizeMB": 256, "storagePath": ""},
        "web": {"enabled": True, "listen": listen},
        "security": {
            "users": [{"username": "admin", "password": "123456", "role": "admin", "enabled": True}],
            "roles": [{"name": "admin", "permissions": ["*"]}],
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate a standalone Python gateway load-test config")
    parser.add_argument("--points", type=int, default=30_000)
    parser.add_argument("--output", default="config.30000-test.json")
    parser.add_argument("--listen", default="127.0.0.1:8089")
    parser.add_argument("--device-address", default="127.0.0.1:15020")
    args = parser.parse_args()
    if args.points < 1 or args.points > 1_000_000:
        raise SystemExit("--points must be between 1 and 1000000")
    output = Path(args.output)
    output.write_text(
        json.dumps(build_config(args.points, args.listen, args.device_address), ensure_ascii=False, separators=(",", ":")),
        encoding="utf-8",
    )
    print(f"generated {args.points} points: {output.resolve()} ({output.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
