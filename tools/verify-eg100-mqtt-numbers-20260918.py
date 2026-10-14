"""Read-only capture of fresh EG100 MQTT reports. Never saves connection secrets."""
import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'apps/gateway-go/deploy/eg100'))
from mysql_live_20260912 import Gateway

gateway = Gateway()
try:
    with gateway.ssh.open_sftp() as sftp:
        with sftp.open('/userdata/weikong/config/config.local.json', 'rb') as source:
            cfg = json.loads(source.read())
        activation = cfg.get('activation', {})
        license_path = activation.get('file') or '/userdata/weikong/config/activation.local.json'
        if not license_path.startswith('/'):
            license_path = '/userdata/weikong/config/' + license_path
        with sftp.open(license_path, 'rb') as source:
            license_data = json.loads(source.read())
    keys = list(filter(None, [cfg.get('gatewayKey'), activation.get('sn'), license_data.get('sn')]))
    assert keys
    # The existing ingest ACL grants only this exact wildcard subscription.
    # Filter in memory to this gateway; do not broaden permissions or attempt
    # to subscribe to third-party manual MQTT topics with platform credentials.
    topics = ['weikong/gateways/+/children/+/telemetry']
    boolean_points = []
    def find_boolean_points(node, device_key=None):
        if isinstance(node, dict):
            device_key = node.get('deviceKey') or device_key
            if node.get('dataType') == 'bool' and node.get('metric') and device_key:
                boolean_points.append(f"{device_key}::{node['metric']}")
            for item in node.values():
                find_boolean_points(item, device_key)
        elif isinstance(node, list):
            for item in node:
                find_boolean_points(item, device_key)
    find_boolean_points(cfg)
    boolean_points = list(set(boolean_points))
    # Only subscribe on the existing platform broker, with its ingest account.
    # No connection parameters or credentials are printed/persisted.
    source = r'''
const mqtt=require('mqtt'),assert=require('node:assert/strict');
const options=OPTIONS;
const client=mqtt.connect(process.env.MQTT_URL||'mqtt://emqx:1883',{clientId:'eg100-number-verification-'+Date.now(),username:process.env.MQTT_USERNAME||'platform-ingest',password:process.env.MQTT_PASSWORD||process.env.MQTT_INGEST_PASSWORD,connectTimeout:8000,reconnectPeriod:0});
const reports=[],tails=[],booleanSamples=[],invalidBooleanSamples=[];let timer;
const fail=e=>{clearTimeout(timer);client.end(true);console.error(e.message);process.exitCode=1};
client.on('error',fail);
client.on('connect',()=>client.subscribe(options.cloudTopics,{qos:0},(error,granted)=>{
 if(error)return fail(error);
 if(!granted.some(item=>item.qos!==128))return fail(new Error('Telemetry subscription denied'));
 timer=setTimeout(()=>{
  try{
   assert.ok(reports.filter(item=>item.cloud).length>=2,'Fresh cloud telemetry missing');
   assert.deepEqual(tails,[],'Fresh MQTT reports still contain floating tails');
   if(options.booleanPoints.length)assert.ok(booleanSamples.length>=2,'Fresh boolean point reports missing');
   assert.deepEqual(invalidBooleanSamples,[],'Configured bool must be a JSON boolean, never 0/1 or text');
   console.log(JSON.stringify({verifiedAt:new Date().toISOString(),freshCloudReports:reports.filter(item=>item.cloud).length,thirdPartyManualReportsCaptured:false,liveNumericSamples:reports.flatMap(item=>item.samples).slice(0,10),expectedBooleanPoints:options.booleanPoints.length,booleanReports:booleanSamples.length,booleanSamples:booleanSamples.slice(0,6),configuredBooleanTypesPreserved:options.booleanPoints.length>0&&booleanSamples.length>=2,noFloatingTails:true,noConfigurationChanges:true,noPointWrites:true}));
   client.end(true);
  }catch(error){fail(error)}
 },15000);
}));
client.on('message',(topic,buffer)=>{
 if(!options.cloudKeys.includes(topic.split('/')[2]))return;
 try{
  const payload=JSON.parse(buffer.toString()),samples=[];
  const metrics=payload.d?.gateway?.Val||{},device=topic.split('/')[4];
  for(const [metric,value] of Object.entries(metrics)){
   if(options.booleanPoints.includes(device+'::'+metric)){
    if(typeof value==='boolean')booleanSamples.push({metric,value});
    else invalidBooleanSamples.push({metric,value});
   }
  }
  const visit=value=>{
   if(typeof value==='number'){
    samples.push(value);
    const text=String(value),mantissa=text.split(/[eE]/)[0].replace(/[-.]/g,'').replace(/^0+/,'');
    for(let digits=1;digits<=mantissa.length-4;digits++){
     const candidate=Number(value.toPrecision(digits));
     if(candidate!==value&&candidate!==0&&Math.abs(candidate-value)<=Math.abs(value)*Number.EPSILON*2){tails.push(text);break}
    }
   }else if(value&&typeof value==='object')Object.values(value).forEach(visit);
  };
  visit(topic.startsWith('weikong/gateways/')?payload.d?.gateway?.Val:payload);
  if(Object.keys(metrics).length)reports.push({cloud:topic.startsWith('weikong/gateways/'),samples});
 }catch{}
});
setTimeout(()=>{if(!timer)fail(new Error('MQTT connection timeout'))},10000).unref();
'''.replace('OPTIONS', json.dumps({'cloudTopics': topics, 'cloudKeys': list(set(keys)), 'booleanPoints': boolean_points}))
    result = subprocess.run(['docker', 'compose', 'exec', '-T', 'ingest', 'node'], cwd=ROOT, input=source, capture_output=True, text=True, encoding='utf8', timeout=40)
    assert result.returncode == 0, result.stderr
    verification = json.loads(result.stdout.strip())
    out = ROOT / os.environ.get('WK_EG100_DEPLOY_OUTPUT', 'artifacts/eg100-mqtt-floating-tail-20260918')
    out.mkdir(parents=True, exist_ok=True)
    (out / 'mqtt-verification.json').write_text(json.dumps(verification, indent=2), encoding='utf8')
    print(json.dumps(verification), flush=True)
finally:
    gateway.ssh.close()
