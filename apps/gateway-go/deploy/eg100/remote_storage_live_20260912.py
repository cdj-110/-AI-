"""EG100 isolated PostgreSQL acceptance. Credentials come only from environment."""
import http.client
from http.cookies import SimpleCookie
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import time
import urllib.parse
import paramiko

BASE = Path('build/remote-storage-live-20260912')
BASE.mkdir(exist_ok=True)
TARGET = 'eg100-pg-integration'
QUEUE = '/mnt/udisk/.remote-storage/' + TARGET + '.db'
TABLE = 'eg100_samples'

class Gateway:
    def __init__(self):
        self.ssh = paramiko.SSHClient()
        self.ssh.load_system_host_keys()
        self.ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        self.ssh.connect('192.168.1.214', username='root', password=os.environ['WK_EG100_PASSWORD'], timeout=10)
        self.cookies = SimpleCookie()

    def shell(self, command, timeout=120):
        _, out, err = self.ssh.exec_command(command, timeout=timeout)
        result, error = out.read().decode(), err.read().decode()
        if out.channel.recv_exit_status():
            raise RuntimeError(result + error)
        return result

    def api(self, path, method='GET', data=None, headers=None, form=False):
        conn = http.client.HTTPConnection('127.0.0.1', 8088, timeout=25)
        conn.sock = self.ssh.get_transport().open_channel('direct-tcpip', ('127.0.0.1', 8088), ('127.0.0.1', 0), timeout=25)
        conn.sock.settimeout(25)
        h = {'Origin': 'http://127.0.0.1:8088', 'Cookie': '; '.join(k + '=' + v.value for k, v in self.cookies.items())}
        h.update(headers or {})
        body = None
        if data is not None:
            body = (urllib.parse.urlencode(data) if form else json.dumps(data)).encode()
            h['Content-Type'] = 'application/x-www-form-urlencoded' if form else 'application/json'
        try:
            conn.request(method, path, body, h)
            response = conn.getresponse()
            for key, value in response.getheaders():
                if key.lower() == 'set-cookie':
                    self.cookies.load(value)
            payload = response.read()
            if response.status >= 400:
                raise RuntimeError(f'{path}: HTTP {response.status}: {payload.decode()[:800]}')
            if path == '/login':
                assert response.status == 303 and response.getheader('Location') == '/', 'Login requires additional action'
                return None
            assert response.status == 200, (path, response.status)
            if path == '/':
                return payload.decode()
            return json.loads(payload)
        finally:
            conn.close()

    def login(self):
        self.api('/login', 'POST', {'username': 'admin', 'password': os.environ['WK_EG100_WEB_PASSWORD']}, form=True)

    def status(self):
        items = self.api('/api/remote-storage/status')['statuses']
        return next(x for x in items if x['id'] == TARGET)

def sql(query):
    result = subprocess.run(['docker','exec','-i','weikong-iot-platform-postgres-1','psql','-U','weikong','-d','eg100_integration','-X','-At','-v','ON_ERROR_STOP=1'], input=query, text=True, encoding='utf-8', capture_output=True, timeout=30)
    if result.returncode:
        raise RuntimeError(result.stderr)
    return result.stdout.strip()

def save(name, value):
    (BASE / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')
    print(name, json.dumps(value, ensure_ascii=False), flush=True)

g = Gateway()
try:
    g.login()
    mode = sys.argv[1]
    if mode == 'inspect':
        p = g.api('/api/point-status')
        save('points-before', [{k:x.get(k) for k in ['deviceKey','metric','value','quality','error','updatedAt','receiveTimestamp']} for x in p])
        save('history-before', g.api('/api/history-storage'))
        cfg = g.api('/api/remote-storage')
        save('remote-before', {'config':cfg['config'],'revision':cfg['revision'],'statuses':cfg['statuses']})
    elif mode == 'configure':
        cfg = g.api('/api/remote-storage')
        assert not cfg['config']['targets'], 'Do not replace existing remote targets'
        points = g.api('/api/point-status')
        selected = [{'deviceKey':x['deviceKey'],'metric':x['metric']} for x in points if x.get('quality') == 'GOOD' and (x.get('deviceKey') in ['noise-meter-1','s7-01'] or (x.get('deviceKey') == 'opcda-matrikon-test' and x.get('metric') in ['boolean','opcda-P15','real4','string','uint8']))][:8]
        assert len(selected) >= 5, 'Need at least five healthy representative points'
        target = {'id':TARGET,'name':'EG100 PostgreSQL 实库联调','enabled':True,'driver':'postgresql','host':'192.168.1.141','port':5432,'database':'eg100_integration','schema':'public','table':TABLE,'username':'eg100_writer','password':os.environ['REMOTE_TEST_PASSWORD'],'tlsMode':'disabled','createTable':True,'intervalSeconds':5,'batchSize':100,'timeoutSeconds':10,'points':selected}
        next_config = {'storagePath':'/mnt/udisk','storageUUID':'a859efdd-6a14-415e-89e5-aa761bc1b46f','maxSizeMB':16,'targets':[target]}
        result = g.api('/api/remote-storage','PUT',next_config,headers={'If-Match':cfg['revision']})
        assert result['config']['targets'][0]['password'] == '***'
        save('configured', {'selectedPoints':selected,'statuses':result['statuses'],'passwordMasked':True})
        for i in range(35):
            status = g.status()
            if status.get('sent',0) >= len(selected)*2 and status.get('pending',0) == 0:
                break
            time.sleep(2)
        save('normal-status',status)
        assert status.get('sent',0) >= len(selected)*2 and status.get('dropped',0) == 0, 'Initial sending did not pass'
        masked = dict(result['config']['targets'][0])
        save('connection-test',g.api('/api/remote-storage/test','POST',masked))
        save('normal-db',sql(f'SELECT count(*),count(DISTINCT record_id),count(DISTINCT (device_key,metric)),min(sampled_ms),max(sampled_ms) FROM public.{TABLE};'))
    elif mode == 'resilience':
        status = g.status()
        assert status.get('connected') and status.get('dropped',0) == 0
        before = g.api('/api/healthz')
        blocked = False
        try:
            # Only this isolated test account is interrupted, not the platform DB/server.
            sql("ALTER ROLE eg100_writer NOLOGIN;")
            blocked = True
            sql("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename='eg100_writer' AND pid<>pg_backend_pid();")
            time.sleep(22)
            status = g.status()
            save('outage-status', status)
            assert status['pending'] > 0 and status.get('lastError'), 'Outage was not reflected in durable queue'
            healthy = g.api('/api/healthz')
            save('collection-during-outage', healthy)
            assert healthy['healthy'] and healthy['lastCollectAt'] != before['lastCollectAt']
            raw = g.shell(f"sqlite3 {QUEUE} 'SELECT payload FROM remote_queue ORDER BY seq LIMIT 100;'")
            queued = [json.loads(line) for line in raw.splitlines()]
            assert queued
            save('queued-before-restart',queued)
            print('Restarting gateway with pending isolated test records.',flush=True)
            g.shell('/etc/init.d/S99weikong-gateway restart',timeout=120)
            for i in range(45):
                try:
                    if g.api('/api/readyz').get('ready'):
                        break
                except Exception:
                    pass
                time.sleep(1)
            else:
                raise RuntimeError('Gateway did not become ready')
            g.login()
            for i in range(10):
                status = g.status()
                if status.get('cacheReady'):
                    break
                time.sleep(1)
            save('pending-after-restart',status)
            assert status['pending'] >= len(queued), 'Pending records missing after restart'
        finally:
            if blocked:
                sql('ALTER ROLE eg100_writer LOGIN;')
                print('Isolated test account restored.',flush=True)
        for i in range(50):
            status = g.status()
            if status.get('connected') and status.get('pending') == 0:
                break
            time.sleep(2)
        save('recovery-status', status)
        assert status.get('connected') and status['pending'] == 0 and status.get('dropped',0) == 0
        ids = [x.get('id',x.get('ID')) for x in queued]
        assert all(isinstance(x,str) and len(x)==48 and all(c in '0123456789abcdef' for c in x) for x in ids)
        quoted = ','.join("'"+x+"'" for x in ids)
        found = int(sql(f'SELECT count(*) FROM public.{TABLE} WHERE record_id IN ({quoted});'))
        assert found == len(ids), 'Not all queued records were recovered'
        save('backfill-verification',{'queuedBeforeRestart':len(ids),'foundAfterRecovery':found,'databaseCounts':sql(f'SELECT count(*),count(DISTINCT record_id) FROM public.{TABLE};')})
        save('health-after',g.api('/api/healthz'))
        try:
            save('history-after',g.api('/api/history-storage'))
        except Exception as error:
            save('history-status-warning',{'error':type(error).__name__,'message':str(error)})
    elif mode == 'verify':
        original = json.loads(Path('build/remote-storage-backup-20260912/config.original.json').read_bytes())
        with g.ssh.open_sftp() as sftp:
            with sftp.open('/userdata/weikong/config/config.local.json') as f:
                current = json.load(f)
        changed = [k for k in set(original) | set(current) if original.get(k) != current.get(k)]
        save('config-change-scope',{'changedTopLevelKeys':sorted(changed)})
        assert changed == ['remoteStorage'] or set(changed) == {'remoteStorage'}, 'Unexpected unrelated configuration changes'
        save('final-status',g.status())
        save('final-health',g.api('/api/healthz'))
        save('final-platform',g.shell('sha256sum /userdata/weikong/bin/weikong-gateway; df -k /userdata /mnt/udisk; pidof vanxoak_pilot || true'))
        save('final-database',sql(f'SELECT count(*),count(DISTINCT record_id),count(DISTINCT (device_key,metric)) FROM public.{TABLE}; SELECT quality,count(*) FROM public.{TABLE} GROUP BY quality; SELECT rolcanlogin FROM pg_roles WHERE rolname=\'eg100_writer\';'))
        assets = re.findall(r'/assets/[^"\s<>]+',g.api('/'))
        save('assets',assets)
        assert '/assets/index-BX0vHiPX.js' in assets and '/assets/index-BDfZiuYi.css' in assets
    else:
        raise RuntimeError('Unsupported mode')
finally:
    g.ssh.close()
