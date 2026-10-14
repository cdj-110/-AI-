const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const {Client}=require('pg');
(async()=>{
 const stateFile=path.resolve(__dirname,'../../../secrets/authorization-20260916/provision-state.json');
 const state=JSON.parse(fs.readFileSync(stateFile,'utf8'));
 if(!state.credentialMigrationNeeded){console.log('Credential encryption key migration already complete or not required.');return;}
 const oldKey=crypto.createHash('sha256').update(state.previousCredentialKey).digest(),newKey=crypto.createHash('sha256').update(state.credentialKey).digest();
 const decode=(value,purpose)=>{const [version,iv,tag,data]=value.split('.');if(version!=='v1')throw new Error('unsupported credential version');const d=crypto.createDecipheriv('aes-256-gcm',oldKey,Buffer.from(iv,'base64url'));d.setAAD(Buffer.from(`factory-gateway:${purpose}:v1`));d.setAuthTag(Buffer.from(tag,'base64url'));return Buffer.concat([d.update(Buffer.from(data,'base64url')),d.final()]);};
 const encode=(raw,purpose)=>{const iv=crypto.randomBytes(12),c=crypto.createCipheriv('aes-256-gcm',newKey,iv);c.setAAD(Buffer.from(`factory-gateway:${purpose}:v1`));const data=Buffer.concat([c.update(raw),c.final()]);return `v1.${iv.toString('base64url')}.${c.getAuthTag().toString('base64url')}.${data.toString('base64url')}`;};
 const env=fs.readFileSync(path.resolve(__dirname,'../.env'),'utf8');const match=env.match(/^DATABASE_URL=(.*)$/m);if(!match)throw new Error('DATABASE_URL missing');
 const client=new Client({connectionString:match[1].replace(/^(['"])(.*)\1$/,'$2')});await client.connect();let committed=false,count=0;
 try{
  await client.query('BEGIN');
  const {rows}=await client.query('SELECT id,"bindCodeEncrypted","nfcTokenEncrypted" FROM "FactoryGateway" FOR UPDATE');
  for(const row of rows){const bind=row.bindCodeEncrypted?encode(decode(row.bindCodeEncrypted,'bind-code'),'bind-code'):null;const nfc=row.nfcTokenEncrypted?encode(decode(row.nfcTokenEncrypted,'nfc-token'),'nfc-token'):null;if(bind||nfc){await client.query('UPDATE "FactoryGateway" SET "bindCodeEncrypted"=$1,"nfcTokenEncrypted"=$2 WHERE id=$3',[bind,nfc,row.id]);count++;}}
  await client.query('COMMIT');committed=true;
  state.credentialMigrationNeeded=false;state.migratedCredentialRows=count;state.credentialMigratedAt=new Date().toISOString();fs.writeFileSync(stateFile,JSON.stringify(state,null,2),{mode:0o600});
  console.log(JSON.stringify({credentialMigrationCommitted:true,rows:count}));
 }catch(e){if(!committed)await client.query('ROLLBACK');throw e;}finally{await client.end();}
})().catch(e=>{console.error(e.message);process.exitCode=1});
