"""User-authorized business reset; keep program, network, web accounts intact.

All removed files are moved to a restricted USB backup, never recursively deleted.
"""
import copy
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
DAEMON = '/userdata/weikong/bin/weikong-gateway'
PRESERVED = ['gatewayKey','schemaVersion','resources','networkPorts','serialPorts','wifi','cellular','web','security','watchdog']

def sha(data):
    return hashlib.sha256(data).hexdigest()

def file_bytes(sftp, path):
    with sftp.open(path, 'rb') as source:
        return source.read()

def exists(sftp, path):
    try:
        return sftp.lstat(path)
    except FileNotFoundError:
        return None

def main():
    if sys.argv[1:] != ['--confirmed-business-reset']:
        raise RuntimeError('Explicit confirmed reset flag is required')
    g = Gateway()
    stopped = False
    modified = False
    completed = False
    moved = []
    backup = '/mnt/udisk/.weikong-deploy/initialization-20260917-' + time.strftime('%H%M%S')
    local = ROOT / 'secrets/initialization-20260917-0950'
    if not (local / 'plan.json').is_file():
        raise RuntimeError('Verified cloud backup plan is required first')
    try:
        with g.ssh.open_sftp() as sftp:
            original = file_bytes(sftp, CONFIG)
            current = json.loads(original)
            before_hash = sha(original)
            binary_hash = g.shell('sha256sum ' + DAEMON).split()[0]
            network_before = g.shell('ip -4 addr show; ip -4 route show')
            network_files = {}
            for p in ['/etc/network/interfaces','/etc/wpa_supplicant.conf','/etc/resolv.conf']:
                if exists(sftp, p):
                    network_files[p] = sha(file_bytes(sftp, p))
            next_cfg = {key:copy.deepcopy(current[key]) for key in PRESERVED if key in current}
            next_cfg.update({
                'collectIntervalSeconds':5,
                'cacheFile':'/userdata/weikong/data/gateway-spool.jsonl',
                'offlineCache':{'enabled':False,'maxSizeMB':16,'storagePath':current.get('offlineCache',{}).get('storagePath','/mnt/udisk')},
                'historyStorage':{'enabled':False,'maxSizeMB':16,'retentionDays':30,'storagePath':current.get('historyStorage',{}).get('storagePath','/mnt/udisk'),'backupEnabled':False,'tasks':[]},
                'remoteStorage':{'storagePath':current.get('remoteStorage',{}).get('storagePath','/mnt/udisk'),'maxSizeMB':16,'targets':[]},
                'activation':{'enabled':True,'file':CONFIG_DIR+'/activation.local.json'},
                'cloudConnectionEnabled':False,
                'mqtt':{'enabled':False,'broker':'','clientId':'','username':'','password':''},
                'mqttChannels':[],
                'remoteAccess':{'enabled':False},
                'channels':[], 'devices':[], 'points':[], 'forwardDevices':[],
                'edgeComputing':{'enabled':False,'rules':[]},
                'alarms':{'enabled':False,'rules':[]},
                'forwardSlave':{'enabled':False,'listen':'0.0.0.0:1502'},
            })
            next_raw = (json.dumps(next_cfg,ensure_ascii=False,indent=2)+'\n').encode('utf8')
            next_hash = sha(next_raw)
            for key in PRESERVED:
                assert next_cfg.get(key) == current.get(key), f'Preserved field changed: {key}'
            g.shell("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && test ! -e " + backup + ' && mkdir -m 700 ' + backup)
            # Strictly scoped backup path is not under either business-data directory.
            assert re.fullmatch(r'/mnt/udisk/\.weikong-deploy/initialization-20260917-\d{6}', backup)
            g.shell(f'gzip -c {DAEMON} > {backup}/weikong-gateway.original.gz && chmod 600 {backup}/weikong-gateway.original.gz && test "$(gzip -dc {backup}/weikong-gateway.original.gz | sha256sum | cut -d " " -f 1)" = {binary_hash}', timeout=180)
            print(json.dumps({'stage':'program-backup-verified','backup':backup}),flush=True)
            g.shell('/etc/init.d/S99weikong-gateway stop; test -z "$(pidof weikong-gateway 2>/dev/null || true)"')
            stopped = True
            assert sha(file_bytes(sftp,CONFIG)) == before_hash, 'Configuration changed before stopping'
            g.shell(f'cp -a {CONFIG_DIR} {backup}/config.before && chmod 700 {backup}/config.before && test "$(sha256sum {backup}/config.before/config.local.json | cut -d " " -f 1)" = {before_hash}')
            sqlite_files = ['/userdata/weikong/config/.runtime/alarm-events.db','/userdata/weikong/config/.runtime/collection-incidents.db','/mnt/udisk/.weikong/history/telemetry-history.db', '/mnt/udisk/.remote-storage/eg100-pg-integration.db','/mnt/udisk/.remote-storage/eg100-mysql-integration.db','/mnt/udisk/.remote-storage/eg100-sqlserver-integration.db']
            for p in sqlite_files:
                if exists(sftp,p):
                    assert g.shell('sqlite3 ' + shlex.quote(p) + " 'PRAGMA quick_check;'",timeout=180).strip() == 'ok', 'Data integrity check failed'
                    print(json.dumps({'stage':'data-integrity-verified','file':p}),flush=True)
            storage_paths = ['/mnt/udisk/.weikong','/mnt/udisk/.remote-storage']
            # Only these two explicitly verified USB business directories are moved.
            for p in storage_paths:
                meta = exists(sftp,p)
                if meta:
                    assert stat.S_ISDIR(meta.st_mode) and not stat.S_ISLNK(meta.st_mode)
                    assert sftp.normalize(p) == p, 'Unexpected resolved business data path'
                    destination = backup + '/data-' + posixpath.basename(p).lstrip('.')
                    sftp.rename(p,destination)
                    moved.append((p,destination))
                    modified = True
            reset_files = []
            for f in sftp.listdir_attr(CONFIG_DIR):
                if stat.S_ISREG(f.st_mode) and (re.fullmatch(r'activation(?:[.-][\w.-]+)?\.json',f.filename) or f.filename in ['.license-state.json','config.local.json.previous','config.local.json.remote.bak','config.local.json.remote.version']):
                    reset_files.append(CONFIG_DIR+'/'+f.filename)
            for base in ['alarm-events.db','alarm-events.jsonl','alarm-events.jsonl.migrated','collection-incidents.db','cloud-alarm-events.jsonl','gateway-spool.jsonl']:
                for suffix in ['', '-wal', '-shm']:
                    reset_files.append(CONFIG_DIR+'/.runtime/'+base+suffix)
            reset_files += ['/userdata/weikong/data/gateway-spool.jsonl','/mnt/udisk/gateway-spool.jsonl']
            staged_dir = backup+'/removed-files'
            sftp.mkdir(staged_dir,0o700)
            for index,p in enumerate(dict.fromkeys(reset_files)):
                meta = exists(sftp,p)
                if not meta:
                    continue
                assert stat.S_ISREG(meta.st_mode) and not stat.S_ISLNK(meta.st_mode), 'Unexpected reset file type'
                assert p.startswith(CONFIG_DIR+'/') or p in ['/userdata/weikong/data/gateway-spool.jsonl','/mnt/udisk/gateway-spool.jsonl']
                assert sftp.normalize(p) == p
                destination = staged_dir + f'/{index:03d}-' + posixpath.basename(p)
                before_file = g.shell('sha256sum '+shlex.quote(p)).split()[0]
                g.shell('mv '+shlex.quote(p)+' '+shlex.quote(destination))
                moved.append((p,destination))
                modified = True
                assert g.shell('sha256sum '+shlex.quote(destination)).split()[0] == before_file
            candidate = CONFIG+'.initialization-new'
            assert not exists(sftp,candidate)
            with sftp.open(candidate,'wb') as output:
                output.write(next_raw)
            sftp.chmod(candidate,0o600)
            assert sha(file_bytes(sftp,candidate)) == next_hash
            g.shell(f'env LD_LIBRARY_PATH=/userdata/weikong/lib {DAEMON} -config {candidate} -check-config',timeout=60)
            sftp.posix_rename(candidate,CONFIG)
            modified = True
            manifest = {'beforeSha256':before_hash,'afterSha256':next_hash,'binarySha256':binary_hash,'backup':backup,'moved':moved,'createdAt':time.strftime('%Y-%m-%d %H:%M:%S')}
            with sftp.open(backup+'/manifest.json','w') as out:
                out.write(json.dumps(manifest,ensure_ascii=False,indent=2))
            sftp.chmod(backup+'/manifest.json',0o600)
            (local/'eg100-manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf8')
            g.shell('sync; /etc/init.d/S99weikong-gateway start')
            stopped = False
            health = None
            for _ in range(60):
                try:
                    health = g.api('/api/healthz')
                    if health.get('healthy'):
                        break
                except Exception:
                    pass
                time.sleep(1)
            assert health and health['healthy'] and health['pointCount'] == 0, 'Gateway failed to become empty and healthy'
            g.login()
            status = g.api('/api/license/status')
            assert status['license']['restricted'] and not status['license']['valid']
            assert sha(file_bytes(sftp,CONFIG)) == next_hash
            assert g.shell('sha256sum '+DAEMON).split()[0] == binary_hash
            actual_cfg = json.loads(file_bytes(sftp,CONFIG))
            for key in PRESERVED:
                assert actual_cfg.get(key) == current.get(key)
            for p,checksum in network_files.items():
                assert sha(file_bytes(sftp,p)) == checksum
            assert g.shell('ip -4 addr show; ip -4 route show') == network_before
            result = {'initialized':True,'restricted':True,'pointCount':0,'collectionDevices':len(actual_cfg['devices']),'forwardDevices':len(actual_cfg['forwardDevices']),'channels':len(actual_cfg['channels']),'alarmRules':len(actual_cfg['alarms']['rules']),'remoteDatabaseTasks':len(actual_cfg['remoteStorage']['targets']),'programUnchanged':True,'networkUnchanged':True,'accountsUnchanged':True,'adminLoginVerified':True,'backup':backup,'movedItems':len(moved),'healthy':True}
            (local/'eg100-result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf8')
            completed = True
            print(json.dumps(result,ensure_ascii=False),flush=True)
    finally:
        if not completed and (stopped or modified):
            g.shell('/etc/init.d/S99weikong-gateway stop')
            with g.ssh.open_sftp() as sftp:
                # Move each exact original item back; do not recursively remove new data.
                for index,(original_path,destination) in enumerate(reversed(moved)):
                    if exists(sftp,original_path):
                        g.shell('mv '+shlex.quote(original_path)+' '+shlex.quote(backup+'/rollback-new-'+str(index)))
                    g.shell('mv '+shlex.quote(destination)+' '+shlex.quote(original_path))
                if modified:
                    with sftp.open(CONFIG+'.rollback','wb') as out:
                        out.write(file_bytes(sftp,backup+'/config.before/config.local.json'))
                    sftp.chmod(CONFIG+'.rollback',0o600)
                    sftp.posix_rename(CONFIG+'.rollback',CONFIG)
            g.shell('sync; /etc/init.d/S99weikong-gateway start')
            print('Initialization failed; restored original configuration and data.',flush=True)
        g.ssh.close()

if __name__ == '__main__':
    main()
