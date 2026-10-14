const assert = require('node:assert/strict');
const { maintenanceState } = require('../dist/gateway-maintenance/maintenance-state');
const { GatewayMaintenanceService } = require('../dist/gateway-maintenance/gateway-maintenance.service');
async function main() {
  const now=new Date('2026-10-13T04:00:00Z');
  const gateway={status:'BOUND',lastSeenAt:new Date(now-15000),licenseActivatedAt:new Date(now-100000),deviceSecretHash:'never-return-this',licenseSequence:10,licenseReportedAt:new Date(now-15000),reportedLicense:{licenseSequence:10,expiresAt:'',valid:true,restricted:false}};
  let state=maintenanceState(gateway,true,now);
  assert.equal(state.online,true);assert.equal(state.canMaintain,true);assert.equal(state.authorizationState,'ACTIVE');
  assert.equal(JSON.stringify(state).includes('never-return-this'),false);
  assert.equal(maintenanceState({...gateway,lastSeenAt:new Date(now-120000)},true,now).online,true);
  assert.equal(maintenanceState({...gateway,lastSeenAt:new Date(now-120001)},true,now).online,false);
  state=maintenanceState({...gateway,licenseActivatedAt:null},true,now);assert.equal(state.online,true);assert.equal(state.canMaintain,false);assert.match(state.reason,/激活/);
  state=maintenanceState(gateway,false,now);assert.equal(state.online,true);assert.equal(state.canMaintain,false);assert.match(state.reason,/通道/);
  state=maintenanceState({...gateway,device:{cloudAccessEnabled:false}},true,now);assert.equal(state.canMaintain,false);assert.match(state.reason,/云连接/);
  state=maintenanceState({...gateway,device:{cloudAccessEnabled:true,tenant:{deviceCloudAccessDisabled:true}}},true,now);assert.equal(state.canMaintain,false);
  assert.equal(maintenanceState({...gateway,lastSeenAt:null},true,now).online,false);
  assert.equal(maintenanceState({...gateway,lastSeenAt:new Date(now.getTime()+60000)},true,now).online,false);
  const service=new GatewayMaintenanceService({factoryGateway:{findUnique:async()=>({...gateway,lastSeenAt:new Date()})}},{});
  service.client={connected:true};service.ready=true;
  assert.equal((await service.status('aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa')).online,true);
  await assert.rejects(service.status('bad'),/编号/);
  console.log('PASS maintenance status: fresh heartbeat, exact timeout, activation separate from online, channel/cloud policy gates, read-only response, no secret exposure.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
