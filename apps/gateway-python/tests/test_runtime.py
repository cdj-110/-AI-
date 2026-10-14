import json
import tempfile
import unittest
from pathlib import Path

from gateway.configuration import ConfigStore
from gateway.runtime import GatewayRuntime


class RuntimeSchedulingTests(unittest.TestCase):
    def test_channel_interval_is_applied_per_device(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "schedule-test",
                "collectIntervalSeconds": 5,
                "channels": [{"channelKey": "fast", "collectIntervalSeconds": 1}],
                "devices": [{
                    "deviceKey": "device-1", "channelKey": "fast", "protocol": "modbus-tcp", "address": "127.0.0.1:502",
                    "points": [{"metric": "point-1", "function": 3, "register": 0}],
                }],
            }), encoding="utf-8")
            runtime = GatewayRuntime(ConfigStore(path))
            groups, intervals = runtime._group_points(runtime.store.get())
            self.assertEqual(len(groups["device-1"]), 1)
            self.assertEqual(intervals["device-1"], 1.0)

    def test_millisecond_interval_takes_precedence(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "schedule-test",
                "collectIntervalSeconds": 5,
                "channels": [{"channelKey": "fast", "collectIntervalSeconds": 1, "collectIntervalMilliseconds": 100}],
                "devices": [{
                    "deviceKey": "device-1", "channelKey": "fast", "protocol": "modbus-tcp", "address": "127.0.0.1:502",
                    "points": [{"metric": "point-1", "function": 3, "register": 0}],
                }],
            }), encoding="utf-8")
            runtime = GatewayRuntime(ConfigStore(path))
            _, intervals = runtime._group_points(runtime.store.get())
            self.assertEqual(intervals["device-1"], 0.1)


if __name__ == "__main__":
    unittest.main()
