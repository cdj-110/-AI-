from __future__ import annotations

import argparse

from pymodbus.datastore import ModbusSequentialDataBlock, ModbusServerContext, ModbusSlaveContext
from pymodbus.server import StartTcpServer


def main() -> None:
    parser = argparse.ArgumentParser(description="Modbus TCP register simulator for gateway load tests")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=15020)
    parser.add_argument("--registers", type=int, default=30_000)
    args = parser.parse_args()
    values = [index % 10_000 for index in range(args.registers + 2)]
    slave = ModbusSlaveContext(hr=ModbusSequentialDataBlock(0, values))
    context = ModbusServerContext(slaves=slave, single=True)
    print(f"Modbus simulator listening on {args.host}:{args.port}, registers={args.registers}", flush=True)
    StartTcpServer(context=context, address=(args.host, args.port))


if __name__ == "__main__":
    main()
