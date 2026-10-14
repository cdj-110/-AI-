import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {mkdir,readFile,writeFile} from 'node:fs/promises';
const {chromium} = await import(process.env.PLAYWRIGHT_MODULE || 'playwright');
const out = 'artifacts/license-report-deployment-20260917';
const source = `const {PrismaClient}=require('@prisma/client');const {JwtService}=require('@nestjs/jwt');
(async()=>{const p=new PrismaClient();try{
const user=await p.user.findFirstOrThrow({where:{role:'SUPER_ADMIN',status:'ACTIVE'},select:{id:true,username:true,nickname:true,role:true,tenantId:true}});
const token=new JwtService().sign({sub:user.id,username:user.username,role:user.role,tenantId:user.tenantId},{secret:process.env.JWT_SECRET,expiresIn:'3m'});
console.log(JSON.stringify({user,token}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
// A short-lived verification session stays in memory; no credentials or tokens
// are written to artifacts/logs, and no real browser profile is used.
const session = JSON.parse(execFileSync('docker',['compose','exec','-T','api','node'],{input:source,encoding:'utf8'}));
const headers = {Authorization:`Bearer ${session.token}`};
const response = await fetch('http://127.0.0.1:3100/api/factory-gateways',{headers});
assert.equal(response.status,200);
const list = (await response.json()).data;
assert.ok(Array.isArray(list.items));
for (const item of list.items) {
  assert.ok(['WAITING_CONFIRMATION','ACTIVE','UPDATE_PENDING','EXPIRED','INVALID','DISABLED'].includes(item.authorizationState));
  assert.equal(item.deviceSecretHash,undefined);
  assert.equal(item.activationSecretEncrypted,undefined);
}
const stats = execFileSync('docker',['compose','exec','-T','postgres','psql','-X','-U','weikong','-d','weikong_iot','-At','-c',`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`],{encoding:'utf8'}).trim();
const before = JSON.parse(await readFile('secrets/production-batch-deploy-20260917070859/before.json','utf8'));
const after = JSON.parse(stats);
for (const key of ['factoryCount','deviceCount','usersHash','tenantsHash']) assert.equal(after[key],before[key],`Unexpected change: ${key}`);
const migration = execFileSync('docker',['compose','exec','-T','postgres','psql','-X','-U','weikong','-d','weikong_iot','-At','-c',`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='20260917160000_license_reporting' AND finished_at IS NOT NULL;`],{encoding:'utf8'}).trim();
assert.equal(migration,'1');
let browser;
try {
  browser = await chromium.launch({headless:true,channel:'chrome'});
  const context = await browser.newContext({viewport:{width:1440,height:950}});
  await context.addInitScript(({token,user})=>{localStorage.setItem('accessToken',token);localStorage.setItem('userInfo',JSON.stringify(user));},session);
  const page = await context.newPage();
  const errors = [],failedApi = [];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('response',r=>{if(new URL(r.url()).pathname.startsWith('/api/')&&r.status()>=400)failedApi.push({path:new URL(r.url()).pathname,status:r.status()});});
  await page.goto('http://127.0.0.1:5176/factory-gateways');
  for (const name of ['绑定状态','授权状态','最近授权回执']) await page.getByRole('columnheader',{name,exact:true}).waitFor();
  await page.getByRole('button',{name:'刷新',exact:true}).click();
  await page.waitForTimeout(1200);
  assert.deepEqual(errors,[]);assert.deepEqual(failedApi,[]);
  await mkdir(out,{recursive:true});
  await page.screenshot({path:out+'/cloud-factory-gateways.png',fullPage:true});
  const result={migrationApplied:true,cloudApiVerified:true,authorizationColumnsVerified:true,credentialsRedacted:true,existingAccountsAndTenantsUnchanged:true,existingRecordCountsUnchanged:true,activeFactoryCount:list.total,pageErrors:errors,failedApi};
  await writeFile(out+'/cloud-verification.json',JSON.stringify(result,null,2));
  console.log(JSON.stringify(result));
} finally {await browser?.close();}
