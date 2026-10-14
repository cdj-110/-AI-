import unittest

from gateway.collectors import _decode_registers, _encode_registers, _read_modbus_points


class _Response:
    def __init__(self, registers):
        self.registers = registers

    def isError(self):
        return False


class _Client:
    connected = True

    def __init__(self):
        self.calls = []

    def read_holding_registers(self, address, count, **_kwargs):
        self.calls.append((address, count))
        return _Response([100 + offset for offset in range(address, address + count)])


class CollectorCodecTests(unittest.TestCase):
    def test_float32_round_trip(self):
        point = {"dataType": "float32", "byteOrder": "big", "wordOrder": "normal", "scale": 1, "offset": 0}
        registers = _encode_registers(12.5, point)
        self.assertAlmostEqual(_decode_registers(registers, point), 12.5)

    def test_scaled_uint16_write(self):
        point = {"dataType": "uint16", "byteOrder": "big", "scale": 0.1, "offset": 0}
        self.assertEqual(_encode_registers(12.3, point), [123])

    def test_unsorted_registers_are_batched_and_returned_in_original_order(self):
        points = [
            {"slaveId": 1, "function": 3, "register": register, "quantity": 1, "dataType": "uint16"}
            for register in (2, 0, 1)
        ]
        client = _Client()

        results = _read_modbus_points(client, points)

        self.assertEqual(client.calls, [(0, 3)])
        self.assertEqual([value for value, error in results if error is None], [102, 100, 101])


if __name__ == "__main__":
    unittest.main()
