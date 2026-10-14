from __future__ import annotations

import math
import struct
from typing import Any


class CollectorError(RuntimeError):
    pass


class CollectorSession:
    """A per-device collector session. Modbus keeps one connection across cycles."""

    def __init__(self, point: dict[str, Any]):
        self.protocol = str(point.get("protocol", "")).lower()
        self.client: Any = None
        if self.protocol in {"modbus-tcp", "modbus-rtu"}:
            self.client = _modbus_client(point, self.protocol)

    def read(self, point: dict[str, Any]) -> Any:
        if self.client is not None:
            return _read_modbus_client(self.client, point)
        return read_point(point)

    def read_many(self, points: list[dict[str, Any]]) -> list[tuple[Any, Exception | None]]:
        if self.client is None or self.protocol not in {"modbus-tcp", "modbus-rtu"}:
            results: list[tuple[Any, Exception | None]] = []
            for point in points:
                try:
                    results.append((self.read(point), None))
                except Exception as exc:
                    results.append((None, exc))
            return results
        return _read_modbus_points(self.client, points)

    def close(self) -> None:
        if self.client is not None:
            try:
                self.client.close()
            finally:
                self.client = None


def read_point(point: dict[str, Any]) -> Any:
    protocol = str(point.get("protocol", "")).lower()
    if protocol in {"modbus-tcp", "modbus-rtu"}:
        return _read_modbus(point, protocol)
    if protocol == "opcua":
        return _read_opcua(point)
    raise CollectorError(f"Python 版本暂未迁移协议采集：{protocol}")


def write_point(point: dict[str, Any], value: Any) -> None:
    protocol = str(point.get("protocol", "")).lower()
    if protocol in {"modbus-tcp", "modbus-rtu"}:
        _write_modbus(point, protocol, value)
        return
    if protocol == "opcua":
        _write_opcua(point, value)
        return
    raise CollectorError(f"Python 版本暂未迁移协议下发：{protocol}")


def _modbus_client(point: dict[str, Any], protocol: str):
    try:
        if protocol == "modbus-tcp":
            from pymodbus.client import ModbusTcpClient

            host, port = _host_port(str(point.get("address", "")), 502)
            return ModbusTcpClient(host, port=port, timeout=float(point.get("timeoutSeconds", 3)))
        from pymodbus.client import ModbusSerialClient

        return ModbusSerialClient(
            port=str(point.get("address", "")),
            baudrate=int(point.get("baudRate", 9600)),
            bytesize=int(point.get("dataBits", 8)),
            parity={"none": "N", "even": "E", "odd": "O"}.get(str(point.get("parity", "none")).lower(), "N"),
            stopbits=int(point.get("stopBits", 1)),
            timeout=float(point.get("timeoutSeconds", 3)),
        )
    except ImportError as exc:
        raise CollectorError("缺少 pymodbus/pyserial，请安装 requirements.txt") from exc


def _read_modbus(point: dict[str, Any], protocol: str) -> Any:
    client = _modbus_client(point, protocol)
    try:
        return _read_modbus_client(client, point)
    finally:
        client.close()


def _read_modbus_client(client: Any, point: dict[str, Any]) -> Any:
    unit = int(point.get("slaveId", 1))
    address = int(point.get("register", 0))
    function = int(point.get("function", 3))
    quantity = max(1, int(point.get("quantity") or _default_quantity(str(point.get("dataType", "uint16")))))
    if not getattr(client, "connected", False) and not client.connect():
        raise CollectorError(f"无法连接 {point.get('address', '')}")
    result = _modbus_request(client, unit, function, address, quantity)
    return _decode_modbus_result(result, point, 0)


def _modbus_request(client: Any, unit: int, function: int, address: int, quantity: int):
    kwargs = {"slave": unit}
    try:
        if function == 1:
            result = client.read_coils(address, count=quantity, **kwargs)
        elif function == 2:
            result = client.read_discrete_inputs(address, count=quantity, **kwargs)
        elif function == 3:
            result = client.read_holding_registers(address, count=quantity, **kwargs)
        elif function == 4:
            result = client.read_input_registers(address, count=quantity, **kwargs)
        else:
            raise CollectorError(f"不支持的 Modbus 功能码：{function}")
    except TypeError:
        kwargs = {"device_id": unit}
        if function == 1:
            result = client.read_coils(address, count=quantity, **kwargs)
        elif function == 2:
            result = client.read_discrete_inputs(address, count=quantity, **kwargs)
        elif function == 3:
            result = client.read_holding_registers(address, count=quantity, **kwargs)
        else:
            result = client.read_input_registers(address, count=quantity, **kwargs)
    if result.isError():
        raise CollectorError(str(result))
    return result


def _decode_modbus_result(result: Any, point: dict[str, Any], offset: int) -> Any:
    function = int(point.get("function", 3))
    quantity = max(1, int(point.get("quantity") or _default_quantity(str(point.get("dataType", "uint16")))))
    raw: Any
    if function in (1, 2):
        raw = bool(result.bits[offset])
    else:
        raw = _decode_registers(result.registers[offset:offset + quantity], point)
    if isinstance(raw, (int, float)) and not isinstance(raw, bool):
        raw = raw * float(point.get("scale", 1) or 1) + float(point.get("offset", 0) or 0)
        decimals = point.get("decimals")
        if decimals is not None and math.isfinite(float(raw)):
            raw = round(raw, int(decimals))
    return raw


def _read_modbus_points(client: Any, points: list[dict[str, Any]]) -> list[tuple[Any, Exception | None]]:
    results: list[tuple[Any, Exception | None]] = [(None, None) for _ in points]
    if not points:
        return results
    if not getattr(client, "connected", False) and not client.connect():
        error = CollectorError(f"无法连接 {points[0].get('address', '')}")
        return [(None, error) for _ in points]

    groups: list[list[tuple[int, dict[str, Any]]]] = []
    current: list[tuple[int, dict[str, Any]]] = []
    current_unit = 0
    current_function = 0
    current_start = 0
    current_end = 0
    indexed_points = sorted(
        enumerate(points),
        key=lambda item: (
            int(item[1].get("slaveId", 1)),
            int(item[1].get("function", 3)),
            int(item[1].get("register", 0)),
        ),
    )
    for index, point in indexed_points:
        unit = int(point.get("slaveId", 1))
        function = int(point.get("function", 3))
        address = int(point.get("register", 0))
        quantity = max(1, int(point.get("quantity") or _default_quantity(str(point.get("dataType", "uint16")))))
        point_end = address + quantity
        if (
            current
            and current_unit == unit
            and current_function == function
            and address >= current_start
            and max(current_end, point_end) - current_start <= 125
        ):
            current.append((index, point))
            current_end = max(current_end, point_end)
            continue
        current = [(index, point)]
        groups.append(current)
        current_unit = unit
        current_function = function
        current_start = address
        current_end = point_end

    for group in groups:
        first = group[0][1]
        start = min(int(point.get("register", 0)) for _, point in group)
        end = max(int(point.get("register", 0)) + max(1, int(point.get("quantity") or _default_quantity(str(point.get("dataType", "uint16"))))) for _, point in group)
        try:
            response = _modbus_request(client, int(first.get("slaveId", 1)), int(first.get("function", 3)), start, end - start)
            for index, point in group:
                try:
                    results[index] = (_decode_modbus_result(response, point, int(point.get("register", 0)) - start), None)
                except Exception as exc:
                    results[index] = (None, exc)
        except Exception as exc:
            for index, _ in group:
                results[index] = (None, exc)
    return results


def _write_modbus(point: dict[str, Any], protocol: str, value: Any) -> None:
    client = _modbus_client(point, protocol)
    unit = int(point.get("slaveId", 1))
    address = int(point.get("register", 0))
    function = int(point.get("function", 3))
    if not client.connect():
        raise CollectorError(f"无法连接 {point.get('address', '')}")
    try:
        kwargs = {"slave": unit}
        try:
            if function == 1:
                result = client.write_coil(address, bool(value), **kwargs)
            elif function == 3:
                registers = _encode_registers(value, point)
                result = client.write_register(address, registers[0], **kwargs) if len(registers) == 1 else client.write_registers(address, registers, **kwargs)
            else:
                raise CollectorError("离散输入和输入寄存器为只读点位")
        except TypeError:
            kwargs = {"device_id": unit}
            if function == 1:
                result = client.write_coil(address, bool(value), **kwargs)
            else:
                registers = _encode_registers(value, point)
                result = client.write_register(address, registers[0], **kwargs) if len(registers) == 1 else client.write_registers(address, registers, **kwargs)
        if result.isError():
            raise CollectorError(str(result))
    finally:
        client.close()


def _decode_registers(registers: list[int], point: dict[str, Any]) -> Any:
    data_type = str(point.get("dataType", "uint16")).lower()
    byte_order = str(point.get("byteOrder", "big")).lower()
    word_order = str(point.get("wordOrder", "normal")).lower()
    words = list(registers)
    if word_order in {"swap", "little"} and len(words) > 1:
        words.reverse()
    endian = ">" if byte_order in {"big", "abcd", "badc"} else "<"
    raw = b"".join(word.to_bytes(2, "big" if endian == ">" else "little") for word in words)
    formats = {"uint16": "H", "int16": "h", "uint32": "I", "int32": "i", "float32": "f", "float64": "d"}
    if data_type == "bool":
        bit = int(point.get("bitIndex", 0) or 0)
        return bool((words[0] >> bit) & 1)
    if data_type == "string":
        return raw.rstrip(b"\x00").decode("utf-8", errors="replace")
    fmt = formats.get(data_type, "H")
    size = struct.calcsize(fmt)
    if len(raw) < size:
        raise CollectorError(f"{data_type} 至少需要 {size // 2} 个寄存器")
    return struct.unpack(endian + fmt, raw[:size])[0]


def _encode_registers(value: Any, point: dict[str, Any]) -> list[int]:
    data_type = str(point.get("dataType", "uint16")).lower()
    scale = float(point.get("scale", 1) or 1)
    offset = float(point.get("offset", 0) or 0)
    raw_value = (float(value) - offset) / scale if data_type != "string" else value
    endian = ">" if str(point.get("byteOrder", "big")).lower() in {"big", "abcd", "badc"} else "<"
    formats = {"uint16": "H", "int16": "h", "uint32": "I", "int32": "i", "float32": "f", "float64": "d", "bool": "H"}
    fmt = formats.get(data_type, "H")
    if data_type == "bool":
        encoded_value: Any = 1 if bool(raw_value) else 0
    elif data_type in {"uint16", "int16", "uint32", "int32"}:
        encoded_value = int(round(raw_value))
    else:
        encoded_value = raw_value
    packed = struct.pack(endian + fmt, encoded_value)
    words = [int.from_bytes(packed[index:index + 2], "big" if endian == ">" else "little") for index in range(0, len(packed), 2)]
    if str(point.get("wordOrder", "normal")).lower() in {"swap", "little"} and len(words) > 1:
        words.reverse()
    return words


def _read_opcua(point: dict[str, Any]) -> Any:
    try:
        from asyncua.sync import Client
    except ImportError as exc:
        raise CollectorError("缺少 asyncua，请安装 requirements.txt") from exc
    client = Client(str(point.get("address", "")), timeout=float(point.get("timeoutSeconds", 5)))
    if point.get("username"):
        client.set_user(str(point["username"]))
        client.set_password(str(point.get("password", "")))
    with client:
        return client.get_node(str(point.get("nodeId", ""))).read_value()


def _write_opcua(point: dict[str, Any], value: Any) -> None:
    try:
        from asyncua.sync import Client
    except ImportError as exc:
        raise CollectorError("缺少 asyncua，请安装 requirements.txt") from exc
    client = Client(str(point.get("address", "")), timeout=float(point.get("timeoutSeconds", 5)))
    if point.get("username"):
        client.set_user(str(point["username"]))
        client.set_password(str(point.get("password", "")))
    with client:
        client.get_node(str(point.get("nodeId", ""))).write_value(value)


def _host_port(address: str, default_port: int) -> tuple[str, int]:
    text = address
    if text.startswith("tcp://"):
        text = text[len("tcp://") :]
    if ":" not in text:
        return text, default_port
    host, port = text.rsplit(":", 1)
    return host, int(port)


def _default_quantity(data_type: str) -> int:
    return {"uint32": 2, "int32": 2, "float32": 2, "float64": 4}.get(data_type.lower(), 1)
