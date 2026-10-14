from __future__ import annotations

import asyncio
import copy
import datetime as dt
import os
from pathlib import Path
import shutil
import threading
import time
from collections import deque
from typing import Any


def iso_now() -> str:
    return dt.datetime.now(dt.timezone.utc).astimezone().isoformat(timespec="milliseconds")


class GatewayState:
    def __init__(self, gateway_key: str, collect_seconds: float):
        self.started = time.monotonic()
        self.gateway_key = gateway_key
        self.collect_seconds = collect_seconds
        self.last_collect_at: str | None = None
        self.last_publish_at: str | None = None
        self.mqtt_enabled = False
        self.mqtt_connected = False
        self.mqtt_channels: dict[str, dict[str, Any]] = {}
        self._points: dict[str, dict[str, Any]] = {}
        self._healthy_count = 0
        self._pending_count = 0
        self._stale_count = 0
        self._error_count = 0
        self._logs: deque[dict[str, Any]] = deque(maxlen=200)
        self._subscribers: set[asyncio.Queue] = set()
        self._sequence = 0
        self._lock = threading.RLock()
        self._loop: asyncio.AbstractEventLoop | None = None
        self._metrics_cached_at = 0.0
        self._metrics_cache: dict[str, Any] = {}
        self._disk_bytes_cached_at = 0.0
        self._disk_bytes_cache = 0

    def attach_loop(self, loop: asyncio.AbstractEventLoop) -> None:
        self._loop = loop

    def reset_config(self, gateway_key: str, collect_seconds: float) -> None:
        with self._lock:
            self.gateway_key = gateway_key
            self.collect_seconds = collect_seconds

    def update_point(self, point: dict[str, Any], value: Any = None, error: str = "") -> dict[str, Any]:
        return self.update_points([(point, value, error)])[0]

    def update_points(self, updates: list[tuple[dict[str, Any], Any, str]]) -> list[dict[str, Any]]:
        """Apply one device cycle and publish only changed values as one diff."""
        items: list[dict[str, Any]] = []
        changed: list[dict[str, Any]] = []
        updated_at = iso_now()
        with self._lock:
            for point, value, error in updates:
                key = f"{point.get('deviceKey', '')}::{point.get('metric', '')}"
                previous = self._points.get(key)
                if error and previous and previous.get("value") is not None:
                    value = previous["value"]
                if previous is not None and previous.get("value") == value and previous.get("error") == error:
                    previous["updatedAt"] = updated_at
                    previous["stale"] = bool(error)
                    items.append(previous)
                    continue
                item = {
                    "deviceKey": point.get("deviceKey", ""),
                    "name": point.get("name") or point.get("metric", ""),
                    "metric": point.get("metric", ""),
                    "protocol": point.get("protocol", ""),
                    "address": point.get("address", ""),
                    "unit": point.get("unit", ""),
                    "value": value,
                    "updatedAt": updated_at,
                    "error": error,
                    "stale": bool(error),
                }
                if previous:
                    self._remove_counts(previous)
                self._points[key] = item
                self._add_counts(item)
                items.append(item)
                changed.append(item)
        if changed:
            self.publish("points.diff", {"points": changed})
        return items

    def replace_points(self, points) -> None:
        """Replace definitions without publishing a full startup diff."""
        new_points: dict[str, dict[str, Any]] = {}
        updated_at = iso_now()
        for point in points:
            item = {
                "deviceKey": point.get("deviceKey", ""),
                "name": point.get("name") or point.get("metric", ""),
                "metric": point.get("metric", ""),
                "protocol": point.get("protocol", ""),
                "address": point.get("address", ""),
                "unit": point.get("unit", ""),
                "value": None,
                "updatedAt": updated_at,
                "error": "",
                "stale": False,
            }
            new_points[f"{item['deviceKey']}::{item['metric']}"] = item
        with self._lock:
            self._points = new_points
            self._healthy_count = 0
            self._pending_count = len(new_points)
            self._stale_count = 0
            self._error_count = 0

    def _add_counts(self, item: dict[str, Any]) -> None:
        self._healthy_count += int(not item.get("error") and item.get("value") is not None)
        self._pending_count += int(item.get("value") is None and not item.get("error"))
        self._stale_count += int(bool(item.get("stale")))
        self._error_count += int(bool(item.get("error")))

    def _remove_counts(self, item: dict[str, Any]) -> None:
        self._healthy_count -= int(not item.get("error") and item.get("value") is not None)
        self._pending_count -= int(item.get("value") is None and not item.get("error"))
        self._stale_count -= int(bool(item.get("stale")))
        self._error_count -= int(bool(item.get("error")))

    def set_collect_time(self) -> None:
        self.last_collect_at = iso_now()
        self.publish("gateway.status", self.summary(include_points=False))

    def log(self, level: str, message: str) -> None:
        entry = {"time": iso_now(), "level": level.upper(), "message": message}
        with self._lock:
            self._logs.appendleft(entry)
        self.publish("logs.diff", {"entries": [entry]})

    def points(self, device_key: str = "", metrics: list[str] | None = None) -> list[dict[str, Any]]:
        with self._lock:
            if device_key and metrics:
                values = [
                    copy.deepcopy(self._points[key])
                    for metric in metrics
                    if (key := f"{device_key}::{metric}") in self._points
                ]
            else:
                values = [
                    copy.deepcopy(item)
                    for item in self._points.values()
                    if not device_key or item.get("deviceKey") == device_key
                ]
        return sorted(values, key=lambda item: (item.get("deviceKey", ""), item.get("metric", "")))

    def summary(self, include_points: bool = True) -> dict[str, Any]:
        with self._lock:
            point_count = len(self._points)
            healthy = self._healthy_count
            pending = self._pending_count
            stale = self._stale_count
            errors = self._error_count
            point_values = list(self._points.values()) if include_points else []
            recent_errors = copy.deepcopy(list(self._logs))
        result = {
            "gatewayKey": self.gateway_key,
            "backend": "python",
            "backendVersion": "0.1.0",
            "uptimeSeconds": int(time.monotonic() - self.started),
            "collectSeconds": self.collect_seconds,
            "collectMilliseconds": round(self.collect_seconds * 1000),
            "mqttEnabled": self.mqtt_enabled,
            "mqttConnected": self.mqtt_connected,
            "mqttChannels": copy.deepcopy(self.mqtt_channels),
            "lastCollectAt": self.last_collect_at,
            "lastPublishAt": self.last_publish_at,
            "pointCount": point_count,
            "healthyCount": healthy,
            "pendingCount": pending,
            "staleCount": stale,
            "errorCount": errors,
            "errors": recent_errors,
            **self._runtime_metrics(),
        }
        if include_points:
            result["points"] = sorted(
                copy.deepcopy(point_values),
                key=lambda item: (item.get("deviceKey", ""), item.get("metric", "")),
            )
        return result

    def _runtime_metrics(self) -> dict[str, Any]:
        with self._lock:
            if self._metrics_cache:
                return copy.deepcopy(self._metrics_cache)
        return self.refresh_metrics(force_disk=True)

    def refresh_metrics(self, force_disk: bool = False) -> dict[str, Any]:
        """Sample live load while keeping the expensive app-size walk separate."""
        now = time.monotonic()
        app_dir = Path(os.environ.get("WEIKONG_APP_DIR", Path(__file__).resolve().parents[1])).resolve()
        total_disk, used_disk, _ = shutil.disk_usage(app_dir)
        memory_total, memory_used = _system_memory()
        cpu_percent = _system_cpu_percent()
        with self._lock:
            disk_bytes = self._disk_bytes_cache
            disk_due = force_disk or not disk_bytes or now - self._disk_bytes_cached_at >= 300
        if disk_due:
            disk_bytes = _configured_directory_size(app_dir)

        value = {
            "systemMetrics": {
                "cpu": {"usedPercent": round(cpu_percent, 1)},
                "memory": {
                    "usedPercent": round(memory_used / memory_total * 100, 1) if memory_total else 0,
                    "usedBytes": memory_used,
                    "totalBytes": memory_total,
                },
                "storage": {
                    "usedPercent": round(used_disk / total_disk * 100, 1) if total_disk else 0,
                    "usedBytes": used_disk,
                    "totalBytes": total_disk,
                },
            },
            "processMetrics": {
                "memoryBytes": _process_rss_bytes(),
                "diskBytes": disk_bytes,
                "diskPath": str(app_dir),
            },
        }
        with self._lock:
            self._metrics_cached_at = now
            self._metrics_cache = value
            if disk_due:
                self._disk_bytes_cached_at = now
                self._disk_bytes_cache = disk_bytes
        return copy.deepcopy(value)

    def subscribe(self) -> asyncio.Queue:
        # Entries are batched messages, not individual point updates.
        queue: asyncio.Queue = asyncio.Queue(maxsize=32)
        self._subscribers.add(queue)
        return queue

    def unsubscribe(self, queue: asyncio.Queue) -> None:
        self._subscribers.discard(queue)

    def message(self, message_type: str, payload: Any) -> dict[str, Any]:
        with self._lock:
            self._sequence += 1
            sequence = self._sequence
        return {"type": message_type, "sequence": sequence, "timestamp": iso_now(), "payload": payload}

    def publish(self, message_type: str, payload: Any) -> None:
        if not self._loop or not self._loop.is_running():
            return
        message = self.message(message_type, payload)
        self._loop.call_soon_threadsafe(self._publish_on_loop, message)

    def _publish_on_loop(self, message: dict[str, Any]) -> None:
        for queue in tuple(self._subscribers):
            if queue.full():
                # A slow browser must resynchronize instead of silently losing
                # only part of a point cycle. Replace its backlog with a full
                # snapshot, equivalent to reconnecting to the Go gateway.
                while not queue.empty():
                    try:
                        queue.get_nowait()
                    except asyncio.QueueEmpty:
                        break
                queue.put_nowait(self.message("snapshot", self.summary()))
                continue
            queue.put_nowait(message)


def _system_memory() -> tuple[int, int]:
    try:
        values: dict[str, int] = {}
        with open("/proc/meminfo", "r", encoding="ascii") as handle:
            for line in handle:
                name, raw = line.split(":", 1)
                values[name] = int(raw.strip().split()[0]) * 1024
        total = values.get("MemTotal", 0)
        available = values.get("MemAvailable", values.get("MemFree", 0))
        return total, max(0, total - available)
    except (OSError, ValueError, KeyError):
        return 0, 0


def _process_rss_bytes() -> int:
    try:
        with open("/proc/self/status", "r", encoding="ascii") as handle:
            for line in handle:
                if line.startswith("VmRSS:"):
                    return int(line.split()[1]) * 1024
    except (OSError, ValueError, IndexError):
        pass
    return 0


def _system_cpu_percent() -> float:
    first = _cpu_sample()
    if first is None:
        return 0.0
    time.sleep(0.12)
    second = _cpu_sample()
    if second is None:
        return 0.0
    idle_delta = second[0] - first[0]
    total_delta = second[1] - first[1]
    if total_delta <= 0 or idle_delta < 0 or idle_delta > total_delta:
        return 0.0
    return min(100.0, max(0.0, (total_delta - idle_delta) * 100.0 / total_delta))


def _cpu_sample() -> tuple[int, int] | None:
    try:
        fields = Path("/proc/stat").read_text(encoding="ascii").splitlines()[0].split()
        if not fields or fields[0] != "cpu":
            return None
        values = [int(value) for value in fields[1:]]
        if len(values) < 4:
            return None
        total = sum(values)
        idle = values[3] + (values[4] if len(values) > 4 else 0)
        return idle, total
    except (OSError, ValueError, IndexError):
        return None


def _directory_size(root: Path) -> int:
    total = 0
    try:
        for directory, names, files in os.walk(root):
            names[:] = [name for name in names if name not in {".git", "__pycache__", ".runtime"}]
            for name in files:
                try:
                    total += (Path(directory) / name).stat().st_size
                except OSError:
                    continue
    except OSError:
        return 0
    return total


def _configured_directory_size(root: Path) -> int:
    """Use the deployment-time size when supplied by small flash targets."""
    try:
        configured = int(os.environ.get("WEIKONG_DISK_BYTES", "0"))
    except ValueError:
        configured = 0
    return configured if configured > 0 else _directory_size(root)
