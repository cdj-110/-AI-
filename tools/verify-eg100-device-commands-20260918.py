"""Read-only gateway login and health acceptance after command deployment."""
import hashlib
import json
import os
import sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway

out=ROOT/os.environ.get('WK_EG100_DEPLOY_OUTPUT','artifacts/eg100-cloud-device-commands-20260918')
deployment=json.loads((out/'deployment.json').read_text(encoding='utf8'))
gateway=Gateway()
try:
    gateway.login()
    assert gateway.api('/api/session')['username']=='admin'
    assert gateway.api('/api/healthz')['healthy']
    assert gateway.api('/api/readyz')['ready']
    with gateway.ssh.open_sftp() as sftp:
        with sftp.open('/userdata/weikong/bin/weikong-gateway','rb') as source:
            assert hashlib.sha256(source.read()).hexdigest()==deployment['newBinarySha256']
    result={'verifiedAt':__import__('datetime').datetime.now(__import__('datetime').timezone.utc).isoformat(),'loginWorks':True,'healthy':True,'ready':True,'binaryMatchesVerifiedBuild':True,'configUnchanged':deployment['configUnchanged'],'authorizationUnchanged':deployment['authorizationUnchanged'],'accountsUnchanged':deployment['accountsUnchanged'],'networkUnchanged':deployment['networkUnchanged'],'nativeLibraryUnchanged':deployment['nativeLibraryUnchanged'],'pointCount':deployment['pointCount'],'noPhysicalPointWrites':True}
    (out/'verification.json').write_text(json.dumps(result,indent=2),encoding='utf8')
    print(json.dumps(result),flush=True)
finally:
    if gateway.cookies:
        try: gateway.api('/logout','POST')
        except Exception: pass
    gateway.ssh.close()
