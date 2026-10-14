import json
import tempfile
import unittest
from pathlib import Path

from gateway.configuration import ConfigStore, ConfigurationError, iter_points


class ConfigurationTests(unittest.TestCase):
    def test_round_trip_and_flatten(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "py-test",
                "devices": [{
                    "deviceKey": "device-1", "name": "设备1", "protocol": "modbus-tcp", "address": "127.0.0.1:502",
                    "points": [{"name": "温度", "metric": "temperature", "function": 3, "register": 0}],
                }],
            }), encoding="utf-8")
            store = ConfigStore(path)
            point = list(iter_points(store.get()))[0]
            self.assertEqual(point["deviceKey"], "device-1")
            self.assertEqual(point["address"], "127.0.0.1:502")
            self.assertEqual(point["metric"], "temperature")

    def test_global_identifier_must_be_unique(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "py-test",
                "devices": [
                    {"deviceKey": "a", "protocol": "modbus-tcp", "points": [{"metric": "same"}]},
                    {"deviceKey": "b", "protocol": "modbus-tcp", "points": [{"metric": "SAME"}]},
                ],
            }), encoding="utf-8")
            with self.assertRaises(ConfigurationError):
                ConfigStore(path)

    def test_device_connection_overrides_stale_point_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "py-test",
                "devices": [{
                    "deviceKey": "device-1",
                    "name": "设备1",
                    "protocol": "modbus-tcp",
                    "address": "192.168.1.232:502",
                    "slaveId": 7,
                    "points": [{
                        "name": "保持寄存器",
                        "metric": "holding-1",
                        "address": "192.168.1.100:502",
                        "slaveId": 1,
                    }],
                }],
            }), encoding="utf-8")

            point = list(iter_points(ConfigStore(path).get()))[0]

            self.assertEqual(point["address"], "192.168.1.232:502")
            self.assertEqual(point["slaveId"], 7)

    def test_compact_config_and_point_window(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "py-test",
                "devices": [{
                    "deviceKey": "device-1",
                    "protocol": "modbus-tcp",
                    "points": [
                        {"name": "coil", "metric": "p1", "function": 1},
                        {"name": "holding-a", "metric": "p2", "function": 3},
                        {"name": "holding-b", "metric": "p3", "function": 3},
                    ],
                }],
            }), encoding="utf-8")
            store = ConfigStore(path)

            compact, counts, total = store.compact()
            rows, filtered_total = store.point_window("device-1", 1, 1, function=3)

            self.assertEqual([], compact["devices"][0]["points"])
            self.assertEqual(3, counts["device-1"])
            self.assertEqual(3, total)
            self.assertEqual(2, filtered_total)
            self.assertEqual("p3", rows[0]["metric"])

    def test_legacy_network_port_and_channel_create_resource_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            path.write_text(json.dumps({
                "gatewayKey": "py-test",
                "networkPorts": [{"name": "网口1", "interface": "eth0", "mode": "dhcp", "enabled": True}],
                "channels": [{
                    "channelKey": "modbus",
                    "resourceKey": "eth0",
                    "name": "Modbus采集",
                    "type": "network",
                    "protocol": "modbus-tcp",
                    "enabled": True,
                }],
            }), encoding="utf-8")

            config = ConfigStore(path).get()

            self.assertEqual(1, len(config["resources"]))
            self.assertEqual("eth0", config["resources"][0]["resourceKey"])
            self.assertEqual("network", config["resources"][0]["type"])
            self.assertEqual("eth0", config["resources"][0]["network"]["interface"])


if __name__ == "__main__":
    unittest.main()
