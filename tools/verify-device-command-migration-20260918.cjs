const {execFileSync}=require('node:child_process');
const {readFileSync}=require('node:fs');
const {join}=require('node:path');
const root=join(__dirname,'..');
const run=(args,input)=>execFileSync('docker',args,{cwd:root,input,encoding:'utf8',stdio:['pipe','pipe','pipe']});
const name='wk-command-migration-test-'+Date.now();
const id=run(['compose','ps','-q','postgres']).trim();
const image=run(['inspect','--format','{{.Config.Image}}',id]).trim();
try{
 run(['run','--rm','-d','--name',name,'-e','POSTGRES_PASSWORD=isolated-test-only','-e','POSTGRES_DB=command_test',image]);
 for(let index=0;index<30;index++){try{run(['exec',name,'pg_isready','-U','postgres','-d','command_test']);break}catch{execFileSync('powershell',['-NoProfile','-Command','Start-Sleep -Milliseconds 200'])}}
 const migration=readFileSync(join(root,'apps/api/prisma/migrations/20260918040000_device_commands/migration.sql'),'utf8');
 const sql=`CREATE TABLE "Device" ("id" TEXT PRIMARY KEY);\n${migration}\nINSERT INTO "Device" VALUES ('gateway-a'),('gateway-b'),('child-a'),('child-b');
 INSERT INTO "DeviceCommand" ("id","requestId","deviceId","gatewayId","deviceKey","metric","value","createdBy","expiresAt") VALUES ('cmd-a','req-a','child-a','gateway-a','same-child-key','setpoint','45.15','admin',NOW()+INTERVAL '30 seconds'),('cmd-b','req-b','child-b','gateway-b','same-child-key','setpoint','46.15','admin',NOW()+INTERVAL '30 seconds');
 DO $$ BEGIN IF (SELECT COUNT(*) FROM "DeviceCommand") <> 2 THEN RAISE EXCEPTION 'Child identity scopes lost'; END IF;
 BEGIN INSERT INTO "DeviceCommand" SELECT 'duplicate',"requestId","deviceId","gatewayId","deviceKey","metric","value","status","message","createdBy","createdAt","expiresAt","completedAt" FROM "DeviceCommand" WHERE "id"='cmd-a'; RAISE EXCEPTION 'Request deduplication missing'; EXCEPTION WHEN unique_violation THEN NULL; END;
 END $$;
 DELETE FROM "Device" WHERE "id"='gateway-a';
 DO $$ BEGIN IF (SELECT COUNT(*) FROM "DeviceCommand") <> 1 THEN RAISE EXCEPTION 'Gateway cascade invalid'; END IF; END $$;
 DELETE FROM "Device" WHERE "id"='child-b';
 DO $$ BEGIN IF (SELECT COUNT(*) FROM "DeviceCommand") <> 0 THEN RAISE EXCEPTION 'Child cascade invalid'; END IF; END $$;`;
 run(['exec','-i',name,'psql','-U','postgres','-d','command_test','-v','ON_ERROR_STOP=1'],sql);
 console.log('Device command migration passed in disposable PostgreSQL: table/indexes, duplicate request rejection, same child key across gateways, gateway/child foreign-key cascades. Production database untouched.');
}finally{try{run(['stop',name])}catch{}}
