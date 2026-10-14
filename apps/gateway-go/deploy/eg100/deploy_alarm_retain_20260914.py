"""Verified EG100 release with rollback, unchanged config and stopped DB backups."""
import gzip
import hashlib
import http.client
import json
import re
import sys
import time
from pathlib import Path
from mysql_live_20260912 import Gateway

BASE = Path('build/alarm-retain-release-20260914')
OLD = 'b8aff00ec48700b4387f2a18132f01baaecce15b7ebe04be90cc05b5382cca9d'
NEW = 'a4cc3ddc3614115247a122d8358a3c4dcc13352002116965a3ec442031382d5b'
BACKUP = '/mnt/udisk/.weikong-deploy/alarm-retain-backup-20260914'
ARCHIVE = '/mnt/udisk/.weikong-deploy/alarm-retain-20260914.gz'

def save(name, value):
    (BASE / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')
    print(name, json.dumps(value, ensure_ascii=False), flush=True)

def raw(g, path):
    c = http.client.HTTPConnection('127.0.0.1', 8088, timeout=25)
    c.sock = g.ssh.get_transport().open_channel('direct-tcpip', ('127.0.0.1', 8088), ('127.0.0.1', 0), timeout=25)
    c.sock.settimeout(25)
    try:
        c.request('GET', path, headers={'Cookie': '; '.join(k+'='+v.value for k,v in g.cookies.items())})
        r = c.getresponse()
        assert r.status == 200, (path, r.status)
        return r.read()
    finally:
        c.close()

def deploy(g):
    payload = (BASE / 'weikong-gateway-alarm-retain-20260914').read_bytes()
    assert hashlib.sha256(payload).hexdigest() == NEW
    save('health-before', g.api('/api/healthz'))
    save('storage-before', g.api('/api/remote-storage/status'))
    assert g.shell('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0] == OLD
    g.shell("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'")
    with g.ssh.open_sftp() as s:
        flash_free = int(g.shell('df -Pk /userdata').splitlines()[-1].split()[3])*1024
        usb_free = int(g.shell('df -Pk /mnt/udisk').splitlines()[-1].split()[3])*1024
        assert flash_free + s.stat('/userdata/weikong/bin/weikong-gateway').st_size - len(payload) > 4*1024*1024
        assert usb_free > 1024*1024*1024
        with s.open('/userdata/weikong/config/config.local.json') as f:
            config_hash = hashlib.sha256(f.read()).hexdigest()
        g.shell(f'test ! -e {BACKUP} && mkdir -m 700 {BACKUP} && gzip -c /userdata/weikong/bin/weikong-gateway > {BACKUP}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {BACKUP}/config.original.json && chmod 600 {BACKUP}/* && sync')
        assert g.shell(f'sha256sum {BACKUP}/config.original.json').split()[0] == config_hash
        local = BASE / 'gateway-original.gz'
        s.get(BACKUP+'/weikong-gateway.original.gz', str(local), max_concurrent_prefetch_requests=8)
        assert hashlib.sha256(gzip.decompress(local.read_bytes())).hexdigest() == OLD
        s.get(BACKUP+'/config.original.json', str(BASE/'config.original.json'), max_concurrent_prefetch_requests=8)
        assert hashlib.sha256((BASE/'config.original.json').read_bytes()).hexdigest() == config_hash
        print('Verified USB and local rollback backups.', flush=True)
        compressed = gzip.compress(payload, mtime=0)
        with s.open(ARCHIVE, 'wb') as target:
            for offset in range(0, len(compressed), 32768):
                target.write(compressed[offset:offset+32768])
        script = Path('deploy/eg100/deploy_mysql_fix_20260912.sh').read_text(encoding='utf-8')
        script = script.replace('/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912', BACKUP).replace('/mnt/udisk/.weikong-deploy/mysql-fix-20260912.gz', ARCHIVE).replace('weikong-gateway.mysql-new', 'weikong-gateway.alarm-retain-new')
        start = script.index('cp -p /userdata/weikong/config/.runtime/alarm-events.db')
        end = script.index('\nsync\necho BACKUPS_VERIFIED', start)
        script = script[:start] + '''
for source in /userdata/weikong/config/.runtime/alarm-events.db /mnt/udisk/.weikong/history/telemetry-history.db /mnt/udisk/.remote-storage/eg100-pg-integration.db /mnt/udisk/.remote-storage/eg100-mysql-integration.db /mnt/udisk/.remote-storage/eg100-sqlserver-integration.db; do
  name=$(basename "$source")
  cp -p "$source" "$backup/$name"
  if [ -f "$source-wal" ]; then cp -p "$source-wal" "$backup/$name-wal"; fi
  chmod 600 "$backup/$name"*
  test "$(sqlite3 "$backup/$name" 'PRAGMA quick_check;')" = ok
  echo "BACKUP_VERIFIED $name"
done
''' + script[end:]
        script = script.replace('"$attempt" -lt 60', '"$attempt" -lt 120')
        with s.open(BACKUP+'/deploy.sh', 'wb') as target:
            target.write(script.encode())
    save('manifest', {'new':NEW, 'old':OLD, 'config':config_hash, 'backup':BACKUP, 'size':len(payload)})
    print('Uploaded release. Backing up stopped databases and replacing service.', flush=True)
    # Keep a device-side log as well in case this SSH session is interrupted.
    save('deployment', g.shell(f'sh {BACKUP}/deploy.sh {NEW} {OLD} {config_hash} > {BACKUP}/deploy.log 2>&1; result=$?; cat {BACKUP}/deploy.log; exit $result', timeout=600))

def verify(g):
    manifest = json.loads((BASE/'manifest.json').read_text(encoding='utf-8'))
    hashes = g.shell('sha256sum /userdata/weikong/bin/weikong-gateway /userdata/weikong/config/config.local.json').splitlines()
    assert hashes[0].split()[0] == NEW and hashes[1].split()[0] == manifest['config']
    first = g.api('/api/healthz')
    assert first['healthy'] and g.api('/api/readyz')['ready']
    before = json.loads((BASE/'health-before.json').read_text(encoding='utf-8'))
    assert first['pointCount'] == before['pointCount']
    index = raw(g, '/').decode()
    assets = re.findall(r'(?:src|href)="(/assets/[^\"]+)"', index)
    assert len(assets) >= 2
    for path in assets:
        local = Path('internal/web/frontend/vue-dist') / path.lstrip('/')
        remote = raw(g, path)
        assert hashlib.sha256(local.read_bytes()).digest() == hashlib.sha256(remote).digest(), path
        if path.endswith('.js'):
            assert b'retainAfterAcknowledged' in remote
    events = g.api('/api/alarms?activeOnly=1')['active']
    assert all('retainAfterAcknowledged' in e for e in events)
    first_storage = g.api('/api/remote-storage/status')['statuses']
    time.sleep(12)
    last = g.api('/api/healthz')
    assert last['healthy'] and last['lastCollectAt'] != first['lastCollectAt']
    statuses = g.api('/api/remote-storage/status')['statuses']
    original = json.loads((BASE/'storage-before.json').read_text(encoding='utf-8'))['statuses']
    assert {s['id'] for s in statuses} == {s['id'] for s in original}
    assert all(s['cacheReady'] for s in statuses)
    for old in original:
        if old['connected']:
            new = next(s for s in statuses if s['id']==old['id'])
            initial = next(s for s in first_storage if s['id']==old['id'])
            assert new['connected'] and new['sent'] > initial['sent']
    save('verification', {'verified':True, 'configUnchanged':True, 'health':last, 'assets':assets, 'alarmCount':len(events), 'policyExposed':True, 'storage':statuses, 'platform':g.shell('df -Pk /userdata /mnt/udisk; /etc/init.d/S99weikong-gateway status')})

if __name__ == '__main__':
    g = Gateway()
    try:
        g.login()
        if sys.argv[1] == 'deploy': deploy(g)
        elif sys.argv[1] == 'verify': verify(g)
        else: raise ValueError('choose deploy or verify')
    finally:
        g.ssh.close()
