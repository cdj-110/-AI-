const cp = require('node:child_process');
const source = `const {PrismaClient}=require('@prisma/client');
const {authorizationStatus}=require('./dist/factory-gateways/license-status');
(async()=>{const p=new PrismaClient();try{
const factories=await p.factoryGateway.findMany();
console.log(JSON.stringify({factoryCount:factories.length,factories:factories.map(r=>({id:r.id,sn:r.sn,hardwareId:r.hardwareId,status:r.status,sequence:r.licenseSequence,reportedSequence:r.reportedLicense?.licenseSequence,licenseReportedAt:r.licenseReportedAt,licenseActivatedAt:r.licenseActivatedAt,...authorizationStatus(r)}))}));
}finally{await p.$disconnect()}})().catch(e=>{console.error(e.message);process.exitCode=1});`;
const r = cp.spawnSync('docker', ['compose', 'exec', '-T', 'api', 'node'], { input: source, encoding: 'utf8' });
process.stdout.write(r.stdout); process.stderr.write(r.stderr); process.exitCode = r.status || 0;
