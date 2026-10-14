"""Deploy the licensed ARM release without changing activation or configuration."""
import gzip
import hashlib
import json
import os
import posixpath
import re
import shlex
import stat
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GATEWAY_ROOT = ROOT / 'apps/gateway-go'
sys.path.insert(0, str(GATEWAY_ROOT / 'deploy/eg100'))
from mysql_live_20260912 import Gateway

DAEMON = '/userdata/weikong/bin/weikong-gateway'
CONFIG = '/userdata/weikong/config/config.local.json'
INIT = '/etc/init.d/S99weikong-gateway'

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def read(sftp, path):
    with sftp.open(path, 'rb') as source:
        return source.read()

def file_hash(sftp, path):
    try:
        return digest(read(sftp, path))
    except FileNotFoundError:
        return None

def main():
    payload = (GATEWAY_ROOT / 'build/remote-access-auto-release-20260916/weikong-gateway').read_bytes()
    build = json.loads((ROOT / 'secrets/authorization-20260916/gateway-build-manifest.json').read_text())
    new_hash = digest(payload)
    assert build['binarySha256'] == new_hash
    assert all(build[key] for key in ['strictSignature', 'strictHardware', 'strictSecureChip'])
    assert payload[:4] == b'\x7fELF'
    g = Gateway()
    try:
        health_before = json.loads(g.shell('wget -T 5 -qO- http://127.0.0.1:8088/api/healthz'))
        assert health_before['healthy']
        before_network = digest(g.shell('ip -4 addr show; ip -4 route show').encode())
        old_hash = g.shell(f'sha256sum {DAEMON}').split()[0]
        stamp = time.strftime('%Y%m%d-%H%M%S')
        usb_root = os.environ.get('WK_EG100_DEPLOY_USB_ROOT', '/mnt/udisk')
        assert usb_root in ['/mnt/udisk', '/media/udisk1'], 'Unexpected staging mount'
        backup = f'{usb_root}/.weikong-deploy/activation-entry-{stamp}'
        assert re.fullmatch(re.escape(usb_root) + r'/\.weikong-deploy/activation-entry-\d{8}-\d{6}', backup)
        with g.ssh.open_sftp() as sftp:
            assert sftp.normalize(usb_root) == usb_root
            mount_check = f"awk '$1==\"/dev/sda4\" && $2==\"{usb_root}\" && $3==\"ext4\" && $4 ~ /(^|,)rw(,|$)/ {{ found=1 }} END {{ exit !found }}' /proc/mounts"
            g.shell(mount_check)
            parent = usb_root + '/.weikong-deploy'
            try:
                assert stat.S_ISDIR(sftp.lstat(parent).st_mode) and sftp.normalize(parent) == parent
            except FileNotFoundError:
                sftp.mkdir(parent, 0o700)
            for path in [DAEMON, CONFIG, INIT]:
                assert stat.S_ISREG(sftp.lstat(path).st_mode) and sftp.normalize(path) == path
            config_raw = read(sftp, CONFIG)
            config_hash = digest(config_raw)
            cfg = json.loads(config_raw)
            assert cfg.get('gatewayKey') == 'WK20265AF2F5C6', 'Wrong target gateway'
            license_path = cfg.get('activation', {}).get('file') or '/userdata/weikong/config/activation.local.json'
            if not license_path.startswith('/'):
                license_path = posixpath.join(posixpath.dirname(CONFIG), license_path)
            preserved = [CONFIG, INIT, license_path, '/userdata/weikong/config/.license-state.json', '/userdata/weikong/config/ota-release.json', '/userdata/weikong/lib/libwk_opcda.so.0']
            before = {path: file_hash(sftp, path) for path in preserved}
            old_size = sftp.stat(DAEMON).st_size
            flash_free = int(g.shell('df -Pk /userdata').splitlines()[-1].split()[3]) * 1024
            usb_free = int(g.shell('df -Pk ' + usb_root).splitlines()[-1].split()[3]) * 1024
            assert flash_free + old_size - len(payload) > 4 * 1024 * 1024, 'Insufficient flash headroom'
            assert usb_free > 512 * 1024 * 1024, 'Insufficient USB backup space'
            g.shell(mount_check + ' && '
                    f'test ! -e {backup} && mkdir -m 700 {backup} && '
                    f'gzip -c {DAEMON} > {backup}/weikong-gateway.original.gz && '
                    f'cp -p {CONFIG} {backup}/config.original.json && '
                    f'cp -p {INIT} {backup}/init.original.sh && '
                    f'cp -a /userdata/weikong/config {backup}/config.before && '
                    f'chmod 700 {backup}/config.before && chmod 600 {backup}/*.gz {backup}/*.json {backup}/*.sh && sync')
            original_archive = read(sftp, backup + '/weikong-gateway.original.gz')
            assert digest(gzip.decompress(original_archive)) == old_hash
            out = ROOT / os.environ.get('WK_EG100_DEPLOY_OUTPUT', 'artifacts/eg100-activation-entry-20260917')
            out.mkdir(parents=True, exist_ok=True)
            (out / ('gateway-original-' + old_hash[:12] + '.gz')).write_bytes(original_archive)
            assert file_hash(sftp, backup + '/config.original.json') == config_hash
            for path in [license_path, '/userdata/weikong/config/.license-state.json']:
                if before[path] is not None:
                    destination = backup + '/' + posixpath.basename(path) + '.original'
                    g.shell(f'cp -p {shlex.quote(path)} {shlex.quote(destination)} && chmod 600 {shlex.quote(destination)}')
                    assert file_hash(sftp, destination) == before[path]
            archive = backup + '/release.gz'
            compressed = gzip.compress(payload, mtime=0)
            with sftp.open(archive, 'wb') as target:
                for offset in range(0, len(compressed), 32768):
                    target.write(compressed[offset:offset + 32768])
            sftp.chmod(archive, 0o600)
            script = (GATEWAY_ROOT / 'deploy/eg100/deploy_remote_config_20260915.sh').read_bytes().replace(b'\r\n', b'\n')
            script = script.replace(b"grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts", mount_check.encode())
            script = script.replace(b"trap 'exit 143' TERM", b"trap 'exit 143' TERM\ntrap 'exit 129' HUP")
            with sftp.open(backup + '/deploy.sh', 'wb') as target:
                target.write(script)
            sftp.chmod(backup + '/deploy.sh', 0o600)
            manifest = {'oldBinarySha256': old_hash, 'newBinarySha256': new_hash, 'configSha256': config_hash, 'preservedFiles': before, 'networkSha256': before_network, 'backup': backup, 'size': len(payload)}
            with sftp.open(backup + '/manifest.json', 'wb') as target:
                target.write(json.dumps(manifest).encode())
            sftp.chmod(backup + '/manifest.json', 0o600)
        print(json.dumps({'stage': 'backup-verified', 'backup': backup, 'configUnchanged': True}), flush=True)
        args = [backup, archive, new_hash, old_hash, config_hash]
        output = g.shell(f'sh {backup}/deploy.sh ' + ' '.join(shlex.quote(arg) for arg in args) + f' > {backup}/deploy.log 2>&1', timeout=180)
        assert 'DEPLOY_OK' in g.shell(f'cat {backup}/deploy.log')
        health = json.loads(g.shell('wget -T 5 -qO- http://127.0.0.1:8088/api/healthz'))
        ready = json.loads(g.shell('wget -T 5 -qO- http://127.0.0.1:8088/api/readyz'))
        assert health['healthy'] and ready['ready']
        assert health['pointCount'] == health_before['pointCount']
        assert g.shell(f'sha256sum {DAEMON}').split()[0] == new_hash
        with g.ssh.open_sftp() as sftp:
            assert {path: file_hash(sftp, path) for path in preserved} == before, 'Config, license, init or native library changed'
        assert digest(g.shell('ip -4 addr show; ip -4 route show').encode()) == before_network, 'Network changed'
        pid = g.shell('pidof weikong-gateway').strip()
        time.sleep(5)
        assert g.shell('pidof weikong-gateway').strip() == pid, 'PID not stable'
        result = {'deployed': True, 'backup': backup, 'newBinarySha256': new_hash, 'configUnchanged': True, 'authorizationUnchanged': True, 'accountsUnchanged': True, 'networkUnchanged': True, 'nativeLibraryUnchanged': True, 'healthy': True, 'ready': True, 'pidStable': True, 'pointCount': health['pointCount']}
        out = ROOT / os.environ.get('WK_EG100_DEPLOY_OUTPUT', 'artifacts/eg100-activation-entry-20260917')
        out.mkdir(parents=True, exist_ok=True)
        (out / 'deployment.json').write_text(json.dumps(result, indent=2), encoding='utf8')
        print(json.dumps(result), flush=True)
    finally:
        g.ssh.close()

if __name__ == '__main__':
    main()
