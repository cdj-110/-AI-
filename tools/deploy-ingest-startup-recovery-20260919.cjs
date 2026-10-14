const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),assert=require('node:assert/strict'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'..'),stamp=new Date().toISOString().replace(/[-:.TZ]/g,'').slice(0,14),out=path.join(root,'artifacts',`ingest-startup-recovery-deploy-${stamp}`);
const docker=args=>cp.execFileSync('docker',args,{cwd:root,maxBuffer:192*1024*1024});
const inspect=service=>JSON.parse(docker(['inspect',docker(['compose','ps','-q',service]).toString().trim()]))[0];
const run=args=>{const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});assert.equal(result.status,0,'Deployment step failed')};
const query=sql=>docker(['compose','exec','-T','postgres','psql','-X','-v','ON_ERROR_STOP=1','-U','weikong','-d','weikong_iot','-At','-c',sql]).toString().trim();
const env=item=>Object.fromEntries(item.Config.Env.map(line=>{const index=line.indexOf('=');return[line.slice(0,index),line.slice(index+1)]}));
const config=JSON.parse(docker(['compose','config','--format','json'])),before=inspect('ingest'),liveEnv=env(before),configured=config.services.ingest.environment;
for(const[key,value]of Object.entries(configured))assert.equal(String(value??''),liveEnv[key]??'','Ingest setting would change: '+key);
const applied=JSON.parse(query(`SELECT COALESCE(json_agg(json_build_object('name',migration_name,'checksum',checksum)),'[]'::json) FROM "_prisma_migrations" WHERE finished_at IS NOT NULL AND rolled_back_at IS NULL;`));
const migrations=fs.readdirSync(path.join(root,'apps/api/prisma/migrations')).filter(name=>fs.existsSync(path.join(root,'apps/api/prisma/migrations',name,'migration.sql')));
for(const item of applied){assert.ok(migrations.includes(item.name));assert.equal(crypto.createHash('sha256').update(fs.readFileSync(path.join(root,'apps/api/prisma/migrations',item.name,'migration.sql'))).digest('hex'),item.checksum,'Historical migration changed: '+item.name)}
assert.deepEqual(migrations.filter(name=>!applied.some(item=>item.name===name)),[],'Unexpected pending database migration');
const preserved=Object.fromEntries(['api','web','admin-web','postgres','timescaledb','redis','emqx'].map(service=>[service,inspect(service).Id]));
const gatewayBefore=JSON.parse(query(`SELECT json_build_object('status',status,'lastSeenAt',"lastSeenAt") FROM "Device" WHERE "deviceType"='GATEWAY' ORDER BY "createdAt" DESC LIMIT 1;`));
fs.mkdirSync(out,{recursive:true});
const backup=JSON.parse(cp.execFileSync('node',['tools/backup-production-batch-deploy.cjs'],{cwd:root,encoding:'utf8',maxBuffer:4*1024*1024}).trim());assert.ok(backup.encryptedBackupVerified);
const rollbackTag=`wk-ingest-startup-recovery-rollback:${stamp}`;docker(['tag',before.Image,rollbackTag]);
const rollbackFile=path.join(out,'compose.rollback.json');fs.writeFileSync(rollbackFile,JSON.stringify({services:{ingest:{image:rollbackTag}}},null,2));
fs.writeFileSync(path.join(out,'before.json'),JSON.stringify({databaseBackupDirectory:backup.backupDirectory,rollbackTag,preserved,gatewayBefore,migrationCount:applied.length},null,2));
console.log(JSON.stringify({stage:'backup-verified',out,databaseBackupVerified:true,serviceToUpdate:'ingest',pendingMigrations:0}));
let replacing=false;
(async()=>{try{
  run(['compose','build','ingest']);replacing=true;run(['compose','up','-d','--no-deps','ingest']);
  let verified;
  for(let attempt=0;attempt<60;attempt++){
    const current=inspect('ingest');
    const subscriptions=docker(['compose','exec','-T','emqx','emqx','ctl','subscriptions','list']).toString();
    const expected=['weikong/devices/+/heartbeat','weikong/devices/+/telemetry','weikong/gateways/+/children/+/heartbeat','weikong/gateways/+/children/+/telemetry','weikong/gateways/+/topology/reported','weikong/gateways/+/events','weikong/factory/+/heartbeat'];
    const gateway=JSON.parse(query(`SELECT json_build_object('status',status,'lastSeenAt',"lastSeenAt") FROM "Device" WHERE "deviceType"='GATEWAY' ORDER BY "createdAt" DESC LIMIT 1;`));
    const logs=docker(['compose','logs','--since','2m','--tail','200','ingest']).toString();
    if(current.State.Running&&expected.every(topic=>subscriptions.includes(topic))&&logs.includes('[ingest] timescale ready')&&logs.includes('[ingest] subscribed to all MQTT ingestion topics')&&gateway.status==='ONLINE'&&new Date(gateway.lastSeenAt)>new Date(gatewayBefore.lastSeenAt)){verified={gateway,expectedTopicCount:expected.length};break}
    await new Promise(resolve=>setTimeout(resolve,1000));
  }
  assert.ok(verified,'Ingest did not restore subscriptions and gateway online state');
  const current=inspect('ingest'),currentEnv=env(current);for(const key of Object.keys(configured))assert.equal(currentEnv[key],liveEnv[key],'Ingest setting changed: '+key);
  for(const[service,id]of Object.entries(preserved))assert.equal(inspect(service).Id,id,'Unrelated service recreated: '+service);
  const after=JSON.parse(query(`SELECT json_build_object('factoryCount',(SELECT count(*) FROM "FactoryGateway"),'deviceCount',(SELECT count(*) FROM "Device"),'usersHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "User" t),'tenantsHash',(SELECT md5(COALESCE(string_agg(row_to_json(t)::text,'' ORDER BY id),'')) FROM "Tenant" t));`));
  for(const key of ['factoryCount','deviceCount','usersHash','tenantsHash'])assert.equal(after[key],backup.before[key],'Preserved data changed: '+key);
  const result={deployedAt:new Date().toISOString(),ingestUpdated:true,databaseMigrationRequired:false,timescaleReady:true,mqttSubscriptionsRestored:true,expectedTopicCount:verified.expectedTopicCount,gatewayOnline:true,gatewayLastSeenAt:verified.gateway.lastSeenAt,credentialsUnchanged:true,accountsUnchanged:true,tenantsUnchanged:true,deviceCountUnchanged:true,unrelatedServicesUnchanged:true,physicalPointWrites:false,backup:out,databaseBackupDirectory:backup.backupDirectory,rollbackImage:rollbackTag};
  fs.writeFileSync(path.join(out,'deployment.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
}catch(error){if(replacing){run(['compose','-f','docker-compose.yml','-f',rollbackFile,'up','-d','--no-deps','--no-build','ingest']);console.error('Ingest rolled back; database data was not restored or removed.')}throw error}})().catch(error=>{console.error(error.message);process.exitCode=1});
