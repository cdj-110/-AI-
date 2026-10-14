from __future__ import annotations

import asyncio
import csv
import json
import os
from pathlib import Path
from typing import Any

from .collectors import CollectorSession, write_point
from .configuration import ConfigStore, iter_points
from .mqtt_client import MQTTManager
from .state import GatewayState, iso_now


METRICS_INTERVAL_SECONDS = 0.25


class GatewayRuntime:
    def __init__(self, store: ConfigStore):
        self.store = store
        config = store.get()
        self._config = config
        self.state = GatewayState(str(config["gatewayKey"]), _config_interval(config))
        self.mqtt = MQTTManager(str(config["gatewayKey"]), self._mqtt_status)
        self._device_tasks: dict[str, asyncio.Task] = {}
        self._metrics_task: asyncio.Task | None = None
        self._device_locks: dict[str, asyncio.Lock] = {}
        self._sessions: dict[str, CollectorSession] = {}
        self._device_points: dict[str, list[dict[str, Any]]] = {}
        self._device_intervals: dict[str, float] = {}
        self._history_lock = asyncio.Lock()
        self._mqtt_lock = asyncio.Lock()
        self._stopping = asyncio.Event()

    async def start(self) -> None:
        # Python 3.8 binds asyncio synchronization primitives to the loop
        # that first uses them. Recreate them inside Uvicorn's running loop
        # instead of retaining objects constructed while the app was built.
        self._history_lock = asyncio.Lock()
        self._mqtt_lock = asyncio.Lock()
        self._stopping = asyncio.Event()
        self.state.attach_loop(asyncio.get_running_loop())
        await asyncio.to_thread(self.state.refresh_metrics, True)
        config = self._config
        self._initialize_points(config)
        try:
            await asyncio.to_thread(self.mqtt.configure, config)
        except Exception as exc:
            self.state.log("ERROR", f"MQTT 初始化失败：{exc}")
        await self._start_device_tasks(config)
        self._metrics_task = asyncio.create_task(self._metrics_loop(), name="system-metrics")

    async def stop(self) -> None:
        self._stopping.set()
        if self._metrics_task:
            self._metrics_task.cancel()
            await asyncio.gather(self._metrics_task, return_exceptions=True)
            self._metrics_task = None
        await self._stop_device_tasks()
        await asyncio.to_thread(self.mqtt.stop)

    async def reload(self) -> None:
        config = self.store.load()
        self._config = config
        await self._stop_device_tasks()
        self.state.reset_config(str(config["gatewayKey"]), _config_interval(config))
        self._initialize_points(config)
        await asyncio.to_thread(self.mqtt.configure, config)
        await self._start_device_tasks(config)
        self.state.log("INFO", "Python 网关配置已重新加载")

    async def collect_once(self) -> dict[str, Any]:
        results = await asyncio.gather(
            *(self._collect_device(device_key) for device_key in self._device_points),
            return_exceptions=True,
        )
        collected = sum(result for result in results if isinstance(result, int))
        return {"ok": True, "collected": collected, "time": self.state.last_collect_at}

    async def write(self, device_key: str, metric: str, value: Any) -> dict[str, Any]:
        point = self.find_point(device_key, metric)
        if not point:
            raise KeyError(f"点位不存在：{device_key}/{metric}")
        await asyncio.to_thread(write_point, point, value)
        self.state.update_point(point, value=value)
        self.state.log("INFO", f"点位下发成功：{device_key}/{metric}={value}")
        return {"ok": True, "deviceKey": device_key, "metric": metric, "value": value}

    def find_point(self, device_key: str, metric: str) -> dict[str, Any] | None:
        candidates = self._device_points.get(device_key, []) if device_key else (
            point for points in self._device_points.values() for point in points
        )
        for point in candidates:
            if str(point.get("metric")) == metric:
                return point
        return None

    def export_history(self, stream) -> None:
        writer = csv.writer(stream)
        writer.writerow(["time", "deviceKey", "metric", "value"])
        path = self._history_path()
        if not path.exists():
            return
        for line in path.read_text(encoding="utf-8").splitlines():
            try:
                record = json.loads(line)
                for metric, value in record.get("metrics", {}).items():
                    writer.writerow([record.get("time", ""), record.get("gatewayKey", ""), metric, value])
            except (ValueError, TypeError):
                continue

    async def _start_device_tasks(self, config: dict[str, Any]) -> None:
        groups, intervals = self._group_points(config)
        self._device_points = groups
        self._device_intervals = intervals
        self._device_locks = {device_key: asyncio.Lock() for device_key in groups}
        self._sessions = {device_key: CollectorSession(points[0]) for device_key, points in groups.items() if points}
        self._device_tasks = {
            device_key: asyncio.create_task(
                self._device_loop(device_key, intervals[device_key]),
                name=f"collector-{device_key}",
            )
            for device_key in groups
        }
        for device_key, task in self._device_tasks.items():
            task.add_done_callback(
                lambda completed, key=device_key: self._device_task_finished(key, completed)
            )

    def _device_task_finished(self, device_key: str, task: asyncio.Task) -> None:
        if task.cancelled():
            return
        error = task.exception()
        if error is not None:
            self.state.log("ERROR", f"设备 {device_key} 采集任务已退出：{error}")

    async def _stop_device_tasks(self) -> None:
        tasks = list(self._device_tasks.values())
        self._device_tasks = {}
        for task in tasks:
            task.cancel()
        if tasks:
            await asyncio.gather(*tasks, return_exceptions=True)
        sessions = list(self._sessions.values())
        self._sessions = {}
        if sessions:
            await asyncio.gather(*(asyncio.to_thread(session.close) for session in sessions), return_exceptions=True)
        self._device_points = {}
        self._device_intervals = {}
        self._device_locks = {}

    async def _device_loop(self, device_key: str, interval: float) -> None:
        loop = asyncio.get_running_loop()
        next_tick = loop.time()
        while not self._stopping.is_set():
            try:
                await self._collect_device(device_key)
            except asyncio.CancelledError:
                raise
            except Exception as exc:
                self.state.log("ERROR", f"设备 {device_key} 采集任务异常：{exc}")
            next_tick += interval
            delay = next_tick - loop.time()
            if delay < 0:
                # Do not add another full interval after an overrun. Resume from
                # the current time so slow devices cannot accumulate drift.
                next_tick = loop.time()
                delay = 0
            if delay > 0:
                try:
                    await asyncio.wait_for(self._stopping.wait(), timeout=delay)
                except asyncio.TimeoutError:
                    pass
            else:
                await asyncio.sleep(0)

    async def _metrics_loop(self) -> None:
        loop = asyncio.get_running_loop()
        next_tick = loop.time() + METRICS_INTERVAL_SECONDS
        while not self._stopping.is_set():
            try:
                await asyncio.sleep(max(0.0, next_tick - loop.time()))
                await asyncio.to_thread(self.state.refresh_metrics)
                self.state.publish("gateway.status", self.state.summary(include_points=False))
                next_tick += METRICS_INTERVAL_SECONDS
                if next_tick < loop.time():
                    next_tick = loop.time() + METRICS_INTERVAL_SECONDS
            except asyncio.CancelledError:
                raise
            except Exception as exc:
                self.state.log("ERROR", f"系统指标采样失败：{exc}")
                next_tick = loop.time() + METRICS_INTERVAL_SECONDS

    async def _collect_device(self, device_key: str) -> int:
        points = self._device_points.get(device_key, [])
        session = self._sessions.get(device_key)
        lock = self._device_locks.get(device_key)
        if not points or session is None or lock is None:
            return 0
        history_enabled = bool(self._config.get("historyStorage", {}).get("enabled"))
        mqtt_enabled = bool(self._config.get("mqtt", {}).get("enabled")) or any(
            item.get("enabled") for item in self._config.get("mqttChannels", [])
        )
        current: dict[str, Any] | None = {} if history_enabled or mqtt_enabled else None
        point_updates: list[tuple[dict[str, Any], Any, str]] = []
        collected = 0
        async with lock:
            readings = await asyncio.to_thread(session.read_many, points)
            for point, (value, read_error) in zip(points, readings):
                if read_error is None:
                    collected += 1
                    if current is not None:
                        current[str(point.get("metric"))] = value
                    point_updates.append((point, value, ""))
                else:
                    point_updates.append((point, None, str(read_error)))
            self.state.update_points(point_updates)
        self.state.set_collect_time()
        if current:
            await self._store_history(current)
            try:
                async with self._mqtt_lock:
                    topics = await asyncio.to_thread(self.mqtt.publish, current)
                if topics:
                    self.state.last_publish_at = iso_now()
            except Exception as exc:
                self.state.log("ERROR", f"MQTT 发布失败：{exc}")
        return collected

    def _group_points(self, config: dict[str, Any]) -> tuple[dict[str, list[dict[str, Any]]], dict[str, float]]:
        groups: dict[str, list[dict[str, Any]]] = {}
        for point in iter_points(config):
            groups.setdefault(str(point.get("deviceKey", "")), []).append(point)
        channels = {str(item.get("channelKey", "")): item for item in config.get("channels", [])}
        devices = {str(item.get("deviceKey", "")): item for item in config.get("devices", [])}
        default_interval = _config_interval(config)
        intervals: dict[str, float] = {}
        for device_key in groups:
            device = devices.get(device_key, {})
            channel = channels.get(str(device.get("channelKey", "")), {})
            intervals[device_key] = _config_interval(device, _config_interval(channel, default_interval))
        return groups, intervals

    def _initialize_points(self, config: dict[str, Any]) -> None:
        self.state.replace_points(iter_points(config))

    def _mqtt_status(self, name: str, connected: bool) -> None:
        config = self._config
        enabled = name == "default" and bool(config.get("mqtt", {}).get("enabled")) or any(
            str(item.get("name")) == name and item.get("enabled") for item in config.get("mqttChannels", [])
        )
        self.state.mqtt_channels[name] = {"name": name, "enabled": enabled, "connected": connected}
        self.state.mqtt_enabled = any(item.get("enabled") for item in self.state.mqtt_channels.values())
        self.state.mqtt_connected = any(item.get("connected") for item in self.state.mqtt_channels.values())
        self.state.publish("communications.diff", {
            "mqttEnabled": self.state.mqtt_enabled,
            "mqttConnected": self.state.mqtt_connected,
            "mqttChannels": self.state.mqtt_channels,
        })

    async def _store_history(self, metrics: dict[str, Any]) -> None:
        config = self._config.get("historyStorage", {})
        if not config.get("enabled"):
            return
        path = self._history_path()
        path.parent.mkdir(parents=True, exist_ok=True)
        record = {"time": iso_now(), "gatewayKey": self.state.gateway_key, "metrics": metrics}
        async with self._history_lock:
            await asyncio.to_thread(_append_json_line, path, record)
            max_bytes = max(16, int(config.get("maxSizeMB", 256))) * 1024 * 1024
            if path.stat().st_size > max_bytes:
                await asyncio.to_thread(_trim_file, path, max_bytes)

    def _history_path(self) -> Path:
        config = self._config.get("historyStorage", {})
        root = Path(str(config.get("storagePath") or self.store.path.parent / ".runtime"))
        return root / "gateway-history.jsonl"


def _append_json_line(path: Path, record: dict[str, Any]) -> None:
    with path.open("a", encoding="utf-8", newline="\n") as stream:
        stream.write(json.dumps(record, ensure_ascii=False, separators=(",", ":")) + "\n")


def _config_interval(value: dict[str, Any], fallback: float = 1.0) -> float:
    milliseconds = int(value.get("collectIntervalMilliseconds") or 0)
    if milliseconds > 0:
        return max(0.01, milliseconds / 1000.0)
    seconds = float(value.get("collectIntervalSeconds") or 0)
    return max(0.01, seconds if seconds > 0 else fallback)


def _trim_file(path: Path, max_bytes: int) -> None:
    with path.open("rb") as stream:
        stream.seek(max(0, path.stat().st_size - max_bytes // 2))
        data = stream.read()
    newline = data.find(b"\n")
    if newline >= 0:
        data = data[newline + 1:]
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_bytes(data)
    os.replace(temporary, path)
