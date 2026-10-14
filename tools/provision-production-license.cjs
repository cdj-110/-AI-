// Provision secrets without printing them. Must run only after a verified DB backup.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),cp=require('node:child_process');
const root=path.resolve(__dirname,'..'),base=path.join(root,'secrets','authorization-20260916');
fs.mkdirSync(base,{recursive:true,mode:0o700});
const stateFile=path.join(base,'provision-state.json');
if(fs.existsSync(stateFile))throw new Error('Provision state already exists; refuse to rotate keys twice');
const inspected=JSON.parse(cp.execFileSync('docker',['inspect','weikong-iot-platform-api-1'],{encoding:'utf8'}))[0];
const oldEnv=Object.fromEntries(inspected.Config.Env.map(v=>{const p=v.indexOf('=');return[v.slice(0,p),v.slice(p+1)]}));
const envPath=path.join(root,'.env'),envRaw=fs.readFileSync(envPath,'utf8');
const settings={};for(const line of envRaw.split(/\r?\n/)){const m=line.match(/^([A-Z0-9_]+)=(.*)$/);if(m)settings[m[1]]=m[2].replace(/^(['"])(.*)\1$/,'$2');}
if(settings.ACTIVATION_SIGNING_PRIVATE_KEY||oldEnv.ACTIVATION_SIGNING_PRIVATE_KEY)throw new Error('Existing signing key detected; do not silently replace it');
const {privateKey,publicKey}=crypto.generateKeyPairSync('ed25519');
const keyId='wk-production-20260916-01';
const privateDer=privateKey.export({format:'der',type:'pkcs8'}).toString('base64');
const publicDer=publicKey.export({format:'der',type:'spki'}).toString('base64');
const credentialKey=oldEnv.FACTORY_CREDENTIAL_ENCRYPTION_KEY||crypto.randomBytes(32).toString('hex');
const updated={ACTIVATION_SIGNING_KEY_ID:keyId,ACTIVATION_SIGNING_PRIVATE_KEY:privateDer,FACTORY_CREDENTIAL_ENCRYPTION_KEY:credentialKey,ACTIVATION_CLOUD_API:'http://192.168.1.141:3100',ACTIVATION_REMOTE_ACCESS_ENDPOINT:'ws://192.168.1.141:3100/api/remote-access/tunnel',MQTT_PUBLIC_HOST:settings.MQTT_PUBLIC_HOST||'192.168.1.141'};
// Verify the deployed tunnel path against source before using this value.
const recovery=crypto.randomBytes(32).toString('base64url'),salt=crypto.randomBytes(16),iv=crypto.randomBytes(12);
const encryptionKey=crypto.scryptSync(recovery,salt,32,{N:32768,maxmem:128*1024*1024});
const cipher=crypto.createCipheriv('aes-256-gcm',encryptionKey,iv);
const backup=JSON.stringify({version:1,createdAt:new Date().toISOString(),keyId,privateDer,publicDer,credentialKey,previousEnvironment:oldEnv,previousRootEnv:envRaw});
const encrypted=Buffer.concat([cipher.update(backup),cipher.final()]);
const envelope=JSON.stringify({version:1,kdf:'scrypt',N:32768,salt:salt.toString('base64'),iv:iv.toString('base64'),tag:cipher.getAuthTag().toString('base64'),data:encrypted.toString('base64')});
for(const name of ['keys-backup-primary.aesgcm.json','keys-backup-secondary.aesgcm.json'])fs.writeFileSync(path.join(base,name),envelope,{mode:0o600,flag:'wx'});
fs.writeFileSync(path.join(base,'backup-recovery-password.txt'),recovery+'\n',{mode:0o600,flag:'wx'});
const decipher=crypto.createDecipheriv('aes-256-gcm',encryptionKey,iv);decipher.setAuthTag(cipher.getAuthTag());
if(Buffer.concat([decipher.update(encrypted),decipher.final()]).toString()!==backup)throw new Error('Encrypted key backup verification failed');
fs.writeFileSync(path.join(base,'platform-secrets.env'),Object.entries(updated).map(([k,v])=>`${k}=${v}`).join('\n')+'\n',{mode:0o600,flag:'wx'});
fs.writeFileSync(path.join(base,'gateway-public-keys.env'),`WK_ACTIVATION_PUBLIC_KEY=${publicDer}\nWK_ACTIVATION_TRUSTED_PUBLIC_KEYS='${JSON.stringify({[keyId]:publicDer})}'\n`,{mode:0o600,flag:'wx'});
// The transition key is needed only for the one-time transactional migration.
fs.writeFileSync(stateFile,JSON.stringify({keyId,publicKeyFingerprint:crypto.createHash('sha256').update(publicDer).digest('hex'),previousCredentialKey:oldEnv.FACTORY_CREDENTIAL_ENCRYPTION_KEY||oldEnv.JWT_SECRET||'weikong_iot_secret',credentialKey,credentialMigrationNeeded:!oldEnv.FACTORY_CREDENTIAL_ENCRYPTION_KEY,createdAt:new Date().toISOString()},{},2),{mode:0o600,flag:'wx'});
let next=envRaw;for(const [k,v] of Object.entries(updated)){const pattern=new RegExp(`^${k}=.*$`,'m');next=pattern.test(next)?next.replace(pattern,`${k}=${v}`):next.replace(/\s*$/,'')+`\n${k}=${v}\n`;}
fs.writeFileSync(envPath+'.license-stage',next,{mode:0o600});fs.renameSync(envPath+'.license-stage',envPath);
console.log(JSON.stringify({keyId,backupVerified:true,credentialMigrationNeeded:!oldEnv.FACTORY_CREDENTIAL_ENCRYPTION_KEY,directory:base}));
