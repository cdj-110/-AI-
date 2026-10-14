"""Restore only the user-confirmed S7 channel from a verified on-device backup."""
import copy
import hashlib
import json
import posixpath
import stat
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway

CONFIG = '/userdata/weikong/config/config.local.json'
SOURCE = '/userdata/weikong/backups/activation-reset-20260923-093400/config.original.json'
INIT = '/etc/init.d/S99weikong-gateway'
DAEMON = '/userdata/weikong/bin/weikong-gateway'
PARENT = '/userdata/weikong/backups'


def read(sftp, path):
    with sftp.open(path, 'rb') as f:
        return f.read()


def write(sftp, path, content):
    with sftp.open(path, 'wx') as f:
        sftp.chmod(path, 0o600)
        f.write(content)
    assert read(sftp, path) == content


def main():
    assert sys.argv[1:] == ['--restore-confirmed-s7-channel']
    g = Gateway()
    stopped = changed = done = False
    try:
        with g.ssh.open_sftp() as s:
            for path in [CONFIG, SOURCE]:
                assert stat.S_ISREG(s.lstat(path).st_mode) and s.normalize(path) == path
            original = read(s, CONFIG)
            cfg, old = json.loads(original), json.loads(read(s, SOURCE))
            assert cfg['gatewayKey'] == old['gatewayKey'] == 'WK20265AF2F5C6'
            assert not any(c['channelKey'] == 'channel-001' for c in cfg['channels']), 'Channel already exists; no overwrite'
            channels = [c for c in old['channels'] if c['channelKey'] == 'channel-001']
            assert len(channels) == 1
            channel = copy.deepcopy(channels[0])
            assert channel['protocol'] == 'siemens-s7' and channel['role'] == 'collect' and channel['enabled']
            assert not channel.get('devices'), 'Do not import nested/duplicate device definitions'
            resource = next(r for r in cfg['resources'] if r['resourceKey'] == channel['resourceKey'])
            assert resource['type'] == 'network' and resource['network']['interface'] == channel['network']['interface'] == 'eth1'
            # Keep current network settings authoritative; restore no old IP configuration.
            channel['network'] = copy.deepcopy(resource['network'])
            device = next(d for d in cfg['devices'] if d['deviceKey'] == 'device-003')
            assert device['channelKey'] == 'channel-001' and device['protocol'] == 'siemens-s7'
            assert device['address'] == '192.168.1.25:102'
            next_cfg = copy.deepcopy(cfg)
            next_cfg['channels'].append(channel)
            assert {k:v for k,v in next_cfg.items() if k!='channels'} == {k:v for k,v in cfg.items() if k!='channels'}
            raw = (json.dumps(next_cfg, ensure_ascii=False, indent=2)+'\n').encode()
            license_path = cfg['activation']['file']
            if not license_path.startswith('/'):
                license_path = posixpath.join(posixpath.dirname(CONFIG), license_path)
            keep = [license_path, '/userdata/weikong/config/.license-state.json', '/userdata/weikong/config/ota-release.json', INIT]
            preserved = {p:read(s,p) for p in keep}
            binary_hash = g.shell('sha256sum '+DAEMON).split()[0]
            health_before = g.api('/api/healthz')
            network = g.shell('ip -4 addr show; ip -4 route show')
            assert s.normalize(PARENT) == PARENT and stat.S_ISDIR(s.lstat(PARENT).st_mode)
            stamp = time.strftime('%Y%m%d-%H%M%S')
            backup = PARENT+'/s7-channel-restore-'+stamp
            s.mkdir(backup, 0o700)
            write(s, backup+'/config.original.json', original)
            candidate = CONFIG+'.s7-restore-'+stamp
            write(s, candidate, raw)
            # Validation only: this does not start a second collector.
            g.shell('env LD_LIBRARY_PATH=/userdata/weikong/lib '+DAEMON+' -config '+candidate+' -check-config')
            assert read(s, CONFIG) == original, 'Configuration changed; aborting'
            print(json.dumps({'stage':'validated','backup':backup,'channel':'channel-001','resource':channel['resourceKey']},ensure_ascii=True),flush=True)
            stopped = True
            g.shell(INIT+' stop; test -z "$(pidof weikong-gateway 2>/dev/null || true)"')
            assert read(s, CONFIG) == original, 'Configuration changed before replacement'
            s.posix_rename(candidate, CONFIG)
            changed = True
            g.shell('sync; '+INIT+' start')
            health = None
            for _ in range(30):
                try:
                    health = g.api('/api/healthz')
                    if health['healthy'] and g.api('/api/readyz')['ready']:
                        break
                except Exception:
                    pass
                time.sleep(1)
            assert health and health['healthy'] and g.api('/api/readyz')['ready']
            assert health['pointCount'] == health_before['pointCount'] == 7
            assert json.loads(read(s, CONFIG)) == next_cfg
            assert {p:read(s,p) for p in keep} == preserved
            assert g.shell('sha256sum '+DAEMON).split()[0] == binary_hash
            assert g.shell('ip -4 addr show; ip -4 route show') == network
            # Readiness can precede the first collection. Require two actual
            # sample timestamps, rather than assuming the first response has one.
            first_sample = health.get('lastCollectAt')
            advancing = False
            for _ in range(20):
                time.sleep(2)
                later = g.api('/api/healthz')
                assert later['healthy']
                sampled = later.get('lastCollectAt')
                if sampled and first_sample and sampled != first_sample:
                    advancing = True
                    break
                if sampled:
                    first_sample = sampled
            assert advancing, 'No advancing sample timestamps after startup'
            manifest = {'restored':True,'backup':backup,'channelKey':'channel-001','resourceKey':channel['resourceKey'],
                        'deviceKey':'device-003','configSha256':hashlib.sha256(raw).hexdigest(),
                        'onlyChannelAdded':True,'otherConfigUnchanged':True,'licenseUnchanged':True,
                        'binaryUnchanged':True,'healthy':True,'ready':True,'collectionAdvancing':True,'pointCount':7}
            write(s, backup+'/manifest.json', json.dumps(manifest,ensure_ascii=True,indent=2).encode())
            done = True
            print(json.dumps(manifest,ensure_ascii=True),flush=True)
    finally:
        try:
            if stopped and not done:
                g.shell(INIT+' stop')
                if changed:
                    with g.ssh.open_sftp() as s:
                        restore = backup+'/config.rollback.json'
                        write(s, restore, original)
                        s.posix_rename(restore, CONFIG)
                g.shell('sync; '+INIT+' start')
                print('Restoration failed; previous configuration restored.',flush=True)
        finally:
            g.ssh.close()


if __name__ == '__main__':
    main()
