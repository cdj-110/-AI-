"""Confirmed activation-only reset, with recoverable backups and rollback."""
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
sys.path.insert(0, str(ROOT / 'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway

CONFIG_DIR = '/userdata/weikong/config'
CONFIG = CONFIG_DIR + '/config.local.json'
INIT = '/etc/init.d/S99weikong-gateway'

def read(sftp, path):
    with sftp.open(path, 'rb') as source:
        return source.read()

def sha(raw):
    return hashlib.sha256(raw).hexdigest()

def main():
    if sys.argv[1:] != ['--confirmed-authorization-only-reset']:
        raise RuntimeError('Explicit authorization-only reset confirmation required')
    g = Gateway()
    backup = '/mnt/udisk/.weikong-deploy/license-reset-' + time.strftime('%Y%m%d-%H%M%S')
    assert re.fullmatch(r'/mnt/udisk/\.weikong-deploy/license-reset-\d{8}-\d{6}', backup)
    moved = []
    stopped = False
    changed_config = False
    completed = False
    try:
        with g.ssh.open_sftp() as sftp:
            original = read(sftp, CONFIG)
            cfg = json.loads(original)
            network = sha(g.shell('ip -4 addr show; ip -4 route show').encode())
            binary = g.shell('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0]
            files = [CONFIG_DIR + '/' + item.filename for item in sftp.listdir_attr(CONFIG_DIR)
                     if (re.fullmatch(r'activation(?:[.-][\w.-]+)?\.json', item.filename) or item.filename == '.license-state.json')]
            current = cfg.get('activation', {}).get('file') or CONFIG_DIR + '/activation.local.json'
            if not current.startswith('/'):
                current = posixpath.join(CONFIG_DIR, current)
            assert current.startswith(CONFIG_DIR + '/') and posixpath.normpath(current) == current
            try:
                sftp.lstat(current)
                files.append(current)
            except FileNotFoundError:
                pass
            for source in dict.fromkeys(files):
                meta = sftp.lstat(source)
                assert stat.S_ISREG(meta.st_mode) and not stat.S_ISLNK(meta.st_mode)
                assert sftp.normalize(source) == source and source.startswith(CONFIG_DIR + '/')
            g.shell("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && "
                    f'test ! -e {backup} && mkdir -m 700 {backup} && cp -p {CONFIG} {backup}/config.original.json && chmod 600 {backup}/config.original.json')
            assert sha(read(sftp, backup + '/config.original.json')) == sha(original)
            g.shell(INIT + ' stop; test -z "$(pidof weikong-gateway 2>/dev/null || true)"')
            stopped = True
            assert sha(read(sftp, CONFIG)) == sha(original), 'Configuration changed before reset'
            for source in dict.fromkeys(files):
                destination = backup + '/' + posixpath.basename(source)
                expected = sha(read(sftp, source))
                g.shell('mv ' + shlex.quote(source) + ' ' + shlex.quote(destination) + ' && chmod 600 ' + shlex.quote(destination))
                moved.append((source, destination))
                assert sha(read(sftp, destination)) == expected
            # Leave every other field intact, including account hashes/network and business settings.
            next_cfg = dict(cfg)
            next_cfg['activation'] = {'enabled': True, 'file': CONFIG_DIR + '/activation.local.json'}
            raw = (json.dumps(next_cfg, ensure_ascii=False, indent=2) + '\n').encode()
            candidate = CONFIG + '.license-reset-new'
            try:
                sftp.lstat(candidate)
                raise RuntimeError('Unexpected reset candidate already exists')
            except FileNotFoundError:
                pass
            with sftp.open(candidate, 'wb') as target:
                target.write(raw)
            sftp.chmod(candidate, 0o600)
            assert sha(read(sftp, candidate)) == sha(raw)
            g.shell('env LD_LIBRARY_PATH=/userdata/weikong/lib /userdata/weikong/bin/weikong-gateway -config ' + candidate + ' -check-config', timeout=60)
            sftp.posix_rename(candidate, CONFIG)
            changed_config = True
            g.shell('sync; ' + INIT + ' start')
            stopped = False
            result = None
            for _ in range(45):
                try:
                    result = g.api('/api/license/status')
                    if result.get('license', {}).get('restricted'):
                        break
                except Exception:
                    pass
                time.sleep(1)
            assert result and result['license']['restricted'] and not result['license']['valid']
            assert result['initialActivationAllowed'], 'Activation evidence remains; anonymous signed activation unavailable'
            assert result['hardwareIdentity']['id'] == '3AF104FF59D83AB08D4880783218BA75'
            assert sha(g.shell('ip -4 addr show; ip -4 route show').encode()) == network
            assert g.shell('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0] == binary
            assert json.loads(read(sftp, CONFIG)) == next_cfg
            for key in cfg:
                if key != 'activation':
                    assert next_cfg[key] == cfg[key]
            pid = g.shell('pidof weikong-gateway').strip()
            time.sleep(5)
            assert g.shell('pidof weikong-gateway').strip() == pid
            health = g.api('/api/healthz')
            assert health['healthy']
            manifest = {'backup': backup, 'moved': moved, 'configBeforeSha256': sha(original), 'configAfterSha256': sha(raw), 'hardwareId': result['hardwareIdentity']['id'], 'authorizationReset': True, 'firstActivationAllowed': True, 'accountsUnchanged': True, 'networkUnchanged': True, 'businessConfigUnchanged': True, 'binarySha256': binary, 'healthy': True, 'pidStable': True}
            with sftp.open(backup + '/manifest.json', 'wb') as target:
                target.write(json.dumps(manifest).encode())
            sftp.chmod(backup + '/manifest.json', 0o600)
            out = ROOT / 'artifacts/license-report-deployment-20260917'
            out.mkdir(parents=True, exist_ok=True)
            (out / 'eg100-reset.json').write_text(json.dumps(manifest, indent=2), encoding='utf8')
            completed = True
            print(json.dumps(manifest), flush=True)
    finally:
        if not completed and (moved or changed_config or stopped):
            g.shell(INIT + ' stop')
            for source, destination in reversed(moved):
                g.shell('test ! -e ' + shlex.quote(source) + ' && mv ' + shlex.quote(destination) + ' ' + shlex.quote(source))
            if changed_config:
                g.shell('cp -p ' + backup + '/config.original.json ' + CONFIG)
            g.shell('sync; ' + INIT + ' start')
            print('Authorization reset failed; previous authorization/config restored.', flush=True)
        g.ssh.close()

if __name__ == '__main__':
    main()
