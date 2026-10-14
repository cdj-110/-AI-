"""Fixed-target release with verified off-device and internal-flash rollback."""
import gzip
import hashlib
import json
import os
from pathlib import Path
import paramiko

binary=Path('build/iec61850-mms-armv7/weikong-gateway-dlt-select')
payload=binary.read_bytes()
expected=hashlib.sha256(payload).hexdigest()
old_expected='51301bb7fbad2e2d8e6841aeff7fa5b94d6ac598de95f1e376decba451d54ae6'
archive=binary.with_suffix('.gz')
archive.write_bytes(gzip.compress(payload,mtime=0))
local=Path('build/dlt-select-backup-20260910')
local.mkdir(exist_ok=True)
c=paramiko.SSHClient()
c.load_system_host_keys()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.1.214',username='root',password=os.environ['WK_EG100_PASSWORD'],timeout=10)

def run(command):
    _,out,err=c.exec_command(command,timeout=240)
    result=out.read().decode()
    error=err.read().decode()
    if out.channel.recv_exit_status()!=0:
        raise RuntimeError(result+'\n'+error)
    return result

try:
    assert run('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0]==old_expected
    s=c.open_sftp()
    s.get_channel().settimeout(30)
    with s.open('/userdata/weikong/config/config.local.json') as f: raw=f.read()
    config_expected=hashlib.sha256(raw).hexdigest()
    cfg=json.loads(raw)
    assert cfg['historyStorage']['storageUUID']=='a859efdd-6a14-415e-89e5-aa761bc1b46f'
    assert cfg['historyStorage']['storagePath']=='/mnt/udisk'
    (local/'config.original.json').write_bytes(raw)
    print('Config and history UUID verified; no config changes.',flush=True)
    backup='/userdata/weikong/backups/dlt-select-20260910'
    run(f'test ! -e {backup} && mkdir -m 700 {backup} && gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && chmod 600 {backup}/* && gzip -t {backup}/weikong-gateway.original.gz')
    s.get(backup+'/weikong-gateway.original.gz',str(local/'weikong-gateway.original.gz'),max_concurrent_prefetch_requests=8)
    assert hashlib.sha256(gzip.decompress((local/'weikong-gateway.original.gz').read_bytes())).hexdigest()==old_expected
    print('Internal and off-device rollback copies verified.',flush=True)
    def upload(source,remote):
        with open(source,'rb') as f,s.open(remote,'wb') as target:
            target.set_pipelined(False)
            count=0
            while chunk:=f.read(32768):
                target.write(chunk)
                count+=len(chunk)
                if count%(1024*1024)==0:print('Uploaded:',count,flush=True)
        assert s.stat(remote).st_size==os.path.getsize(source)
    upload(archive,'/mnt/udisk/.weikong-deploy/dlt-select-20260910.gz')
    upload('deploy/eg100/deploy_dlt_select_20260910.sh','/tmp/weikong-deploy-dlt-select.sh')
    s.close()
    print('Package uploaded; applying verified release.',flush=True)
    print(run(f'sh /tmp/weikong-deploy-dlt-select.sh {expected} {old_expected} {config_expected}'),flush=True)
    print('New SHA256:',expected,flush=True)
finally:
    c.close()
