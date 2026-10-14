from __future__ import annotations

import json
import ssl
import threading
import urllib.parse
from typing import Any, Callable


class MQTTChannel:
    def __init__(self, name: str, config: dict[str, Any], gateway_key: str, on_status: Callable[[bool], None]):
        self.name = name
        self.config = config
        self.gateway_key = gateway_key
        self.on_status = on_status
        self._client: Any = None
        self._connected = False
        self._lock = threading.RLock()

    @property
    def connected(self) -> bool:
        return self._connected

    def start(self) -> None:
        if not self.config.get("enabled") or not self.config.get("broker"):
            return
        try:
            import paho.mqtt.client as mqtt
        except ImportError as exc:
            raise RuntimeError("缺少 paho-mqtt，请安装 requirements.txt") from exc
        parsed = urllib.parse.urlparse(str(self.config["broker"]) if "://" in str(self.config["broker"]) else "mqtt://" + str(self.config["broker"]))
        client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2, client_id=str(self.config.get("clientId") or f"{self.gateway_key}-{self.name}"))
        if self.config.get("username"):
            client.username_pw_set(str(self.config["username"]), str(self.config.get("password", "")))
        if parsed.scheme in {"mqtts", "ssl", "tls"} or self.config.get("caFile"):
            client.tls_set(
                ca_certs=self.config.get("caFile") or None,
                certfile=self.config.get("certFile") or None,
                keyfile=self.config.get("keyFile") or None,
                tls_version=ssl.PROTOCOL_TLS_CLIENT,
            )
        client.on_connect = self._on_connect
        client.on_disconnect = lambda _client, _userdata, _flags, _reason, _properties: self._set_connected(False)
        client.connect_async(parsed.hostname or "127.0.0.1", parsed.port or (8883 if parsed.scheme in {"mqtts", "ssl", "tls"} else 1883), 30)
        client.loop_start()
        self._client = client

    def _on_connect(self, _client: Any, _userdata: Any, _flags: Any, reason: Any, _properties: Any) -> None:
        """Accept both legacy integer codes and Paho 2.x ReasonCode values."""
        value = getattr(reason, "value", reason)
        try:
            connected = int(value) == 0
        except (TypeError, ValueError):
            connected = not bool(getattr(reason, "is_failure", True))
        self._set_connected(connected)

    def stop(self) -> None:
        with self._lock:
            client, self._client = self._client, None
        if client:
            try:
                client.disconnect()
                client.loop_stop()
            finally:
                self._set_connected(False)

    def publish(self, metrics: dict[str, Any]) -> str:
        if not self._client or not self._connected:
            raise RuntimeError(f"MQTT 通道 {self.name} 未连接")
        topic = str(self.config.get("topicTemplate") or f"weikong/devices/{self.gateway_key}/telemetry")
        topic = topic.replace("{gatewayKey}", self.gateway_key).replace("{clientId}", str(self.config.get("clientId", "")))
        payload = json.dumps(metrics, ensure_ascii=False, separators=(",", ":"))
        info = self._client.publish(topic, payload, qos=int(self.config.get("qos", 1)), retain=bool(self.config.get("retain", False)))
        info.wait_for_publish(timeout=5)
        if info.rc != 0:
            raise RuntimeError(f"MQTT 发布失败，错误码 {info.rc}")
        return topic

    def _set_connected(self, value: bool) -> None:
        self._connected = value
        self.on_status(value)


class MQTTManager:
    def __init__(self, gateway_key: str, on_status: Callable[[str, bool], None]):
        self.gateway_key = gateway_key
        self.on_status = on_status
        self.channels: dict[str, MQTTChannel] = {}

    def configure(self, config: dict[str, Any]) -> None:
        self.stop()
        candidates = [("default", config.get("mqtt", {}))]
        candidates.extend((str(item.get("name") or f"mqtt-{index + 1}"), item) for index, item in enumerate(config.get("mqttChannels", [])))
        for name, channel_config in candidates:
            channel = MQTTChannel(name, channel_config, self.gateway_key, lambda connected, key=name: self.on_status(key, connected))
            self.channels[name] = channel
            if channel_config.get("enabled"):
                channel.start()

    def stop(self) -> None:
        for channel in self.channels.values():
            channel.stop()
        self.channels.clear()

    def publish(self, metrics: dict[str, Any]) -> list[str]:
        topics: list[str] = []
        for channel in self.channels.values():
            if channel.config.get("enabled") and channel.connected:
                topics.append(channel.publish(metrics))
        return topics
