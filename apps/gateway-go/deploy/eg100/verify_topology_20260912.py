"""Read-only live EG100 release, overview and database-progress verification."""
import hashlib
import http.client
import json
import time
from pathlib import Path
from mysql_live_20260912 import Gateway

base = Path('build/topology-deployment-20260912-1642')
manifest = json.loads((base / 'manifest.json').read_text(encoding='utf-8'))
g = Gateway()

def raw(path):
    c = http.client.HTTPConnection('127.0.0.1', 8088, timeout=25)
    c.sock = g.ssh.get_transport().open_channel('direct-tcpip', ('127.0.0.1',8088), ('127.0.0.1',0), timeout=25)
    c.sock.settimeout(25)
    try:
        c.request('GET', path, headers={'Cookie': '; '.join(k+'='+v.value for k,v in g.cookies.items())})
        response = c.getresponse()
        assert response.status == 200, (path, response.status)
        return response.read()
    finally:
        c.close()

try:
    g.login()
    hashes = g.shell('sha256sum /userdata/weikong/bin/weikong-gateway /userdata/weikong/config/config.local.json').splitlines()
    assert hashes[0].split()[0] == manifest['new']
    assert hashes[1].split()[0] == manifest['config']
    health1 = g.api('/api/healthz')
    assert health1['healthy'] and health1['pointCount'] == 52
    assert g.api('/api/readyz')['ready']
    overview = g.api('/api/status/overview')
    assert sum(d['total'] for d in overview['devices']) == 52
    assert len(overview['issues']) <= 5
    index = raw('/').decode()
    assets = ['index-CVmGG6CE.js', 'index-_lBtkwW6.css']
    for asset in assets:
        assert asset in index
        remote = raw('/assets/' + asset)
        local = Path('internal/web/frontend/vue-dist/assets', asset).read_bytes()
        assert hashlib.sha256(remote).digest() == hashlib.sha256(local).digest(), asset
    statuses1 = g.api('/api/remote-storage/status')['statuses']
    first = {s['id']: s for s in statuses1}
    expected = {'eg100-pg-integration', 'eg100-mysql-integration', 'eg100-sqlserver-integration'}
    # Sampling may catch the legitimate just-enqueued 8-point batch. Verify each
    # queue drains across several cycles, rather than require one atomic zero.
    drained = set()
    for _ in range(10):
        time.sleep(2)
        statuses2 = g.api('/api/remote-storage/status')['statuses']
        drained.update(s['id'] for s in statuses2 if s['pending'] == 0)
    assert set(first) == expected and {s['id'] for s in statuses2} == expected
    assert drained == expected, ('queues did not drain', drained, statuses2)
    for s in statuses2:
        assert s['connected'] and s['cacheReady'] and s['dropped'] == 0, s
        assert s['sent'] > first[s['id']]['sent'], s
    health2 = g.api('/api/healthz')
    assert health2['healthy'] and health2['lastCollectAt'] != health1['lastCollectAt']
    platform = g.shell("date; df -Pk /userdata /mnt/udisk; grep /mnt/udisk /proc/mounts; test -z \"$(pidof vanxoak_pilot 2>/dev/null || true)\"; /etc/init.d/S99weikong-gateway status")
    result = {'verified': True, 'health': health2, 'configUnchanged': True, 'assets': assets, 'overview': overview, 'databases': statuses2, 'queuesDrained': sorted(drained), 'platform': platform}
    (base/'verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps(result, ensure_ascii=False, indent=2))
finally:
    g.ssh.close()
