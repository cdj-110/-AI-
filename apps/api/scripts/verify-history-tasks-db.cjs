const {execFileSync}=require('node:child_process'),{readFileSync}=require('node:fs'),{randomBytes}=require('node:crypto'),assert=require('node:assert/strict');
const {Pool}=require('pg'),{PrismaClient}=require('@prisma/client');
const docker=args=>execFileSync('docker',args,{encoding:'utf8',timeout:60000}).trim();
async function main(){let container,pool,prisma;const password=randomBytes(16).toString('hex');
 try{
  container=docker(['run','--rm','-d','--name',`weikong-history-task-test-${process.pid}`,'-e',`POSTGRES_PASSWORD=${password}`,'-e','POSTGRES_DB=task_test','-p','127.0.0.1::5432','postgres:16']);assert.match(container,/^[a-f0-9]{64}$/);
  const port=Number(docker(['port',container,'5432/tcp']).split(':').pop());pool=new Pool({host:'127.0.0.1',port,user:'postgres',password,database:'task_test',connectionTimeoutMillis:2000});
  let ready=false;for(let i=0;i<30;i++){try{await pool.query('SELECT 1');ready=true;break}catch{await new Promise(r=>setTimeout(r,500))}}assert.ok(ready);
  await pool.query('CREATE TABLE "Tenant" (id TEXT PRIMARY KEY); INSERT INTO "Tenant" VALUES (\'tenant-a\'),(\'tenant-b\');CREATE TABLE telemetry_events (value INTEGER);INSERT INTO telemetry_events VALUES(41);');
  await pool.query(readFileSync('prisma/migrations/20261013140000_history_data_tasks/migration.sql','utf8'));
  prisma=new PrismaClient({datasources:{db:{url:`postgresql://postgres:${password}@127.0.0.1:${port}/task_test`}}});
  const pairs=[{value1:'aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',value2:'temperature'}];
  const task=await prisma.historyDataTask.create({data:{tenantId:'tenant-a',name:'历史任务测试',createdById:'user-a',createdByName:'测试用户',pairs,deviceNames:['设备A'],startAt:new Date('2026-10-13T00:00:00Z'),endAt:new Date('2026-10-13T01:00:00Z')}});
  assert.ok(task.id);assert.deepEqual(task.pairs,pairs);assert.deepEqual(task.deviceNames,['设备A']);assert.equal(task.startAt.toISOString(),'2026-10-13T00:00:00.000Z');
  assert.equal(await prisma.historyDataTask.count({where:{tenantId:'tenant-b'}}),0);assert.equal(await prisma.historyDataTask.count({where:{tenantId:'tenant-a',createdById:'user-b'}}),0);
  await prisma.historyDataTask.update({where:{id:task.id},data:{name:'修改后的历史任务'}});assert.equal((await prisma.historyDataTask.findUnique({where:{id:task.id}})).name,'修改后的历史任务');
  await prisma.historyDataTask.delete({where:{id:task.id}});assert.equal(await prisma.historyDataTask.count(),0);assert.equal((await pool.query('SELECT COUNT(*)::int AS n FROM telemetry_events')).rows[0].n,1);
  console.log('PASS isolated PostgreSQL task migration and Prisma persistence: generated id, JSON points, time range, edit, tenant/member filters, task deletion does not delete telemetry.');
 }finally{if(prisma)await prisma.$disconnect();if(pool)await pool.end();if(container&&/^[a-f0-9]{64}$/.test(container))docker(['stop',container]);}}
main().catch(e=>{console.error(e.message);process.exitCode=1});
