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
backup=json.loads(Path('build/alarm-sound-switch-backup-20260912/config.original.json').read_text(encoding='utf-8'))
original_rules=backup.get('alarms',{}).get('rules',[])
assert {r['ruleId']:r.get('allowManualRecovery',False) for r in rules} == {r['ruleId']:r.get('allowManualRecovery',False) for r in original_rules}
print('Alarm recovery policies preserved:', len(rules))
assert {r['ruleId']:r.get('popupEnabled',True) for r in rules} == {r['ruleId']:r.get('popupEnabled',True) for r in original_rules}
policies={r['ruleId']:r.get('popupEnabled',True) for r in rules}
for event in alarms['active']:
    parent=event['ruleId'] if event['ruleId'] in policies else event['ruleId'].rsplit('::',1)[0]
    assert isinstance(event.get('popupEnabled'),bool)
    assert event['popupEnabled']==policies[parent]
print('Per-rule popup policies match active event API:', len(alarms['active']))
rule_index={r['ruleId']:r for r in rules}
groups={}
for event in alarms['active']:
    parent=event.get('parentRuleId') or event['ruleId']
    rule=rule_index[parent]
    if rule.get('limits'):
        assert event['parentRuleId']==rule['ruleId']
        assert event['parentRuleName']==rule['name']
        assert event.get('limitKey') in rule['limits'] and event.get('limitName')
        assert event['ruleId']==parent+'::'+event['limitKey']
    key=(parent,event['deviceKey'],event['metric'])
    groups.setdefault(key,[]).append(event.get('limitKey') or 'single')
print('Verified active notification groups:',json.dumps(list(groups.values()),ensure_ascii=True))
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
with sqlite3.connect('build/alarm-sound-switch-backup-20260912/alarm-events.original.db') as db:
    assert history['total'] >= db.execute('SELECT count(*) FROM alarm_history').fetchone()[0]
dictionary=json.loads(get('/api/dlt645/dictionary'))
print('Dictionary items:',len(dictionary['items']))
points=json.loads(get('/api/point-status'))
print('Point count:',len(points))
print('DLT645:',json.dumps([{k:p.get(k) for k in ['metric','quality','actualDataType','error']} for p in points if p.get('protocol')=='dlt645'],ensure_ascii=True))
print('Health:',get('/api/healthz').decode())
import time
previous=[]
for iteration in range(12):
    sample=json.loads(get('/api/alarms?activeOnly=1'))['active']
    ids=[e['eventId'] for e in sample]
    shared=set(previous)&set(ids)
    assert [i for i in previous if i in shared]==[i for i in ids if i in shared], 'Existing alarms changed order'
    for a,b in zip(sample,sample[1:]):
        if a['occurredAt']==b['occurredAt']:
            assert (a['ruleId'],a['eventId'])<=(b['ruleId'],b['eventId'])
    previous=ids
    if iteration<11: time.sleep(1)
print('Active ordering stable across 12 live snapshots:', [e['ruleId'] for e in sample])
print('Final health:',get('/api/healthz').decode())


for pattern in ('Buzzer1-*.wav','Buzzer4-*.wav','Alarm2-*.WAV','Siren1-*.wav'):
    files=list((root/'assets').glob(pattern))
    assert len(files)==1
    file=files[0]
    assert hashlib.sha256(get('/assets/'+file.name)).digest()==hashlib.sha256(file.read_bytes()).digest()
    print('WAV verified:',file.name)
