"""Fixed, backed up EG100 MySQL-only code update; leaves configuration unchanged."""
import gzip
import hashlib
from pathlib import Path
from mysql_live_20260912 import Gateway, BASE, save

NEW='057b748881ac1115536eec35c393c6a4deb1b6345659b5829103ced1e6f367f6'
OLD='493ebd9f346532f00981a1f602358886cc3c3293bdca9725def2e0d8c4ecb061'
backup='/mnt/udisk/.weikong-deploy/mysql-fix-backup-20260912'
binary=Path('build/iec61850-mms-armv7/weikong-gateway-mysql-fix')
payload=binary.read_bytes()
assert hashlib.sha256(payload).hexdigest()==NEW
archive=BASE/'gateway-mysql-fix.gz'
archive.write_bytes(gzip.compress(payload,mtime=0))
g=Gateway()
try:
    assert g.shell('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0]==OLD
    g.shell("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'")
    with g.ssh.open_sftp() as s:
        free=int(g.shell('df -Pk /userdata').splitlines()[-1].split()[3])*1024
        assert free+s.stat('/userdata/weikong/bin/weikong-gateway').st_size-len(payload)>2*1024*1024
        with s.open('/userdata/weikong/config/config.local.json') as f: config=f.read()
        config_hash=hashlib.sha256(config).hexdigest()
        # Original config includes secrets, so keep it only inside root-only USB backup.
        g.shell(f'test ! -e {backup} && mkdir -m 700 {backup} && gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && chmod 600 {backup}/* && sync')
        assert g.shell(f'sha256sum {backup}/config.original.json').split()[0]==config_hash
        local=BASE/'gateway-before-mysql.gz'
        s.get(backup+'/weikong-gateway.original.gz',str(local),max_concurrent_prefetch_requests=8)
        assert hashlib.sha256(gzip.decompress(local.read_bytes())).hexdigest()==OLD
        def upload(local,remote):
            with open(local,'rb') as source, s.open(remote,'wb') as target:
                target.set_pipelined(False)
                while chunk:=source.read(32768): target.write(chunk)
        upload(archive,'/mnt/udisk/.weikong-deploy/mysql-fix-20260912.gz')
        upload('deploy/eg100/deploy_mysql_fix_20260912.sh','/tmp/weikong-deploy-mysql-fix.sh')
    print('Original binary rollback copies verified; package uploaded.',flush=True)
    save('deployment',g.shell(f'sh /tmp/weikong-deploy-mysql-fix.sh {NEW} {OLD} {config_hash}',timeout=240))
    save('deployment-hashes',{'new':NEW,'old':OLD,'config':config_hash,'backup':backup})
finally: g.ssh.close()
