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
with client.open(base+'/login',data=form,timeout=60) as response:
    assert '/login' not in response.geturl() and '/change-password' not in response.geturl(), 'Login requires additional verification'
def get(path):
    with client.open(base+path,timeout=60) as response:
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
alarms=json.loads(get('/api/alarms?activeOnly=1'))
rules=json.loads(get('/api/alarm-rules'))['rules']
backup=json.loads(Path('build/alarm-limits-backup-20260910/config.original.json').read_text(encoding='utf-8'))
original_rules=backup.get('alarms',{}).get('rules',[])
assert {r['ruleId']:r.get('allowManualRecovery',False) for r in rules} == {r['ruleId']:r.get('allowManualRecovery',False) for r in original_rules}
print('Alarm recovery policies preserved:', len(rules))
settings=json.loads(get('/api/alarms/settings'))
expected_duration=backup.get('alarms',{}).get('popupDurationSeconds',10)
assert settings['popupDurationSeconds']==expected_duration
assert alarms['popupDurationSeconds']==expected_duration
print('Popup settings and active snapshot agree:', settings)
print('Active alarms:', json.dumps([{k:e.get(k) for k in ['ruleId','value','unit','recoveryStatus','recoveryHint','allowManualRecovery']} for e in alarms['active']], ensure_ascii=True))
assert all(e.get('recoveryStatus') and e.get('recoveryHint') for e in alarms['active'])
history=json.loads(get('/api/alarms/history?page=1&pageSize=1'))
print('Preserved alarm history rows:',history['total'])
import sqlite3
with sqlite3.connect('build/alarm-limits-backup-20260910/alarm-events.original.db') as db:
    assert history['total'] >= db.execute('SELECT count(*) FROM alarm_history').fetchone()[0]
dictionary=json.loads(get('/api/dlt645/dictionary'))
print('Dictionary items:',len(dictionary['items']))
points=json.loads(get('/api/point-status'))
print('Point count:',len(points))
print('DLT645:',json.dumps([{k:p.get(k) for k in ['metric','quality','actualDataType','error']} for p in points if p.get('protocol')=='dlt645'],ensure_ascii=True))
print('Health:',get('/api/healthz').decode())
