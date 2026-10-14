"""Read-only RJGT102 capability check for EG100."""

import os

from mysql_live_20260912 import Gateway


def main():
    gateway = Gateway()
    try:
        print(gateway.shell(r'''
echo "== version =="
cat /etc/version 2>/dev/null || true
echo "== vendor files =="
ls -l /usr/lib/libcryproSdr.so /usr/lib/libcryptoSdr.a /usr/include/cryproSdr.h /usr/include/cryptoSdr.h 2>/dev/null || true
echo "== i2c device 0x68 =="
find /sys/bus/i2c/devices -maxdepth 1 \( -iname '*-0068' -o -iname '*-0068:*' \) -print 2>/dev/null || true
echo "== related modules =="
lsmod 2>/dev/null | grep -iE 'crypto|sdr|rjgt' || true
echo "== i2c buses =="
ls -1 /dev/i2c-* 2>/dev/null || true
'''))
    finally:
        gateway.ssh.close()


if __name__ == "__main__":
    if not os.environ.get("WK_EG100_PASSWORD"):
        raise SystemExit("WK_EG100_PASSWORD is required")
    main()
