"""Read runtime status and test rejection only; never install an activation."""
import base64
import hashlib
import json
import time
from pathlib import Path
from mysql_live_20260912 import Gateway

ROOT = Path(__file__).resolve().parents[4]
CONFIG = '/userdata/weikong/config/config.local.json'

def main():
    g = Gateway()
    try:
        g.login()
        status = g.api('/api/license/status')
        assert status['license']['restricted'] and not status['license']['valid']
        assert status['hardwareIdentity']['available']
        before = g.shell(f'sha256sum {CONFIG}').split()[0]
        fixture = {'enabled':True,'licenseVersion':2,'keyId':'untrusted-test-only','licenseSequence':1,'hardwareId':status['hardwareIdentity']['id'],'chipUsid':status['secureChipIdentity'].get('usid',''),'sn':status['gatewayKey'],'deviceSecret':'CHANGE_ME_LOCAL_ONLY','broker':'tcp://192.168.1.141:1883','cloudApi':'http://192.168.1.141:3100','remoteAccessEndpoint':'ws://192.168.1.141:3100/api/remote-access/tunnel','issuedAt':'2026-09-16T00:00:00.000Z','expiresAt':'2030-09-16T00:00:00.000Z','features':[],'nonce':'test-only','signature':base64.b64encode(bytes(64)).decode()}
        checks = []
        for name, content in [('unknown-key', fixture), ('unsigned', {k:v for k,v in fixture.items() if k!='signature'})]:
            try:
                g.api('/api/activation','POST',{'content':json.dumps(content)})
            except RuntimeError as error:
                assert 'HTTP 400' in str(error), 'Unexpected activation validation response'
                checks.append({'case':name,'rejected':True})
            else:
                raise AssertionError('Invalid test authorization accepted')
        assert g.shell(f'sha256sum {CONFIG}').split()[0] == before
        assert g.api('/api/license/status')['license']['restricted']
        health = g.api('/api/healthz')
        assert health['healthy'] and not health.get('lastCollectAt')
        result = {'status':status,'negativeChecks':checks,'invalidPreflightDidNotChangeConfig':True,'businessCollectionStopped':True}
        out = ROOT / 'artifacts/license-unactivated-20260916'
        out.mkdir(parents=True, exist_ok=True)
        (out/'runtime-verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf8')
        print(json.dumps(result, ensure_ascii=False), flush=True)
    finally:
        g.ssh.close()

if __name__ == '__main__':
    main()
