const cp = require('node:child_process');
const source = `require('reflect-metadata');
const c=require('crypto'),{PrismaClient}=require('@prisma/client'),{ConfigService}=require('@nestjs/config'),{FactoryGatewaysService}=require('./dist/factory-gateways/factory-gateways.service'),bcrypt=require('bcryptjs');
(async()=>{const p=new PrismaClient(),s=new FactoryGatewaysService(p,new ConfigService(process.env));try{
const rows=await p.factoryGateway.findMany({select:{bindCodeEncrypted:true,bindCodeHash:true}});let count=0;
for(const r of rows){if(r.bindCodeEncrypted){if(!await bcrypt.compare(s.decryptCredential(r.bindCodeEncrypted,'bind-code'),r.bindCodeHash))throw Error('Existing credential mismatch');count++}}
const a=s.createActivationFile({hardwareId:'TEST-NOT-A-REAL-DEVICE',chipUsid:'0000000000000000',sn:'TEST-ONLY',deviceSecret:c.randomBytes(32).toString('hex'),licenseSequence:1}),sig=a.signature;delete a.signature;
const k=c.createPrivateKey({key:Buffer.from(process.env.ACTIVATION_SIGNING_PRIVATE_KEY,'base64'),type:'pkcs8',format:'der'});
if(!c.verify(null,Buffer.from(JSON.stringify(a)),c.createPublicKey(k),Buffer.from(sig,'base64')))throw Error('Invalid production signature');
console.log(JSON.stringify({apiReady:true,existingCredentialsVerified:count,keyId:a.keyId,signatureValid:true,credentialKeyConfigured:!!process.env.FACTORY_CREDENTIAL_ENCRYPTION_KEY}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
const result=cp.spawnSync('docker',['exec','-i','weikong-iot-platform-api-1','node'],{input:source,encoding:'utf8'});
process.stdout.write(result.stdout);process.stderr.write(result.stderr);process.exitCode=result.status||0;
