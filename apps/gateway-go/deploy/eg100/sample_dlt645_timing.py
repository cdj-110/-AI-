"""Read-only EG100 trace capture; no serial requests or configuration changes."""
import argparse
import datetime
import http.cookiejar
import json
import os
from pathlib import Path
import time
import urllib.parse
import urllib.request

parser=argparse.ArgumentParser()
parser.add_argument('--seconds',type=int,default=90)
parser.add_argument('--fresh',action='store_true')
args=parser.parse_args()
assert 10 <= args.seconds <= 600
base = 'http://192.168.1.214:8088'
client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
client.open(base+'/login', data=urllib.parse.urlencode({'username':'admin','password':os.environ['WK_EG100_WEB_PASSWORD'],'next':'/'}).encode(), timeout=20).read()
frames = {}
end = time.monotonic()+args.seconds
baseline=None
while True:
    data = json.load(client.open(base+'/api/packets?protocol=dlt645&limit=1000', timeout=20))
    if baseline is None:
        baseline=max((f['id'] for f in data['frames']),default=0) if args.fresh else 0
    for frame in data['frames']:
        if frame['id'] > baseline:
            frames[frame['id']] = frame
    print('Captured frames:',len(frames),flush=True)
    if time.monotonic() >= end:
        break
    time.sleep(10)
rows = sorted(frames.values(),key=lambda f:f['id'])
def wire_di(frame):
    raw = bytes.fromhex(frame.get('hex','')).lstrip(b'\xfe')
    if len(raw)<16 or raw[0]!=0x68 or raw[7]!=0x68 or raw[8]&0x1f not in (1,2,17,18) or raw[8]&0x40:
        return None
    size = 4 if raw[8]&0x1f in (17,18) else 2
    return bytes((x-0x33)&255 for x in raw[10:10+size])[::-1].hex().upper()
last = None
delays, anomalies = [], []
for frame in rows:
    if frame['direction']=='tx':
        last = frame
        continue
    if not last:
        continue
    delay = (datetime.datetime.fromisoformat(frame['time'])-datetime.datetime.fromisoformat(last['time'])).total_seconds()*1000
    if frame.get('error') or wire_di(frame)!=wire_di(last):
        anomalies.append({'tx':last,'rx':frame,'milliseconds':round(delay,1),'requestedDI':wire_di(last),'receivedDI':wire_di(frame)})
    else:
        delays.append(delay)
delays.sort()
summary = {'first':rows[0]['time'] if rows else None,'last':rows[-1]['time'] if rows else None,'frames':len(rows),'matched':len(delays),'anomalies':len(anomalies),'minMs':round(delays[0],1) if delays else None,'p95Ms':round(delays[int((len(delays)-1)*.95)],1) if delays else None,'maxMs':round(delays[-1],1) if delays else None}
folder=Path('build/dlt645-response-diagnosis-20260910');folder.mkdir(exist_ok=True)
path=folder/('capture-'+datetime.datetime.now().strftime('%H%M%S')+'.json')
path.write_text(json.dumps({'summary':summary,'anomalies':anomalies,'frames':rows},ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(summary,ensure_ascii=False),flush=True)
for row in anomalies:
    print(json.dumps({'txTime':row['tx']['time'],'rxTime':row['rx']['time'],'txHex':row['tx'].get('hex'),'rxHex':row['rx'].get('hex'),'requestedDI':row['requestedDI'],'receivedDI':row['receivedDI'],'milliseconds':row['milliseconds'],'error':row['rx'].get('error')},ensure_ascii=False),flush=True)
print('Saved:',path,flush=True)
