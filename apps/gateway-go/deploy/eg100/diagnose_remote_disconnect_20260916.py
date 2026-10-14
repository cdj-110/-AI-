"""Read-only diagnostics for EG100 remote tunnel disconnects."""

import os
import sys

from mysql_live_20260912 import Gateway


def main() -> int:
    gateway = Gateway()
    try:
        command = r'''
echo "== process =="
PIDS="$(pidof weikong-gateway 2>/dev/null || true)"
echo "pids=$PIDS"
for PID in $PIDS; do
  echo "pid=$PID cmd=$(tr '\000' ' ' </proc/$PID/cmdline 2>/dev/null)"
  ls -l /proc/$PID/fd/1 /proc/$PID/fd/2 2>/dev/null || true
done
echo "== likely logs =="
find /userdata/weikong /mnt/udisk /var/log /tmp -maxdepth 4 -type f \
  \( -name '*.log' -o -name '*gateway*' -o -name 'messages' \) \
  -mmin -1440 -print 2>/dev/null | head -80
echo "== remote log lines =="
for FILE in $(find /userdata/weikong /mnt/udisk /var/log /tmp -maxdepth 4 -type f \
  \( -name '*.log' -o -name '*gateway*' -o -name 'messages' \) \
  -mmin -1440 -print 2>/dev/null); do
  grep -aE '\[REMOTE\]|tunnel disconnected|i/o timeout|websocket' "$FILE" 2>/dev/null | tail -40 | sed "s#^#$FILE: #"
done
echo "== sockets =="
netstat -tnp 2>/dev/null | grep -E '(:3100|:3201|:22)' || true
echo "== activation binding =="
ACTIVATION_FILE="$(sed -n 's/.*"file"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' /userdata/weikong/config/config.local.json | head -1)"
case "$ACTIVATION_FILE" in
  /*) ;;
  "") ACTIVATION_FILE=/userdata/weikong/config/activation.local.json ;;
  *) ACTIVATION_FILE=/userdata/weikong/config/$(basename "$ACTIVATION_FILE") ;;
esac
CONFIGURED_ID="$(sed -n 's/.*"hardwareId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$ACTIVATION_FILE" 2>/dev/null | head -1)"
RAW_ID=""
ID_SOURCE="unavailable"
for CANDIDATE in /sys/block/mmcblk0/device/cid /sys/class/dmi/id/product_uuid /sys/class/dmi/id/product_serial /sys/class/dmi/id/board_serial; do
  if [ -r "$CANDIDATE" ] && [ -n "$(tr -d '[:space:]' <"$CANDIDATE")" ]; then
    RAW_ID="$(cat "$CANDIDATE")"
    ID_SOURCE="$CANDIDATE"
    break
  fi
done
if [ -z "$RAW_ID" ]; then
  RAW_ID="$(sed -n 's/^[Ss]erial[[:space:]]*:[[:space:]]*//p' /proc/cpuinfo | head -1)"
  if [ -n "$RAW_ID" ]; then ID_SOURCE=/proc/cpuinfo; fi
fi
if [ -z "$RAW_ID" ] && [ -r /etc/machine-id ]; then
  RAW_ID="$(cat /etc/machine-id)"
  if [ -n "$RAW_ID" ]; then ID_SOURCE=/etc/machine-id; fi
fi
if [ -n "$RAW_ID" ]; then
  CURRENT_ID="$(printf '%s' "$RAW_ID" | sha256sum | cut -c1-32 | tr 'a-f' 'A-F')"
else
  CURRENT_ID=""
fi
if [ -n "$CONFIGURED_ID" ] && [ "$CONFIGURED_ID" = "$CURRENT_ID" ]; then MATCH=true; else MATCH=false; fi
echo "activation_file=$ACTIVATION_FILE"
echo "identity_source=$ID_SOURCE"
echo "configured_hardware_id=$CONFIGURED_ID"
echo "current_hardware_id=$CURRENT_ID"
echo "hardware_id_match=$MATCH"
'''
        print(gateway.shell(command, timeout=60))
    finally:
        gateway.ssh.close()
    return 0


if __name__ == "__main__":
    if not os.environ.get("WK_EG100_PASSWORD"):
        print("WK_EG100_PASSWORD is required", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main())
