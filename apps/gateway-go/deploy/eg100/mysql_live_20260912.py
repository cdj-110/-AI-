"""Isolated MySQL acceptance. Secrets only from environment; no gateway restart here."""
import copy
import hashlib
import http.client
from http.cookies import SimpleCookie
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import urllib.parse
import paramiko

BASE = Path('build/mysql-integration-20260912')
TARGET = 'eg100-mysql-integration'
QUEUE = '/mnt/udisk/.remote-storage/' + TARGET + '.db'
CONTAINER = TARGET

def save(name, value):
    (BASE / (name + '.json')).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding='utf-8')
    print(name, json.dumps(value, ensure_ascii=False), flush=True)

def docker(*args, data=None):
    r = subprocess.run(['docker', *args], input=data, text=True, encoding='utf-8', capture_output=True, timeout=60)
    if r.returncode:
        raise RuntimeError(r.stderr)
    return r.stdout.strip()

def sql(query):
    return docker('exec', '-i', '-e', 'MYSQL_PWD', CONTAINER, 'mysql', '-uroot', '--default-character-set=utf8mb4', '--batch', '--skip-column-names', 'eg100_integration', data=query)

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
        c = http.client.HTTPConnection('127.0.0.1', 8088, timeout=25)
        c.sock = self.ssh.get_transport().open_channel('direct-tcpip', ('127.0.0.1',8088), ('127.0.0.1',0), timeout=25)
        c.sock.settimeout(25)
        h = {'Origin':'http://127.0.0.1:8088', 'Cookie':'; '.join(k+'='+v.value for k,v in self.cookies.items())}
        h.update(headers or {})
        body = None
        if data is not None:
            body = (urllib.parse.urlencode(data) if form else json.dumps(data)).encode()
            h['Content-Type'] = 'application/x-www-form-urlencoded' if form else 'application/json'
        try:
            c.request(method,path,body,h)
            r=c.getresponse()
            for k,v in r.getheaders():
                if k.lower()=='set-cookie': self.cookies.load(v)
            body=r.read()
            if path=='/login':
                assert r.status==303 and r.getheader('Location')=='/', 'Login needs user action'
                return
            if r.status!=200: raise RuntimeError(f'{path}: HTTP {r.status}: {body.decode()[:600]}')
            return json.loads(body)
        finally: c.close()

    def login(self):
        self.api('/login','POST',{'username':'admin','password':os.environ['WK_EG100_WEB_PASSWORD']},form=True)

    def status(self):
        return next(x for x in self.api('/api/remote-storage/status')['statuses'] if x['id']==TARGET)

def counts():
    return sql('SELECT COUNT(*),COUNT(DISTINCT record_id),COUNT(DISTINCT device_key,metric) FROM eg100_samples; SELECT quality,COUNT(*) FROM eg100_samples GROUP BY quality;')

def run(mode):
    g=Gateway()
    try:
        g.login()
        if mode=='inspect':
            cfg=g.api('/api/remote-storage')
            save('before',cfg)
            save('health-before',g.api('/api/healthz'))
            save('platform-before',g.shell('sha256sum /userdata/weikong/bin/weikong-gateway; df -Pk /userdata /mnt/udisk; pidof vanxoak_pilot || true'))
        elif mode=='configure':
            cfg=g.api('/api/remote-storage')
            assert all(t['id']!=TARGET for t in cfg['config']['targets']), 'Target already exists; do not overwrite'
            assert len(cfg['config']['targets'])<4
            pg=next(t for t in cfg['config']['targets'] if t['id']=='eg100-pg-integration')
            candidate=copy.deepcopy(cfg['config'])
            target={'id':TARGET,'name':'EG100 MySQL 实库联调','enabled':True,'driver':'mysql','host':'192.168.1.141','port':3306,'database':'eg100_integration','schema':'','table':'eg100_samples','username':'eg100_writer','password':os.environ['REMOTE_TEST_PASSWORD'],'tlsMode':'disabled','createTable':True,'intervalSeconds':5,'batchSize':100,'timeoutSeconds':10,'points':copy.deepcopy(pg['points'])}
            candidate['targets'].append(target)
            save('config-before-add',cfg)
            response=g.api('/api/remote-storage','PUT',candidate,headers={'If-Match':cfg['revision']})
            assert response['config']['targets'][:-1]==cfg['config']['targets'], 'Existing target changed'
            save('configured',response)
            for i in range(40):
                status=g.status()
                if status.get('sent',0)>=len(target['points'])*2 and status.get('pending')==0: break
                time.sleep(2)
            save('normal-status',status)
            assert status.get('sent',0)>=len(target['points'])*2 and status.get('dropped',0)==0
            masked=next(t for t in response['config']['targets'] if t['id']==TARGET)
            assert masked['password']=='***'
            save('connection-test',g.api('/api/remote-storage/test','POST',masked))
            save('normal-database',counts())
        elif mode=='resilience':
            before=g.api('/api/healthz')
            assert g.status().get('connected')
            stopped=False
            try:
                docker('stop','-t','20',CONTAINER)
                stopped=True
                for i in range(25):
                    status=g.status()
                    if status.get('pending',0)>=24 and status.get('lastError'): break
                    time.sleep(2)
                save('outage-status',status)
                assert status.get('pending',0)>=24 and status.get('lastError')
                health=g.api('/api/healthz')
                assert health['healthy'] and health['lastCollectAt']!=before['lastCollectAt']
                save('health-during-outage',health)
                save('other-targets-during-outage',g.api('/api/remote-storage/status'))
                raw=g.shell(f"sqlite3 {QUEUE} 'SELECT payload FROM remote_queue ORDER BY seq LIMIT 200;'")
                queued=[json.loads(line) for line in raw.splitlines()]
                assert queued
                save('queued-during-outage',queued)
            finally:
                if stopped: docker('start',CONTAINER)
            for i in range(60):
                status=g.status()
                if status.get('connected') and status.get('pending')==0: break
                time.sleep(2)
            save('recovery-status',status)
            assert status.get('connected') and status.get('pending')==0 and status.get('dropped',0)==0
            ids=[r.get('id',r.get('ID')) for r in queued]
            assert all(isinstance(x,str) and re.fullmatch('[0-9a-f]{48}',x) for x in ids)
            quoted=','.join("'"+x+"'" for x in ids)
            found=int(sql(f'SELECT COUNT(*) FROM eg100_samples WHERE record_id IN ({quoted});'))
            assert found==len(ids)
            save('backfill',{'queued':len(ids),'found':found,'counts':counts(),'gatewayRestarted':False})
        elif mode=='verify':
            assert sql("SELECT COUNT(*) FROM mysql.user WHERE user='root' AND host<>'localhost' AND account_locked='N';")=='0', 'Unexpected remote root login enabled'
            with g.ssh.open_sftp() as s:
                with s.open('/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912/config.original.json') as f:
                    original=json.load(f)
                with s.open('/userdata/weikong/config/config.local.json') as f:
                    current=json.load(f)
            changed=[k for k in set(original)|set(current) if original.get(k)!=current.get(k)]
            assert changed==['remoteStorage'], 'Unexpected unrelated config change'
            assert current['remoteStorage']['targets'][:-1]==original['remoteStorage']['targets'], 'Existing PostgreSQL target changed'
            save('config-scope',{'changedTopLevelKeys':changed,'existingTargetsUnchanged':True})
            save('final-status',g.api('/api/remote-storage/status'))
            save('final-health',g.api('/api/healthz'))
            save('final-database',counts())
            save('mysql-account',sql("SELECT user,host,account_locked FROM mysql.user WHERE user IN ('root','eg100_writer','eg100_tester'); SHOW GRANTS FOR 'eg100_writer'@'%'; SELECT VERSION(); SELECT ENGINE,TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA='eg100_integration' AND TABLE_NAME='eg100_samples';"))
        else: raise ValueError(mode)
    finally: g.ssh.close()

if __name__=='__main__': run(sys.argv[1])
