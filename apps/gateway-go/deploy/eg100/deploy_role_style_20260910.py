"""Fixed EG100 deployment with off-device and USB rollback; no configuration edits."""
import gzip
import hashlib
import os
from pathlib import Path
import paramiko

binary = Path('build/iec61850-mms-armv7/weikong-gateway-role-style')
expected = '0e47aba2639b3e49094a83bd719222d03dc0e9326a8587f522b1e1810ebe4be9'
old_expected = '315ed71f841cb1b12c046e261c88148ea93c23e41a75518df73d38586f51e094'
payload = binary.read_bytes()
assert hashlib.sha256(payload).hexdigest() == expected
archive = binary.with_suffix('.gz')
archive.write_bytes(gzip.compress(payload, mtime=0))
local = Path('build/role-style-backup-20260910')
local.mkdir(exist_ok=True)
backup = '/mnt/udisk/.weikong-deploy/role-style-backup-20260910'
c = paramiko.SSHClient()
c.load_system_host_keys()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.1.214', username='root', password=os.environ['WK_EG100_PASSWORD'], timeout=10)

def run(command):
    _, out, err = c.exec_command(command, timeout=240)
    result, error = out.read().decode(), err.read().decode()
    if out.channel.recv_exit_status() != 0:
        raise RuntimeError(result + '\n' + error)
    return result

try:
    assert run('sha256sum /userdata/weikong/bin/weikong-gateway').split()[0] == old_expected
    run("grep -q '^/dev/sda4 /mnt/udisk ext4 rw' /proc/mounts && blkid /dev/sda4 | grep -q 'a859efdd-6a14-415e-89e5-aa761bc1b46f'")
    s = c.open_sftp()
    s.get_channel().settimeout(30)
    with s.open('/userdata/weikong/config/config.local.json') as f:
        config = f.read()
    config_hash = hashlib.sha256(config).hexdigest()
    (local / 'config.original.json').write_bytes(config)
    run(f'test ! -e {backup} && mkdir -m 700 {backup} && gzip -c /userdata/weikong/bin/weikong-gateway > {backup}/weikong-gateway.original.gz && cp -p /userdata/weikong/config/config.local.json {backup}/config.original.json && chmod 600 {backup}/* && sync')
    s.get(backup + '/weikong-gateway.original.gz', str(local / 'weikong-gateway.original.gz'), max_concurrent_prefetch_requests=8)
    assert hashlib.sha256(gzip.decompress((local / 'weikong-gateway.original.gz').read_bytes())).hexdigest() == old_expected
    print('USB and off-device rollback verified; current config preserved.', flush=True)
    def upload(source, remote):
        with open(source, 'rb') as f, s.open(remote, 'wb') as target:
            target.set_pipelined(False)
            count = 0
            while chunk := f.read(32768):
                target.write(chunk)
                count += len(chunk)
                if count % (1024 * 1024) == 0:
                    print('Uploaded:', count, flush=True)
        assert s.stat(remote).st_size == os.path.getsize(source)
    upload(archive, '/mnt/udisk/.weikong-deploy/role-style-20260910.gz')
    upload('deploy/eg100/deploy_role_style_20260910.sh', '/tmp/weikong-deploy-role-style.sh')
    s.close()
    print('Applying checked package.', flush=True)
    print(run(f'sh /tmp/weikong-deploy-role-style.sh {expected} {old_expected} {config_hash}'), flush=True)
finally:
    c.close()
