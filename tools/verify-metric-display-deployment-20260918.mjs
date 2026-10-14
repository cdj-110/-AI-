import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright');
const source=`const {PrismaClient}=require('@prisma/client'),{JwtService}=require('@nestjs/jwt');
(async()=>{const p=new PrismaClient();try{
const device=await p.device.findFirstOrThrow({where:{metrics:{some:{identifier:'mbrtu-P01'}}},select:{id:true,tenantId:true,deviceKey:true,name:true}});
const tenant=await p.user.findFirstOrThrow({where:{tenantId:device.tenantId,role:'TENANT_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const admin=await p.user.findFirstOrThrow({where:{role:'SUPER_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const session=user=>({user,token:new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'})});
console.log(JSON.stringify({device,tenant:session(tenant),admin:session(admin)}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
// Temporary verification tokens remain only in process/browser memory.
const sessions=JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8'}));
const out='artifacts/metric-display-deployment-verification-20260918';
let browser;
try{
 browser=await chromium.launch({headless:true,channel:'chrome'});await mkdir(out,{recursive:true});
 const portals=[];
 for(const [kind,port] of [['tenant',5174],['admin',5176]]){
  const context=await browser.newContext({viewport:{width:1500,height:1000}});
  await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user))},sessions[kind]);
  const page=await context.newPage(),errors=[],failedApi=[];let mutations=0,actualItems=0;
  page.on('pageerror',error=>errors.push(error.message));
  page.on('request',request=>{if(new URL(request.url()).pathname.startsWith('/api/')&&!['GET','HEAD','OPTIONS'].includes(request.method()))mutations++});
  page.on('response',response=>{if(new URL(response.url()).pathname.startsWith('/api/')&&response.status()>=400)failedApi.push(response.status())});
  // Replay the reported floating-tail cases only in the isolated browser, never in the database/gateway.
  await page.route(`**/api/devices/${sessions.device.id}/telemetry*`,async route=>{
   const response=await route.fetch();assert.ok(response.ok());const payload=await response.json();
   actualItems=payload.data.items.length;assert.ok(actualItems>0,'Real telemetry must be available');
   payload.data.items[0].metrics={mbrtu_P01:44.300000000000004,text_P01:'44.300000000000004',...payload.data.items[0].metrics};
   await route.fulfill({response,json:payload});
  });
  await page.goto(`http://127.0.0.1:${port}/devices`);
  const search=page.getByPlaceholder(kind==='tenant'?'搜索设备名称或编号':'搜索名称或设备编号');
  await search.fill(sessions.device.deviceKey);await search.press('Enter');
  const row=page.locator('.el-table__body tr').filter({hasText:sessions.device.name}).filter({hasText:sessions.device.deviceKey});
  await row.getByRole('button',{name:'查看数据',exact:true}).click();
  const panel=page.locator(kind==='tenant'?'.el-dialog':'.el-drawer').filter({hasText:sessions.device.name});
  await panel.waitFor();await page.waitForTimeout(1500);
  assert.ok(actualItems>0,`${kind}: telemetry replay must target the opened device`);
  assert.ok((await panel.innerText()).includes('mbrtu_P01: 44.3'),`${kind}: ${await panel.innerText()}`);
  const text=await panel.innerText();
  assert.ok(text.includes('text_P01: 44.3'));assert.ok(!text.includes('44.300000000000004'));
  assert.ok(!/\d+\.\d{10,}/.test(text),'Actual telemetry should not expose floating tails');
  assert.equal(mutations,0);assert.deepEqual(errors,[]);assert.deepEqual(failedApi,[]);
  await panel.screenshot({path:`${out}/${kind}-telemetry.png`});
  portals.push({portal:kind,actualTelemetryItems:actualItems,numericTailFormatted:true,numericTextTailFormatted:true,noTelemetryFloatingTails:true,noWrites:true,pageErrors:errors,failedApi});
  await context.close();
 }
 const result={verifiedAt:new Date().toISOString(),productionAssets:true,reportedCasesReplayedInBrowserOnly:true,portals};
 await writeFile(`${out}/verification.json`,JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}finally{await browser?.close()}
