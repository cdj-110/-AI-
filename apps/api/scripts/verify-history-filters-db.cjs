const {execFileSync}=require('node:child_process'),{randomBytes}=require('node:crypto'),assert=require('node:assert/strict');
const {Pool}=require('pg'),{DevicesService}=require('../dist/devices/devices.service');
const docker=args=>execFileSync('docker',args,{encoding:'utf8',timeout:60000}).trim();
async function main(){let container,pool;const password=randomBytes(16).toString('hex');
 try{
  container=docker(['run','--rm','-d','--name',`weikong-history-filter-test-${process.pid}`,'-e',`POSTGRES_PASSWORD=${password}`,'-e','POSTGRES_DB=filter_test','-p','127.0.0.1::5432','postgres:16']);assert.match(container,/^[a-f0-9]{64}$/);
  const port=Number(docker(['port',container,'5432/tcp']).split(':').pop());pool=new Pool({host:'127.0.0.1',port,user:'postgres',password,database:'filter_test',connectionTimeoutMillis:2000});
  let ready=false;for(let i=0;i<30;i++){try{await pool.query('SELECT 1');ready=true;break}catch{await new Promise(r=>setTimeout(r,500))}}assert.ok(ready);
  await pool.query('CREATE TABLE telemetry_events (time timestamptz,device_id text,metrics jsonb)');
  const id='aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',other='bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb',now=Date.now()-10000;
  for(const [i,value] of [-5,0,10,20,'10',null].entries())await pool.query('INSERT INTO telemetry_events VALUES($1,$2,$3)',[new Date(now-i*1000),id,JSON.stringify({value})]);
  await pool.query('INSERT INTO telemetry_events VALUES($1,$2,$3)',[new Date(now),other,JSON.stringify({value:10})]);
  const s=Object.create(DevicesService.prototype);s.timescale=pool;s.prisma={device:{findMany:async()=>[{id,tenantId:'tenant',name:'设备A',metrics:[{identifier:'value',name:'值'}]}]}};
  s.subscriptionLimits={history:async()=>({queryEarliestAt:new Date(now-86400000)})};
  const actor={sub:'member',tenantId:'tenant',role:'TENANT_ADMIN'},pairs=[{value1:id,value2:'value'}],start=new Date(now-60000).toISOString(),end=new Date(now+1000).toISOString();
  const first=await s.telemetryHistoryTable(actor,pairs,start,end,1,1,undefined,false,{min:0,max:10});assert.equal(first.total,2);assert.equal(first.dataset.source[0].metric_0,0);assert.equal(first.hasNext,true);
  const second=await s.telemetryHistoryTable(actor,pairs,start,end,2,1,undefined,false,{min:0,max:10});assert.equal(second.total,2);assert.equal(second.dataset.source[0].metric_0,10);
  const keyword=await s.telemetryHistoryTable(actor,pairs,start,end,1,20,'10',false,{min:0,max:10});assert.equal(keyword.total,1);
  const csv=await s.exportHistory(actor,pairs,start,end,undefined,{min:0,max:10});assert.equal(csv.exported,2);assert.equal(csv.total,2);
  assert.equal((await s.telemetryHistoryTable(actor,pairs,start,end,1,20)).total,6);
  assert.equal((await s.telemetryHistoryTable(actor,pairs,start,end,1,20,undefined,false,{min:100})).total,0);
  console.log('PASS isolated PostgreSQL filters: numeric inclusive bounds/zero/negative, exclude string/null, all-page count, pagination, combined keyword, export parity, unfiltered compatibility, exclude unselected device.');
 }finally{if(pool)await pool.end();if(container&&/^[a-f0-9]{64}$/.test(container))docker(['stop',container]);}
}
main().catch(e=>{console.error(e);process.exitCode=1});
