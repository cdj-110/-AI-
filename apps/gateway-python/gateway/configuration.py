from __future__ import annotations

import copy
import json
import os
import tempfile
import threading
from pathlib import Path
from typing import Any, Iterable


DEFAULT_CONFIG: dict[str, Any] = {
    "gatewayKey": "gateway-python-local",
    "collectIntervalSeconds": 5,
    "collectIntervalMilliseconds": 0,
    "activation": {"enabled": False},
    "mqtt": {"enabled": False, "broker": "", "clientId": "", "username": "", "password": ""},
    "mqttChannels": [],
    "resources": [],
    "serialPorts": [],
    "networkPorts": [],
    "channels": [],
    "devices": [],
    "forwardDevices": [],
    "edgeComputing": {"groups": []},
    "offlineCache": {"enabled": False, "maxSizeMB": 16, "storagePath": ""},
    "historyStorage": {"enabled": False, "maxSizeMB": 256, "storagePath": ""},
    "web": {"enabled": True, "listen": "0.0.0.0:8089"},
    "security": {
        "users": [{"username": "admin", "password": "123456", "role": "admin", "enabled": True}],
        "roles": [{"name": "admin", "permissions": ["*"]}],
    },
}


class ConfigurationError(ValueError):
    pass


class ConfigStore:
    def __init__(self, path: str | Path):
        self.path = Path(path)
        self._lock = threading.RLock()
        self._value: dict[str, Any] = {}
        self.load()

    def load(self) -> dict[str, Any]:
        with self._lock:
            if self.path.exists():
                value = json.loads(self.path.read_text(encoding="utf-8-sig"))
            else:
                value = copy.deepcopy(DEFAULT_CONFIG)
                self.path.parent.mkdir(parents=True, exist_ok=True)
                self._atomic_write(value)
            value = normalize(value)
            validate(value)
            self._value = value
            return copy.deepcopy(value)

    def get(self, redact: bool = False) -> dict[str, Any]:
        with self._lock:
            value = copy.deepcopy(self._value)
        if redact:
            redact_secrets(value)
        return value

    def compact(self, redact: bool = False) -> tuple[dict[str, Any], dict[str, int], int]:
        """Return configuration metadata without copying large point arrays."""
        with self._lock:
            value = {
                key: copy.deepcopy(item)
                for key, item in self._value.items()
                if key not in {"devices", "forwardDevices", "points"}
            }
            counts: dict[str, int] = {}
            total = 0
            for collection in ("devices", "forwardDevices"):
                compact_devices = []
                for device in self._value.get(collection, []):
                    compact_device = {
                        key: copy.deepcopy(item)
                        for key, item in device.items()
                        if key != "points"
                    }
                    count = len(device.get("points", []))
                    counts[str(device.get("deviceKey", ""))] = count
                    total += count
                    compact_device["points"] = []
                    compact_devices.append(compact_device)
                value[collection] = compact_devices
        if redact:
            redact_secrets(value)
        return value, counts, total

    def point_window(
        self,
        device_key: str,
        offset: int,
        limit: int,
        search: str = "",
        function: int = 0,
    ) -> tuple[list[dict[str, Any]], int]:
        search = search.strip().lower()
        with self._lock:
            for collection in ("devices", "forwardDevices"):
                for device in self._value.get(collection, []):
                    if str(device.get("deviceKey", "")) != device_key:
                        continue
                    rows: list[dict[str, Any]] = []
                    total = 0
                    for point in device.get("points", []):
                        if function > 0 and int(point.get("function") or 0) != function:
                            continue
                        haystack = f"{point.get('name', '')} {point.get('metric', '')}".lower()
                        if search and search not in haystack:
                            continue
                        if total >= offset and len(rows) < limit:
                            rows.append(copy.deepcopy(point))
                        total += 1
                    return rows, total
        raise KeyError(device_key)

    def replace_compact(self, value: dict[str, Any]) -> dict[str, Any]:
        """Save metadata from the lightweight UI while retaining point arrays."""
        with self._lock:
            current_points = {
                str(device.get("deviceKey", "")): device.get("points", [])
                for collection in ("devices", "forwardDevices")
                for device in self._value.get(collection, [])
            }
        candidate = copy.deepcopy(value)
        for collection in ("devices", "forwardDevices"):
            for device in candidate.get(collection, []):
                device["points"] = current_points.get(str(device.get("deviceKey", "")), [])
        return self.replace(candidate)

    def replace(self, value: dict[str, Any]) -> dict[str, Any]:
        candidate = normalize(copy.deepcopy(value))
        validate(candidate)
        with self._lock:
            self._atomic_write(candidate)
            self._value = candidate
            return copy.deepcopy(candidate)

    def _atomic_write(self, value: dict[str, Any]) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        fd, temporary = tempfile.mkstemp(prefix=self.path.name + ".", suffix=".tmp", dir=self.path.parent)
        try:
            with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as stream:
                json.dump(value, stream, ensure_ascii=False, indent=2)
                stream.write("\n")
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, self.path)
        finally:
            if os.path.exists(temporary):
                os.unlink(temporary)


def normalize(value: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ConfigurationError("配置文件根节点必须是 JSON 对象")
    result = copy.deepcopy(DEFAULT_CONFIG)
    result.update(value)
    for key in ("activation", "mqtt", "offlineCache", "historyStorage", "web", "security", "edgeComputing"):
        merged = copy.deepcopy(DEFAULT_CONFIG.get(key, {}))
        if isinstance(value.get(key), dict):
            merged.update(value[key])
        result[key] = merged
    for key in ("mqttChannels", "resources", "serialPorts", "networkPorts", "channels", "devices", "forwardDevices"):
        if not isinstance(result.get(key), list):
            result[key] = []
    if not result["resources"]:
        result["resources"] = _resources_from_legacy(result)
    milliseconds = int(result.get("collectIntervalMilliseconds") or 0)
    seconds = int(result.get("collectIntervalSeconds") or 0)
    if milliseconds > 0:
        result["collectIntervalMilliseconds"] = max(10, milliseconds)
    else:
        result["collectIntervalMilliseconds"] = 0
        result["collectIntervalSeconds"] = max(1, seconds or 5)
    return result


def _resources_from_legacy(value: dict[str, Any]) -> list[dict[str, Any]]:
    resources: list[dict[str, Any]] = []
    seen: set[str] = set()
    serial_by_name = {str(item.get("name", "")): item for item in value.get("serialPorts", [])}
    network_by_name = {str(item.get("name", "")): item for item in value.get("networkPorts", [])}
    for index, channel in enumerate(value.get("channels", [])):
        key = str(
            channel.get("resourceKey")
            or channel.get("interfaceName")
            or channel.get("name")
            or f"resource-{index + 1:03d}"
        )
        if key in seen:
            continue
        seen.add(key)
        resource_type = str(channel.get("type") or ("serial" if channel.get("protocol") == "modbus-rtu" else "network"))
        name = str(channel.get("interfaceName") or key)
        resource = {
            "resourceKey": key,
            "name": name,
            "type": resource_type,
            "enabled": channel.get("enabled", True),
        }
        if resource_type == "serial":
            serial = channel.get("serial") or serial_by_name.get(name) or serial_by_name.get(key) or {
                "name": name, "port": name, "baudRate": 9600, "dataBits": 8, "stopBits": 1, "parity": "none",
            }
            resource["serial"] = copy.deepcopy(serial)
        else:
            network = channel.get("network") or network_by_name.get(name) or network_by_name.get(key)
            if network is None and value.get("networkPorts"):
                interface = str(channel.get("interfaceName") or "")
                network = next(
                    (item for item in value["networkPorts"] if str(item.get("interface", "")) == interface),
                    value["networkPorts"][0] if len(value["networkPorts"]) == 1 else None,
                )
            resource["network"] = copy.deepcopy(network or {
                "name": name, "interface": key, "mode": "dhcp", "enabled": True,
            })
        resources.append(resource)
    if resources:
        return resources
    for serial in value.get("serialPorts", []):
        key = str(serial.get("name") or f"serial-{len(resources) + 1}")
        resources.append({
            "resourceKey": key,
            "name": key,
            "type": "serial",
            "enabled": serial.get("enabled", True),
            "serial": copy.deepcopy(serial),
        })
    for network in value.get("networkPorts", []):
        key = str(network.get("name") or network.get("interface") or f"network-{len(resources) + 1}")
        resources.append({
            "resourceKey": key,
            "name": key,
            "type": "network",
            "enabled": network.get("enabled", True),
            "network": copy.deepcopy(network),
        })
    return resources


def validate(value: dict[str, Any]) -> None:
    if not str(value.get("gatewayKey", "")).strip():
        raise ConfigurationError("gatewayKey 不能为空")
    seen: dict[str, str] = {}
    device_keys: set[str] = set()
    for device in value.get("devices", []):
        key = str(device.get("deviceKey", "")).strip()
        if not key:
            raise ConfigurationError("设备 deviceKey 不能为空")
        if key in device_keys:
            raise ConfigurationError(f"设备编号重复：{key}")
        device_keys.add(key)
        protocol = str(device.get("protocol", "")).lower()
        if protocol not in {"modbus-tcp", "modbus-rtu", "opcua", "siemens-s7", "iec104", "iec61850", "none"}:
            raise ConfigurationError(f"设备 {key} 使用了不支持的协议：{protocol}")
        for point in device.get("points", []):
            metric = str(point.get("metric", "")).strip()
            if not metric:
                raise ConfigurationError(f"设备 {key} 存在空标识符点位")
            normalized = metric.lower()
            if normalized in seen:
                raise ConfigurationError(f"标识符必须全局唯一：{metric} 同时属于 {seen[normalized]} 和 {key}")
            seen[normalized] = key


def iter_points(config: dict[str, Any]) -> Iterable[dict[str, Any]]:
    for device in config.get("devices", []):
        if device.get("enabled", True) is False:
            continue
        # Do not deepcopy the device's full points array for every point. With
        # 1,000 points that created roughly one million copied point objects
        # during startup and every configuration lookup.
        device_base = {key: copy.deepcopy(value) for key, value in device.items() if key != "points"}
        for point in device.get("points", []):
            if point.get("enabled", True) is False:
                continue
            merged = copy.deepcopy(device_base)
            merged.update(copy.deepcopy(point))
            merged["deviceKey"] = device.get("deviceKey", "")
            merged["deviceName"] = device.get("name", device.get("deviceKey", ""))
            merged["protocol"] = point.get("protocol") or device.get("protocol", "")
            # Connection settings belong to the device. Point rows created by
            # the UI may still contain a snapshot of the old address/slave ID;
            # letting that snapshot win means changing the device connection
            # never reaches the running collector.
            merged["address"] = device.get("address") or point.get("address", "")
            device_slave_id = device.get("slaveId")
            merged["slaveId"] = device_slave_id if device_slave_id is not None else point.get("slaveId", 1)
            yield merged


def redact_secrets(value: dict[str, Any]) -> None:
    for mqtt in [value.get("mqtt", {}), *value.get("mqttChannels", [])]:
        for key in ("password", "deviceSecret"):
            if mqtt.get(key):
                mqtt[key] = "********"
    activation = value.get("activation", {})
    if activation.get("deviceSecret"):
        activation["deviceSecret"] = "********"
    for device in value.get("devices", []):
        if device.get("password"):
            device["password"] = "********"
    value.pop("security", None)
