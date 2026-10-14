const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),crypto=require('node:crypto');
const root=path.resolve(__dirname,'..'),base=path.join(root,'secrets/authorization-20260916');
const publicEnv=fs.readFileSync(path.join(base,'gateway-public-keys.env'),'utf8');
const values={};for(const line of publicEnv.trim().split(/\r?\n/)){const p=line.indexOf('=');values[line.slice(0,p)]=line.slice(p+1).replace(/^'(.*)'$/,'$1');}
const state=JSON.parse(fs.readFileSync(path.join(base,'provision-state.json'),'utf8'));
const ring=JSON.parse(values.WK_ACTIVATION_TRUSTED_PUBLIC_KEYS);if(!ring[state.keyId])throw new Error('Signing key is not trusted by gateway build');
const args=['run','--rm','-e',`WK_ACTIVATION_PUBLIC_KEY=${values.WK_ACTIVATION_PUBLIC_KEY}`,'-e',`WK_ACTIVATION_TRUSTED_PUBLIC_KEYS=${values.WK_ACTIVATION_TRUSTED_PUBLIC_KEYS}`,'-v',`${path.join(root,'apps/gateway-go')}:/src`,'wk-eg100-go-build:hotfix','sh','/src/deploy/eg100/build_remote_access_auto_20260916.sh'];
if(process.env.WK_OTA_TRUSTED_PUBLIC_KEYS) args.splice(6,0,'-e',`WK_OTA_TRUSTED_PUBLIC_KEYS=${process.env.WK_OTA_TRUSTED_PUBLIC_KEYS}`);
if(process.env.WK_APP_VERSION) args.splice(6,0,'-e',`WK_APP_VERSION=${process.env.WK_APP_VERSION}`);
if(process.env.WK_BUILD_TIME) args.splice(6,0,'-e',`WK_BUILD_TIME=${process.env.WK_BUILD_TIME}`);
const result=cp.spawnSync('docker',args,{cwd:root,stdio:'inherit'});if(result.status!==0)process.exit(result.status||1);
const binary=fs.readFileSync(path.join(root,'apps/gateway-go/build/remote-access-auto-release-20260916/weikong-gateway'));
if(!binary.includes(Buffer.from(values.WK_ACTIVATION_TRUSTED_PUBLIC_KEYS)))throw new Error('Production key ring missing from binary');
if(process.env.WK_OTA_TRUSTED_PUBLIC_KEYS && !binary.includes(Buffer.from(process.env.WK_OTA_TRUSTED_PUBLIC_KEYS)))throw new Error('OTA public key ring missing from binary');
if(binary.includes(Buffer.from(state.credentialKey)))throw new Error('Private platform credential encryption key leaked into binary');
fs.writeFileSync(path.join(base,'gateway-build-manifest.json'),JSON.stringify({keyId:state.keyId,publicKeyFingerprint:state.publicKeyFingerprint,binarySha256:crypto.createHash('sha256').update(binary).digest('hex'),version:process.env.WK_APP_VERSION||'V0.7',strictSignature:true,strictHardware:true,strictSecureChip:true,builtAt:new Date().toISOString()},null,2));
console.log('Production gateway build verified: correct public key ring, no platform credential key.');
