"""Deploy the dashboard/topology build, preserving configuration and all queues."""
import gzip
import hashlib
import json
from pathlib import Path
from mysql_live_20260912 import Gateway

BASE = Path('build/topology-deployment-20260912-1642')
BASE.mkdir(exist_ok=True)
OLD = '057b748881ac1115536eec35c393c6a4deb1b6345659b5829103ced1e6f367f6'
NEW = 'b8aff00ec48700b4387f2a18132f01baaecce15b7ebe04be90cc05b5382cca9d'
backup = '/mnt/udisk/.weikong-deploy/topology-backup-20260912-1642'
archive = '/mnt/udisk/.weikong-deploy/topology-20260912-1642.gz'

def save(name, value):
    (BASE / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')
    print(name, json.dumps(value, ensure_ascii=False), flush=True)

payload = Path('build/iec61850-mms-armv7/weikong-gateway-topology-20260912').read_bytes()
assert hashlib.sha256(payload).hexdigest() == NEW
g = Gateway()
try:
    g.login()
    save('health-before', g.api('/api/healthz'))
    save('storage-before', g.api('/api/remote-storage/status'))
    assert g.shell('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0] == OLD
    g.shell("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'")
    with g.ssh.open_sftp() as s:
        free = int(g.shell('df -Pk /userdata').splitlines()[-1].split()[3]) * 1024
        assert free + s.stat('/userdata/weikong/bin/weikong-gateway').st_size - len(payload) > 2*1024*1024
        with s.open('/userdata/weikong/config/config.local.json') as f:
            config_hash = hashlib.sha256(f.read()).hexdigest()
        g.shell(f'test ! -e {backup} && mkdir -m 700 {backup} && gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && chmod 600 {backup}/* && sync')
        assert g.shell(f'sha256sum {backup}/config.original.json').split()[0] == config_hash
        local = BASE / 'gateway-original.gz'
        s.get(backup + '/weikong-gateway.original.gz', str(local), max_concurrent_prefetch_requests=8)
        assert hashlib.sha256(gzip.decompress(local.read_bytes())).hexdigest() == OLD
        print('Verified USB and local rollback binary backups.', flush=True)
        with s.open(archive, 'wb') as target:
            compressed = gzip.compress(payload, mtime=0)
            for offset in range(0, len(compressed), 32768):
                target.write(compressed[offset:offset+32768])
        script = Path('deploy/eg100/deploy_mysql_fix_20260912.sh').read_text(encoding='utf-8')
        script = script.replace('/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912', backup)
        script = script.replace('/mnt/udisk/.weikong-deploy/mysql-fix-20260912.gz', archive)
        script = script.replace('weikong-gateway.mysql-new', 'weikong-gateway.topology-new')
        start = script.index('cp -p /userdata/weikong/config/.runtime/alarm-events.db')
        end = script.index('\nsync\necho BACKUPS_VERIFIED', start)
        # Copy stopped databases plus any remaining WAL under matching names.
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
        with s.open(backup + '/deploy.sh', 'wb') as target:
            target.write(script.encode())
    save('manifest', {'new': NEW, 'old': OLD, 'config': config_hash, 'backup': backup, 'size': len(payload)})
    print('Uploaded verified release; starting stopped-database backup and service replacement.', flush=True)
    save('deployment', g.shell(f'sh {backup}/deploy.sh {NEW} {OLD} {config_hash}', timeout=600))
finally:
    g.ssh.close()
