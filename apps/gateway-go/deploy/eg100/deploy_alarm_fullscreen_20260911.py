"""Fixed EG100 deployment with off-device and USB rollback; no configuration edits."""
import gzip
import hashlib
import os
import json
import sqlite3
from pathlib import Path
import paramiko

binary = Path('build/iec61850-mms-armv7/weikong-gateway-alarm-fullscreen')
expected = '39cfb7c628ad9f6a362128df7fc587a0bbe821525f26ff76ce61b8cefd26e3bf'
old_expected = 'e87923c07ef0c9fff47b63b65a6e46067a8b6a8375cae5a1b8d8229dd7df794e'
payload = binary.read_bytes()
assert hashlib.sha256(payload).hexdigest() == expected
archive = binary.with_suffix('.gz')
archive.write_bytes(gzip.compress(payload, mtime=0))
local = Path('build/alarm-fullscreen-backup-20260911')
local.mkdir(exist_ok=True)
backup = '/mnt/udisk/.weikong-deploy/alarm-fullscreen-backup-20260911'
c = paramiko.SSHClient()
c.load_system_host_keys()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.1.214', username='root', password=os.environ['WK_EG100_PASSWORD'], timeout=10)

def run(command):
    _, out, err = c.exec_command(command, timeout=240)
    result, error = out.read().decode(), err.read().decode()
    if out.channel.recv_exit_status() != 0:
        raise RuntimeError(result + '\n' + error)
    return result

try:
    assert run('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0] == old_expected
    run("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'")
    s = c.open_sftp()
    s.get_channel().settimeout(30)
    with s.open('/userdata/weikong/config/config.local.json') as f:
        config = f.read()
    config_hash = hashlib.sha256(config).hexdigest()
    alarm_path = json.loads(config).get('alarms', {}).get('storagePath') or '.runtime/alarm-events.db'
    if not alarm_path.startswith('/'):
        alarm_path = '/userdata/weikong/config/' + alarm_path
    assert alarm_path == '/userdata/weikong/config/.runtime/alarm-events.db', 'Unexpected alarm DB location'
    (local / 'config.original.json').write_bytes(config)
    run(f'test ! -e {backup} && mkdir -m 700 {backup} && gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && chmod 600 {backup}/* && sync')
    s.get(backup + '/weikong-gateway.original.gz', str(local / 'weikong-gateway.original.gz'), max_concurrent_prefetch_requests=8)
    assert hashlib.sha256(gzip.decompress((local / 'weikong-gateway.original.gz').read_bytes())).hexdigest() == old_expected
    run(f"""sqlite3 {alarm_path} ".backup '{backup}/alarm-events.original.db'" && chmod 600 {backup}/alarm-events.original.db""")
    assert run(f'sqlite3 {backup}/alarm-events.original.db "PRAGMA integrity_check;"').strip() == 'ok'
    s.get(backup + '/alarm-events.original.db', str(local / 'alarm-events.original.db'), max_concurrent_prefetch_requests=8)
    db = sqlite3.connect(str(local / 'alarm-events.original.db'))
    assert db.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
    print('Alarm history backup rows:', db.execute('SELECT count(*) FROM alarm_history').fetchone()[0], flush=True)
    db.close()
    print('USB and off-device binary/config/alarm DB rollback verified; current config preserved.', flush=True)
    def upload(source, remote):
        with open(source, 'rb') as f, s.open(remote, 'wb') as target:
            target.set_pipelined(False)
            count = 0
            while chunk := f.read(32768):
                target.write(chunk)
                count += len(chunk)
                if count % (1024 * 1024) == 0:
                    print('Uploaded:', count, flush=True)
        assert s.stat(remote).st_size == os.path.getsize(source)
    upload(archive, '/mnt/udisk/.weikong-deploy/alarm-fullscreen-20260911.gz')
    upload('deploy/eg100/deploy_alarm_fullscreen_20260911.sh', '/tmp/weikong-deploy-alarm-fullscreen.sh')
    s.close()
    print('Applying checked package.', flush=True)
    print(run(f'sh /tmp/weikong-deploy-alarm-fullscreen.sh {expected} {old_expected} {config_hash}'), flush=True)
finally:
    c.close()
