#!/bin/sh
set -eu

echo "FIX_DHCPCD"
cp /etc/dhcpcd.conf "/etc/dhcpcd.conf.bak-weikong-$(date +%Y%m%d%H%M%S)"
sed -i \
  -e '/^n#/d' \
  -e '/^# Managed by weikong: keep eth0 static/d' \
  -e '/^denyinterfaces eth0/d' \
  /etc/dhcpcd.conf
{
  echo ""
  echo "# Managed by weikong: keep eth0 static via /etc/network/interfaces.d/eth0"
  echo "denyinterfaces eth0"
} >> /etc/dhcpcd.conf
tail -8 /etc/dhcpcd.conf

echo "RESTART_DHCPCD"
/etc/init.d/S41dhcpcd restart || true
sleep 2

echo "CLEAN_ADDR"
ip link set eth0 up
ip addr add 192.168.1.214/24 dev eth0 2>/dev/null || true
ip addr del 192.168.1.81/24 dev eth0 2>/dev/null || true
ip addr del 192.168.1.82/24 dev eth0 2>/dev/null || true
ip route replace 192.168.1.0/24 dev eth0 src 192.168.1.214
ip route replace default via 192.168.1.1 dev eth0 src 192.168.1.214 metric 100

echo "RESULT_ADDR"
ip -br addr show eth0
echo "RESULT_ROUTE"
ip route
echo "PING"
ping -c 2 -W 2 www.baidu.com
