const assert=require('node:assert/strict');
const {parseLicenseReport,persistLicenseReport}=require('../dist/license-report');
const gateway={sn:'WK-TEST',hardwareId:'A'.repeat(32),chipUsid:'B'.repeat(16),licenseSequence:4};
const report={sn:gateway.sn,hardwareId:gateway.hardwareId,chipUsid:gateway.chipUsid,licenseVersion:2,licenseSequence:4,keyId:'key-1',licenseId:'c'.repeat(64),issuedAt:'2026-09-17T00:00:00.000Z',expiresAt:'2036-09-17T00:00:00.000Z',features:['collection','cloud'],valid:true,restricted:false};
gateway.issuedLicense={...report};
const parse=value=>parseLicenseReport(Buffer.from(JSON.stringify({authorization:value})),gateway);
assert.deepEqual(parse({...report,deviceSecret:'must-not-store',signature:'must-not-store',broker:'must-not-store'}),report);
for(const change of [{sn:'OTHER'},{hardwareId:'D'.repeat(32)},{chipUsid:'D'.repeat(16)},{licenseSequence:5},{licenseSequence:-1},{licenseSequence:1.5},{licenseId:'d'.repeat(64)},{keyId:'other-key'},{expiresAt:'invalid'},{expiresAt:'2040-01-01T00:00:00Z'},{features:['forged-feature']},{valid:'true'},{restricted:true},{licenseVersion:1}])assert.equal(parse({...report,...change}),undefined,JSON.stringify(change));
assert.equal(parseLicenseReport(Buffer.from('{'),gateway),undefined);
assert.equal(parseLicenseReport(Buffer.alloc(16385),gateway),undefined);
assert.equal(parseLicenseReport(Buffer.from('{"timestamp":"old-heartbeat"}'),gateway),undefined);
assert.equal(parse({...report,licenseSequence:3}).licenseSequence,3,'old active license should remain visible as pending update');
assert.equal(parse({...report,valid:false,restricted:true}).valid,false);
assert.equal(parse({...report,expiresAt:''}),undefined,'timed license report forged perpetual expiry');
const perpetual={...report,expiresAt:''};
gateway.issuedLicense={...perpetual};
assert.deepEqual(parse(perpetual),perpetual);
for(const expiresAt of [undefined,null,'invalid',report.expiresAt])assert.equal(parse({...perpetual,expiresAt}),undefined,'perpetual license metadata was not checked');
gateway.issuedLicense={...report};
assert.ok(parseLicenseReport(Buffer.from(JSON.stringify({authorization:report})),{...gateway,issuedLicense:null}),'existing signed devices can report without fabricated historical activation');
console.log('License report validation passed: identities, sequence, issuance fingerprint, whitelist, expiry, legacy heartbeat and malformed inputs.');
(async()=>{
 const now=new Date('2099-01-01T00:00:00Z'),identity={...gateway,id:'test-only',status:'UNBOUND'};
 for(const [value,count,expected] of [[perpetual,1,2],[perpetual,0,1],[{...perpetual,valid:false,restricted:true},1,1],[report,1,1]]){
  const calls=[];
  const tx={factoryGateway:{updateMany:async args=>{calls.push(args);return{count}}}};
  assert.equal(await persistLicenseReport(tx,identity,value,now),count>0);
  assert.equal(calls.length,expected,'incorrect first activation confirmation');
  if(expected===2)assert.equal(calls[1].data.licenseActivatedAt,now);
 }
 console.log('Perpetual report persistence passed: first activation, duplicate rejection, restricted state and finite expiry.');
})().catch(error=>{console.error(error);process.exitCode=1});
