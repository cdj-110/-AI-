const assert=require('node:assert/strict');
const {DevicesService}=require('../dist/devices/devices.service');
async function main(){
 const service=Object.create(DevicesService.prototype);
 const actor={sub:'member',tenantId:'tenant',role:'TENANT_USER',deviceScopeMode:'CUSTOM'};
 let metricWhere;
 service.prisma={device:{findFirst:async args=>{
  assert.equal(args.include.children.where.tenantId,'tenant');
  assert.ok(args.include.children.where.OR,'child devices remain independently access-filtered');
  return {id:'gateway',name:'网关',deviceType:'GATEWAY',children:[{id:'child',name:'子设备'}]};
 }},deviceMetric:{findMany:async args=>{metricWhere=args.where;return [{deviceId:'gateway',identifier:'cpu'},{deviceId:'child',identifier:'value'}];}}};
 const rows=await service.metrics(actor,'gateway');
 assert.deepEqual(metricWhere.deviceId.in,['gateway','child']);
 assert.equal(rows[0].deviceName,'网关');assert.equal(rows[0].isGatewayMetric,true);
 assert.equal(rows[1].deviceName,'子设备');assert.equal(rows[1].isGatewayMetric,false);
 console.log('PASS gateway point owners: own/child distinction, actual owner identity, independent child access filter. Stub-only, no business writes.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
