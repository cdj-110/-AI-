"""SQL Server acceptance, reusing the existing authenticated gateway transport."""
import copy
import json
import os
from pathlib import Path
import sys
import time
import mysql_live_20260912 as live

live.BASE=Path('build/sqlserver-integration-20260912')
live.TARGET='eg100-sqlserver-integration'
live.CONTAINER=live.TARGET
live.QUEUE='/mnt/udisk/.remote-storage/'+live.TARGET+'.db'

def sql(query):
    return live.docker('exec','-i','-e','SQLCMDPASSWORD',live.CONTAINER,
        '/opt/mssql-tools18/bin/sqlcmd','-S','localhost','-U','eg100_writer',
        '-d','eg100_integration','-C','-b','-l','10','-h','-1','-W','-w','65535',
        data='SET NOCOUNT ON;\n'+query+'\nGO\n').strip()

def counts():
    return sql('SELECT COUNT(*),COUNT(DISTINCT record_id) FROM dbo.eg100_samples; SELECT COUNT(*) FROM (SELECT DISTINCT device_key,metric FROM dbo.eg100_samples) d; SELECT quality,COUNT(*) FROM dbo.eg100_samples GROUP BY quality;')

live.sql=sql
live.counts=counts

def main(mode):
    if mode in ('inspect','resilience'):
        return live.run(mode)
    g=live.Gateway()
    try:
        g.login()
        if mode=='configure':
            cfg=g.api('/api/remote-storage')
            assert not any(t['id']==live.TARGET for t in cfg['config']['targets']), 'Target already exists'
            assert len(cfg['config']['targets'])<4
            pg=next(t for t in cfg['config']['targets'] if t['id']=='eg100-pg-integration')
            candidate=copy.deepcopy(cfg['config'])
            target={'id':live.TARGET,'name':'EG100 SQL Server 实库联调','enabled':True,
                'driver':'sqlserver','host':'192.168.1.141','port':1433,'database':'eg100_integration',
                'schema':'dbo','table':'eg100_samples','username':'eg100_writer',
                'password':os.environ['REMOTE_TEST_PASSWORD'],'tlsMode':'disabled',
                'createTable':False,'intervalSeconds':5,'batchSize':100,'timeoutSeconds':10,
                'points':copy.deepcopy(pg['points'])}
            # Read-only preflight must succeed before adding a live task.
            live.save('connection-test-before-save',g.api('/api/remote-storage/test','POST',target))
            candidate['targets'].append(target)
            live.save('config-before-add',cfg)
            # Keep a masked-independent digest to verify all other configuration after save.
            with g.ssh.open_sftp() as s:
                with s.open('/userdata/weikong/config/config.local.json') as f: original=json.load(f)
            result=g.api('/api/remote-storage','PUT',candidate,headers={'If-Match':cfg['revision']})
            assert result['config']['targets'][:-1]==cfg['config']['targets']
            with g.ssh.open_sftp() as s:
                with s.open('/userdata/weikong/config/config.local.json') as f: current=json.load(f)
            changed=[k for k in set(original)|set(current) if original.get(k)!=current.get(k)]
            assert changed==['remoteStorage']
            assert current['remoteStorage']['targets'][:-1]==original['remoteStorage']['targets']
            live.save('config-scope',{'changedTopLevelKeys':changed,'existingTargetsUnchanged':True})
            live.save('configured',result)
            for _ in range(40):
                status=g.status()
                if status.get('sent',0)>=16 and status.get('pending')==0: break
                time.sleep(2)
            live.save('normal-status',status)
            assert status.get('sent',0)>=16 and status.get('pending')==0 and status.get('dropped',0)==0
            live.save('normal-database',counts())
        elif mode=='verify':
            status=g.api('/api/remote-storage/status')
            live.save('final-status',status)
            assert all(t.get('connected') and t.get('dropped',0)==0 for t in status['statuses'])
            live.save('final-health',g.api('/api/healthz'))
            live.save('final-database',counts())
            live.save('writer-permissions',sql("SELECT IS_SRVROLEMEMBER('sysadmin'),IS_MEMBER('db_owner'),HAS_PERMS_BY_NAME('dbo.eg100_samples','OBJECT','SELECT'),HAS_PERMS_BY_NAME('dbo.eg100_samples','OBJECT','INSERT'),HAS_PERMS_BY_NAME('dbo.eg100_samples','OBJECT','DELETE'),HAS_PERMS_BY_NAME('dbo.eg100_samples','OBJECT','ALTER');"))
            live.save('final-version',g.shell('sha256sum /userdata/weikong/bin/weikong-gateway'))
        else: raise ValueError(mode)
    finally: g.ssh.close()

if __name__=='__main__': main(sys.argv[1])
