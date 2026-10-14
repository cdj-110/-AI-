import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdir,writeFile} from 'node:fs/promises';
const {chromium}=await import(process.env.PLAYWRIGHT_MODULE||'playwright');
const source=`const {PrismaClient}=require('@prisma/client');const {JwtService}=require('@nestjs/jwt');(async()=>{const p=new PrismaClient();try{const user=await p.user.findFirstOrThrow({where:{role:'SUPER_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});const token=new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'});console.log(JSON.stringify({user,token}));}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
// Verification-only session stays in memory and uses a separate browser context.
let session;
for(let i=0;i<20;i++){
  try{session=JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8',stdio:['pipe','pipe','pipe']}));const r=await fetch('http://127.0.0.1:3100/api/factory-gateways',{headers:{Authorization:`Bearer ${session.token}`}});if(r.status===200)break;}catch{}
  await new Promise(r=>setTimeout(r,1000));
}
assert.ok(session,'API did not become available');
const headers={Authorization:`Bearer ${session.token}`};
const response=await fetch('http://127.0.0.1:3100/api/factory-gateways',{headers});assert.equal(response.status,200);
const list=(await response.json()).data;
for(const row of list.items){assert.ok(row.product?.code);for(const key of ['deviceSecretHash','activationSecretEncrypted','bindCodeHash','bindCodeEncrypted','nfcTokenHash','nfcTokenEncrypted'])assert.equal(row[key],undefined);}
const sample=list.items.find(row=>row.batchNo);
if(sample){const search=await fetch(`http://127.0.0.1:3100/api/factory-gateways?keyword=${encodeURIComponent(sample.batchNo)}`,{headers});assert.equal(search.status,200);assert.ok((await search.json()).data.items.some(row=>row.id===sample.id),'Batch search failed');}
let browser;
try{
  browser=await chromium.launch({headless:true,channel:'chrome'});
  const context=await browser.newContext({viewport:{width:1440,height:950}});
  await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user));},session);
  const page=await context.newPage(),errors=[],failed=[];
  page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(new URL(r.url()).pathname.startsWith('/api/')&&r.status()>=400)failed.push(r.status());});
  await page.goto('http://127.0.0.1:5176/factory-gateways');
  const table=page.locator('.factory-gateway-table');
  await table.getByRole('columnheader',{name:'SN',exact:true}).waitFor();await table.locator('.el-loading-mask').waitFor({state:'hidden'});
  assert.deepEqual((await table.getByRole('columnheader').allTextContents()).map(t=>t.trim()).filter(Boolean),['SN','产品型号','绑定状态','授权状态','归属租户','最近心跳','操作']);
  const out='artifacts/factory-details-live-verification';await mkdir(out,{recursive:true});
  await page.screenshot({path:out+'/list.png',fullPage:true});
  if(list.items.length){const row=list.items[0];await table.getByRole('button',{name:row.sn,exact:true}).click();const drawer=page.getByRole('dialog',{name:'出厂网关详情'});await drawer.waitFor();await page.waitForFunction(()=>getComputedStyle(document.querySelector('.factory-details-drawer')).transform==='none');for(const name of ['设备身份','生产信息','授权信息','绑定信息','连接信息'])await drawer.getByRole('heading',{name,exact:true}).waitFor();await drawer.getByText(row.hardwareId,{exact:true}).waitFor();await drawer.getByRole('button',{name:'刷新详情',exact:true}).click();await table.locator('.el-loading-mask').waitFor({state:'hidden'});await page.screenshot({path:out+'/details.png',fullPage:true});}
  assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);
  const result={verifiedAt:new Date().toISOString(),apiReady:true,productDetails:true,batchSearchVerified:!!sample,compactListVerified:true,detailDrawerVerified:list.items.length>0,secretsRedacted:true,pageErrors:errors,failedApi:failed};await writeFile(out+'/verification.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}finally{await browser?.close();}
