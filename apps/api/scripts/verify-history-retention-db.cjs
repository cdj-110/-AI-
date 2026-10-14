// Creates and stops its own disposable container only. No existing DB or Docker volume is used.
const { execFileSync } = require('node:child_process');
const { readFileSync } = require('node:fs');
const { randomBytes } = require('node:crypto');
const assert = require('node:assert/strict');
const { Pool } = require('pg');
const { DevicesService } = require('../dist/devices/devices.service');
const { historyRetention } = require('../dist/subscriptions/history-retention');
const docker=(args)=>execFileSync('docker',args,{encoding:'utf8',timeout:60000}).trim();
async function main(){
  let container, pool;
  const password=randomBytes(16).toString('hex');
  try{
    container=docker(['run','--rm','-d','--name',`weikong-history-test-${process.pid}`,'-e',`POSTGRES_PASSWORD=${password}`,'-e','POSTGRES_DB=retention_test','-p','127.0.0.1::5432','timescale/timescaledb:latest-pg16']);
    assert.match(container,/^[a-f0-9]{64}$/);
    const port=Number(docker(['port',container,'5432/tcp']).split(':').pop());
    pool=new Pool({host:'127.0.0.1',port,user:'postgres',password,database:'retention_test',connectionTimeoutMillis:2000});
    let ready=false;for(let i=0;i<30;i++){try{await pool.query('SELECT 1');ready=true;break}catch{await new Promise(r=>setTimeout(r,500))}}
    assert.ok(ready,'test database ready');
    await pool.query(`
      CREATE TABLE "SubscriptionPlan" ("id" text PRIMARY KEY, "code" text, "name" text);
      INSERT INTO "SubscriptionPlan" VALUES ('trial','TRIAL','体验版'),('basic','BASIC','基础版'),('pro','PROFESSIONAL','专业版'),('enterprise','ENTERPRISE','企业版');
      CREATE TABLE "TenantSubscription" ("id" text PRIMARY KEY);
      CREATE TABLE "SubscriptionOrder" ("id" text PRIMARY KEY);
      CREATE TABLE "EntitlementChange" ("id" text PRIMARY KEY);
      CREATE TABLE "QuotaAddon" ("id" text PRIMARY KEY,"code" text UNIQUE,"name" text,"description" text,"resourceType" text,"quantity" integer,"priceCents" integer,"enabled" boolean,"updatedAt" timestamp);
    `);
    await pool.query(readFileSync('prisma/migrations/20261013090000_history_retention/migration.sql','utf8'));
    const plans=(await pool.query('SELECT * FROM "SubscriptionPlan" ORDER BY "id"')).rows;
    assert.equal(plans.find(p=>p.code==='BASIC').historyRetentionMonths,6);
    assert.equal(plans.find(p=>p.code==='ENTERPRISE').historyRetentionMonths,9);
    assert.equal(plans.find(p=>p.code==='PROFESSIONAL').historyRetentionMonths,12);
    assert.equal(plans.find(p=>p.code==='TRIAL').name,'体验版');
    assert.equal(plans.find(p=>p.code==='BASIC').name,'基础版');
    assert.equal((await pool.query('SELECT * FROM "QuotaAddon"')).rows[0].pricePending,true);
    await assert.rejects(pool.query('UPDATE "SubscriptionPlan" SET "historyRetentionDays"=7 WHERE "code"=\'BASIC\''));
    await pool.query(`CREATE EXTENSION IF NOT EXISTS timescaledb; CREATE TABLE telemetry_events(time timestamptz NOT NULL,device_id text,device_key text,metrics jsonb);SELECT create_hypertable('telemetry_events','time');`);
    const a='aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',b='bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb';
    const now=new Date(),t=new Date(now.getTime()-60000);
    await pool.query('INSERT INTO telemetry_events VALUES ($1,$2,$2,$3),($1,$2,$2,$4),($5,$2,$2,$3),($1,$6,$6,$3)',[t,a,{temperature:2},{temperature:4},new Date(now.getTime()-9*86400000),b]);
    const svc=Object.create(DevicesService.prototype);
    svc.prisma={device:{findMany:async()=>[{id:a,tenantId:'tenant-a',name:'设备A',metrics:[{identifier:'temperature',name:'温度'}]}]}};
    svc.subscriptionLimits={history:async()=>historyRetention({historyRetentionDays:7,historyRetentionMonths:0},null,now)};
    svc.timescale=pool;
    const actor={role:'TENANT_ADMIN',tenantId:'tenant-a',sub:'user-a'},pairs=[{value1:a,value2:'temperature'}];
    const start=new Date(now.getTime()-3600000).toISOString(),end=now.toISOString();
    const raw=await svc.telemetryHistoryTable(actor,pairs,start,end,1,10);
    assert.equal(raw.total,2);assert.deepEqual(raw.dataset.source.map(r=>r.metric_0).sort(),[2,4]);
    const chart=await svc.telemetryHistoryTable(actor,pairs,start,end,1,100,undefined,true);
    assert.equal(chart.total,1);assert.equal(chart.dataset.source[0].metric_0,3);assert.equal(chart.aggregation.method,'AVG');
    const exported=await svc.exportHistory(actor,pairs,start,end);
    assert.equal(exported.exported,2);assert.equal(exported.limited,false);assert.match(exported.csv,/温度/);
    // Validate bounded hypertable deletion SQL against test data only, with a tenant-A scope.
    await pool.query(`WITH doomed AS (SELECT tableoid,ctid FROM telemetry_events WHERE device_id=ANY($1::text[]) AND time<$2 ORDER BY time LIMIT 1000)
      DELETE FROM telemetry_events e USING doomed d WHERE e.tableoid=d.tableoid AND e.ctid=d.ctid`,[[a],new Date(now.getTime()-8*86400000)]);
    assert.equal((await pool.query('SELECT COUNT(*)::int AS n FROM telemetry_events WHERE device_id=$1',[a])).rows[0].n,2);
    assert.equal((await pool.query('SELECT COUNT(*)::int AS n FROM telemetry_events WHERE device_id=$1',[b])).rows[0].n,1);
    console.log('PASS isolated TimescaleDB: additive migration, unchanged tier names, pending-price draft, raw pagination, AVG aggregation, CSV export, tenant-scoped bounded cleanup.');
  } finally {if(pool)await pool.end();if(container&&/^[a-f0-9]{64}$/.test(container))docker(['stop',container]);}
}
main().catch(e=>{console.error(e.message);process.exitCode=1});
