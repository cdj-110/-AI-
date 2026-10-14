const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'..'),stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14),out=path.join(root,'artifacts',`cloud-boolean-commands-deploy-${stamp}`);
const docker=args=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:192*1024*1024});
const inspect=service=>JSON.parse(docker(['inspect',docker(['compose','ps','-q',service]).toString().trim()]))[0];
const run=args=>{const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});assert.equal(result.status,0,'Cloud deployment step failed')};
const query=sql=>docker(['compose','exec','-T','postgres','psql','-X','-v','ON_ERROR_STOP=1','-U','weikong','-d','weikong_iot','-At','-c',sql]).toString().trim();
const env=item=>Object.fromEntries(item.Config.Env.map(line=>{const i=line.indexOf('=');return[line.slice(0,i),line.slice(i+1)]}));
const config=JSON.parse(docker(['compose','config','--format','json'])),beforeApi=inspect('api'),live=env(beforeApi);
const configured=config.services.api.environment;
for(const[key,value]of Object.entries(configured))assert.equal(String(value??''),live[key]??'','API setting would change: '+key);
const applied=JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
const local=fs.readdirSync(path.join(root,'apps/api/prisma/migrations')).filter(name=>fs.existsSync(path.join(root,'apps/api/prisma/migrations',name,'migration.sql')));
for(const item of applied){assert.ok(local.includes(item.name));assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(root,'apps/api/prisma/migrations',item.name,'migration.sql'))).digest('hex'),item.checksum,'Historical migration changed: '+item.name)}
assert.deepEqual(local.filter(name=>!applied.some(item=>item.name===name)),[],'Unexpected pending database migration');
const preserved=Object.fromEntries(['ingest','admin-web','postgres','timescaledb','redis','emqx'].map(name=>[name,inspect(name).Id]));
fs.mkdirSync(out,{recursive:true});
const backup=JSON.parse(cp.execFileSync('node',['tools/backup-production-batch-deploy.cjs'],{cwd:root,encoding:'utf8',maxBuffer:4*1024*1024}).trim());
assert.ok(backup.encryptedBackupVerified);
const rollbackServices={};for(const service of ['api','web']){const original=inspect(service),tag=`wk-boolean-commands-rollback-${service}:${stamp}`;docker(['tag',original.Image,tag]);rollbackServices[service]={image:tag}}
const rollbackFile=path.join(out,'compose.rollback.json');fs.writeFileSync(rollbackFile,JSON.stringify({services:rollbackServices},null,2));
fs.writeFileSync(path.join(out,'before.json'),JSON.stringify({databaseBackupDirectory:backup.backupDirectory,preserved,rollbackServices,migrationCount:applied.length},null,2));
console.log(JSON.stringify({stage:'backup-verified',out,databaseBackupVerified:true,servicesToUpdate:['api','web'],pendingMigrations:0}));
let replacing=false;
(async()=>{try{
 run(['compose','build','api','web']);
 replacing=true;run(['compose','up','-d','--no-deps','api','web']);
 let healthy=false;for(let i=0;i<60;i++){try{if((await fetch('http://127.0.0.1:3100/api/devices',{signal:AbortSignal.timeout(2000)})).status===401){healthy=true;break}}catch{}await new Promise(resolve=>setTimeout(resolve,500))}
 assert.ok(healthy,'API did not recover');
 const currentApi=inspect('api'),currentWeb=inspect('web');assert.ok(currentApi.State.Running&&currentWeb.State.Running);
 const afterEnv=env(currentApi);for(const key of Object.keys(configured))assert.equal(afterEnv[key],live[key],'Protected API setting changed: '+key);
 for(const[service,id]of Object.entries(preserved))assert.equal(inspect(service).Id,id,'Unrelated service recreated: '+service);
 const after=JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
 for(const key of ['factoryCount','deviceCount','usersHash','tenantsHash'])assert.equal(after[key],backup.before[key],'Preserved data changed: '+key);
 assert.equal(query('SELECT count(*) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;'),String(applied.length));
 assert.ok((await fetch('http://127.0.0.1:5174/',{signal:AbortSignal.timeout(5000)})).ok);docker(['exec',currentWeb.Id,'nginx','-t']);
 const result={deployedAt:new Date().toISOString(),apiUpdated:true,tenantFrontendUpdated:true,databaseMigrationRequired:false,credentialsUnchanged:true,accountsUnchanged:true,tenantsUnchanged:true,deviceCountUnchanged:true,unrelatedServicesUnchanged:true,physicalPointWrites:false,backup:out,databaseBackupDirectory:backup.backupDirectory,rollbackServices};
 fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}catch(error){if(replacing){run(['compose','-f','docker-compose.yml','-f',rollbackFile,'up','-d','--no-deps','--no-build','api','web']);console.error('API and tenant frontend rolled back; database data was not restored or removed.')}throw error}})().catch(error=>{console.error(error.message);process.exitCode=1});
