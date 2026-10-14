const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'..'),stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14),out=path.join(root,'artifacts',`cloud-device-commands-deploy-${stamp}`);
const migration='20260918040000_device_commands';
const docker=(args)=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:192*1024*1024});
const inspect=service=>JSON.parse(docker(['inspect',docker(['compose','ps','-q',service]).toString().trim()]))[0];
const run=args=>{const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});if(result.status!==0)throw Error('Cloud deployment step failed: '+args.slice(0,3).join(' '))};
const query=sql=>docker(['compose','exec','-T','postgres','psql','-X','-v','ON_ERROR_STOP=1','-U','weikong','-d','weikong_iot','-At','-c',sql]).toString().trim();
const env=item=>Object.fromEntries(item.Config.Env.map(line=>{const i=line.indexOf('=');return[line.slice(0,i),line.slice(i+1)]}));
const config=JSON.parse(docker(['compose','config','--format','json'])),beforeApi=inspect('api');
const protectedKeys=['DATABASE_URL','JWT_SECRET','FACTORY_CREDENTIAL_ENCRYPTION_KEY','ACTIVATION_SIGNING_PRIVATE_KEY','ACTIVATION_SIGNING_KEY_ID','MQTT_INGEST_USERNAME','MQTT_INGEST_PASSWORD','MQTT_PUBLIC_HOST','REMOTE_ACCESS_PUBLIC_URL'];
const live=env(beforeApi);
for(const key of protectedKeys)if(String(config.services.api.environment[key]??'')!==(live[key]??''))throw Error('Deployment would change protected API setting: '+key);
const beforeIngest=inspect('ingest'),liveIngest=env(beforeIngest),ingestKeys=['DATABASE_URL','TIMESCALE_DATABASE_URL','MQTT_URL','MQTT_USERNAME','MQTT_PASSWORD'];
for(const key of ingestKeys)if(String(config.services.ingest.environment[key]??'')!==(liveIngest[key]??''))throw Error('Deployment would change protected ingest setting: '+key);
const applied=JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
const local=fs.readdirSync(path.join(root,'apps/api/prisma/migrations')).filter(name=>fs.existsSync(path.join(root,'apps/api/prisma/migrations',name,'migration.sql')));
for(const item of applied){assert.ok(local.includes(item.name),'Applied migration missing locally: '+item.name);assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(root,'apps/api/prisma/migrations',item.name,'migration.sql'))).digest('hex'),item.checksum,'Historical migration changed: '+item.name)}
const pending=local.filter(name=>!applied.some(item=>item.name===name));
assert.ok(pending.length===0||(pending.length===1&&pending[0]===migration),'Unexpected pending migrations');
const commandCount=query('SELECT count(*) FROM "DeviceCommand";');
const preserved=Object.fromEntries(['admin-web','postgres','timescaledb','redis','emqx'].map(name=>[name,inspect(name).Id]));
fs.mkdirSync(out,{recursive:true});
const backup=JSON.parse(cp.execFileSync('node',['tools/backup-production-batch-deploy.cjs'],{cwd:root,encoding:'utf8',maxBuffer:4*1024*1024}).trim());
const rollbackServices={};
for(const service of ['api','web','ingest']){const original=inspect(service),tag=`wk-device-commands-rollback-${service}:${stamp}`;docker(['tag',original.Image,tag]);rollbackServices[service]={image:tag}}
const rollbackFile=path.join(out,'compose.rollback.json');fs.writeFileSync(rollbackFile,JSON.stringify({services:rollbackServices},null,2));
fs.writeFileSync(path.join(out,'before.json'),JSON.stringify({databaseBackupDirectory:backup.backupDirectory,databaseBackupVerified:backup.encryptedBackupVerified,preserved,rollbackServices,pendingMigrations:pending,commandCount},null,2));
console.log(JSON.stringify({stage:'backup-verified',databaseBackupVerified:backup.encryptedBackupVerified,out,pendingMigrations:pending,credentialsUnchanged:true}));
let replacing=false;
(async()=>{try{
 run(['compose','build','api','web','ingest']);
 if(pending.length)run(['compose','run','--rm','--no-deps','api','npx','prisma','migrate','deploy']);
 else run(['compose','run','--rm','--no-deps','api','npx','prisma','migrate','status']);
 assert.equal(query(`SELECT count(*) FROM "_prisma_migrations" WHERE migration_name='${migration}' AND finished_at IS NOT NULL AND rolled_back_at IS NULL;`),'1');
 assert.equal(query(`SELECT count(*) FROM "DeviceCommand";`),commandCount);
 // MQTT authorization is served by API. Start ingestion only after API is ready,
 // otherwise individual subscriptions can fail during the authorization outage.
 replacing=true;run(['compose','up','-d','--no-deps','api','web']);
 let healthy=false;for(let index=0;index<40;index++){try{const response=await fetch('http://127.0.0.1:3100/api/devices');if(response.status===401){healthy=true;break}}catch{}await new Promise(resolve=>setTimeout(resolve,500))}
 assert.ok(healthy,'API did not recover');
 run(['compose','up','-d','--no-deps','ingest']);
 const currentApi=inspect('api'),currentWeb=inspect('web');
 assert.ok(currentApi.State.Running&&currentWeb.State.Running);
 const currentEnv=env(currentApi);for(const key of protectedKeys)if(currentEnv[key]!==live[key])throw Error('Protected setting changed after deployment: '+key);
 const currentIngest=inspect('ingest');assert.ok(currentIngest.State.Running,'Ingest did not recover');
 const currentIngestEnv=env(currentIngest);for(const key of ingestKeys)if(currentIngestEnv[key]!==liveIngest[key])throw Error('Protected ingest setting changed after deployment: '+key);
 for(const[service,id]of Object.entries(preserved))assert.equal(inspect(service).Id,id,'Unrelated service was recreated: '+service);
 const after=JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
 for(const key of ['factoryCount','deviceCount','usersHash','tenantsHash'])assert.equal(after[key],backup.before[key],'Preserved account/tenant/device information changed: '+key);
 assert.ok((await fetch('http://127.0.0.1:5174/')).ok);docker(['exec',currentWeb.Id,'nginx','-t']);
 const result={deployedAt:new Date().toISOString(),migrationsApplied:pending,databaseSchemaUpToDate:true,apiUpdated:true,ingestUpdated:true,tenantFrontendUpdated:true,credentialsUnchanged:true,accountsUnchanged:true,tenantsUnchanged:true,deviceCountUnchanged:true,unrelatedServicesUnchanged:true,physicalPointWrites:false,backup:out,databaseBackupDirectory:backup.backupDirectory,rollbackServices};
 fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}catch(error){if(replacing){run(['compose','-f','docker-compose.yml','-f',rollbackFile,'up','-d','--no-deps','--no-build','api','web','ingest']);console.error('API, ingest and tenant frontend rolled back. Existing database data was not restored or removed.')}throw error}})().catch(error=>{console.error(error.message);process.exitCode=1});
