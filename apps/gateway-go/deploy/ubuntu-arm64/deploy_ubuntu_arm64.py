#!/usr/bin/env python3
"""Deploy the ARM64 gateway test build to the designated Ubuntu host.

Credentials are read from environment variables and are never written into the
repository. The OPC DA password is written only to the target's mode-0600
configuration file.
"""

from __future__ import annotations

import argparse
import io
import json
import os
import shlex
import sys
from pathlib import Path

import paramiko


REMOTE_ROOT = "/opt/weikong-gateway"


def connect(host: str, user: str, password: str) -> paramiko.SSHClient:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, username=user, password=password, timeout=15)
    return client


def run(client: paramiko.SSHClient, command: str, *, password: str = "", sudo: bool = False) -> str:
    remote_command = command
    if sudo:
        remote_command = "sudo -S -p '' sh -c " + shlex.quote(command)
    stdin, stdout, stderr = client.exec_command(remote_command, timeout=120)
    if sudo:
        stdin.write(password + "\n")
        stdin.flush()
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status = stdout.channel.recv_exit_status()
    if status != 0:
        raise RuntimeError(f"remote command failed ({status}): {error.strip() or output.strip()}")
    return output


def inspect(client: paramiko.SSHClient) -> None:
    command = """
set -eu
uname -a
getconf GNU_LIBC_VERSION
printf 'systemctl='; command -v systemctl || true
printf 'logrotate='; command -v logrotate || true
printf 'python3='; command -v python3 || true
printf 'gcc='; command -v gcc || true
printf 'go='; command -v go || true
go version 2>/dev/null || true
free -m
df -h /
ip -4 route
if ping -c 1 -W 2 192.168.1.163 >/dev/null; then echo OPC_SERVER_REACHABLE; else echo OPC_SERVER_UNREACHABLE; fi
python3 - <<'PY'
import socket
s = socket.socket()
s.settimeout(3)
try:
    s.connect(('192.168.1.163', 135))
    print('RPC135_OPEN')
except OSError as exc:
    print('RPC135_CLOSED', exc)
finally:
    s.close()
PY
ss -lntp | head -20
"""
    print(run(client, command))


def status(client: paramiko.SSHClient, password: str) -> None:
    command = """
set -eu
systemctl --version | head -1
systemctl --no-pager --full status weikong-gateway.service || true
systemctl show weikong-gateway.service -p MainPID -p MemoryCurrent -p TasksCurrent -p NRestarts
curl -fsS http://127.0.0.1:8088/api/healthz || true
echo
pid=$(systemctl show weikong-gateway.service -p MainPID --value)
if [ "${pid:-0}" -gt 0 ]; then
    ps -o pid,ppid,rss,vsz,etime,cmd -p "$pid"
    printf 'FD_COUNT='; find "/proc/$pid/fd" -mindepth 1 -maxdepth 1 | wc -l
    printf 'CHILD_COUNT='; pgrep -P "$pid" | wc -l
    for fd in /proc/$pid/fd/*; do
        target=$(readlink "$fd" 2>/dev/null || true)
        case "$target" in
            socket:*) printf 'SOCKET_FD_%s_FLAGS=' "${fd##*/}"; awk '/^flags:/{print $2}' "/proc/$pid/fdinfo/${fd##*/}" ;;
        esac
    done
fi
ss -ntp | grep -E '192.168.1.163:(135|[0-9]+)' || true
if [ -f /var/log/weikong-gateway/gateway.log ]; then
    stat -c 'LOG_SIZE_BYTES=%s LOG_MODIFIED=%y' /var/log/weikong-gateway/gateway.log
    echo '--- RECENT_GATEWAY_LOG ---'
    tail -80 /var/log/weikong-gateway/gateway.log
fi
"""
    print(run(client, command, password=password, sudo=True))


def diagnose(client: paramiko.SSHClient, web_password: str) -> None:
    """Collect a compact, read-only failure snapshot without reconnecting OPC DA."""
    command = """
set -eu
systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID -p NRestarts -p MemoryCurrent -p TasksCurrent
pid=$(systemctl show weikong-gateway.service -p MainPID --value)
if [ "${pid:-0}" -gt 0 ]; then
    ps -o rss=,etime= -p "$pid" | awk '{print "PROCESS_RSS_KB=" $1 " PROCESS_ELAPSED=" $2}'
    printf 'FD_COUNT='; find "/proc/$pid/fd" -mindepth 1 -maxdepth 1 | wc -l
fi
"""
    print(run(client, command))

    script = f"""
import collections
import http.cookiejar
import json
import socket
import time
import urllib.parse
import urllib.request

base = 'http://127.0.0.1:8088'
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
login = urllib.parse.urlencode({{'username': 'admin', 'password': {web_password!r}, 'next': '/'}}).encode()
opener.open(urllib.request.Request(base + '/login', data=login), timeout=10).read()

def get_json(path):
    response = opener.open(base + path, timeout=10)
    return response.status, json.loads(response.read())

health = opener.open(base + '/api/healthz', timeout=10)
print('HEALTH_HTTP=' + str(health.status))

try:
    _, wrapper = get_json('/api/config')
    config = json.loads(wrapper.get('content', '{{}}')) if isinstance(wrapper, dict) and isinstance(wrapper.get('content'), str) else wrapper
    devices = list(config.get('devices', [])) if isinstance(config, dict) else []
    if isinstance(config, dict):
        for channel in config.get('channels', []):
            devices.extend(channel.get('devices', []))
    target = next((item for item in devices if item.get('deviceKey') == 'opcda-matrikon-test'), None)
    if target is None:
        target = next((item for item in devices if str(item.get('protocol', '')).lower() == 'opcda'), {{}})
    print('CONFIG_READ_MODE=' + str(target.get('readMode', 'unknown')))
    print('CONFIG_ENABLED=' + str(target.get('enabled', 'unknown')).lower())
    address = str(target.get('address', 'unknown'))
    print('CONFIG_ADDRESS=' + address)
    configured_points = target.get('points', []) if isinstance(target, dict) else []
    print('CONFIG_POINT_COUNT=' + str(len(configured_points)))
    for point_index, point in enumerate(configured_points[:10], 1):
        print('CONFIG_POINT_' + str(point_index) + '=' + str(point.get('itemId') or point.get('address') or point.get('name') or '')[:160])
    sock = socket.socket()
    sock.settimeout(3)
    try:
        sock.connect((address, 135))
        print('CONFIGURED_OPC_RPC135=reachable')
    except OSError as socket_error:
        print('CONFIGURED_OPC_RPC135=unreachable ERROR=' + str(socket_error)[:160].replace('\\n', ' '))
    finally:
        sock.close()
except Exception as exc:
    print('CONFIG_READ_ERROR=' + type(exc).__name__ + ':' + str(exc)[:160].replace('\\n', ' '))

def points_snapshot():
    _, payload = get_json('/api/point-status')
    if isinstance(payload, dict):
        return payload.get('points', payload.get('items', []))
    return payload if isinstance(payload, list) else []

first = points_snapshot()
time.sleep(2)
second = points_snapshot()
print('POINT_COUNT=' + str(len(second)))
print('QUALITY_COUNTS=' + json.dumps(collections.Counter(str(item.get('quality', '')).upper() or 'EMPTY' for item in second), ensure_ascii=False, sort_keys=True))
print('MODE_COUNTS=' + json.dumps(collections.Counter(str(item.get('collectionMode', '')) or 'EMPTY' for item in second), ensure_ascii=False, sort_keys=True))
print('STALE_COUNT=' + str(sum(1 for item in second if item.get('stale'))))
print('ERROR_COUNT=' + str(sum(1 for item in second if item.get('error'))))

def key(item):
    return str(item.get('identifier') or item.get('pointKey') or item.get('itemId') or item.get('name') or '')

first_by_key = {{key(item): item for item in first}}
source_advanced = receive_advanced = value_changed = 0
for item in second:
    previous = first_by_key.get(key(item), {{}})
    source_advanced += bool(item.get('sourceTimestamp') and item.get('sourceTimestamp') != previous.get('sourceTimestamp'))
    receive_advanced += bool(item.get('receiveTimestamp') and item.get('receiveTimestamp') != previous.get('receiveTimestamp'))
    value_changed += item.get('value') != previous.get('value')
print('SOURCE_TIME_ADVANCED_2S=' + str(source_advanced))
print('RECEIVE_TIME_ADVANCED_2S=' + str(receive_advanced))
print('VALUE_CHANGED_2S=' + str(value_changed))

seen = set()
for item in second:
    error = str(item.get('error') or '').replace('\\r', ' ').replace('\\n', ' ')
    diagnostic = str(item.get('diagnostic') or item.get('status') or '').replace('\\r', ' ').replace('\\n', ' ')
    if not error and str(item.get('quality', '')).upper() == 'GOOD':
        continue
    signature = (error, diagnostic, str(item.get('quality', '')))
    if signature in seen:
        continue
    seen.add(signature)
    print('AFFECTED_POINT=' + key(item)[:120])
    print('AFFECTED_QUALITY=' + str(item.get('quality', '')))
    print('AFFECTED_DIAGNOSTIC=' + diagnostic[:300])
    print('AFFECTED_ERROR=' + error[:700])
    if len(seen) >= 5:
        break
"""
    stdin, stdout, stderr = client.exec_command("python3 -", timeout=30)
    stdin.write(script)
    stdin.channel.shutdown_write()
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status_code = stdout.channel.recv_exit_status()
    if status_code != 0:
        raise RuntimeError(error.strip() or output.strip())
    print(output)


def http_test(client: paramiko.SSHClient, web_password: str) -> None:
    script = f"""
import http.cookiejar
import json
import urllib.parse
import urllib.request

base = 'http://127.0.0.1:8088'
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
login = urllib.parse.urlencode({{'username': 'admin', 'password': {web_password!r}, 'next': '/'}}).encode()
response = opener.open(urllib.request.Request(base + '/login', data=login), timeout=10)
print('LOGIN_FINAL_STATUS=' + str(response.status))
print('LOGIN_FINAL_PATH=' + urllib.parse.urlparse(response.url).path)
for path in ('/', '/api/status', '/api/config/bootstrap', '/api/point-status'):
    response = opener.open(base + path, timeout=10)
    body = response.read()
    print('HTTP ' + path + ' ' + str(response.status) + ' bytes=' + str(len(body)))
    if path == '/api/status':
        payload = json.loads(body)
        print('STATUS_POINT_COUNT=' + str(payload.get('pointCount', payload.get('totalPoints', 'unknown'))))
    elif path == '/api/config/bootstrap':
        payload = json.loads(body)
        encoded = json.dumps(payload).lower()
        print('BOOTSTRAP_HAS_OPCDA=' + str('opcda' in encoded).lower())
    elif path == '/api/point-status':
        payload = json.loads(body)
        if isinstance(payload, list):
            count = len(payload)
        elif isinstance(payload, dict):
            values = payload.get('points', payload.get('items', []))
            count = len(values) if isinstance(values, list) else 0
        else:
            count = 0
        print('POINT_STATUS_COUNT=' + str(count))
        if count:
            point = payload[0] if isinstance(payload, list) else values[0]
            print('POINT_COLLECTION_MODE=' + str(point.get('collectionMode', '')))
            print('POINT_QUALITY=' + str(point.get('quality', '')))
            print('POINT_STALE=' + str(bool(point.get('stale'))).lower())
            error = str(point.get('error', '')).replace('\\r', ' ').replace('\\n', ' ')
            print('POINT_ERROR=' + error[:500])

connection = {{
    'deviceKey': 'opcda-matrikon-test',
    'protocol': 'opcda',
    'address': '192.168.1.163',
    'clsid': 'F8582CF2-88FB-11D0-B850-00C0F0104305',
    'domain': 'CDJ-PC',
    'workstation': 'UBUNTU219',
    'username': 'opcuser',
    'password': '***',
    'securityMode': 'packet_integrity',
    'authentication': 'ntlm',
    'connectTimeoutMilliseconds': 10000,
    'requestTimeoutMilliseconds': 10000,
    'groupName': 'WK_OPCDA_UBUNTU219_TEST',
    'requestedUpdateRateMilliseconds': 1000,
    'localeId': 1033,
}}
def post_json(path, payload):
    request = urllib.request.Request(
        base + path,
        data=json.dumps(payload).encode(),
        headers={{'Content-Type': 'application/json'}},
    )
    response = opener.open(request, timeout=90)
    return response.status, json.loads(response.read())

code, payload = post_json('/api/connection/test', connection)
print('OPCDA_CONNECTION_TEST_HTTP=' + str(code))
print('OPCDA_CONNECTION_TEST_OK=' + str(bool(payload.get('ok'))).lower())
print('OPCDA_CONNECTION_TEST_STAGE=' + str(payload.get('stage', '')))
browse = dict(connection)
browse.update({{'recursive': True, 'limit': 1000, 'branchItemId': ''}})
code, payload = post_json('/api/opcda/browse', browse)
print('OPCDA_BROWSE_HTTP=' + str(code))
print('OPCDA_BROWSE_OK=' + str(bool(payload.get('ok'))).lower())
print('OPCDA_BROWSE_COUNT=' + str(payload.get('count', 0)))
"""
    stdin, stdout, stderr = client.exec_command("python3 -", timeout=60)
    stdin.write(script)
    stdin.channel.shutdown_write()
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status_code = stdout.channel.recv_exit_status()
    if status_code != 0:
        raise RuntimeError(error.strip() or output.strip())
    print(output)


def restart_test(client: paramiko.SSHClient, password: str) -> None:
    command = """
set -eu
systemctl restart weikong-gateway.service
for attempt in $(seq 1 20); do
    if curl -fsS http://127.0.0.1:8088/api/healthz >/dev/null; then
        echo RESTART_HEALTHY_AFTER_SECONDS=$attempt
        systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID -p NRestarts
        exit 0
    fi
    sleep 1
done
echo RESTART_HEALTH_TIMEOUT
exit 1
"""
    print(run(client, command, password=password, sudo=True))


def artifact_test(client: paramiko.SSHClient, password: str) -> None:
    command = f"""
set -eu
sha256sum {REMOTE_ROOT}/bin/weikong-gateway {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
file {REMOTE_ROOT}/bin/weikong-gateway {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
ldd {REMOTE_ROOT}/bin/weikong-gateway
ldd {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
stat -c 'CONFIG_MODE=%a CONFIG_OWNER=%U:%G' {REMOTE_ROOT}/config/config.local.json
systemd-analyze verify /etc/systemd/system/weikong-gateway.service
"""
    print(run(client, command, password=password, sudo=True))


def mode_test(client: paramiko.SSHClient, web_password: str, read_mode: str) -> None:
    before_pid = run(client, "systemctl show weikong-gateway.service -p MainPID --value").strip()
    script = f"""
import http.cookiejar
import json
import time
import urllib.error
import urllib.parse
import urllib.request

base = 'http://127.0.0.1:8088'
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
login = urllib.parse.urlencode({{'username': 'admin', 'password': {web_password!r}, 'next': '/'}}).encode()
opener.open(urllib.request.Request(base + '/login', data=login), timeout=10).read()
response = opener.open(base + '/api/config', timeout=10)
etag = response.headers.get('ETag', '')
wrapper = json.loads(response.read())
config = json.loads(wrapper['content'])
changed = 0
for device in config.get('devices', []):
    if device.get('deviceKey') == 'opcda-matrikon-test':
        device['readMode'] = {read_mode!r}
        changed += 1
for channel in config.get('channels', []):
    for device in channel.get('devices', []):
        if device.get('deviceKey') == 'opcda-matrikon-test':
            device['readMode'] = {read_mode!r}
            changed += 1
if not changed:
    raise RuntimeError('OPC DA test device missing from configuration')
request = urllib.request.Request(
    base + '/api/config',
    data=json.dumps(config, ensure_ascii=False).encode(),
    headers={{'Content-Type': 'application/vnd.weikong.config+json', 'If-Match': etag}},
    method='PUT',
)
try:
    response = opener.open(request, timeout=60)
except urllib.error.HTTPError as exc:
    detail = exc.read().decode('utf-8', 'replace')[:500]
    raise RuntimeError('mode save failed HTTP ' + str(exc.code) + ': ' + detail)
result = json.loads(response.read())
print('MODE_SAVE_HTTP=' + str(response.status))
print('MODE_TARGET=' + {read_mode!r})
print('MODE_RESTART_REQUIRED=' + str(bool(result.get('restartRequired'))).lower())
print('MODE_SESSION_RELOADED=' + str(bool(result.get('opcdaSessionReloaded'))).lower())
print('MODE_SESSION_DEVICE_COUNT=' + str(len(result.get('opcdaSessionDevices', []))))
for attempt in range(1, 21):
    response = opener.open(base + '/api/point-status', timeout=10)
    points = json.loads(response.read())
    if isinstance(points, dict):
        points = points.get('points', points.get('items', []))
    point = points[0] if points else {{}}
    if point.get('collectionMode') == {read_mode!r} and str(point.get('quality', '')).upper() == 'GOOD':
        print('MODE_READY_AFTER_SECONDS=' + str(attempt))
        print('MODE_COLLECTION_MODE=' + str(point.get('collectionMode')))
        print('MODE_QUALITY=' + str(point.get('quality', '')))
        print('MODE_HAS_SOURCE_TIME=' + str(bool(point.get('sourceTimestamp'))).lower())
        print('MODE_HAS_RECEIVE_TIME=' + str(bool(point.get('receiveTimestamp'))).lower())
        print('MODE_STALE=' + str(bool(point.get('stale'))).lower())
        break
    time.sleep(1)
else:
    raise RuntimeError({read_mode!r} + ' mode did not become GOOD within 20 seconds')
"""
    stdin, stdout, stderr = client.exec_command("python3 -", timeout=90)
    stdin.write(script)
    stdin.channel.shutdown_write()
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status_code = stdout.channel.recv_exit_status()
    if status_code != 0:
        raise RuntimeError(error.strip() or output.strip())
    after_pid = run(client, "systemctl show weikong-gateway.service -p MainPID --value").strip()
    print(output)
    print("MODE_PID_UNCHANGED=" + str(before_pid == after_pid).lower())


def sample_test(client: paramiko.SSHClient, web_password: str) -> None:
    script = f"""
import http.cookiejar
import json
import time
import urllib.parse
import urllib.request

base = 'http://127.0.0.1:8088'
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
login = urllib.parse.urlencode({{'username': 'admin', 'password': {web_password!r}, 'next': '/'}}).encode()
opener.open(urllib.request.Request(base + '/login', data=login), timeout=10).read()
samples = []
for index in range(3):
    response = opener.open(base + '/api/point-status', timeout=10)
    points = json.loads(response.read())
    if isinstance(points, dict):
        points = points.get('points', points.get('items', []))
    samples.append(points[0] if points else {{}})
    if index < 2:
        time.sleep(2)
print('SAMPLE_COUNT=' + str(len(samples)))
print('SAMPLE_MODES=' + ','.join(str(item.get('collectionMode', '')) for item in samples))
print('SAMPLE_QUALITIES=' + ','.join(str(item.get('quality', '')) for item in samples))
print('SOURCE_TIME_DISTINCT=' + str(len({{item.get('sourceTimestamp') for item in samples}})))
print('RECEIVE_TIME_DISTINCT=' + str(len({{item.get('receiveTimestamp') for item in samples}})))
print('ERROR_COUNT=' + str(sum(1 for item in samples if item.get('error'))))
"""
    stdin, stdout, stderr = client.exec_command("python3 -", timeout=30)
    stdin.write(script)
    stdin.channel.shutdown_write()
    output = stdout.read().decode("utf-8", "replace")
    error = stderr.read().decode("utf-8", "replace")
    status_code = stdout.channel.recv_exit_status()
    if status_code != 0:
        raise RuntimeError(error.strip() or output.strip())
    print(output)


def config_document(opcda_password: str) -> dict:
    enabled = True
    document = {
        "schemaVersion": 2,
        "gatewayKey": "UBUNTU219-OPCDA-TEST",
        "collectIntervalMilliseconds": 1000,
        "cacheFile": f"{REMOTE_ROOT}/data/gateway-spool.jsonl",
        "offlineCache": {
            "enabled": False,
            "maxSizeMB": 16,
            "storagePath": f"{REMOTE_ROOT}/data",
        },
        "historyStorage": {
            "enabled": False,
            "maxSizeMB": 16,
            "storagePath": f"{REMOTE_ROOT}/data",
        },
        "activation": {"enabled": False},
        "mqtt": {
            "enabled": False,
            "broker": "",
            "clientId": "",
            "username": "",
            "password": "",
        },
        "channels": [
            {
                "channelKey": "opcda-test-channel",
                "name": "OPC DA 测试采集通道",
                "type": "network",
                "role": "collect",
                "protocol": "opcda",
                "collectIntervalMilliseconds": 1000,
                "enabled": True,
                "devices": [
                    {
                        "deviceKey": "opcda-matrikon-test",
                        "channelKey": "opcda-test-channel",
                        "name": "Matrikon OPC Simulation",
                        "enabled": enabled,
                        "protocol": "opcda",
                        "address": "192.168.1.163",
                        "clsid": "F8582CF2-88FB-11D0-B850-00C0F0104305",
                        "domain": "CDJ-PC",
                        "workstation": "UBUNTU219",
                        "securityMode": "packet_integrity",
                        "authentication": "ntlm",
                        "connectTimeoutMilliseconds": 10000,
                        "requestTimeoutMilliseconds": 10000,
                        "readMode": "sync",
                        "readSource": "device",
                        "batchSize": 100,
                        "autoReconnect": True,
                        "groupName": "WK_OPCDA_UBUNTU219_TEST",
                        "requestedUpdateRateMilliseconds": 1000,
                        "localeId": 1033,
                        "username": "opcuser",
                        "password": opcda_password,
                        "reconnectIntervalSeconds": 5,
                        "points": [
                            {
                                "deviceKey": "opcda-matrikon-test",
                                "channelKey": "opcda-test-channel",
                                "name": "Random Real8",
                                "metric": "opcda_random_real8",
                                "protocol": "opcda",
                                "address": "192.168.1.163",
                                "itemId": "Random.Real8",
                                "access": "read",
                                "requestedDataType": "auto",
                                "active": True,
                                "enabled": True,
                                "dataType": "auto",
                                "scale": 1,
                                "offset": 0,
                                "decimals": 3,
                            }
                        ],
                    }
                ],
            }
        ],
        "devices": [],
        "forwardDevices": [],
        "edgeComputing": {"enabled": False, "groups": []},
        "forwardSlave": {"enabled": False},
        "web": {"enabled": True, "listen": "0.0.0.0:8088"},
        "watchdog": {"enabled": False, "hardwareEnabled": False},
    }
    # Persist devices in the canonical top-level collection and use ChannelKey
    # for ownership. Keeping the same device both here and nested in Channels
    # makes a subsequent /api/config save look like a duplicate point set.
    device = document["channels"][0]["devices"].pop()
    document["devices"] = [device]
    return document


def upload(sftp: paramiko.SFTPClient, source: Path, destination: str) -> None:
    sftp.put(str(source), destination)


def deploy(client: paramiko.SSHClient, args: argparse.Namespace, password: str) -> None:
    required = [args.gateway, args.library, args.service, args.logrotate]
    for path in required:
        if not path.is_file():
            raise FileNotFoundError(path)

    sftp = client.open_sftp()
    try:
        upload(sftp, args.gateway, "/tmp/weikong-gateway")
        upload(sftp, args.library, "/tmp/libwk_opcda.so.0.1.0")
        upload(sftp, args.service, "/tmp/weikong-gateway.service")
        upload(sftp, args.logrotate, "/tmp/weikong-gateway.logrotate")
        payload = json.dumps(
            config_document(args.opcda_password), ensure_ascii=False, indent=2
        ).encode("utf-8")
        sftp.putfo(io.BytesIO(payload), "/tmp/weikong-config.local.json")
    finally:
        sftp.close()

    install = f"""
set -eu
install -d -m 0755 {REMOTE_ROOT}/bin {REMOTE_ROOT}/lib {REMOTE_ROOT}/config {REMOTE_ROOT}/data
install -m 0755 /tmp/weikong-gateway {REMOTE_ROOT}/bin/weikong-gateway
install -m 0755 /tmp/libwk_opcda.so.0.1.0 {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
ln -sfn libwk_opcda.so.0.1.0 {REMOTE_ROOT}/lib/libwk_opcda.so.0
ln -sfn libwk_opcda.so.0 {REMOTE_ROOT}/lib/libwk_opcda.so
install -m 0600 /tmp/weikong-config.local.json {REMOTE_ROOT}/config/config.local.json
install -m 0644 /tmp/weikong-gateway.service /etc/systemd/system/weikong-gateway.service
install -m 0644 /tmp/weikong-gateway.logrotate /etc/logrotate.d/weikong-gateway
rm -f /tmp/weikong-gateway /tmp/libwk_opcda.so.0.1.0 /tmp/weikong-config.local.json /tmp/weikong-gateway.service /tmp/weikong-gateway.logrotate
systemctl daemon-reload
systemctl enable weikong-gateway.service
systemctl restart weikong-gateway.service
"""
    run(client, install, password=password, sudo=True)
    print(run(client, "systemctl --no-pager --full status weikong-gateway.service | head -30", password=password, sudo=True))


def update_library(client: paramiko.SSHClient, library: Path, password: str) -> None:
    if not library.is_file():
        raise FileNotFoundError(library)
    sftp = client.open_sftp()
    try:
        upload(sftp, library, "/tmp/libwk_opcda.so.0.1.0")
    finally:
        sftp.close()
    command = f"""
set -eu
install -m 0755 /tmp/libwk_opcda.so.0.1.0 {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
rm -f /tmp/libwk_opcda.so.0.1.0
systemctl restart weikong-gateway.service
"""
    run(client, command, password=password, sudo=True)
    print(run(client, "systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID"))


def update_gateway(client: paramiko.SSHClient, gateway: Path, password: str) -> None:
    if not gateway.is_file():
        raise FileNotFoundError(gateway)
    sftp = client.open_sftp()
    try:
        upload(sftp, gateway, "/tmp/weikong-gateway")
    finally:
        sftp.close()
    command = f"""
set -eu
ldd /tmp/weikong-gateway >/tmp/weikong-gateway.ldd
if grep -q 'not found' /tmp/weikong-gateway.ldd; then
    cat /tmp/weikong-gateway.ldd >&2
    exit 1
fi
backup_dir={REMOTE_ROOT}/backups
mkdir -p "$backup_dir"
stamp=$(date +%Y%m%d-%H%M%S)
backup="$backup_dir/weikong-gateway-$stamp"
cp -a {REMOTE_ROOT}/bin/weikong-gateway "$backup"
install -m 0755 /tmp/weikong-gateway {REMOTE_ROOT}/bin/weikong-gateway.new
mv -f {REMOTE_ROOT}/bin/weikong-gateway.new {REMOTE_ROOT}/bin/weikong-gateway
rm -f /tmp/weikong-gateway
systemctl restart weikong-gateway.service
ready=0
for attempt in $(seq 1 30); do
    if systemctl is-active --quiet weikong-gateway.service && curl -fsS http://127.0.0.1:8088/api/readyz >/dev/null 2>&1; then
        ready=1
        break
    fi
    sleep 1
done
if [ "$ready" -ne 1 ]; then
    cp -a "$backup" {REMOTE_ROOT}/bin/weikong-gateway
    systemctl restart weikong-gateway.service
    echo "new gateway failed readiness; previous binary restored" >&2
    exit 1
fi
find "$backup_dir" -maxdepth 1 -type f -name 'weikong-gateway-*' -printf '%T@ %p\n' | sort -nr | awk 'NR>5 {{sub(/^[^ ]+ /, ""); print}}' | xargs -r rm -f
"""
    run(client, command, password=password, sudo=True)
    print(run(client, "systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID"))


def rollback_latest_gateway(client: paramiko.SSHClient, password: str) -> None:
    command = f"""
set -eu
backup=$(find {REMOTE_ROOT}/backups -maxdepth 1 -type f -name 'weikong-gateway-*' -printf '%T@ %p\n' | sort -nr | head -1 | cut -d' ' -f2-)
test -n "$backup"
install -m 0755 "$backup" {REMOTE_ROOT}/bin/weikong-gateway
systemctl restart weikong-gateway.service
for attempt in $(seq 1 15); do
    if systemctl is-active --quiet weikong-gateway.service && curl -fsS http://127.0.0.1:8088/api/healthz >/dev/null 2>&1; then
        echo "RESTORED=$backup"
        systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID
        exit 0
    fi
    sleep 1
done
systemctl --no-pager --full status weikong-gateway.service
exit 1
"""
    print(run(client, command, password=password, sudo=True))


def incident_status(client: paramiko.SSHClient, password: str) -> None:
    command = f"""
set -eu
python3 - <<'PY'
import json
import os
import sqlite3

path = {str(REMOTE_ROOT + '/config/.runtime/collection-incidents.db')!r}
print('INCIDENT_DB=' + path)
print('INCIDENT_DB_EXISTS=' + str(os.path.isfile(path)).lower())
if not os.path.isfile(path):
    raise SystemExit(0)
connection = sqlite3.connect(path)
try:
    counts = dict(connection.execute('SELECT state, COUNT(*) FROM collection_incidents GROUP BY state').fetchall())
    rows = connection.execute('SELECT device_key,point_name,metric,protocol,address,state,explanation,reason,occurred_at,recovered_at FROM collection_incidents ORDER BY id DESC LIMIT 10').fetchall()
    print('INCIDENT_COUNTS=' + json.dumps(counts, ensure_ascii=False, sort_keys=True))
    for row in rows:
        print('INCIDENT=' + json.dumps({{
            'deviceKey': row[0], 'pointName': row[1], 'metric': row[2],
            'protocol': row[3], 'address': row[4], 'state': row[5],
            'explanation': row[6], 'reason': row[7], 'occurredAt': row[8],
            'recoveredAt': row[9],
        }}, ensure_ascii=False))
finally:
    connection.close()
PY
"""
    print(run(client, command, password=password, sudo=True))


def incident_clear(client: paramiko.SSHClient, password: str) -> None:
    command = f"""
set -eu
python3 - <<'PY'
import os
import sqlite3

path = {str(REMOTE_ROOT + '/config/.runtime/collection-incidents.db')!r}
if not os.path.isfile(path):
    print('INCIDENTS_CLEARED=0')
    raise SystemExit(0)
connection = sqlite3.connect(path, timeout=10)
try:
    connection.execute('PRAGMA busy_timeout=10000')
    count = connection.execute('SELECT COUNT(*) FROM collection_incidents').fetchone()[0]
    connection.execute('DELETE FROM collection_incidents')
    connection.commit()
    print('INCIDENTS_CLEARED=' + str(count))
finally:
    connection.close()
PY
"""
    print(run(client, command, password=password, sudo=True))


def update_runtime(client: paramiko.SSHClient, gateway: Path, library: Path, password: str) -> None:
    if not gateway.is_file():
        raise FileNotFoundError(gateway)
    if not library.is_file():
        raise FileNotFoundError(library)
    sftp = client.open_sftp()
    try:
        upload(sftp, gateway, "/tmp/weikong-gateway")
        upload(sftp, library, "/tmp/libwk_opcda.so.0.1.0")
    finally:
        sftp.close()
    command = f"""
set -eu
install -m 0755 /tmp/libwk_opcda.so.0.1.0 {REMOTE_ROOT}/lib/libwk_opcda.so.0.1.0
ln -sfn libwk_opcda.so.0.1.0 {REMOTE_ROOT}/lib/libwk_opcda.so.0
install -m 0755 /tmp/weikong-gateway {REMOTE_ROOT}/bin/weikong-gateway
rm -f /tmp/weikong-gateway /tmp/libwk_opcda.so.0.1.0
systemctl restart weikong-gateway.service
"""
    run(client, command, password=password, sudo=True)
    print(run(client, "systemctl show weikong-gateway.service -p ActiveState -p SubState -p MainPID -p NRestarts"))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("inspect", "status", "diagnose", "incident-status", "incident-clear", "artifact-test", "http-test", "mode-test", "sample-test", "restart-test", "gateway-update", "gateway-rollback", "library-update", "runtime-update", "deploy"))
    parser.add_argument("--host", default="192.168.1.219")
    parser.add_argument("--user", default="ubuntu")
    parser.add_argument("--gateway", type=Path)
    parser.add_argument("--library", type=Path)
    parser.add_argument("--service", type=Path)
    parser.add_argument("--logrotate", type=Path)
    parser.add_argument("--read-mode", choices=("sync", "subscription"), default="subscription")
    args = parser.parse_args()
    if args.command == "deploy":
        missing = [name for name in ("gateway", "library", "service", "logrotate") if getattr(args, name) is None]
        if missing:
            parser.error("deploy requires " + ", ".join("--" + name for name in missing))
    elif args.command == "library-update" and args.library is None:
        parser.error("library-update requires --library")
    elif args.command == "gateway-update" and args.gateway is None:
        parser.error("gateway-update requires --gateway")
    elif args.command == "runtime-update" and (args.gateway is None or args.library is None):
        parser.error("runtime-update requires --gateway and --library")
    args.opcda_password = os.environ.get("WK_OPCDA_PASSWORD", "")
    args.web_password = os.environ.get("WK_WEB_PASSWORD", "")
    return args


def main() -> int:
    args = parse_args()
    password = os.environ.get("WK_REMOTE_PASSWORD", "")
    if not password:
        print("WK_REMOTE_PASSWORD is required", file=sys.stderr)
        return 2
    if args.command == "deploy" and not args.opcda_password:
        print("WK_OPCDA_PASSWORD is required for deploy", file=sys.stderr)
        return 2
    client = connect(args.host, args.user, password)
    try:
        if args.command == "inspect":
            inspect(client)
        elif args.command == "status":
            status(client, password)
        elif args.command == "diagnose":
            if not args.web_password:
                raise RuntimeError("WK_WEB_PASSWORD is required for diagnose")
            diagnose(client, args.web_password)
        elif args.command == "artifact-test":
            artifact_test(client, password)
        elif args.command == "http-test":
            if not args.web_password:
                raise RuntimeError("WK_WEB_PASSWORD is required for http-test")
            http_test(client, args.web_password)
        elif args.command == "mode-test":
            if not args.web_password:
                raise RuntimeError("WK_WEB_PASSWORD is required for mode-test")
            mode_test(client, args.web_password, args.read_mode)
        elif args.command == "sample-test":
            if not args.web_password:
                raise RuntimeError("WK_WEB_PASSWORD is required for sample-test")
            sample_test(client, args.web_password)
        elif args.command == "restart-test":
            restart_test(client, password)
        elif args.command == "library-update":
            update_library(client, args.library, password)
        elif args.command == "gateway-update":
            update_gateway(client, args.gateway, password)
        elif args.command == "gateway-rollback":
            rollback_latest_gateway(client, password)
        elif args.command == "incident-status":
            incident_status(client, password)
        elif args.command == "incident-clear":
            incident_clear(client, password)
        elif args.command == "runtime-update":
            update_runtime(client, args.gateway, args.library, password)
        else:
            deploy(client, args, password)
    finally:
        client.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
