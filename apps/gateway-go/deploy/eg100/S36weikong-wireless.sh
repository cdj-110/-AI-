#!/bin/sh

# Load the vendor WiFi/BT stack only when the persistent opt-in marker exists.
# EG100 units have very little RAM and the AIC8800 driver consumes roughly
# 36 MiB of unreclaimable slab memory even while wlan0 is idle.

MARKER="/userdata/weikong/config/wireless-enabled"
VENDOR_INIT="/userdata/weikong/lib/wireless/S36wifibt-init.vendor.sh"

case "${1:-start}" in
	start)
		if [ -f "$MARKER" ]; then
			"$VENDOR_INIT" start
		else
			echo "WiFi/Bluetooth disabled (use: weikong-wireless enable, then reboot)"
		fi
		;;
	stop)
		if [ -f "$MARKER" ]; then
			"$VENDOR_INIT" stop
		fi
		;;
	restart)
		if [ -f "$MARKER" ]; then
			"$VENDOR_INIT" restart
		else
			echo "WiFi/Bluetooth disabled"
		fi
		;;
	*)
		echo "Usage: $0 {start|stop|restart}" >&2
		exit 3
		;;
esac
