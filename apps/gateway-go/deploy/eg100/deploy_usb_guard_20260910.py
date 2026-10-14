"""Fixed-target deployment; credentials come from the caller's environment."""
import hashlib
import json
import os
import paramiko

c = paramiko.SSHClient()
c.load_system_host_keys()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("192.168.1.214", username="root", password=os.environ["WK_EG100_PASSWORD"], timeout=10)
s = c.open_sftp()
s.get_channel().settimeout(30)
with s.open("/userdata/weikong/config/config.local.json") as f:
    raw = f.read()
cfg = json.loads(raw)
assert cfg["historyStorage"]["storagePath"] == "/media/udisk1"
original_hash = hashlib.sha256(raw).hexdigest()
cfg["historyStorage"]["storagePath"] = "/mnt/udisk"
cfg["historyStorage"]["storageUUID"] = "a859efdd-6a14-415e-89e5-aa761bc1b46f"
candidate = "/userdata/weikong/config/config.usb-guard-new.json"
with s.open(candidate, "w") as f:
    s.chmod(candidate, 0o600)
    f.write(json.dumps(cfg, ensure_ascii=False, indent=2) + "\n")

# The board's gesftpserver stalled with paramiko's pipelined put(). Wait for
# each chunk acknowledgement; never launch deployment after a partial upload.
def upload(local, remote):
    count = 0
    with open(local, "rb") as source, s.open(remote, "wb") as target:
        target.set_pipelined(False)
        while True:
            chunk = source.read(32768)
            if not chunk:
                break
            target.write(chunk)
            count += len(chunk)
            if count % (1024 * 1024) == 0:
                print("Uploaded bytes:", count, flush=True)
    assert s.stat(remote).st_size == os.path.getsize(local)

upload("build/iec61850-mms-armv7/weikong-gateway-usb-guard.gz", "/mnt/udisk/.weikong-deploy/usb-guard-20260910.gz")
upload("deploy/eg100/deploy_usb_guard_20260910.sh", "/tmp/weikong-deploy-usb-guard.sh")
s.close()
_, out, err = c.exec_command("sh /tmp/weikong-deploy-usb-guard.sh " + original_hash, timeout=240)
for line in out:
    print(line.rstrip(), flush=True)
print(err.read().decode())
code = out.channel.recv_exit_status()
c.close()
raise SystemExit(code)
