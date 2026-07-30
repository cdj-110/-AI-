import asyncio
import unittest

from gateway.state import GatewayState


class StateBatchTests(unittest.IsolatedAsyncioTestCase):
    async def test_point_cycle_is_published_as_one_diff(self):
        state = GatewayState("test", 1.0)
        state.attach_loop(asyncio.get_running_loop())
        queue = state.subscribe()
        points = [
            {"deviceKey": "device-1", "metric": "p1", "protocol": "modbus-tcp"},
            {"deviceKey": "device-1", "metric": "p2", "protocol": "modbus-tcp"},
        ]

        state.update_points([(points[0], 10, ""), (points[1], 20, "")])
        message = await asyncio.wait_for(queue.get(), 1)

        self.assertEqual("points.diff", message["type"])
        self.assertEqual(2, len(message["payload"]["points"]))
        with self.assertRaises(asyncio.TimeoutError):
            await asyncio.wait_for(queue.get(), 0.02)

    async def test_unchanged_values_are_not_published_again(self):
        state = GatewayState("test", 1.0)
        state.attach_loop(asyncio.get_running_loop())
        queue = state.subscribe()
        point = {"deviceKey": "device-1", "metric": "p1", "protocol": "modbus-tcp"}

        state.update_points([(point, 10, "")])
        await asyncio.wait_for(queue.get(), 1)
        state.update_points([(point, 10, "")])

        with self.assertRaises(asyncio.TimeoutError):
            await asyncio.wait_for(queue.get(), 0.02)

    def test_compact_summary_uses_incremental_counts(self):
        state = GatewayState("test", 1.0)
        state.replace_points([
            {"deviceKey": "device-1", "metric": "p1"},
            {"deviceKey": "device-1", "metric": "p2"},
        ])
        state.update_points([({"deviceKey": "device-1", "metric": "p1"}, 10, "")])
        summary = state.summary(include_points=False)

        self.assertEqual(2, summary["pointCount"])
        self.assertEqual(1, summary["healthyCount"])
        self.assertEqual(1, summary["pendingCount"])
        self.assertNotIn("points", summary)


if __name__ == "__main__":
    unittest.main()
