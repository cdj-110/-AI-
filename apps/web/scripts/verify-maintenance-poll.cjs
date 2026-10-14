const fs=require('node:fs'),assert=require('node:assert/strict'),ts=require('typescript');
const source=fs.readFileSync('src/views/FactoryGateways.vue','utf8');
function functionSource(name){const start=source.indexOf(`async function ${name}(`);assert.ok(start>=0);const next=source.indexOf('\n}',start)+2;return source.slice(start,next);}
const refs=()=>({value:undefined});
async function main(){
 const maintenanceVisible={value:true},maintenanceGatewayId={value:'a'},maintenanceState=refs(),maintenanceTasks=refs(),maintenanceError={value:''},gateways={value:[{id:'a',lastSeenAt:'stale'}]};
 let mode='ok',pending=[],calls=0;
 const apiRequest=async options=>{calls++;if(mode==='fail')throw Error('network');if(mode==='deferred')return new Promise(resolve=>pending.push({options,resolve}));return options.url.endsWith('status')?{online:true,canMaintain:true,lastSeenAt:'fresh',activated:true}:[];};
 const output=ts.transpileModule(functionSource('loadMaintenanceTasks'),{compilerOptions:{target:ts.ScriptTarget.ES2020}}).outputText;
 const test=new Function('maintenanceVisible','maintenanceGatewayId','maintenanceState','maintenanceTasks','maintenanceError','gateways','apiRequest',`let maintenanceGeneration=1,maintenanceInFlight=-1;${output};return {load:loadMaintenanceTasks,close:()=>{maintenanceGeneration++;maintenanceVisible.value=false;}};`)(maintenanceVisible,maintenanceGatewayId,maintenanceState,maintenanceTasks,maintenanceError,gateways,apiRequest);
 await test.load();assert.equal(maintenanceState.value.online,true);assert.equal(gateways.value[0].lastSeenAt,'fresh');assert.equal(calls,2);
 mode='fail';await test.load();assert.equal(maintenanceState.value,undefined);assert.match(maintenanceError.value,/获取失败/);
 mode='deferred';const loading=test.load();await test.load();assert.equal(pending.length,2,'poll requests do not overlap');test.close();pending.forEach(p=>p.resolve(p.options.url.endsWith('status')?{online:true,lastSeenAt:'late'}:[]));await loading;
 assert.equal(maintenanceState.value,undefined,'closed dialog rejects old responses');assert.equal(gateways.value[0].lastSeenAt,'fresh');
 assert.match(source,/\/status/);assert.ok(!source.includes('gatewayRecentlyOnline'));
 console.log('PASS maintenance polling: latest state fetch, row heartbeat synchronization, failure not offline, no overlap, closed-dialog stale response ignored.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
