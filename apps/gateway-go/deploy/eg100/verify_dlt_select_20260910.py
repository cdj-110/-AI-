import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import re
import urllib.parse
import urllib.request

base='http://192.168.1.214:8088'
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
form=urllib.parse.urlencode({'username':'admin','password':os.environ['WK_EG100_WEB_PASSWORD'],'next':'/'}).encode()
with client.open(base+'/login',data=form,timeout=20) as response:
    assert '/login' not in response.geturl() and '/change-password' not in response.geturl(), 'Login requires additional verification'
def get(path):
    with client.open(base+path,timeout=20) as response:
        assert response.status==200
        return response.read()
print('Authenticated:',json.loads(get('/api/session')).get('username'))
root=Path('internal/web/frontend/vue-dist')
html=get('/').decode()
local_html=(root/'index.html').read_text(encoding='utf-8')
assets=re.findall(r'(?:src|href)="(/assets/[^\"]+)"',local_html)
for asset in assets:
    assert asset in html, 'Old HTML is still being served'
    assert hashlib.sha256(get(asset)).digest()==hashlib.sha256((root/asset.lstrip('/')).read_bytes()).digest()
print('Deployed HTML/CSS/JS match the verified local build:',assets)
dictionary=json.loads(get('/api/dlt645/dictionary'))
print('Dictionary items:',len(dictionary['items']))
status=json.loads(get('/api/history-storage'))
print('History:',json.dumps({k:status.get(k) for k in ['active','storagePath','storageUUID','storageError','samples','writable','paused','lastWriteAt','lastError','pendingSamples','droppedSamples']},ensure_ascii=False))
assert status['active'] and not status.get('storageError')
assert status['writable'] and not status['paused'] and not status.get('lastError')
assert status['storageUUID']=='a859efdd-6a14-415e-89e5-aa761bc1b46f'
points=json.loads(get('/api/point-status'))
print('Point count:',len(points))
print('DLT645:',json.dumps([{k:p.get(k) for k in ['metric','quality','actualDataType','error']} for p in points if p.get('protocol')=='dlt645'],ensure_ascii=False))
print('Health:',get('/api/healthz').decode())
