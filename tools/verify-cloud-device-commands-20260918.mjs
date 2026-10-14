import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright');
const source=`const {PrismaClient}=require('@prisma/client'),{JwtService}=require('@nestjs/jwt');(async()=>{const p=new PrismaClient();try{
const device=await p.device.findFirstOrThrow({where:{deviceType:'GATEWAY_CHILD',metrics:{some:{identifier:'mbrtu-P01'}}},include:{gateway:true,metrics:true}});
const user=await p.user.findFirstOrThrow({where:{tenantId:device.tenantId,role:'TENANT_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const token=new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'});
console.log(JSON.stringify({user,token,deviceId:device.id,deviceKey:device.deviceKey,gatewayId:device.gatewayId,gatewayKey:device.gateway.deviceKey,metrics:device.metrics.map(m=>({identifier:m.identifier,accessMode:m.accessMode,enabled:m.enabled,ignored:m.ignored}))}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
const session=JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8'}));
const base='http://127.0.0.1:5174',out=process.env.CLOUD_DEVICE_COMMANDS_VERIFICATION_OUT||'artifacts/cloud-device-commands-verification-20260918';
async function get(path){const response=await fetch(base+path,{headers:{Authorization:'Bearer '+session.token},signal:AbortSignal.timeout(12000)});assert.equal(response.status,200,'Acceptance GET failed: '+path);return(await response.json()).data}
const current=await get(`/api/devices/${session.gatewayId}/remote-configs/current`);
assert.ok(!current.stale,'Gateway config must come from live read, not stale cache');
assert.equal(current.config.pointWriteVersion,1);assert.ok(Array.isArray(current.config.writablePoints));
assert.equal(current.config.pointAccessVersion,1,'Updated gateway must explicitly report read/write capabilities');
assert.ok(Array.isArray(current.config.pointAccesses));
assert.ok(current.config.pointAccesses.every(point=>typeof point.canRead==='boolean'&&typeof point.canWrite==='boolean'));
const caps=await get(`/api/devices/${session.deviceId}/commands/capabilities`),history=await get(`/api/devices/${session.deviceId}/commands`);
assert.ok(!caps.reason?.includes('请更新 EG100'),'Deployed gateway capability should be recognized');
const reportedWritable=new Set(current.config.pointAccessVersion===1
 ?current.config.pointAccesses.filter(point=>point.deviceKey===session.deviceKey&&point.canWrite===true).map(point=>point.metric)
 :current.config.writablePoints.filter(point=>point.deviceKey===session.deviceKey).map(point=>point.metric));
const allowed=new Set(session.metrics.filter(m=>m.enabled&&!m.ignored&&reportedWritable.has(m.identifier)).map(m=>m.identifier));
assert.ok(caps.points.every(point=>allowed.has(point.identifier)),'Only gateway-reported writable points can be offered');
const displayedMetrics=await get(`/api/devices/${session.deviceId}/metrics`);
assert.ok(caps.points.every(point=>displayedMetrics.some(metric=>metric.identifier===point.identifier&&metric.accessMode==='READ_WRITE')),'Cloud model must display gateway permissions automatically');
const ownReportedPoints=current.config.pointAccesses.filter(point=>point.deviceKey===session.deviceKey);
for(const point of ownReportedPoints){const model=session.metrics.find(metric=>metric.identifier===point.metric);if(model)assert.equal(model.accessMode,point.canWrite?'READ_WRITE':'READ_ONLY','Ingest must persist gateway-reported permissions automatically')}
const previousCommandIds=history.map(task=>task.id);
const subscriptions=execFileSync('docker',['compose','exec','-T','emqx','emqx','ctl','subscriptions','list'],{encoding:'utf8'});
assert.ok(subscriptions.includes(`topic:weikong/gateways/${session.gatewayKey}/commands/set qos:1`),'Gateway command subscription missing');
assert.ok(subscriptions.includes('topic:weikong/gateways/+/commands/reply qos:1'),'API receipt subscription missing');
let browser;
try{
 browser=await chromium.launch({headless:true,channel:'chrome'});const context=await browser.newContext({viewport:{width:1500,height:1000}});
 await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user))},session);
 const page=await context.newPage(),errors=[],failedApi=[];let writes=0;
 page.on('pageerror',error=>errors.push(error.message));page.on('request',request=>{if(new URL(request.url()).pathname.startsWith('/api/')&&!['GET','HEAD','OPTIONS'].includes(request.method()))writes++});
 page.on('response',response=>{if(new URL(response.url()).pathname.startsWith('/api/')&&response.status()>=400)failedApi.push(response.status())});
 await page.goto(`${base}/devices/${session.deviceId}`);await page.locator('.detail-tabs').waitFor();await page.locator('.detail-tabs button').filter({hasText:'实时数据'}).click();
 let dialog;
 if(caps.available){
  const capabilityLoad=page.waitForResponse(response=>new URL(response.url()).pathname.endsWith('/commands/capabilities'));
  const card=page.locator('.metric-card').filter({hasText:caps.points[0].identifier});
  await card.getByRole('button',{name:'下发',exact:true}).click();dialog=page.getByRole('dialog',{name:'点位下发',exact:true});await dialog.waitFor();
  await capabilityLoad;await dialog.locator('.command-point').filter({hasText:caps.points[0].identifier}).waitFor();
  assert.equal(await dialog.locator('.el-select').count(),0,'Card downlink must lock its point');
 }else{
  for(const point of caps.points){const action=page.locator('.metric-card').filter({hasText:point.identifier}).getByRole('button',{name:'下发',exact:true});await action.waitFor();assert.equal(await action.isDisabled(),true)}
 }
 assert.equal(await page.getByRole('button',{name:'下发操作',exact:true}).count(),0,'Section-wide entry must be removed');
 for(const point of ownReportedPoints.filter(point=>!point.canWrite)){const card=page.locator('.metric-card').filter({hasText:point.metric});if(await card.count())assert.equal(await card.getByRole('button',{name:'下发',exact:true}).count(),0,'Read-only card must not offer downlink')}
 assert.equal(writes,0);assert.deepEqual(errors,[]);assert.deepEqual(failedApi,[]);
 const afterHistory=await get(`/api/devices/${session.deviceId}/commands`);
 assert.deepEqual(afterHistory.map(task=>task.id),previousCommandIds,'Acceptance must not create command records');
 await mkdir(out,{recursive:true});await (dialog||page.locator('.metrics-section')).screenshot({path:out+'/command-dialog.png'});
 const result={verifiedAt:new Date().toISOString(),liveGatewayConfigRead:true,pointWriteVersion:1,pointAccessVersion:1,explicitReadWriteCapabilities:true,ingestAutomaticallySyncedPermissions:true,reportedWritablePointCount:reportedWritable.size,gatewayCommandSubscription:true,apiReceiptSubscription:true,perCardDownlink:true,readOnlyCardsHaveNoDownlink:true,gatewayAuthoritativePermissions:true,automaticModelPermissionDisplay:true,cloudWritablePointCount:caps.points.length,downlinkAvailable:caps.available,availabilityReason:caps.reason,noPhysicalPointWrites:true,noCommandRecordsCreated:true,pageErrors:errors,failedApi};
 await writeFile(out+'/verification.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}finally{await browser?.close()}
