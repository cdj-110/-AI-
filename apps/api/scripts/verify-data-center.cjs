require('reflect-metadata');
const assert=require('node:assert/strict');
const {plainToInstance}=require('class-transformer');
const {validate}=require('class-validator');
const {HistoryQueryDto}=require('../dist/devices/dto/history-query.dto');
const {DevicesController}=require('../dist/devices/devices.controller');
async function main(){
 const valid={pairs:[{value1:'aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',value2:'temperature'}],start:'2026-10-13T01:00:00Z',end:'2026-10-13T02:00:00Z',page:1,pageSize:20};
 assert.deepEqual(await validate(plainToInstance(HistoryQueryDto,valid)),[]);
 for(const data of [{...valid,pairs:[]},{...valid,pairs:Array(21).fill(valid.pairs[0])},{...valid,pairs:[{value1:'bad',value2:'temp'}]},{...valid,pairs:[{value1:valid.pairs[0].value1,value2:'sql; DROP'}]},{...valid,pageSize:101},{...valid,start:'invalid'}])assert.ok((await validate(plainToInstance(HistoryQueryDto,data))).length);
 let called;const user={role:'TENANT_USER',tenantId:'tenant-a',sub:'user-a'};
 const controller=new DevicesController({historyWindow:a=>(called=['window',a]),telemetryHistoryTable:(...args)=>(called=['query',...args]),exportHistory:(...args)=>(called=['export',...args])},{});
 controller.historyRetention(user);assert.equal(called[1],user);
 controller.historyData(user,valid);assert.equal(called[1],user);assert.deepEqual(called[2],valid.pairs);assert.equal(called[5],1);
 controller.exportData(user,valid);assert.equal(called[1],user);assert.deepEqual(called[2],valid.pairs);
 assert.deepEqual(Reflect.getMetadata('roles',DevicesController.prototype.historyData),['TENANT_ADMIN','TENANT_USER']);
 assert.deepEqual(Reflect.getMetadata('roles',DevicesController.prototype.exportData),['TENANT_ADMIN','TENANT_USER']);
  assert.deepEqual(Reflect.getMetadata('permissions',DevicesController),['DEVICE_VIEW']);
 console.log('PASS data center: nested point/time/paging validation, tenant roles, actor propagation to existing scoped retention/query/export services.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
