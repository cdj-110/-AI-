<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, triggerRef, watch } from 'vue'
import { api } from '../api'
import { confirmAction } from '../confirm'
import { openGatewayRealtime } from '../realtime'
import { useConfigStore } from '../stores/config'
import type { PointStatus } from '../types'
import DevicePointWorkspace from './DevicePointWorkspace.vue'
import LargeDevicePointWorkspace from './LargeDevicePointWorkspace.vue'
import ResourceWorkspace from './ResourceWorkspace.vue'
import ChannelWorkspace from './ChannelWorkspace.vue'

type TreeSelection={resourceKey?:string;channelKey?:string;deviceKey?:string;action?:'add-channel'|'add-device';nonce?:number}
const props=defineProps<{selection?:TreeSelection}>()
const emit=defineEmits<{selectionChange:[selection:TreeSelection]}>()
const mode=defineModel<'run'|'config'>('mode',{default:'run'})
const store=useConfigStore(), resourceKey=ref(''), channelKey=ref(''), deviceKey=ref(''), pointMetric=ref('')
const detailLevel=ref<'resource'|'channel'|'device'>('channel')
const configView=ref<'form'|'json'>('form'), jsonText=ref('')
const saving=ref(false), message=ref(''), error=ref(''), search=ref('')
const channelModal=ref(false), newChannelRole=ref('collect'), newChannelProtocol=ref('modbus-tcp')
const unsavedOpen=ref(false), savedSnapshot=ref(''), suppressModeChange=ref(false)
const networkApplyOpen=ref(false), pendingNetworkPorts=ref<any[]>([])
const writeValue=ref('')
const liveByMetric=shallowRef(new Map<string,PointStatus>())
const statusByKey=new Map<string,PointStatus>()
const statusByDevice=new Map<string,Map<string,PointStatus>>()
let forwardTargetsBySource=new Map<string,string[]>()
const networkInterfaces=ref<any[]>([])
let stopRealtime=()=>{}
const cfg=computed(()=>store.value)
const forwardSlave=computed(()=>{if(!cfg.value.forwardSlave)cfg.value.forwardSlave={enabled:false,modbusListen:'0.0.0.0:1502',iec104Listen:'0.0.0.0:2404',iec61850Listen:'0.0.0.0:102'};return cfg.value.forwardSlave})
const resources=computed<any[]>(()=>{if(!Array.isArray(cfg.value.resources))cfg.value.resources=[];return cfg.value.resources})
const channels=computed<any[]>(()=>{if(!Array.isArray(cfg.value.channels))cfg.value.channels=[];return cfg.value.channels})
const collectDevices=computed<any[]>(()=>{if(!Array.isArray(cfg.value.devices))cfg.value.devices=[];return cfg.value.devices})
const forwardDevices=computed<any[]>(()=>{if(!Array.isArray(cfg.value.forwardDevices))cfg.value.forwardDevices=[];return cfg.value.forwardDevices})
const resource=computed(()=>resources.value.find(item=>item.resourceKey===resourceKey.value))
const resourceChannels=computed(()=>channels.value.filter(item=>item.resourceKey===resourceKey.value))
const channel=computed(()=>channels.value.find(item=>item.channelKey===channelKey.value&&item.resourceKey===resourceKey.value))
const isForward=computed(()=>channel.value?.role?channel.value.role==='forward':Boolean(channel.value?.forwardProtocol))
const channelDevices=computed(()=>{const list=isForward.value?forwardDevices.value:collectDevices.value;return list.filter(item=>item.channelKey===channelKey.value)})
const device=computed(()=>channelDevices.value.find(item=>item.deviceKey===deviceKey.value))
const points=computed<any[]>(()=>Array.isArray(device.value?.points)?device.value.points:[])
const allConfiguredPoints=computed<any[]>(()=>[...collectDevices.value.flatMap(item=>item.points||[]),...(cfg.value.edgeComputing?.groups||[]).flatMap((item:any)=>item.points||[])])
// The legacy inline editor is kept only as a compatibility fallback and is hidden
// behind DevicePointWorkspace. Never let it create tens of thousands of hidden DOM
// nodes; the active workspace owns the complete virtualized point list.
const legacyPointRenderLimit=80
const visiblePoints=computed(()=>{
  const q=search.value.trim().toLowerCase()
  const source=q?points.value.filter(item=>`${item.name||''} ${item.metric||''}`.toLowerCase().includes(q)):points.value
  return source.slice(0,legacyPointRenderLimit)
})
const point=computed(()=>points.value.find(item=>item.metric===pointMetric.value))
const networkCollectProtocols=[['modbus-tcp','Modbus TCP'],['iec104','IEC 60870-5-104'],['siemens-s7','Siemens S7'],['opcua','OPC UA'],['iec61850','IEC 61850 MMS'],['iec61850-goose','IEC 61850 GOOSE']]
const serialCollectProtocols=[['modbus-rtu','Modbus RTU']]
const forwardProtocols=[['modbus-tcp-slave','Modbus TCP 从站'],['iec104-server','IEC104 服务端'],['iec61850-mms-server','IEC61850 MMS 服务端'],['iec61850-goose-publisher','IEC61850 GOOSE 发布']]
const modalProtocols=computed(()=>newChannelRole.value==='forward'?forwardProtocols:(resource.value?.type==='serial'?serialCollectProtocols:networkCollectProtocols))
const channelProtocols=computed(()=>isForward.value?forwardProtocols:(resource.value?.type==='serial'?serialCollectProtocols:networkCollectProtocols))
const channelProtocol=computed({get:()=>isForward.value?(channel.value?.forwardProtocol||channel.value?.protocol||'modbus-tcp-slave'):(channel.value?.protocol||'modbus-tcp'),set:(value:string)=>{if(!channel.value)return;if(isForward.value){channel.value.forwardProtocol=value;channel.value.protocol='none'}else channel.value.protocol=value;changeDeviceProtocol()}})
const dataTypes=['bool','int16','uint16','int32','uint32','float32','float64','string']
const modbusFunctions=[[1,'01 线圈'],[2,'02 离散量'],[3,'03 保持寄存器'],[4,'04 输入寄存器']]

function key(prefix:string, used:string[]){let i=1,next='';do{next=`${prefix}-${String(i++).padStart(3,'0')}`}while(used.includes(next));return next}
function selectResource(value:string){resourceKey.value=value;channelKey.value='';deviceKey.value='';pointMetric.value='';detailLevel.value='resource';emit('selectionChange',{resourceKey:value})}
function selectChannel(value:string){channelKey.value=value;deviceKey.value='';pointMetric.value='';detailLevel.value='channel';emit('selectionChange',{resourceKey:resourceKey.value,channelKey:value})}
function selectDevice(value:string){deviceKey.value=value;pointMetric.value=points.value[0]?.metric||'';detailLevel.value='device';emit('selectionChange',{resourceKey:resourceKey.value,channelKey:channelKey.value,deviceKey:value})}
function applySelection(value?:TreeSelection){if(value?.resourceKey)resourceKey.value=value.resourceKey;if(value?.deviceKey){channelKey.value=value.channelKey||'';deviceKey.value=value.deviceKey}else if(value?.channelKey){channelKey.value=value.channelKey;deviceKey.value='';pointMetric.value=''}else if(value?.resourceKey){channelKey.value='';deviceKey.value='';pointMetric.value=''}if(!resource.value)resourceKey.value=resources.value[0]?.resourceKey||'';detailLevel.value=value?.deviceKey?'device':value?.channelKey?'channel':value?.resourceKey?'resource':detailLevel.value;if(value?.action==='add-channel')addChannel();else if(value?.action==='add-device')addDeviceForChannel(value.channelKey)}
function ensureSelection(){if(!resource.value)resourceKey.value=resources.value[0]?.resourceKey||'';if(!resourceChannels.value.some(item=>item.channelKey===channelKey.value))channelKey.value=resourceChannels.value[0]?.channelKey||'';if(channel.value?.protocol==='iec104'&&!channel.value.iec104)channel.value.iec104={acquisitionMode:'auto',generalInterrogationOnStart:true,generalInterrogationIntervalSeconds:0,clockSyncOnStart:true,clockSyncIntervalSeconds:3600,counterInterrogationOnStart:false,counterInterrogationIntervalSeconds:0};if(!channelDevices.value.some(item=>item.deviceKey===deviceKey.value))deviceKey.value=channelDevices.value[0]?.deviceKey||'';if(!points.value.some(item=>item.metric===pointMetric.value))pointMetric.value=points.value[0]?.metric||''}
function addResource(){const value=key('resource',resources.value.map(item=>item.resourceKey));resources.value.push({resourceKey:value,name:`资源${resources.value.length+1}`,type:'network',enabled:true,network:{name:value,interface:'eth0',mode:'static',prefixLength:24,enabled:true}});selectResource(value);mode.value='config'}
async function removeResource(){if(!resource.value||!await confirmAction({title:'删除资源',message:`确认删除资源“${resource.value.name}”？`,detail:'该资源下的全部通道、设备和点位也会一并删除。',confirmText:'确认删除',danger:true}))return;const removeChannels=new Set(resourceChannels.value.map(item=>item.channelKey));cfg.value.channels=channels.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.devices=collectDevices.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.forwardDevices=forwardDevices.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.resources=resources.value.filter(item=>item.resourceKey!==resourceKey.value);resourceKey.value='';ensureSelection()}
function changeResourceType(){if(!resource.value)return;if(resource.value.type==='serial'){resource.value.network=undefined;resource.value.serial={name:resource.value.name||resource.value.resourceKey,port:'/dev/ttyS1',baudRate:9600,dataBits:8,stopBits:1,parity:'none',enabled:true}}else{resource.value.serial=undefined;resource.value.network={name:resource.value.name||resource.value.resourceKey,interface:'eth0',mode:'static',prefixLength:24,dns:[],enabled:true}}}
function addChannel(){if(!resource.value){error.value='请先新增资源';return}newChannelRole.value='collect';newChannelProtocol.value=resource.value.type==='serial'?'modbus-rtu':'modbus-tcp';channelModal.value=true}
function changeNewChannelRole(){newChannelProtocol.value=newChannelRole.value==='forward'?'modbus-tcp-slave':resource.value?.type==='serial'?'modbus-rtu':'modbus-tcp'}
function confirmAddChannel(){if(!resource.value)return;const value=key('channel',channels.value.map(item=>item.channelKey));const forward=newChannelRole.value==='forward';channels.value.push({channelKey:value,resourceKey:resourceKey.value,name:`通道${resourceChannels.value.length+1}`,type:resource.value.type,role:newChannelRole.value,protocol:forward?'none':newChannelProtocol.value,forwardProtocol:forward?newChannelProtocol.value:'',collectIntervalSeconds:5,interfaceName:resource.value.name,enabled:true});if(forward)forwardSlave.value.enabled=true;channelModal.value=false;selectChannel(value);mode.value='config'}
async function removeChannel(){if(!channel.value||!await confirmAction({title:'删除通道',message:`确认删除通道“${channel.value.name||channel.value.channelKey}”？`,detail:'该通道下挂的全部设备和点位也会一并删除。',confirmText:'确认删除',danger:true}))return;const parentResourceKey=resourceKey.value,targetChannelKey=channelKey.value;cfg.value.devices=collectDevices.value.filter(item=>item.channelKey!==targetChannelKey);cfg.value.forwardDevices=forwardDevices.value.filter(item=>item.channelKey!==targetChannelKey);cfg.value.channels=channels.value.filter(item=>item.channelKey!==targetChannelKey);selectResource(parentResourceKey)}
function changeChannelRole(){if(!channel.value)return;if(channel.value.role==='forward'){channel.value.forwardProtocol=channel.value.protocol||'modbus-tcp';channel.value.protocol=channel.value.forwardProtocol}else channel.value.forwardProtocol='';deviceKey.value='';ensureSelection()}
function addDevice(){if(!channel.value){error.value='请先新增通道';return}const all=[...collectDevices.value,...forwardDevices.value,...(cfg.value.edgeComputing?.groups||[])].map(item=>item.deviceKey||item.groupKey);const value=key('device',all);if(isForward.value){const protocol=channel.value.forwardProtocol||'modbus-tcp-slave',index=channelDevices.value.length+1;forwardDevices.value.push({deviceKey:value,channelKey:channelKey.value,name:`转发设备${index}`,protocol,enabled:true,unitId:1,commonAddress:1,iedName:'WEIKONG',logicalDevice:`LD${index}`,interfaceName:resource.value?.network?.interface||'eth0',goCbRef:`WEIKONGLD${index}/LLN0$GO$gcb01`,dataSetRef:`WEIKONGLD${index}/LLN0$Dataset01`,appId:4096,destinationMac:'01:0c:cd:01:00:01',vlanId:0,vlanPriority:4,timeAllowedToLiveMilliseconds:2000,confRev:1,points:[]});forwardSlave.value.enabled=true}else{const protocol=channel.value.protocol||'modbus-tcp';collectDevices.value.push({deviceKey:value,channelKey:channelKey.value,name:`采集设备${channelDevices.value.length+1}`,protocol,interfaceType:resource.value?.type||'network',interfaceName:resource.value?.name||'',address:protocol==='modbus-rtu'?(resource.value?.serial?.port||'/dev/ttyS1'):protocol==='iec61850-goose'?(resource.value?.network?.interface||'eth0'):'192.168.1.100:502',slaveId:1,commonAddress:1,reconnectIntervalSeconds:30,goCbRef:'WEIKONGLD1/LLN0$GO$gcb01',dataSetRef:'WEIKONGLD1/LLN0$Dataset01',appId:4096,destinationMac:'01:0c:cd:01:00:01',vlanId:0,vlanPriority:4,points:[]})}selectDevice(value);mode.value='config'}
function addDeviceForChannel(value?:string){let target=value||resourceChannels.value[0]?.channelKey;if(!target&&resource.value?.type==='serial'){target=key('channel',channels.value.map(item=>item.channelKey));channels.value.push({channelKey:target,resourceKey:resourceKey.value,name:resource.value.name||target,type:'serial',role:'collect',protocol:'modbus-rtu',collectIntervalSeconds:resource.value.collectIntervalSeconds||1,interfaceName:resource.value.name,enabled:true})}if(!target){error.value='当前资源尚未建立设备通道';return}selectChannel(target);addDevice()}
function selectDeviceFromResource(targetChannel:string,targetDevice:string){selectChannel(targetChannel);selectDevice(targetDevice)}
async function removeDevice(){if(!device.value||!await confirmAction({title:'删除设备',message:`确认删除设备“${device.value.name||device.value.deviceKey}”？`,detail:'该设备下的全部点位也会一并删除。',confirmText:'确认删除',danger:true}))return;const target=isForward.value?'forwardDevices':'devices';cfg.value[target]=(cfg.value[target]||[]).filter((item:any)=>item.deviceKey!==deviceKey.value);deviceKey.value='';ensureSelection()}
function nextMetric(){const used=[...collectDevices.value.flatMap(item=>item.points||[]),...forwardDevices.value.flatMap(item=>item.points||[]),...(cfg.value.edgeComputing?.groups||[]).flatMap((item:any)=>item.points||[])].map((item:any)=>item.metric);return key('point',used)}
function addPoint(){if(!device.value){error.value='请先新增设备';return}const metric=nextMetric();if(isForward.value)device.value.points.push({name:`转发点位${points.value.length+1}`,metric,sourceDeviceKey:'',sourceMetric:'',function:3,register:0,ioa:points.value.length+1,quantity:1,dataType:'uint16',byteOrder:'big',wordOrder:'big',scale:1});else device.value.points.push({deviceKey:device.value.deviceKey,channelKey:channelKey.value,name:`点位${points.value.length+1}`,metric,protocol:device.value.protocol,address:device.value.address,slaveId:device.value.slaveId||1,commonAddress:device.value.commonAddress||1,function:3,register:0,ioa:points.value.length+1,quantity:1,dataType:'uint16',byteOrder:'big',wordOrder:'big',scale:1,offset:0,decimals:0});pointMetric.value=metric;mode.value='config'}
function duplicatePoint(){if(!point.value)return;const copy=JSON.parse(JSON.stringify(point.value));copy.metric=nextMetric();copy.name=`${point.value.name||point.value.metric} 副本`;points.value.push(copy);pointMetric.value=copy.metric}
async function removePoint(){if(!point.value||!await confirmAction({title:'删除点位',message:`确认删除点位“${point.value.name||point.value.metric}”？`,detail:'删除后需保存配置才会正式生效。',confirmText:'确认删除',danger:true}))return;device.value.points=points.value.filter(item=>item!==point.value);pointMetric.value=device.value.points[0]?.metric||''}
function sourceDevices(){return collectDevices.value.filter(item=>item.deviceKey!==deviceKey.value)}
function sourcePoints(){const source=collectDevices.value.find(item=>item.deviceKey===point.value?.sourceDeviceKey);return source?.points||[]}
function syncForwardSource(){const source=sourcePoints().find((item:any)=>item.metric===point.value.sourceMetric);if(source){point.value.name=point.value.name||source.name;point.value.dataType=point.value.dataType||source.dataType;point.value.quantity=point.value.quantity||source.quantity||1}}
function changeDeviceProtocol(){if(!channel.value)return;if(channel.value.protocol==='iec104'&&!channel.value.iec104)channel.value.iec104={acquisitionMode:'auto',generalInterrogationOnStart:true,generalInterrogationIntervalSeconds:0,clockSyncOnStart:true,clockSyncIntervalSeconds:3600,counterInterrogationOnStart:false,counterInterrogationIntervalSeconds:0};if(isForward.value){channel.value.protocol='none';for(const [index,item] of channelDevices.value.entries()){item.protocol=channel.value.forwardProtocol||'modbus-tcp-slave';if(item.protocol==='iec61850-mms-server'||item.protocol==='iec61850-goose-publisher'){item.iedName=item.iedName||'WEIKONG';item.logicalDevice=item.logicalDevice||`LD${index+1}`}if(item.protocol==='iec61850-goose-publisher'){item.interfaceName=item.interfaceName||resource.value?.network?.interface||'eth0';item.goCbRef=item.goCbRef||`${item.iedName}${item.logicalDevice}/LLN0$GO$gcb01`;item.dataSetRef=item.dataSetRef||`${item.iedName}${item.logicalDevice}/LLN0$Dataset01`;item.appId=item.appId||4096;item.destinationMac=item.destinationMac||'01:0c:cd:01:00:01';item.vlanPriority=item.vlanPriority??4;item.timeAllowedToLiveMilliseconds=item.timeAllowedToLiveMilliseconds||2000;item.confRev=item.confRev||1}}return}for(const item of channelDevices.value){item.protocol=channel.value.protocol;if(item.protocol==='iec61850-goose'){item.address=resource.value?.network?.interface||item.address||'eth0';item.goCbRef=item.goCbRef||'WEIKONGLD1/LLN0$GO$gcb01';item.dataSetRef=item.dataSetRef||'WEIKONGLD1/LLN0$Dataset01';item.appId=item.appId||4096;item.destinationMac=item.destinationMac||'01:0c:cd:01:00:01';item.vlanPriority=item.vlanPriority??4}for(const p of item.points||[]){p.protocol=item.protocol;p.address=item.address;p.goCbRef=item.goCbRef;p.dataSetRef=item.dataSetRef;p.appId=item.appId;p.destinationMac=item.destinationMac;p.vlanId=item.vlanId;p.vlanPriority=item.vlanPriority}}}
function rebuildLiveStatus(){
  forwardTargetsBySource=new Map()
  if(!isForward.value){
    liveByMetric.value=statusByDevice.get(deviceKey.value)||new Map()
    return
  }
  const mapped=new Map<string,PointStatus>()
  for(const point of points.value){
    const sourceKey=`${point.sourceDeviceKey}::${point.sourceMetric}`
    const targets=forwardTargetsBySource.get(sourceKey)||[]
    targets.push(point.metric)
    forwardTargetsBySource.set(sourceKey,targets)
    const status=statusByKey.get(sourceKey)
    if(status)mapped.set(point.metric,status)
  }
  liveByMetric.value=mapped
}
function replaceStatus(points:PointStatus[]){
  statusByKey.clear()
  statusByDevice.clear()
  for(const point of points){
    const key=`${point.deviceKey}::${point.metric}`
    statusByKey.set(key,point)
    let devicePoints=statusByDevice.get(point.deviceKey)
    if(!devicePoints){devicePoints=new Map();statusByDevice.set(point.deviceKey,devicePoints)}
    devicePoints.set(point.metric,point)
  }
  rebuildLiveStatus()
}
function mergeStatus(points:PointStatus[]){
  if(!points.length)return
  let activeChanged=false
  for(const point of points){
    const key=`${point.deviceKey}::${point.metric}`
    statusByKey.set(key,point)
    let devicePoints=statusByDevice.get(point.deviceKey)
    if(!devicePoints){devicePoints=new Map();statusByDevice.set(point.deviceKey,devicePoints)}
    devicePoints.set(point.metric,point)
    if(!isForward.value&&point.deviceKey===deviceKey.value)activeChanged=true
    if(isForward.value){
      for(const targetMetric of forwardTargetsBySource.get(key)||[]){
        liveByMetric.value.set(targetMetric,point)
        activeChanged=true
      }
    }
  }
  if(activeChanged)triggerRef(liveByMetric)
}
function clearStatus(){
  statusByKey.clear()
  statusByDevice.clear()
  forwardTargetsBySource.clear()
  liveByMetric.value=new Map()
}
async function loadStatus(){if(store.largeMode){clearStatus();return}try{replaceStatus(await api<PointStatus[]>('/api/point-status'))}catch{} }
async function loadNetworkInterfaces(){try{networkInterfaces.value=(await api<{interfaces:any[]}>('/api/network/interfaces')).interfaces||[]}catch{}}
function handleRealtime(message:any){
  if(mode.value==='config')return
  if(message.type==='snapshot'&&Array.isArray(message.payload?.points))replaceStatus(message.payload.points)
  else if(message.type==='points.diff'){
    const incoming=Array.isArray(message.payload?.points)?message.payload.points:[]
    if(message.payload?.replace)replaceStatus(incoming)
    else mergeStatus(incoming)
  }
}
async function collectNow(){try{await api('/api/collect-now',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'});message.value='已触发立即采集';setTimeout(loadStatus,700)}catch(cause){error.value=cause instanceof Error?cause.message:'采集失败'}}
async function writePoint(){if(!device.value||!point.value||writeValue.value==='')return;try{let value:any=writeValue.value;if(point.value.dataType==='bool')value=['1','true','on'].includes(writeValue.value.toLowerCase());else if(point.value.dataType!=='string')value=Number(writeValue.value);await api('/api/points/write',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({deviceKey:device.value.deviceKey,metric:point.value.metric,value})});message.value=`点位 ${point.value.name||point.value.metric} 写入成功`;writeValue.value='';setTimeout(loadStatus,500)}catch(cause){error.value=cause instanceof Error?cause.message:'点位写入失败'}}
async function writeWorkspacePoint(target:any,raw:string,done?:(success:boolean,value?:unknown)=>void){if(!device.value){done?.(false);return}try{let value:any=raw;if(target.dataType==='bool')value=['1','true','on'].includes(raw.toLowerCase());else if(target.dataType!=='string')value=Number(raw);await api('/api/points/write',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({deviceKey:device.value.deviceKey,metric:target.metric,value})});message.value=`点位 ${target.name||target.metric} 写入成功`;done?.(true,value);setTimeout(loadStatus,500)}catch(cause){error.value=cause instanceof Error?cause.message:'点位写入失败';done?.(false)}}
async function testDeviceConnection(){if(!device.value)return;message.value='';error.value='';try{const result=await api<{message?:string;address?:string}>('/api/connection/test',{method:'POST',headers:{'Content-Type':'application/json'},timeoutMs:20000,body:JSON.stringify({protocol:device.value.protocol,address:device.value.address,slaveId:device.value.slaveId,commonAddress:device.value.commonAddress,rack:device.value.rack,slot:device.value.slot,localTsap:device.value.localTsap,remoteTsap:device.value.remoteTsap,username:device.value.username,password:device.value.password})});message.value=result.message||`连接成功：${result.address||device.value.address}` }catch(cause){error.value=cause instanceof Error?cause.message:'连接测试失败'}}
function markSaved(){savedSnapshot.value=JSON.stringify(cfg.value)}
function enterRunMode(){suppressModeChange.value=true;mode.value='run'}
function networkPortValue(item:any){return{interface:item?.interface||'',mode:item?.mode||'static',ipAddress:item?.ipAddress||'',prefixLength:Number(item?.prefixLength||24),gateway:item?.gateway||'',dns:Array.isArray(item?.dns)?item.dns.map((value:any)=>String(value).trim()).filter(Boolean):[],enabled:item?.enabled!==false}}
function networkPortsOf(value:any){
  const result=new Map<string,any>()
  for(const item of value?.resources||[])if(item.type==='network'&&item.network)result.set(item.resourceKey,{resourceKey:item.resourceKey,name:item.name||item.resourceKey,...networkPortValue(item.network)})
  return result
}
function systemNetworkMismatch(current:any){
  const actual=networkInterfaces.value.find(item=>item.name===current.interface)
  if(!actual)return undefined
  const addressMismatch=current.enabled!==false&&(
    current.mode!==actual.configuredMode||
    (current.mode==='static'&&(current.ipAddress!==actual.ipv4Address||Number(current.prefixLength)!==Number(actual.prefixLength)||(current.gateway||'')!==(actual.gateway||'')))
  )
  const dnsMismatch=Boolean(current.gateway)&&current.dns.some((server:string)=>!(actual.dns||[]).includes(server))
  return addressMismatch||dnsMismatch?actual:undefined
}
function changedNetworkPorts(){
  let previous:any={}
  try{previous=savedSnapshot.value?JSON.parse(savedSnapshot.value):{}}catch{}
  const before=networkPortsOf(previous),after=networkPortsOf(cfg.value),changed:any[]=[]
  for(const [resourceKey,current] of after){
    const old=before.get(resourceKey)
    const system=systemNetworkMismatch(current)
    if(JSON.stringify(networkPortValue(old))!==JSON.stringify(networkPortValue(current))||system)changed.push({...current,previous:old,system})
  }
  return changed
}
function networkAddressChanged(item:any){const baseline=item.system||item.previous;return !baseline||item.interface!==(baseline.interface||baseline.name)||item.mode!==(baseline.mode||baseline.configuredMode)||item.ipAddress!==(baseline.ipAddress||baseline.ipv4Address)||Number(item.prefixLength)!==Number(baseline.prefixLength)||item.gateway!==(baseline.gateway||'')}
function requestSave(){
  const changed=changedNetworkPorts()
  if(changed.length){pendingNetworkPorts.value=changed;networkApplyOpen.value=true;return}
  void commitSave(false)
}
async function commitSave(applyNetwork:boolean){
  const networkChanges=applyNetwork?[...pendingNetworkPorts.value]:[]
  saving.value=true;error.value='';message.value=''
  try{
    await store.save()
    await Promise.all([store.load(),loadStatus()])
    markSaved();ensureSelection();unsavedOpen.value=false;networkApplyOpen.value=false;enterRunMode()
    if(networkChanges.length){
      for(const item of networkChanges){
        const port={name:item.name,interface:item.interface,mode:item.mode,ipAddress:item.ipAddress,prefixLength:item.prefixLength,gateway:item.gateway,dns:item.dns,enabled:item.enabled}
        await api('/api/network/apply',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(port),timeoutMs:10000})
      }
      const changedAddress=networkChanges.find(networkAddressChanged)
      message.value=changedAddress&&changedAddress.mode==='static'
        ?`网口配置正在应用，页面可能短暂断开。请稍后使用 ${location.protocol}//${changedAddress.ipAddress}:${location.port||'80'} 重新访问；网卡未能获得目标地址时会自动回滚。`
        :'网口配置正在应用，DNS 与网络参数稍后生效；网卡应用失败时会自动回滚。'
    }else message.value='配置已保存并热加载'
  }catch(cause){error.value=cause instanceof Error?cause.message:'保存失败'}
  finally{saving.value=false;pendingNetworkPorts.value=[]}
}
async function discardAndRun(){unsavedOpen.value=false;await store.load();markSaved();ensureSelection();enterRunMode()}
function setConfigView(value:'form'|'json'){configView.value=value;if(value==='json')jsonText.value=JSON.stringify(cfg.value,null,2);document.documentElement.dataset.configView=value}
async function saveJSON(){saving.value=true;error.value='';try{const parsed=JSON.parse(jsonText.value);await store.save(parsed);await store.load();jsonText.value=JSON.stringify(store.value,null,2);message.value='JSON 配置已保存并热加载'}catch(cause){error.value=cause instanceof Error?cause.message:'JSON 配置保存失败'}finally{saving.value=false}}
function openMonitor(){if(!device.value)return;const query=new URLSearchParams({deviceKey:device.value.deviceKey,deviceName:device.value.name||device.value.deviceKey,protocol:device.value.protocol||''});window.open(`/packet-monitor?${query}`,'_blank','noopener')}
async function projectImported(){await Promise.all([store.load(),loadStatus()]);markSaved();ensureSelection();message.value='工程配置已重新加载'}
watch(()=>props.selection,(value)=>applySelection(value),{deep:true})
watch(()=>channel.value?.protocol,changeDeviceProtocol)
watch(()=>[deviceKey.value,isForward.value,points.value.length],rebuildLiveStatus)
watch(detailLevel,(value)=>{document.documentElement.dataset.gatewayDetail=value},{immediate:true})
watch(mode,(value,previous)=>{if(suppressModeChange.value){suppressModeChange.value=false;return}if(value==='run'&&previous==='config'&&savedSnapshot.value&&JSON.stringify(cfg.value)!==savedSnapshot.value){suppressModeChange.value=true;mode.value='config';unsavedOpen.value=true}})
watch(mode,value=>{
  if(value==='config')clearStatus()
  else void loadStatus()
})
onMounted(async()=>{document.documentElement.dataset.configView=configView.value;if(!store.value.gatewayKey)await store.load();markSaved();applySelection(props.selection);if(!props.selection?.resourceKey){resourceKey.value=resources.value[0]?.resourceKey||'';detailLevel.value='resource'}await Promise.all([loadStatus(),loadNetworkInterfaces()]);if(!store.largeMode)stopRealtime=openGatewayRealtime(handleRealtime)})
onBeforeUnmount(()=>stopRealtime())
</script>

<template><div class="view-stack gateway-editor"><p v-if="message" class="notice good">{{message}}</p><p v-if="error" class="notice bad">{{error}}</p>
<section class="card"><div class="section-title"><div><h2>智能网关</h2><p class="meta">从左侧选择串口、网口或设备，右侧只展示当前对象的相关配置。</p></div><div class="point-mode-actions"><div class="mode-switch"><button :class="{active:mode==='config'}" @click="mode='config'">配置</button><button :class="{active:mode==='run'}" @click="mode='run'">运行</button></div><button @click="projectImported">重新加载</button><button @click="collectNow">立即采集一次</button><button class="primary" :disabled="saving||mode==='run'" @click="configView==='json'?saveJSON():requestSave()">{{saving?'保存中…':'保存配置'}}</button></div></div><div class="mode-switch config-view-switch"><button :class="{active:configView==='form'}" @click="setConfigView('form')">表单配置</button><button :class="{active:configView==='json'}" @click="setConfigView('json')">JSON 高级编辑</button></div>
<div class="gateway-columns"><aside><div class="column-head"><strong>资源</strong><button :disabled="mode==='run'" @click="addResource">＋</button></div><button v-for="item in resources" :key="item.resourceKey" :class="{active:item.resourceKey===resourceKey}" @click="selectResource(item.resourceKey)"><span>{{item.name||item.resourceKey}}</span><small>{{item.type==='serial'?'串口':'网口'}}</small></button><p v-if="!resources.length" class="empty-mini">暂无资源</p></aside>
<aside><div class="column-head"><strong>通道</strong><button :disabled="mode==='run'||!resource" @click="addChannel">＋</button></div><button v-for="item in resourceChannels" :key="item.channelKey" :class="{active:item.channelKey===channelKey}" @click="selectChannel(item.channelKey)"><span>{{item.name||item.channelKey}}</span><small>{{item.role==='forward'?'转发':'采集'}} · {{item.protocol}}</small></button><p v-if="!resourceChannels.length" class="empty-mini">暂无通道</p></aside>
<aside><div class="column-head"><strong>设备</strong><button :disabled="mode==='run'||!channel" @click="addDevice">＋</button></div><button v-for="item in channelDevices" :key="item.deviceKey" :class="{active:item.deviceKey===deviceKey}" @click="selectDevice(item.deviceKey)"><span>{{item.name||item.deviceKey}}</span><small>{{item.deviceKey}}</small></button><p v-if="!channelDevices.length" class="empty-mini">暂无设备</p></aside></div></section>

<section v-if="resource" class="content-card"><div class="section-head"><div><h2>资源配置</h2><p class="mono">{{resource.resourceKey}}</p></div><div class="button-row"><button :disabled="mode==='run'" @click="addChannel">新增通道</button><button class="danger" :disabled="mode==='run'" @click="removeResource">删除资源</button></div></div><div class="config-grid"><label>资源名称<input v-model.trim="resource.name" :disabled="mode==='run'"/></label><label>资源类型<select v-model="resource.type" :disabled="mode==='run'" @change="changeResourceType"><option value="serial">串口</option><option value="network">网口</option></select></label><label>状态<select v-model="resource.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label><template v-if="resource.type==='serial'&&resource.serial"><label>设备文件<input v-model.trim="resource.serial.port" :disabled="mode==='run'"/></label><label>波特率<input v-model.number="resource.serial.baudRate" type="number" :disabled="mode==='run'"/></label><label>数据位<input v-model.number="resource.serial.dataBits" type="number" :disabled="mode==='run'"/></label><label>停止位<input v-model.number="resource.serial.stopBits" type="number" step="0.5" :disabled="mode==='run'"/></label><label>校验<select v-model="resource.serial.parity" :disabled="mode==='run'"><option value="none">无</option><option value="even">偶校验</option><option value="odd">奇校验</option></select></label></template><template v-if="resource.type==='network'&&resource.network"><label>系统接口<input v-model.trim="resource.network.interface" :disabled="mode==='run'"/></label><label>地址模式<select v-model="resource.network.mode" :disabled="mode==='run'"><option value="static">静态</option><option value="dhcp">DHCP</option></select></label><label>IP 地址<input v-model.trim="resource.network.ipAddress" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label><label>前缀长度<input v-model.number="resource.network.prefixLength" type="number" min="1" max="32" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label><label>网关<input v-model.trim="resource.network.gateway" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label></template></div></section>

<section v-if="channel" class="content-card"><div class="section-head"><div><h2>通道配置</h2><p class="mono">{{channel.channelKey}}</p></div><div class="button-row"><button :disabled="mode==='run'" @click="addDevice">新增设备</button><button class="danger" :disabled="mode==='run'" @click="removeChannel">删除通道</button></div></div><div class="config-grid"><label>通道名称<input v-model.trim="channel.name" :disabled="mode==='run'"/></label><label>用途<select v-model="channel.role" :disabled="mode==='run'" @change="changeChannelRole"><option value="collect">采集</option><option value="forward">转发</option></select></label><label>协议<select v-model="channelProtocol" :disabled="mode==='run'"><option v-for="item in channelProtocols" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label><label>状态<select v-model="channel.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label><label v-if="!isForward">采集周期（毫秒）<input v-model.number="channel.collectIntervalMilliseconds" type="number" min="10" step="10" :placeholder="String((channel.collectIntervalSeconds||1)*1000)" :disabled="mode==='run'"/></label><template v-if="channel.protocol==='iec104'&&!isForward"><label>采集模式<select v-model="channel.iec104.acquisitionMode" :disabled="mode==='run'"><option value="auto">自动兼容</option><option value="periodic">周期总召</option></select></label><label>启动总召<select v-model="channel.iec104.generalInterrogationOnStart" :disabled="mode==='run'"><option :value="true">开启</option><option :value="false">关闭</option></select></label><label>周期总召（秒）<input v-model.number="channel.iec104.generalInterrogationIntervalSeconds" type="number" min="0" :disabled="mode==='run'||channel.iec104.acquisitionMode==='auto'"/></label></template></div></section>

<section v-if="device" class="content-card"><div class="section-head"><div><h2>{{isForward?'转发设备':'采集设备'}}</h2><p class="mono">{{device.deviceKey}}</p></div><div class="button-row"><button class="danger" :disabled="mode==='run'" @click="removeDevice">删除设备</button></div></div><div class="config-grid"><label>设备名称<input v-model.trim="device.name" :disabled="mode==='run'"/></label><label>设备标识<input v-model.trim="device.deviceKey" disabled/></label><template v-if="isForward"><label>协议<input :value="device.protocol" disabled/></label><label v-if="device.protocol==='modbus-tcp-slave'">Unit ID<input v-model.number="device.unitId" type="number" min="1" max="247" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec104-server'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec61850-mms-server'">IED 名称<input v-model.trim="device.iedName" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec61850-mms-server'">逻辑设备<input v-model.trim="device.logicalDevice" :disabled="mode==='run'"/></label><label>状态<select v-model="device.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label></template><template v-else><label>协议<input :value="device.protocol" disabled/></label><label>连接地址<input v-model.trim="device.address" :disabled="mode==='run'" placeholder="IP:端口 或串口设备"/></label><label v-if="device.protocol.startsWith('modbus')">从站 ID<input v-model.number="device.slaveId" type="number" min="1" max="247" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="mode==='run'"/></label><label v-if="device.protocol==='siemens-s7'">PLC 型号<select v-model="device.plcModel" :disabled="mode==='run'"><option value="s7-200-smart">S7-200 SMART</option><option value="s7-300">S7-300</option><option value="s7-1200">S7-1200</option><option value="s7-1500">S7-1500</option></select></label><label v-if="device.protocol==='siemens-s7'">Rack / Slot<input :value="`${device.rack||0} / ${device.slot||0}`" disabled/></label><label v-if="device.protocol==='iec61850'">IED 名称<input v-model.trim="device.iedName" :disabled="mode==='run'"/></label><label v-if="device.protocol==='opcua'">用户名<input v-model.trim="device.username" :disabled="mode==='run'"/></label><label v-if="device.protocol==='opcua'">密码<input v-model="device.password" type="password" autocomplete="new-password" :disabled="mode==='run'"/></label><label>重连间隔（秒）<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="mode==='run'"/></label></template></div></section>

<section v-if="device" class="content-card"><div class="section-head"><div><h2>点位配置</h2><p>{{points.length}} 个点位，标识符在全网关内唯一。</p></div><div class="button-row"><input v-model.trim="search" class="toolbar-search" placeholder="搜索名称或标识"/><button :disabled="mode==='run'" @click="addPoint">新增点位</button><button :disabled="mode==='run'||!point" @click="duplicatePoint">复制</button><button class="danger" :disabled="mode==='run'||!point" @click="removePoint">删除</button></div></div><div class="point-workspace"><div class="point-list"><button v-for="item in visiblePoints" :key="item.metric" :class="{active:item.metric===pointMetric}" @click="pointMetric=item.metric"><span>{{item.name||item.metric}}</span><code>{{item.metric}}</code><small :class="liveByMetric.get(item.metric)?.error?'bad-text':''">{{liveByMetric.get(item.metric)?.error||String(liveByMetric.get(item.metric)?.value??'--')}}</small></button><p v-if="!visiblePoints.length" class="empty-mini">暂无点位</p></div><div v-if="point" class="point-editor"><div class="config-grid"><label>点位名称<input v-model.trim="point.name" :disabled="mode==='run'"/></label><label>标识符 metric<input v-model.trim="point.metric" :disabled="mode==='run'" @change="pointMetric=point.metric"/></label><template v-if="isForward"><label>源设备<select v-model="point.sourceDeviceKey" :disabled="mode==='run'" @change="point.sourceMetric=''"><option value="">请选择</option><option v-for="item in sourceDevices()" :key="item.deviceKey" :value="item.deviceKey">{{item.name}}（{{item.deviceKey}}）</option></select></label><label>源点位<select v-model="point.sourceMetric" :disabled="mode==='run'||!point.sourceDeviceKey" @change="syncForwardSource"><option value="">请选择</option><option v-for="item in sourcePoints()" :key="item.metric" :value="item.metric">{{item.name}}（{{item.metric}}）</option></select></label></template><template v-else-if="device.protocol.startsWith('modbus')"><label>功能码<select v-model.number="point.function" :disabled="mode==='run'"><option v-for="item in modbusFunctions" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label><label>寄存器地址<input v-model.number="point.register" type="number" min="0" max="65535" :disabled="mode==='run'"/></label><label>寄存器数量<input v-model.number="point.quantity" type="number" min="1" max="125" :disabled="mode==='run'"/></label></template><template v-else-if="device.protocol==='iec104'"><label>IOA 地址<input v-model.number="point.ioa" type="number" min="1" max="16777215" :disabled="mode==='run'"/></label><label>点类型<select v-model="point.pointType" :disabled="mode==='run'"><option value="M_SP_NA_1">单点遥信</option><option value="M_DP_NA_1">双点遥信</option><option value="M_ME_NC_1">短浮点遥测</option><option value="M_IT_NA_1">累计量</option></select></label></template><template v-else-if="device.protocol==='siemens-s7'"><label>区域<select v-model="point.area" :disabled="mode==='run'"><option value="DB">DB</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></label><label>DB 编号<input v-model.number="point.dbNumber" type="number" min="0" :disabled="mode==='run'"/></label><label>字节地址<input v-model.number="point.register" type="number" min="0" :disabled="mode==='run'"/></label></template><template v-else-if="device.protocol==='opcua'"><label class="span-2">Node ID<input v-model.trim="point.nodeId" :disabled="mode==='run'" placeholder="ns=2;s=Demo.Static.Scalar.Float"/></label></template><template v-else-if="device.protocol==='iec61850'"><label class="span-2">对象引用<input v-model.trim="point.objectRef" :disabled="mode==='run'"/></label><label>功能约束 FC<input v-model.trim="point.fc" :disabled="mode==='run'" placeholder="MX / ST"/></label></template><label>数据类型<select v-model="point.dataType" :disabled="mode==='run'"><option v-for="item in dataTypes" :key="item">{{item}}</option></select></label><label>字节序<select v-model="point.byteOrder" :disabled="mode==='run'"><option value="big">大端</option><option value="little">小端</option></select></label><label>字序<select v-model="point.wordOrder" :disabled="mode==='run'"><option value="big">高字在前</option><option value="little">低字在前</option></select></label><label>倍率<input v-model.number="point.scale" type="number" step="any" :disabled="mode==='run'"/></label><label>偏移<input v-model.number="point.offset" type="number" step="any" :disabled="mode==='run'"/></label><label>单位<input v-model.trim="point.unit" :disabled="mode==='run'"/></label><label>小数位<input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="mode==='run'"/></label></div><div class="live-detail"><span>实时值</span><strong :class="liveByMetric.get(point.metric)?.error?'bad-text':''">{{liveByMetric.get(point.metric)?.error||String(liveByMetric.get(point.metric)?.value??'--')}}</strong><small>{{liveByMetric.get(point.metric)?.updatedAt?new Date(liveByMetric.get(point.metric)!.updatedAt!).toLocaleString():'尚无采集结果'}}</small></div></div></div></section>
<section v-if="device&&point&&mode==='run'" class="content-card"><div class="section-head"><div><h2>点位控制</h2><p>仅对协议和点位类型支持写入的点位生效，操作会立即下发到设备。</p></div></div><div class="form-row"><label>写入值<input v-model="writeValue" :placeholder="point.dataType==='bool'?'true / false':'请输入目标值'" @keydown.enter="writePoint" /></label><button class="primary" :disabled="writeValue===''" @click="writePoint">确认写入</button></div></section>
<section v-if="configView==='json'" class="content-card json-config-card"><textarea v-model="jsonText" spellcheck="false"/><div class="button-row"><button @click="jsonText=JSON.stringify(cfg,null,2)">格式化</button><button class="primary" :disabled="saving||mode==='run'" @click="saveJSON">保存 JSON 配置</button></div></section>
<ResourceWorkspace v-if="resource&&detailLevel==='resource'" :resource="resource" :channels="resourceChannels" :collect-devices="collectDevices" :forward-devices="forwardDevices" :readonly="mode==='run'" @add-channel="addChannel" @add-device="addDeviceForChannel" @select-channel="selectChannel" @select-device="selectDeviceFromResource" />
<ChannelWorkspace v-if="resource&&channel&&detailLevel==='channel'" :resource="resource" :channel="channel" :devices="channelDevices" :forward-slave="forwardSlave" :readonly="mode==='run'" @add-device="addDevice" @remove-channel="removeChannel" @select-device="selectDevice" @protocol-change="changeDeviceProtocol" />
<LargeDevicePointWorkspace v-if="device&&store.largeMode" :device="device" :channel-name="channel?.name" :readonly="mode==='run'" :is-forward="isForward" @back="detailLevel='channel';deviceKey=''" @monitor="openMonitor" @test="testDeviceConnection" @remove-device="removeDevice" />
<DevicePointWorkspace v-else-if="device" :device="device" :channel-name="channel?.name" :interface-name="resource?.network?.interface||resource?.serial?.port||channel?.interfaceName||device?.interfaceName" :points="points" :all-points="allConfiguredPoints" :live-by-metric="liveByMetric" :readonly="mode==='run'" :is-forward="isForward" :source-devices="sourceDevices()" :source-channels="channels" :source-resources="resources" @back="detailLevel='channel';deviceKey=''" @monitor="openMonitor" @test="testDeviceConnection" @remove-device="removeDevice" @write="writeWorkspacePoint" />
<div v-if="channelModal" class="restart-modal"><div class="dialog-panel"><h3>新增通道</h3><p>先选择通道用途，再选择该用途对应的协议。</p><div class="channel-dialog-grid"><label>通道用途<select v-model="newChannelRole" @change="changeNewChannelRole"><option value="collect">采集</option><option value="forward">转发</option></select></label><label>协议<select v-model="newChannelProtocol"><option v-for="item in modalProtocols" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label></div><div class="dialog-actions"><button @click="channelModal=false">取消</button><button class="primary" @click="confirmAddChannel">确定新增</button></div></div></div>
<div v-if="unsavedOpen" class="unsaved-config-modal"><div class="dialog-panel"><h3>配置尚未保存</h3><p>当前配置有修改，切换到运行模式前请选择处理方式。</p><div class="dialog-actions"><button @click="unsavedOpen=false">继续编辑</button><button class="danger" @click="discardAndRun">放弃修改并运行</button><button class="primary" :disabled="saving" @click="requestSave">保存修改并运行</button></div></div></div>
<div v-if="networkApplyOpen" class="restart-modal"><div class="dialog-panel network-apply-dialog"><h3>应用网口配置</h3><p>保存后将立即修改网关系统网络参数，页面、采集连接和云平台连接可能短暂中断。</p><div class="network-change-list"><div v-for="item in pendingNetworkPorts" :key="item.resourceKey"><strong>{{item.name}}（{{item.interface}}）</strong><span v-if="item.mode==='dhcp'">自动获取地址（DHCP）</span><span v-else>{{item.system?.ipv4Address||item.previous?.ipAddress||'未配置'}} → {{item.ipAddress}} / {{item.prefixLength}}</span><small>网关 {{item.gateway||'-'}} · DNS {{item.dns.join(', ')||'未配置'}}</small></div></div><p class="meta">若网卡未能启用或未获得目标地址，系统将在约 30 秒后恢复原网络配置。</p><div class="dialog-actions"><button :disabled="saving" @click="networkApplyOpen=false;pendingNetworkPorts=[]">取消</button><button class="primary" :disabled="saving" @click="commitSave(true)">{{saving?'正在保存':'保存并应用'}}</button></div></div></div>
</div></template>
