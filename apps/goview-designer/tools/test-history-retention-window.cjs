const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),ts=require('typescript');
const {parse,compileScript,compileTemplate}=require('@vue/compiler-sfc');
let calls=0,token='tenant-a';
const now=Date.now(),cutoff=new Date(now-7*86400000).toISOString();
const source=fs.readFileSync(path.resolve(__dirname,'../src/utils/historyRetentionWindow.ts'),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText;
const mod={exports:{}};
new Function('exports','require','module',compiled)(mod.exports,name=>{
 if(name==='vue')return require('vue');
 if(name==='@/utils/platformAuth')return{readPlatformToken:()=>token};
 if(name==='@/utils/request')return{request:async()=>{calls++;return{queryEarliestAt:cutoff,latestAt:new Date().toISOString(),label:'7天'}}};
 if(name==='./platformHistoryChart')return{formatHistoryDate:date=>new Date(date.getTime()+8*3600000).toISOString().slice(0,19).replace('T',' ')};
 throw Error(name);
},mod);
async function main(){
 const h=mod.exports;
 assert.equal(h.historyDateDisabled(now-86400000),true,'fail closed before entitlement available');
 await Promise.all([h.refreshHistoryWindow(),h.refreshHistoryWindow()]);assert.equal(calls,1,'deduplicated');
 assert.equal(h.historyDateDisabled(now+86400000),true);
 assert.equal(h.historyDateDisabled(now-9*86400000),true);
 assert.equal(h.historyDateDisabled(now-86400000),false);
 assert.equal(h.historyHoursAllowed(24),true);assert.equal(h.historyHoursAllowed(24*8),false);
 assert.equal(h.normalizeHistoryRange(['2020-01-01 00:00:00','2020-01-02 00:00:00']).adjusted,true);
 assert.equal(h.normalizeHistoryRange(['2099-01-01 00:00:00','2099-01-02 00:00:00']).adjusted,true);
 const normal=h.normalizeHistoryRange(null);assert.equal(h.normalizeHistoryRange(normal.range).adjusted,false);
 token='tenant-b';await h.refreshHistoryWindow();assert.equal(calls,2,'session change invalidates cache');
 for(const file of ['src/views/preview/components/HistoryQueryChart.vue','src/packages/components/Tables/Tables/TablesAdvanced/index.vue','src/packages/components/Tables/Tables/TablesBasic/index.vue','src/views/chart/ContentConfigurations/components/ChartData/components/ChartDataRequest/components/RequestTargetConfig/index.vue']){
  const {descriptor,errors}=parse(fs.readFileSync(path.resolve(__dirname,'..',file),'utf8'),{filename:file});assert.equal(errors.length,0);
  const script=compileScript(descriptor,{id:file});const result=compileTemplate({source:descriptor.template.content,filename:file,id:file,compilerOptions:{bindingMetadata:script.bindings}});assert.equal(result.errors.length,0);
 }
 console.log('PASS history window: dates, shortcuts, old-range adjustment, deduplication, session isolation, four Vue SFCs.');
}
main().catch(e=>{console.error(e);process.exitCode=1});
