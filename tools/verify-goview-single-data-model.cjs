const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const root=path.resolve(__dirname,'../apps/goview-designer'),vue=require(path.join(root,'node_modules/vue')),ts=require(path.join(root,'node_modules/typescript')),{parse,compileTemplate}=require(path.join(root,'node_modules/@vue/compiler-sfc'));
const file=path.join(root,'src/views/chart/ContentConfigurations/components/ChartData/components/ChartDataRequest/components/RequestTargetConfig/index.vue');
const source=fs.readFileSync(file,'utf8'),parsed=parse(source);assert.equal(parsed.errors.length,0);
assert.equal(compileTemplate({source:parsed.descriptor.template.content,filename:file,id:'single-model-test'}).errors.length,0);
const capability={exports:{}};new Function('exports',ts.transpileModule(fs.readFileSync(path.join(root,'src/utils/platformBindingCapabilities.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText)(capability.exports);
const script=parsed.descriptor.scriptSetup.content.replace(/^import .*$/gm,'');
const body=ts.transpileModule(script+'\nreturn {currentSelectOptions,requestHttpType,isGeoTrue};',{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText;
function init(key,saved,geo=false){const props=vue.reactive({targetList:{chartConfig:{key,chartKey:geo?'VMapAmap':key,title:geo?'高德地图':key}},targetDataRequest:{requestHttpType:saved,requestParams:{Params:[]}}});
 const names=['toRefs','onMounted','ref','watch','computed','defineProps','sessionStorage','useMessage','icon','selectTypeOptions','componentKeyOf','platformDataModelsOf'];
 const args=[vue.toRefs,()=>{},vue.ref,vue.watch,vue.computed,()=>props,{getItem:()=>null},()=>({warning:()=>{}}),{ionicons5:{}},[{value:'实时数据',label:'实时数据'},{value:'历史数据',label:'历史数据'}],capability.exports.componentKeyOf,capability.exports.platformDataModelsOf];
 const scope=vue.effectScope();const result=scope.run(()=>new Function(...names,body)(...args));return {result,props,stop:()=>scope.stop()};}
for(const [key,model] of [['Number','实时数据'],['TablesBasic','历史数据'],['TablesAdvanced','历史数据']]){const t=init(key,'');assert.equal(t.result.currentSelectOptions.value.length,1);assert.equal(t.result.requestHttpType.value,model);t.stop();}
const both=init('Bar','历史数据');assert.equal(both.result.currentSelectOptions.value.length,2);assert.equal(both.result.requestHttpType.value,'历史数据');both.stop();
const old=init('Number','历史数据');assert.equal(old.result.requestHttpType.value,'实时数据');old.stop();
const geo=init('VMapAmap','地图标点-高德',true);assert.equal(geo.result.requestHttpType.value,'地图标点-高德');geo.stop();
assert.match(source,/v-if="currentSelectOptions.length === 1"/);assert.match(source,/<n-select v-else v-model:value="requestHttpType"/);
console.log('PASS single data model: setup initialization without TDZ error, realtime/history auto-selection, invalid old value correction, multiple-model saved selection preserved, map unchanged, template compilation.');
