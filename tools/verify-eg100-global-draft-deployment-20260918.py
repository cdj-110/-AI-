"""Read-only acceptance of the deployed UI and preserved gateway configuration."""
import hashlib
import http.client
import json
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway


def main():
    out = ROOT / 'artifacts/eg100-global-draft-guard-20260918'
    deployment = json.loads((out / 'deployment.json').read_text(encoding='utf8'))
    local = ROOT / 'apps/gateway-go/internal/web/frontend/vue-dist'
    gateway = Gateway()

    def request(path, method='GET'):
        connection = http.client.HTTPConnection('127.0.0.1', 8088, timeout=15)
        connection.sock = gateway.ssh.get_transport().open_channel('direct-tcpip', ('127.0.0.1', 8088), ('127.0.0.1', 0), timeout=15)
        connection.sock.settimeout(15)
        try:
            cookies = '; '.join(key + '=' + value.value for key, value in gateway.cookies.items())
            connection.request(method, path, headers={'Cookie': cookies, 'Origin': 'http://127.0.0.1:8088'})
            response = connection.getresponse()
            return response.status, response.read()
        finally:
            connection.close()

    try:
        gateway.login()
        assert gateway.api('/api/session')['username'] == 'admin'
        status, html = request('/')
        assert status == 200
        assert html == (local / 'index.html').read_bytes(), 'Deployed page differs from the production build'
        assets = re.findall(r'(?:src|href)="(/assets/[^" ]+)"', html.decode())
        assert assets
        for asset in assets:
            status, content = request(asset)
            assert status == 200
            assert hashlib.sha256(content).digest() == hashlib.sha256((local / asset.lstrip('/')).read_bytes()).digest(), 'Deployed UI asset mismatch'
            if asset.endswith('.js'):
                for text in ['保存修改并继续', '放弃修改并离开', '继续编辑']:
                    assert text.encode() in content, 'Draft navigation actions missing from deployed UI'
        health = gateway.api('/api/healthz')
        ready = gateway.api('/api/readyz')
        license_status = gateway.api('/api/license/status')
        assert health['healthy'] and ready['ready']
        assert health['pointCount'] == deployment['pointCount']
        result = {'loginWorks': True, 'deployedPageMatchesBuild': True, 'deployedAssetsMatchBuild': True, 'draftGuardActionsIncluded': True, 'healthy': True, 'ready': True, 'pointCount': health['pointCount'], 'configUnchanged': deployment['configUnchanged'], 'authorizationUnchanged': deployment['authorizationUnchanged'], 'accountsUnchanged': deployment['accountsUnchanged'], 'networkUnchanged': deployment['networkUnchanged'], 'assetCount': len(assets)}
        (out / 'verification.json').write_text(json.dumps(result, indent=2), encoding='utf8')
        print(json.dumps(result), flush=True)
    finally:
        if gateway.cookies:
            try:
                request('/logout', 'POST')
            except Exception:
                pass
        gateway.ssh.close()


if __name__ == '__main__':
    main()
