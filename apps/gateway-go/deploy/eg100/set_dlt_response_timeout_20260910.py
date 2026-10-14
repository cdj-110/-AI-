"""Change only device-006 response timeout with optimistic concurrency."""
import copy
import http.cookiejar
import json
import os
from pathlib import Path
import urllib.parse
import urllib.request

base='http://192.168.1.214:8088'
client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
client.open(base+'/login',data=urllib.parse.urlencode({'username':'admin','password':os.environ['WK_EG100_WEB_PASSWORD'],'next':'/'}).encode(),timeout=20).read()
response=client.open(base+'/api/config',timeout=20)
revision=response.headers['ETag']
original=json.loads(json.load(response)['content'])
updated=copy.deepcopy(original)
devices=[d for d in updated['devices'] if d.get('deviceKey')=='device-006']
assert len(devices)==1 and devices[0]['protocol']=='dlt645' and devices[0]['address']=='/dev/ttyS2'
assert devices[0]['dlt645']['meterAddress']=='010000000000'
targets=[devices[0]]
targets += [p for p in updated['points'] if p.get('deviceKey')=='device-006' and p.get('protocol')=='dlt645']
assert len(targets)==6, 'Unexpected target point count; nothing changed'
# Serialized device points may include explicit per-point options as well.
targets += [p for p in devices[0].get('points',[]) if 'responseTimeoutMilliseconds' in (p.get('dlt645') or {})]
for target in targets:
    options=target['dlt645']
    assert options['responseTimeoutMilliseconds']==1500, 'Timeout changed externally; nothing changed'
    options['responseTimeoutMilliseconds']=3000
folder=Path('build/dlt-response-backup-20260910')
assert (folder/'config.original.json').exists(), 'Full deployment backup required first'
snapshot=folder/'config.before-timeout.redacted.json'
assert not snapshot.exists(), 'Do not overwrite an earlier backup'
snapshot.write_text(json.dumps(original,ensure_ascii=False,indent=2),encoding='utf-8')
body=json.dumps({'content':json.dumps(updated,ensure_ascii=False)}).encode()
request=urllib.request.Request(base+'/api/config',data=body,method='PUT',headers={'Content-Type':'application/json','If-Match':revision})
with client.open(request,timeout=60) as response:
    result=json.load(response)
assert result.get('ok'),result
actual=json.loads(json.load(client.open(base+'/api/config',timeout=20))['content'])
def differences(a,b,path=''):
    if type(a)!=type(b):return [path]
    if isinstance(a,dict):
        keys=set(a)|set(b)
        return [p for key in sorted(keys) for p in ([path+'/'+key] if key not in a or key not in b else differences(a[key],b[key],path+'/'+key))]
    if isinstance(a,list):
        if len(a)!=len(b):return [path]
        return [p for i,(x,y) in enumerate(zip(a,b)) for p in differences(x,y,path+'/'+str(i))]
    return [] if a==b else [path]
unexpected=differences(updated,actual)
assert not unexpected, 'Unexpected normalized config differences: '+str(unexpected)
print('Applied and read back: device-006 response timeout 1500 -> 3000 ms; device and point settings consistent.')
print('Only changed:',differences(original,actual))
print('API result:',result)
