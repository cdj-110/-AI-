"""Deploy strict licensing without activating; preserve config and rollback backups."""
import copy
import gzip
import hashlib
import json
import os
import posixpath
import re
import shlex
import time
from pathlib import Path
from mysql_live_20260912 import Gateway

ROOT = Path(__file__).resolve().parents[4]
PRIVATE = ROOT / 'secrets/authorization-20260916'
DAEMON = '/userdata/weikong/bin/weikong-gateway'
CONFIG = '/userdata/weikong/config/config.local.json'
PENDING = '/userdata/weikong/config/activation.pending-test.json'

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def read(sftp, path):
    with sftp.open(path, 'rb') as stream:
        return stream.read()

def write(sftp, path, raw):
    with sftp.open(path, 'wb') as stream:
        for offset in range(0, len(raw), 32768):
            stream.write(raw[offset:offset+32768])
    sftp.chmod(path, 0o600)

def main():
    payload = (ROOT / 'apps/gateway-go/build/remote-access-auto-release-20260916/weikong-gateway').read_bytes()
    release = json.loads((PRIVATE / 'gateway-build-manifest.json').read_text())
    assert digest(payload) == release['binarySha256'], 'Release manifest mismatch'
    stamp = time.strftime('%Y%m%d-%H%M%S')
    backup = f'/mnt/udisk/.weikong-deploy/license-unactivated-backup-{stamp}'
    local = PRIVATE / f'eg100-backup-{stamp}'
    local.mkdir(mode=0o700)
    gateway = Gateway()
    try:
        with gateway.ssh.open_sftp() as sftp:
            original = read(sftp, CONFIG)
            current = json.loads(original)
            old_hash = gateway.shell(f'sha256sum {DAEMON}').split()[0]
            old_size = sftp.stat(DAEMON).st_size
            flash_free = int(gateway.shell('df -Pk /userdata').splitlines()[-1].split()[3]) * 1024
            usb_free = int(gateway.shell('df -Pk /mnt/udisk').splitlines()[-1].split()[3]) * 1024
            assert flash_free + old_size - len(payload) >= 4*1024*1024, 'Insufficient flash headroom'
            assert usb_free >= 512*1024*1024, 'Insufficient USB backup space'
            candidate = copy.deepcopy(current)
            candidate.setdefault('activation', {})['file'] = PENDING
            changed = [k for k in set(current)|set(candidate) if current.get(k) != candidate.get(k)]
            assert changed == ['activation'], 'Unexpected config change'
            pending = json.dumps(candidate, ensure_ascii=False, indent=2).encode()
            gateway.shell(f"grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && test ! -e {backup} && test ! -e {PENDING} && mkdir -m 700 {backup} && gzip -c {DAEMON} > {backup}/weikong-gateway.original.gz && cp -p {CONFIG} {backup}/config.original.json && cp -p /etc/init.d/S99weikong-gateway {backup}/init.original.sh && chmod 600 {backup}/* && sync")
            archived = read(sftp, f'{backup}/weikong-gateway.original.gz')
            assert digest(gzip.decompress(archived)) == old_hash, 'Old binary backup invalid'
            (local / 'weikong-gateway.original.gz').write_bytes(archived)
            (local / 'config.original.json').write_bytes(original)
            (local / 'init.original.sh').write_bytes(read(sftp, f'{backup}/init.original.sh'))
            old_license = current.get('activation', {}).get('file', '')
            extra_backups = []
            for name, source in [('activation.original.json', old_license), ('license-state.original.json', '/userdata/weikong/config/.license-state.json')]:
                if not source:
                    continue
                if not source.startswith('/'):
                    source = posixpath.join(posixpath.dirname(CONFIG), source)
                try:
                    raw = read(sftp, source)
                except FileNotFoundError:
                    continue
                write(sftp, f'{backup}/{name}', raw)
                (local / name).write_bytes(raw)
                extra_backups.append({'path':source, 'backupName':name, 'sha256':digest(raw)})
            write(sftp, f'{backup}/config.pending.json', pending)
            write(sftp, f'{backup}/release.gz', gzip.compress(payload, mtime=0))
            write(sftp, f'{backup}/deploy.sh', Path(__file__).with_suffix('.sh').read_bytes().replace(b'\r\n', b'\n'))
            manifest = {'oldBinarySha256':old_hash, 'newBinarySha256':digest(payload), 'oldConfigSha256':digest(original), 'pendingConfigSha256':digest(pending), 'remoteBackup':backup, 'localBackup':str(local), 'extraBackups':extra_backups, 'configChangedKeys':changed}
            (local / 'manifest.json').write_text(json.dumps(manifest, indent=2), encoding='utf8')
        print('EG100 backups verified; starting unactivated deployment.', flush=True)
        args = [backup, f'{backup}/release.gz', digest(payload), old_hash, digest(original), digest(pending)]
        gateway.shell(f"sh {backup}/deploy.sh " + ' '.join(shlex.quote(a) for a in args) + f" > {backup}/deploy.log 2>&1", timeout=600)
        deploy_log = gateway.shell(f'cat {backup}/deploy.log')
        (local / 'deploy.log').write_text(deploy_log, encoding='utf8')
        assert 'DEPLOY_UNACTIVATED_OK' in deploy_log
        health = json.loads(gateway.shell('wget -T 5 -qO- http://127.0.0.1:8088/api/healthz'))
        ready = json.loads(gateway.shell('wget -T 5 -qO- http://127.0.0.1:8088/api/readyz'))
        assert health['healthy'] and ready['ready']
        assert not health.get('lastCollectAt'), 'Collection unexpectedly active without license'
        assert gateway.shell(f'sha256sum {CONFIG}').split()[0] == digest(pending)
        assert gateway.shell(f'sha256sum {DAEMON}').split()[0] == digest(payload)
        pid_before = gateway.shell('pidof weikong-gateway').strip()
        time.sleep(10)
        assert gateway.shell('pidof weikong-gateway').strip() == pid_before, 'Gateway restarted during verification'
        network = gateway.shell("netstat -tn 2>/dev/null | grep ESTABLISHED | grep -E '192.168.1.141:(1883|3100|3306|5432|1433)' || true").strip()
        assert not network, 'Business/cloud connections unexpectedly active'
        with gateway.ssh.open_sftp() as sftp:
            actual = json.loads(read(sftp, CONFIG))
            assert actual == candidate
            index = gateway.shell('wget -T 5 -qO- http://127.0.0.1:8088/login')
            assert '登录' in index
        result = {'deployedUnactivated':True, 'health':health, 'ready':ready, 'businessConnectionsAbsent':True, 'pidStable':True, 'originalAccountsAndConfigPreserved':True, 'remoteBackup':backup, 'localBackup':str(local)}
        password = os.environ.get('WK_EG100_WEB_PASSWORD')
        if password:
            gateway.login()
            status = gateway.api('/api/license/status')
            assert status['license']['restricted'] and not status['license']['valid']
            result['licenseStatus'] = status
        (local / 'verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf8')
        print(json.dumps(result, ensure_ascii=False), flush=True)
    finally:
        gateway.ssh.close()

if __name__ == '__main__':
    main()
