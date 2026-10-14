"""Read-only EG100 initialization inventory; never print credentials."""
import hashlib
import json
import os
import posixpath
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway

def digest(data):
    return hashlib.sha256(data).hexdigest()

def main():
    g = Gateway()
    try:
        with g.ssh.open_sftp() as sftp:
            with sftp.open('/userdata/weikong/config/config.local.json') as source:
                raw = source.read()
            cfg = json.loads(raw)
            inventory = []
            for directory in ['/userdata/weikong/config', '/userdata/weikong/config/.runtime', '/userdata/weikong/data', '/mnt/udisk/.weikong', '/mnt/udisk/.weikong/history', '/mnt/udisk/.remote-storage']:
                try:
                    inventory += [{'path':posixpath.join(directory, f.filename),'bytes':f.st_size,'mode':f.st_mode} for f in sftp.listdir_attr(directory)]
                except FileNotFoundError:
                    pass
            result = {
                'gatewayKey':cfg.get('gatewayKey'),
                'configSha256':digest(raw),
                'configFields':list(cfg),
                'counts':{key:len(cfg.get(key) or []) for key in ['resources','channels','devices','points','forwardDevices','mqttChannels']},
                'resourceTypes':[{'key':r.get('resourceKey'), 'type':r.get('type'), 'keys':list(r)} for r in cfg.get('resources',[])],
                'alarmRules':len((cfg.get('alarms') or {}).get('rules') or []),
                'historyPath':(cfg.get('historyStorage') or {}).get('storagePath'),
                'historyBackupPath':(cfg.get('historyStorage') or {}).get('backupPath'),
                'remoteStoragePath':(cfg.get('remoteStorage') or {}).get('storagePath'),
                'cacheFile':cfg.get('cacheFile'),
                'offlineCachePath':(cfg.get('offlineCache') or {}).get('storagePath'),
                'activationFile':(cfg.get('activation') or {}).get('file'),
                'preservedFields':{key:digest(json.dumps(cfg.get(key),sort_keys=True).encode()) for key in ['networkPorts','wifi','cellular','serialPorts','security','web','watchdog']},
                'files':inventory,
                'platform':g.shell('df -Pk /userdata /mnt/udisk; sha256sum /userdata/weikong/bin/weikong-gateway; ip -4 addr show; ip -4 route show'),
            }
            print(json.dumps(result,ensure_ascii=False,indent=2))
    finally:
        g.ssh.close()

if __name__ == '__main__':
    main()
