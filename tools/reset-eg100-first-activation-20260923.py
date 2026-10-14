"""Reversible, activation-only lab reset. Leaves firmware and business data intact."""
import copy
import hashlib
import json
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

DIRECTORY = '/userdata/weikong/config'
CONFIG = DIRECTORY + '/config.local.json'
INIT = '/etc/init.d/S99weikong-gateway'
BINARY = '/userdata/weikong/bin/weikong-gateway'
BACKUPS = '/userdata/weikong/backups'
EXPECTED_SN = 'WK20265AF2F5C6'


def read(sftp, path):
    with sftp.open(path, 'rb') as source:
        return source.read()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def regular(sftp, path):
    assert stat.S_ISREG(sftp.lstat(path).st_mode), 'Not a regular file: ' + path
    assert sftp.normalize(path) == path, 'Unexpected resolved path: ' + path


def write_private(sftp, path, data):
    with sftp.open(path, 'wx') as target:
        sftp.chmod(path, 0o600)
        target.write(data)
    assert read(sftp, path) == data


def main():
    assert sys.argv[1:] == ['--confirmed-first-activation-reset']
    g = Gateway()
    backup = BACKUPS + '/activation-reset-' + time.strftime('%Y%m%d-%H%M%S')
    assert re.fullmatch(BACKUPS + r'/activation-reset-\d{8}-\d{6}', backup)
    moved = []
    touched = completed = changed_config = False
    try:
        with g.ssh.open_sftp() as sftp:
            for directory in [DIRECTORY, BACKUPS]:
                assert sftp.normalize(directory) == directory
                assert stat.S_ISDIR(sftp.lstat(directory).st_mode)
            regular(sftp, CONFIG)
            original = read(sftp, CONFIG)
            cfg = json.loads(original)
            assert cfg['gatewayKey'] == EXPECTED_SN, 'Wrong gateway'
            current = cfg.get('activation', {}).get('file') or 'activation.local.json'
            if not current.startswith('/'):
                current = posixpath.join(DIRECTORY, current)
            assert posixpath.dirname(current) == DIRECTORY and posixpath.normpath(current) == current
            license_data = json.loads(read(sftp, current))
            assert license_data.get('sn') == EXPECTED_SN, 'License identity mismatch'
            files = sorted({DIRECTORY + '/' + f.filename for f in sftp.listdir_attr(DIRECTORY)
                            if (f.filename.startswith('activation-') and f.filename.endswith('.json'))
                            or f.filename in ['activation.local.json', '.license-state.json']} | {current})
            contents = {}
            for path in files:
                regular(sftp, path)
                contents[path] = read(sftp, path)
            binary_hash = g.shell('sha256sum ' + BINARY).split()[0]
            network = g.shell('ip -4 addr show; ip -4 route show')
            release = read(sftp, DIRECTORY + '/ota-release.json')
            sftp.mkdir(backup, 0o700)
            write_private(sftp, backup + '/config.original.json', original)
            for index, path in enumerate(files):
                write_private(sftp, backup + '/original-' + str(index) + '.json', contents[path])
            print(json.dumps({'backupVerified': backup, 'authorizationFiles': files,
                              'gateway': EXPECTED_SN, 'release': json.loads(release)}), flush=True)
            touched = True
            g.shell(INIT + ' stop; test -z "$(pidof weikong-gateway 2>/dev/null || true)"')
            assert read(sftp, CONFIG) == original, 'Config changed during preparation'
            for path in files:
                assert read(sftp, path) == contents[path], 'Authorization changed during preparation'
                destination = backup + '/' + posixpath.basename(path)
                sftp.rename(path, destination)
                moved.append((path, destination))
                sftp.chmod(destination, 0o600)
                assert read(sftp, destination) == contents[path]
            next_cfg = copy.deepcopy(cfg)
            next_cfg['activation'] = {'enabled': True, 'file': DIRECTORY + '/activation.local.json'}
            raw = (json.dumps(next_cfg, ensure_ascii=False, indent=2) + '\n').encode()
            candidate = CONFIG + '.first-activation-reset-' + time.strftime('%Y%m%d%H%M%S')
            write_private(sftp, candidate, raw)
            g.shell('env LD_LIBRARY_PATH=/userdata/weikong/lib ' + BINARY +
                    ' -config ' + shlex.quote(candidate) + ' -check-config', timeout=60)
            sftp.posix_rename(candidate, CONFIG)
            changed_config = True
            g.shell('sync; ' + INIT + ' start')
            result = None
            for _ in range(30):
                try:
                    result = g.api('/api/license/status')
                    if result.get('initialActivationAllowed'):
                        break
                except Exception:
                    pass
                time.sleep(1)
            assert result and result['initialActivationAllowed'], 'First activation unavailable'
            assert result['license']['restricted'] and not result['license']['valid']
            assert result['gatewayKey'] == EXPECTED_SN
            assert g.shell('sha256sum ' + BINARY).split()[0] == binary_hash
            assert read(sftp, DIRECTORY + '/ota-release.json') == release
            assert g.shell('ip -4 addr show; ip -4 route show') == network
            assert json.loads(read(sftp, CONFIG)) == next_cfg
            pid = g.shell('pidof weikong-gateway').strip()
            time.sleep(5)
            assert g.shell('pidof weikong-gateway').strip() == pid
            assert g.api('/api/healthz')['healthy']
            manifest = {'gateway': EXPECTED_SN, 'backup': backup, 'moved': moved,
                        'configBeforeSha256': digest(original), 'configAfterSha256': digest(raw),
                        'restricted': True, 'licenseValid': False, 'initialActivationAllowed': True,
                        'healthy': True, 'firmwareUnchanged': True, 'networkUnchanged': True,
                        'businessConfigurationUnchanged': True, 'release': json.loads(release)}
            write_private(sftp, backup + '/manifest.json', json.dumps(manifest, indent=2).encode())
            completed = True
            print(json.dumps(manifest), flush=True)
    finally:
        try:
            if touched and not completed:
                g.shell(INIT + ' stop')
                with g.ssh.open_sftp() as sftp:
                    for source, destination in reversed(moved):
                        sftp.posix_rename(destination, source)
                    if changed_config:
                        restore = backup + '/config.restore.json'
                        write_private(sftp, restore, original)
                        sftp.posix_rename(restore, CONFIG)
                g.shell('sync; ' + INIT + ' start')
                print('Reset failed; original authorization and configuration restored.', flush=True)
        finally:
            g.ssh.close()


if __name__ == '__main__':
    main()
