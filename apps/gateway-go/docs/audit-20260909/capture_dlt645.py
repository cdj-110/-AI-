"""Read existing authenticated gateway packet records; never trigger collection."""
import http.cookiejar
import json
import os
import time
import urllib.parse
import urllib.request
from datetime import datetime

base = 'http://192.168.1.214:8088'
jar = http.cookiejar.CookieJar()
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))
body = urllib.parse.urlencode({'username': 'admin', 'password': os.environ['WK_WEB_PASSWORD'], 'next': '/api/packets?deviceKey=device-006&protocol=dlt645&limit=1000'}).encode()
response = client.open(urllib.request.Request(base+'/login', data=body), timeout=10)
if '/login' in response.url or '/change-password' in response.url:
    raise RuntimeError('Login requires additional verification: '+response.url)
seen = set()
frames = []
end = time.monotonic()+75
while True:
    response = client.open(base+'/api/packets?deviceKey=device-006&protocol=dlt645&limit=1000', timeout=10)
    payload = json.load(response)
    for frame in sorted(payload.get('frames', []), key=lambda item: item['id']):
        if frame['id'] not in seen:
            seen.add(frame['id'])
            frames.append(frame)
    if time.monotonic() >= end:
        break
    time.sleep(2)
print('CAPTURE_DONE frames='+str(len(seen)), flush=True)
def stamp(frame):
    return datetime.fromisoformat(frame['time']).timestamp()
pending = None
pairs = []
unmatched = []
for frame in frames:
    if frame['direction'] == 'tx':
        if pending: unmatched.append(pending)
        pending = frame
    elif pending:
        pairs.append({'tx': pending['time'], 'rx': frame['time'], 'di': pending.get('itemId'), 'elapsedMs': round((stamp(frame)-stamp(pending))*1000,3), 'error': frame.get('error'), 'request': pending.get('hex'), 'response': frame.get('hex')})
        pending = None
print(json.dumps({'first':frames[0]['time'] if frames else None,'last':frames[-1]['time'] if frames else None,'pairs':len(pairs),'slowPairs':[p for p in pairs if p['elapsedMs']>800 or p['error']],'unmatched':unmatched,'minMs':min((p['elapsedMs'] for p in pairs),default=0),'maxMs':max((p['elapsedMs'] for p in pairs),default=0)}, ensure_ascii=False), flush=True)
