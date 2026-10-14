<script setup lang="ts">
import { dataTypeLabel } from '../dataTypeLabels.ts'
import { computed, ref } from 'vue'
import { api } from '../api'
import { generatedPointName, nextGeneratedPointMetric } from '../pointNaming'
import OpcuaTreeNode from './OpcuaTreeNode.vue'
import Iec61850TreeNode from './Iec61850TreeNode.vue'

const props=defineProps<{device:any;points:any[];allPoints?:any[];namePrefix?:string;nextMetric?:(preferred?:string)=>string;pointName?:(metric:string)=>string;readonly:boolean}>()
const open=ref(false),busy=ref(false),error=ref(''),message=ref(''),filter=ref('')
const opcNodes=ref<any[]>([]),opcSelected=ref<string[]>([])
const s7Candidates=ref<any[]>([]),s7Area=ref('auto'),s7DB=ref(1),s7Start=ref(0),s7End=ref(20),s7Type=ref('auto')
const iecNodes=ref<any[]>([]),iecSelected=ref<string[]>([]),iecFile=ref<HTMLInputElement>()
const dltItems=ref<any[]>([]),dltSelected=ref<string[]>([]),dltCategory=ref('全部')

const isOPCBrowser=computed(()=>props.device.protocol==='opcua'||props.device.protocol==='opcda')
const label=computed(()=>props.device.protocol==='opcua'?'浏览 OPC UA 节点':props.device.protocol==='opcda'?'一键浏览 OPC DA 点位':props.device.protocol==='dlt645'?'选择标准点位':props.device.protocol==='siemens-s7'?'一键扫描 S7 点位':props.device.protocol==='iec61850'?'选择 IEC61850 点位':'')
const modalTitle=computed(()=>props.device.protocol==='opcua'?'浏览服务器中的 OPC UA 节点':props.device.protocol==='opcda'?'浏览 OPC DA 服务器点位':props.device.protocol==='dlt645'?'选择 DL/T 645 标准点位':props.device.protocol==='siemens-s7'?'扫描并选择 S7 点位':'选择 IEC61850 点位')
const selectedCandidates=computed(()=>props.device.protocol==='dlt645'?dltSelected.value.length:props.device.protocol==='siemens-s7'?s7Candidates.value.filter(item=>item.selected).length:iecSelected.value.length)
const dltCategories=computed(()=>['全部',...new Set(dltItems.value.map(item=>String(item.category||'其他')))])
const filteredDLTItems=computed(()=>{const query=filter.value.trim().toLowerCase();return dltItems.value.filter(item=>(dltCategory.value==='全部'||item.category===dltCategory.value)&&(!query||`${item.name} ${item.key} ${item.di2007||''} ${item.di1997||''}`.toLowerCase().includes(query)))})

function reset(){error.value='';message.value='';filter.value='';opcNodes.value=[];opcSelected.value=[];s7Candidates.value=[];iecNodes.value=[];iecSelected.value=[];dltSelected.value=[];dltCategory.value='全部'}
async function show(){reset();open.value=true;if(isOPCBrowser.value)await loadOpcRoot();else if(props.device.protocol==='dlt645')await loadDLTDictionary()}
function close(){if(!busy.value)open.value=false}
function nodeFrom(raw:any){return{...raw,children:[],expanded:false,loaded:false,loading:false,selecting:false}}
async function browseOpc(nodeId:string,recursive=false){return api<{ok:boolean;nodes:any[];message?:string}>('/api/opcua/browse',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({address:props.device.address||'',username:props.device.username||'',password:props.device.password||'',nodeId,recursive}),timeoutMs:recursive?30000:20000})}
function opcdaBrowseBody(branchItemId:string,recursive=false){return{deviceKey:props.device.deviceKey||'',protocol:'opcda',address:props.device.address||'',progId:props.device.progId||'',clsid:props.device.clsid||'',domain:props.device.domain||'',workstation:props.device.workstation||'',username:props.device.username||'',password:props.device.password||'',securityMode:props.device.securityMode||'',authentication:props.device.authentication||'',authenticationLevel:props.device.authenticationLevel||'',connectTimeoutMilliseconds:props.device.connectTimeoutMilliseconds||0,requestTimeoutMilliseconds:props.device.requestTimeoutMilliseconds||0,groupName:props.device.groupName||'',requestedUpdateRateMilliseconds:props.device.requestedUpdateRateMilliseconds||0,deadband:props.device.deadband||0,localeId:props.device.localeId||0,localBindIp:props.device.localBindIp||'',branchItemId,recursive,limit:recursive?10000:1000}}
async function browseOpcda(branchItemId:string,recursive=false){return api<{ok:boolean;nodes:any[];message?:string}>('/api/opcda/browse',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(opcdaBrowseBody(branchItemId,recursive)),timeoutMs:recursive?120000:60000})}
async function browseCurrentOpc(nodeId:string,recursive=false){return props.device.protocol==='opcda'?browseOpcda(nodeId,recursive):browseOpc(nodeId,recursive)}
function normalizeOpcNode(raw:any){const type=String(raw.dataType||'').toLowerCase(),supportedArray=type==='float64[]'||type==='string[]';return nodeFrom(props.device.protocol==='opcda'?{...raw,nodeId:raw.itemId,displayName:raw.name,browseName:raw.name,nodeClass:raw.isBranch?'目录':type.endsWith('[]')?(supportedArray?'数组':'数组（当前版本暂不支持）'):'点位',hasChildren:Boolean(raw.isBranch),selectable:Boolean(raw.selectable)}:raw)}
const opcRootId=()=>props.device.protocol==='opcda'?'':'i=84'
async function loadOpcRoot(){busy.value=true;error.value='';try{const result=await browseCurrentOpc(opcRootId());opcNodes.value=(result.nodes||[]).map(normalizeOpcNode);message.value='已连接，展开目录并选择点位'}catch(reason){error.value=reason instanceof Error?reason.message:`浏览 ${props.device.protocol==='opcda'?'OPC DA':'OPC UA'} 点位失败`}finally{busy.value=false}}
async function expandOpc(node:any){if(!node.hasChildren)return;if(node.expanded){node.expanded=false;return}node.expanded=true;if(node.loaded)return;node.loading=true;try{const result=await browseCurrentOpc(node.nodeId);node.children=(result.nodes||[]).map(normalizeOpcNode);node.loaded=true}catch(reason){error.value=reason instanceof Error?reason.message:'加载子节点失败'}finally{node.loading=false}}
function toggleOpc(node:any){opcSelected.value=opcSelected.value.includes(node.nodeId)?opcSelected.value.filter(key=>key!==node.nodeId):[...opcSelected.value,node.nodeId];node._selectedData=node}
function collectLoaded(nodes:any[],target:Map<string,any>){for(const node of nodes){if(node.selectable)target.set(node.nodeId,node);collectLoaded(node.children||[],target)}}
function selectedOpcNodes(){const found=new Map<string,any>();collectLoaded(opcNodes.value,found);return opcSelected.value.map(key=>found.get(key)).filter(Boolean)}
async function selectOpcFolder(node:any){node.selecting=true;error.value='';try{const result=await browseCurrentOpc(node.nodeId,true);const normalized=(result.nodes||[]).map(normalizeOpcNode),known=new Set(opcSelected.value);for(const child of normalized)if(child.selectable&&child.nodeId)known.add(child.nodeId);opcSelected.value=[...known];const byId=new Map(normalized.map(child=>[child.nodeId,child]));node._recursiveNodes=byId;message.value=`已选择 ${normalized.filter(item=>item.selectable).length} 个下级点位`}catch(reason){error.value=reason instanceof Error?reason.message:'选择下级点位失败'}finally{node.selecting=false}}
async function selectAllOpc(){busy.value=true;try{const result=await browseCurrentOpc(opcRootId(),true),nodes=(result.nodes||[]).map(normalizeOpcNode).filter(item=>item.selectable&&item.nodeId);opcSelected.value=nodes.map(item=>item.nodeId);(opcNodes.value as any)._recursiveNodes=new Map(nodes.map(item=>[item.nodeId,item]));message.value=`已选择 ${nodes.length} 个点位`}catch(reason){error.value=reason instanceof Error?reason.message:'选择全部点位失败'}finally{busy.value=false}}

function mapOpcType(value:string){const type=String(value||'').toLowerCase();if(type==='float64[]'||type==='string[]')return type;if(type.includes('bool'))return'bool';if(type.includes('double')||type.includes('float64'))return'float64';if(type.includes('float'))return'float32';if(type.includes('uint64'))return'uint64';if(type.includes('int64'))return'int64';if(type.includes('uint32'))return'uint32';if(type.includes('int32'))return'int32';if(type.includes('uint16'))return'uint16';if(type.includes('int16'))return'int16';if(type.includes('uint8'))return'uint8';if(type.includes('int8'))return'int8';if(type.includes('datetime')||type.includes('date'))return'datetime';if(type.includes('string'))return'string';return'auto'}
function normalizeOpcdaAccess(value:unknown){const access=String(value||'').trim().toLowerCase();if(access==='write')return'write';if(access==='read_write'||access==='readwrite'||access==='read/write')return'read_write';return'read'}
function appendPoint(source:any,prefix:string){
  const{selected:_,value:__,...candidate}=source
  // Reserve identifiers through the same allocator as manual point creation.
  const metric=props.nextMetric?.()||nextGeneratedPointMetric([...(props.allPoints||[]),...props.points],prefix)
  const name=props.pointName?props.pointName(metric):generatedPointName(props.namePrefix||props.device.name||props.device.deviceKey,metric)
  props.points.push({...candidate,name,deviceKey:props.device.deviceKey,channelKey:props.device.channelKey,protocol:props.device.protocol,address:props.device.address,metric,quantity:candidate.quantity||1,dataType:candidate.dataType||'auto',byteOrder:candidate.byteOrder||'big',wordOrder:candidate.wordOrder||'big',scale:candidate.scale??1,offset:candidate.offset??0,decimals:candidate.decimals??0})
}
function confirmOpc(){let nodes=selectedOpcNodes();const recursiveMaps:any[]=[];for(const node of opcNodes.value){if(node._recursiveNodes)recursiveMaps.push(node._recursiveNodes);const stack=[...(node.children||[])];while(stack.length){const child=stack.shift();if(child._recursiveNodes)recursiveMaps.push(child._recursiveNodes);stack.push(...(child.children||[]))}}const rootMap=(opcNodes.value as any)._recursiveNodes;if(rootMap)recursiveMaps.push(rootMap);const byId=new Map(nodes.map(node=>[node.nodeId,node]));for(const map of recursiveMaps)for(const[key,node]of map)if(opcSelected.value.includes(key))byId.set(key,node);nodes=[...byId.values()];if(!nodes.length){error.value='请至少选择一个点位';return}if(props.device.protocol==='opcda'){const existing=new Set(props.points.map(item=>item.itemId));for(const node of nodes)if(!existing.has(node.nodeId))appendPoint({name:node.displayName||node.nodeId,itemId:node.nodeId,access:normalizeOpcdaAccess(node.access),requestedDataType:'auto',dataType:mapOpcType(node.dataType),active:true,enabled:true},'opcda')}else{const existing=new Set(props.points.map(item=>item.nodeId));for(const node of nodes)if(!existing.has(node.nodeId))appendPoint({name:node.displayName||node.browseName||node.nodeId,nodeId:node.nodeId,dataType:mapOpcType(node.dataType)},'opcua')}open.value=false}

async function loadDLTDictionary(){busy.value=true;try{const result=await api<{items:any[]}>('/api/dlt645/dictionary');dltItems.value=result.items||[];message.value=`已加载 ${dltItems.value.length} 个标准数据项`}catch(reason){error.value=reason instanceof Error?reason.message:'加载 DL/T 645 标准点位失败'}finally{busy.value=false}}
function toggleDLT(item:any){dltSelected.value=dltSelected.value.includes(item.key)?dltSelected.value.filter(key=>key!==item.key):[...dltSelected.value,item.key]}
function selectAllDLT(value:boolean){dltSelected.value=value?filteredDLTItems.value.map(item=>item.key):[]}
function confirmDLT(){
  const existing=new Set(props.points.map(point=>point.dlt645?.itemKey).filter(Boolean))
  for(const item of dltItems.value)if(dltSelected.value.includes(item.key)&&!existing.has(item.key)){
    appendPoint({dataType:item.dataType,unit:item.unit||'',decimals:item.decimals||0,access:item.access||'read',enabled:true,dlt645:{itemKey:item.key,dataIdentifier:'',valueFormat:item.valueFormat,dataLength:item.dataLength,access:item.access||'read'}},'dlt645')
    existing.add(item.key)
  }
  open.value=false
}

async function scanS7(){busy.value=true;error.value='';message.value='';try{const result=await api<{points:any[];warnings?:string[]}>('/api/s7-points/scan',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({deviceKey:props.device.deviceKey,deviceName:props.device.name,plcModel:props.device.plcModel||'s7-200-smart',address:props.device.address,area:s7Area.value,dbNumber:s7DB.value,rack:props.device.rack||0,slot:props.device.slot||0,localTsap:props.device.localTsap||'',remoteTsap:props.device.remoteTsap||'',start:s7Start.value,end:s7End.value,dataType:s7Type.value}),timeoutMs:60000});s7Candidates.value=(result.points||[]).map(item=>({...item,selected:item.selected!==false}));message.value=(result.warnings||[]).join('；')||`扫描到 ${s7Candidates.value.length} 个候选点位`}catch(reason){error.value=reason instanceof Error?reason.message:'S7 点位扫描失败'}finally{busy.value=false}}
function confirmS7(){for(const item of s7Candidates.value.filter(item=>item.selected))appendPoint(item,'s7');open.value=false}

function iecRef(parts:string[]){return parts.length<2?(parts[0]||''):`${parts[0]}/${parts.slice(1).join('.')}`}
function iecParts(value:string){const ref=String(value||'').trim(),slash=ref.indexOf('/');return slash<0?ref.split('.').filter(Boolean):[ref.slice(0,slash),...ref.slice(slash+1).split('.')].filter(Boolean)}
function iecKey(node:any){return `${node.objectRef||node.name||'node'}::${node.fc||''}`}
function normalizeIECNode(raw:any,depth=0):any{const children=(raw.children||[]).map((child:any)=>normalizeIECNode(child,depth+1)),selectable=Boolean(raw.leaf||(!children.length&&raw.objectRef&&raw.fc));return{...raw,name:raw.name||raw.objectRef,children,selectable,hasChildren:children.length>0,expanded:depth<1,key:iecKey(raw),kind:depth===0?'逻辑设备':depth===1?'逻辑节点':depth===2?'数据对象':'数据属性'}}
function flattenIEC(nodes:any[],result:any[]=[]){for(const node of nodes||[]){if(node.selectable)result.push(node);flattenIEC(node.children||[],result)}return result}
function treeFromIECPoints(points:any[]){const roots:any[]=[];for(const point of points||[]){const parts=iecParts(point.objectRef);if(!parts.length)continue;let level=roots;for(let index=0;index<parts.length;index++){const objectRef=iecRef(parts.slice(0,index+1));let node=level.find(item=>item.objectRef===objectRef);if(!node){node={name:parts[index],objectRef,children:[]};level.push(node)}if(index===parts.length-1)Object.assign(node,point,{name:point.name||parts[index],objectRef});level=node.children}}return roots.map(node=>normalizeIECNode(node))}
function setIECTree(nodes:any[]){iecNodes.value=nodes.map(node=>normalizeIECNode(node));iecSelected.value=[]}
function expandIEC(node:any){if(node.hasChildren)node.expanded=!node.expanded}
function toggleIEC(node:any){iecSelected.value=iecSelected.value.includes(node.key)?iecSelected.value.filter(key=>key!==node.key):[...iecSelected.value,node.key]}
function selectIECFolder(node:any){const selected=new Set(iecSelected.value);for(const child of flattenIEC(node.children||[]))selected.add(child.key);iecSelected.value=[...selected];message.value=`已选择 ${flattenIEC(node.children||[]).length} 个下级点位`}
function selectAllIEC(value:boolean){iecSelected.value=value?flattenIEC(iecNodes.value).map(node=>node.key):[]}
async function scanIEC(){busy.value=true;error.value='';try{const result=await api<{ok:boolean;nodes:any[]}>('/api/iec61850/browse',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({address:props.device.address||'',iedName:props.device.iedName||'',parentRef:'',recursive:true}),timeoutMs:45000});setIECTree(result.nodes||[]);message.value=`扫描到 ${flattenIEC(iecNodes.value).length} 个候选点位，展开层级后选择需要导入的点位`}catch(reason){error.value=reason instanceof Error?reason.message:'IEC61850 模型扫描失败'}finally{busy.value=false}}
async function previewIECFile(event:Event){const input=event.target as HTMLInputElement,file=input.files?.[0];if(!file)return;busy.value=true;error.value='';try{const form=new FormData();form.append('file',file);form.append('deviceKey',props.device.deviceKey||'');form.append('protocol','iec61850');form.append('address',props.device.address||'');const result=await api<{points:any[];warnings?:string[]}>('/api/iec61850/cid/preview',{method:'POST',body:form,timeoutMs:30000});iecNodes.value=treeFromIECPoints(result.points||[]);iecSelected.value=[];message.value=(result.warnings||[]).join('；')||`识别到 ${flattenIEC(iecNodes.value).length} 个候选点位`}catch(reason){error.value=reason instanceof Error?reason.message:'CID/ICD 解析失败'}finally{busy.value=false;input.value=''}}
function confirmIEC(){const selected=new Set(iecSelected.value),existing=new Set(props.points.map(item=>`${item.objectRef||''}::${item.fc||''}`));for(const item of flattenIEC(iecNodes.value)){const key=`${item.objectRef||''}::${item.fc||''}`;if(selected.has(item.key)&&!existing.has(key)){appendPoint(item,'iec61850');existing.add(key)}}open.value=false}
function toggleCandidate(item:any){item.selected=!item.selected}
function selectAllCandidates(value:boolean){for(const item of s7Candidates.value)item.selected=value}
</script>

<template>
  <button v-if="label" type="button" :disabled="readonly" @click="show">{{label}}</button>
  <Teleport to="body"><div v-if="open" class="protocol-point-modal" @click.self="close"><div class="protocol-point-dialog" :class="{'has-message':error||message}">
    <header><div><h3>{{modalTitle}}</h3><p>{{device.address||'尚未配置连接地址'}}</p></div><button type="button" aria-label="关闭" @click="close">×</button></header>
    <p v-if="error" class="protocol-tool-message bad-text">{{error}}</p><p v-else-if="message" class="protocol-tool-message">{{message}}</p>
    <template v-if="isOPCBrowser">
      <div class="protocol-tool-toolbar"><input v-model.trim="filter" :placeholder="device.protocol==='opcda'?'过滤已加载的名称或 Item ID':'过滤已加载的名称或 NodeId'"/><button :disabled="busy" @click="selectAllOpc">选择全部点位</button><button :disabled="busy" @click="loadOpcRoot">重新连接</button></div>
      <div class="opcua-browser-tree"><p v-if="busy&&!opcNodes.length" class="opcua-browser-empty">正在连接 {{device.protocol==='opcda'?'OPC DA':'OPC UA'}} 服务器…</p><OpcuaTreeNode v-for="node in opcNodes" :key="node.nodeId" :node="node" :selected="opcSelected" :filter="filter" @expand="expandOpc" @toggle="toggleOpc" @folder="selectOpcFolder"/></div>
      <footer><span>已选择 {{opcSelected.length}} 个点位</span><div><button @click="close">取消</button><button class="primary" @click="confirmOpc">添加所选点位</button></div></footer>
    </template>
    <template v-else-if="device.protocol==='dlt645'">
      <div class="protocol-tool-toolbar"><input v-model.trim="filter" placeholder="搜索数据项、DI 或分类"/><select v-model="dltCategory"><option v-for="category in dltCategories" :key="category">{{category}}</option></select><button @click="selectAllDLT(true)">选择当前结果</button><button @click="selectAllDLT(false)">清空</button></div>
      <div class="protocol-candidate-list"><label v-for="item in filteredDLTItems" :key="item.key"><input type="checkbox" :checked="dltSelected.includes(item.key)" @change="toggleDLT(item)"/><span><strong>{{item.name}}</strong><small>{{item.category}} · 2007 DI {{item.di2007||'-'}} · 1997 DI {{item.di1997||'-'}} · {{item.valueFormat}} / {{item.dataLength}} 字节</small></span><code>{{item.unit||dataTypeLabel(item.dataType)}}</code></label><p v-if="!busy&&!filteredDLTItems.length">没有匹配的标准数据项。</p></div>
      <footer><span>已选择 {{dltSelected.length}} / {{dltItems.length}}</span><div><button @click="close">取消</button><button class="primary" :disabled="!dltSelected.length" @click="confirmDLT">添加所选点位</button></div></footer>
    </template>
    <template v-else-if="device.protocol==='siemens-s7'">
      <div class="s7-scan-form"><label>区域<select v-model="s7Area"><option value="auto">自动扫描</option><option value="DB">DB</option><option value="V">V</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></label><label>DB块<input v-model.number="s7DB" type="number" min="0"/></label><label>起始地址<input v-model.number="s7Start" type="number" min="0"/></label><label>结束地址<input v-model.number="s7End" type="number" min="0"/></label><label>数据类型<select v-model="s7Type"><option value="auto">自动</option><option v-for="type in ['bool','uint16','int16','uint32','int32','float32']" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></label><button class="primary" :disabled="busy" @click="scanS7">{{busy?'扫描中…':'开始扫描'}}</button></div>
      <div class="protocol-candidate-list"><label v-for="item in s7Candidates" :key="item.metric"><input type="checkbox" :checked="item.selected" @change="toggleCandidate(item)"/><span><strong>{{item.name||item.metric}}</strong><small>{{item.area}}{{item.dbNumber?` ${item.dbNumber}`:''}} / {{item.register}} · {{dataTypeLabel(item.dataType)}} · 当前值 {{String(item.value??'-')}}</small></span><code>{{item.metric}}</code></label><p v-if="!busy&&!s7Candidates.length">设置扫描范围后点击“开始扫描”。</p></div>
      <footer><span>已选择 {{selectedCandidates}} / {{s7Candidates.length}}</span><div><button @click="selectAllCandidates(true)">全选</button><button @click="selectAllCandidates(false)">清空</button><button @click="close">取消</button><button class="primary" :disabled="!selectedCandidates" @click="confirmS7">添加所选点位</button></div></footer>
    </template>
    <template v-else>
      <div class="protocol-tool-toolbar"><input v-model.trim="filter" placeholder="搜索名称、对象引用、FC 或数据类型"/><button class="primary" :disabled="busy" @click="scanIEC">{{busy?'扫描中…':'扫描设备模型'}}</button><button :disabled="busy" @click="iecFile?.click()">导入 CID / ICD</button><input ref="iecFile" class="hidden-file" type="file" accept=".cid,.icd,.scd" @change="previewIECFile"/></div>
      <div class="opcua-browser-tree iec61850-browser-tree"><p v-if="busy&&!iecNodes.length" class="opcua-browser-empty">正在扫描 IEC61850 设备模型…</p><Iec61850TreeNode v-for="node in iecNodes" :key="node.key" :node="node" :selected="iecSelected" :filter="filter" @expand="expandIEC" @toggle="toggleIEC" @folder="selectIECFolder"/><p v-if="!busy&&!iecNodes.length" class="opcua-browser-empty">扫描设备模型，或导入 CID / ICD 文件后按层级选择点位。</p></div>
      <footer><span>已选择 {{selectedCandidates}} / {{flattenIEC(iecNodes).length}}</span><div><button @click="selectAllIEC(true)">全选点位</button><button @click="selectAllIEC(false)">清空</button><button @click="close">取消</button><button class="primary" :disabled="!selectedCandidates" @click="confirmIEC">添加所选点位</button></div></footer>
    </template>
  </div></div></Teleport>
</template>
