require('reflect-metadata');
const assert=require('node:assert/strict'),{plainToInstance}=require('class-transformer'),{validate}=require('class-validator');
const {HistoryDataTasksService,SaveHistoryTaskDto}=require('../dist/devices/history-data-tasks.service');
const {HistoryDataTasksController}=require('../dist/devices/history-data-tasks.controller');
async function main(){
 const a={sub:'user-a',tenantId:'tenant-a',role:'TENANT_USER',username:'A'},b={...a,sub:'user-b'},admin={...a,role:'TENANT_ADMIN'};
 const points=[{value1:'aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa',value2:'temperature'}];
 const dto={name:'历史查询',pairs:points,start:'2026-10-13T01:00:00Z',end:'2026-10-13T02:00:00Z'};
 assert.deepEqual(await validate(plainToInstance(SaveHistoryTaskDto,dto)),[]);
 for(const bad of [{...dto,start:undefined},{...dto,end:undefined},{...dto,name:''},{...dto,pairs:[]},{...dto,pairs:Array(21).fill(points[0])}])assert.ok((await validate(plainToInstance(SaveHistoryTaskDto,bad))).length);
 const rows=[],calls=[];let allowed=true;
 const matches=(r,w)=>r.tenantId===w.tenantId&&(!w.createdById||r.createdById===w.createdById)&&(!w.id||r.id===w.id);
 const prisma={device:{findMany:async()=>allowed?[{id:points[0].value1,name:'设备A',deviceKey:'a'}]:[]},historyDataTask:{
 create:async({data})=>{const row={id:'task-a',createdAt:new Date(),updatedAt:new Date(),...data};rows.push(row);return row;},
 findFirst:async({where})=>rows.find(r=>matches(r,where))||null,
 findMany:async({where})=>rows.filter(r=>matches(r,where)),count:async({where})=>rows.filter(r=>matches(r,where)).length,
 update:async({where,data})=>Object.assign(rows.find(r=>r.id===where.id),data),delete:async({where})=>{rows.splice(rows.findIndex(r=>r.id===where.id),1);},
 }};
 const devices={telemetryHistoryTable:async(...args)=>{calls.push(['query',...args]);return{total:1};},exportHistory:async(...args)=>{calls.push(['export',...args]);return{csv:'mock'};}};
 const svc=new HistoryDataTasksService(prisma,devices);
 const created=await svc.save(a,dto);assert.equal(created.tenantId,'tenant-a');assert.equal(created.createdById,'user-a');assert.equal(calls[0][1],a);
 assert.equal((await svc.list(a)).total,1);assert.equal((await svc.list(b)).total,0);assert.equal((await svc.list(admin)).total,1);
 await assert.rejects(svc.get(b,created.id),/无权/);await assert.rejects(svc.get({...a,tenantId:'tenant-b'},created.id),/无权/);
 await svc.query(a,created.id,2,20);assert.equal(calls.at(-1)[1],a);assert.deepEqual(calls.at(-1)[2],points);assert.equal(calls.at(-1)[3],dto.start.replace('Z','.000Z'));
 await svc.export(a,created.id);assert.equal(calls.at(-1)[0],'export');assert.deepEqual(calls.at(-1)[2],points);
 const filter={deviceId:points[0].value1,pointKey:JSON.stringify([points[0].value1,points[0].value2]),keyword:'设备A',valueMin:'0',valueMax:'42'};
 await svc.query(a,created.id,1,20,filter);assert.equal(calls.at(-1)[7],'设备A');assert.deepEqual(calls.at(-1)[9],{min:0,max:42});
 await svc.export(a,created.id,filter);assert.equal(calls.at(-1)[5],'设备A');assert.deepEqual(calls.at(-1)[6],{min:0,max:42});
 for(const invalid of [{deviceId:'outside'},{pointKey:'outside'},{valueMin:'0'},{...filter,valueMin:'Infinity'},{...filter,valueMin:'50'},{start:'2020-01-01T00:00:00Z'}])await assert.rejects(svc.query(a,created.id,1,20,invalid));
 assert.deepEqual(rows[0].pairs,points,'view filters do not modify task');
 allowed=false;assert.equal((await svc.list(a)).items[0].canView,false);await assert.rejects(svc.query(a,created.id),/权限已变更/);await assert.rejects(svc.export(a,created.id),/权限已变更/);
 allowed=true;await svc.save(a,{...dto,name:'修改名称'},created.id);assert.equal(rows[0].name,'修改名称');
 await assert.rejects(svc.save(a,{...dto,pairs:[...points,...points]}),/重复/);
 await assert.rejects(svc.remove(b,created.id),/无权/);await svc.remove(a,created.id);assert.equal(rows.length,0);
 assert.deepEqual(Reflect.getMetadata('roles',HistoryDataTasksController),['TENANT_ADMIN','TENANT_USER']);assert.deepEqual(Reflect.getMetadata('permissions',HistoryDataTasksController),['DEVICE_VIEW']);
 console.log('PASS history tasks: required DTO fields, saved config, tenant/member isolation, admin scope, revoked device access, fixed-condition query/export, edit/delete without telemetry writes.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
