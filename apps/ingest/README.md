# MQTT ingest service

Subscribes to:

- `weikong/devices/+/heartbeat`
- `weikong/devices/+/telemetry`

The `+` segment is the device `deviceKey`. Any heartbeat payload marks the device online and refreshes `lastSeenAt`. Devices without a heartbeat within `DEVICE_OFFLINE_TIMEOUT_SECONDS` are marked offline.

Telemetry payloads are JSON objects and are written to TimescaleDB:

```json
{ "temperature": 23.6, "humidity": 58 }
```

Example:

```bash
mosquitto_pub -h localhost -p 1883 \
  -t weikong/devices/wk-1000/heartbeat \
  -m '{"timestamp":"2026-06-02T00:00:00Z"}'
```
