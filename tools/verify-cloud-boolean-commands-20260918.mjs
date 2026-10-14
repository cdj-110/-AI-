import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright');
const source=`const {PrismaClient}=require('@prisma/client'),{JwtService}=require('@nestjs/jwt');(async()=>{const p=new PrismaClient();try{
const device=await p.device.findFirstOrThrow({where:{deviceType:'GATEWAY_CHILD',metrics:{some:{identifier:'mbtcp_P01'}}},include:{metrics:true}});
const user=await p.user.findFirstOrThrow({where:{tenantId:device.tenantId,role:'TENANT_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const token=new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'});
const draft=await p.gatewayTopologyDraft.findUnique({where:{gatewayDeviceId:device.gatewayId}});
console.log(JSON.stringify({user,token,deviceId:device.id,metric:'mbtcp_P01',storedType:device.metrics.find(m=>m.identifier==='mbtcp_P01').dataType,reportedType:draft.items.find(d=>d.deviceKey===device.deviceKey).metrics.find(m=>m.identifier==='mbtcp_P01').dataType}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
const session=JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8'}));
const base='http://127.0.0.1:5174',out='artifacts/cloud-boolean-command-verification-20260918';
const get=async path=>{const response=await fetch(base+path,{headers:{Authorization:'Bearer '+session.token},signal:AbortSignal.timeout(12000)});assert.equal(response.status,200);return(await response.json()).data};
const caps=await get(`/api/devices/${session.deviceId}/commands/capabilities`),metrics=await get(`/api/devices/${session.deviceId}/metrics`),before=await get(`/api/devices/${session.deviceId}/commands`);
assert.equal(caps.points.find(point=>point.identifier===session.metric)?.dataType,'BOOLEAN');
assert.equal(metrics.find(point=>point.identifier===session.metric)?.dataType,'BOOLEAN');assert.ok(caps.available,'Gateway currently unavailable for UI acceptance');
let browser;
try{
 browser=await chromium.launch({headless:true,channel:'chrome'});const context=await browser.newContext({viewport:{width:1500,height:1000}});
 await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user))},session);
 const page=await context.newPage(),errors=[],failedApi=[];let writes=0;
 page.on('pageerror',error=>errors.push(error.message));page.on('response',response=>{if(new URL(response.url()).pathname.startsWith('/api/')&&response.status()>=400)failedApi.push(response.status())});
 await page.route('**/api/**',route=>{if(!['GET','HEAD','OPTIONS'].includes(route.request().method())){writes++;return route.abort()}return route.continue()});
 await page.goto(`${base}/devices/${session.deviceId}`);await page.locator('.detail-tabs').waitFor();await page.locator('.detail-tabs button').filter({hasText:'实时数据'}).click();
 const card=page.locator('.metric-card').filter({hasText:session.metric});await card.getByRole('button',{name:'下发',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'点位下发',exact:true});await dialog.getByRole('radiogroup',{name:'布尔下发值'}).waitFor();
 const radios=dialog.locator('.el-radio-button');assert.equal(await radios.count(),2);assert.equal(await dialog.locator('.el-input').count(),0);
 for(let i=0;i<2;i++)assert.equal(await radios.nth(i).locator('input').isChecked(),false);
 await dialog.getByRole('button',{name:'确认下发',exact:true}).click();await page.getByText('请选择 true 或 false',{exact:true}).waitFor();
 assert.equal(await page.getByRole('dialog',{name:'确认下发',exact:true}).count(),0);
 for(const value of [true,false]){await radios.nth(value?0:1).click();await dialog.locator('.el-radio-button.is-active').filter({hasText:String(value)}).waitFor();assert.equal(await radios.nth(value?0:1).locator('input').isChecked(),true)}
 await mkdir(out,{recursive:true});await dialog.screenshot({path:out+'/boolean-choice.png'});
 assert.equal(writes,0);assert.deepEqual(errors,[]);assert.deepEqual(failedApi,[]);
 const after=await get(`/api/devices/${session.deviceId}/commands`);assert.deepEqual(after.map(task=>task.id),before.map(task=>task.id));
 const result={verifiedAt:new Date().toISOString(),storedType:session.storedType,reportedType:session.reportedType,capabilityType:'BOOLEAN',modelType:'BOOLEAN',trueFalseChoices:true,textInputRemoved:true,explicitSelectionRequired:true,pageErrors:errors,failedApi,noPhysicalPointWrites:true,noCommandRecordsCreated:true};
 await writeFile(out+'/verification.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}finally{await browser?.close()}
