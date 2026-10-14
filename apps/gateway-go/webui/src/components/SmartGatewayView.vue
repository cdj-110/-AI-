<script setup lang="ts">
import {usePageDraftGuard} from '../composables/usePageDraftGuard.ts'
import {initializeConfigDraft} from '../configDraftDefaults.ts'
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, triggerRef, watch } from 'vue'
import { ApiError, api } from '../api'
import { confirmAction } from '../confirm'
import { openGatewayRealtime } from '../realtime'
import { useConfigStore } from '../stores/config'
import type { PointStatus } from '../types'
import { recordPointStatuses } from '../pointHistory'
import { applySerialBusProtocol, isSerialBusProtocol, serialBusProtocol, type SerialBusProtocol } from '../serialBusProtocol'
import DevicePointWorkspace from './DevicePointWorkspace.vue'
import LargeDevicePointWorkspace from './LargeDevicePointWorkspace.vue'
import ResourceWorkspace from './ResourceWorkspace.vue'
import ChannelWorkspace from './ChannelWorkspace.vue'

type TreeSelection={resourceKey?:string;channelKey?:string;deviceKey?:string;action?:'add-channel'|'add-device';role?:'collect'|'forward';nonce?:number}
type ConnectionTestResult={deviceKey:string;deviceName:string;endpoint:string;detail:string;ok:boolean}
type OPCDASessionReload={deviceKeys:string[];visible:boolean;phase:'reconnecting'|'online'|'failed';detail:string}
const props=defineProps<{selection?:TreeSelection}>()
const emit=defineEmits<{selectionChange:[selection:TreeSelection]}>()
const mode=defineModel<'run'|'config'>('mode',{default:'run'})
const store=useConfigStore(), resourceKey=ref(''), channelKey=ref(''), deviceKey=ref(''), pointMetric=ref('')
const detailLevel=ref<'resource'|'channel'|'device'>('channel')
const configView=ref<'form'|'json'>('form'), jsonText=ref('')
const saving=ref(false), message=ref(''), error=ref(''), search=ref('')
let messageTimer=0, errorTimer=0
function clearNotices(){
  window.clearTimeout(messageTimer);window.clearTimeout(errorTimer)
  messageTimer=0;errorTimer=0;message.value='';error.value=''
}
const opcdaSessionReload=ref<OPCDASessionReload>()
let opcdaSessionReloadSequence=0, opcdaSessionReloadTimer=0
function finishOPCDASessionReload(){
  opcdaSessionReloadSequence++
  window.clearTimeout(opcdaSessionReloadTimer)
  opcdaSessionReloadTimer=0
  opcdaSessionReload.value=undefined
}
const connectionTest=ref<ConnectionTestResult>()
const largeWorkspace=ref<InstanceType<typeof LargeDevicePointWorkspace>>()
const channelModal=ref(false), newChannelRole=ref('collect'), newChannelProtocol=ref('modbus-tcp')
const unsavedOpen=ref(false), savedSnapshot=ref(''), suppressModeChange=ref(false)
const networkApplyOpen=ref(false), pendingNetworkPorts=ref<any[]>([])
const writeValue=ref('')
const writableMetrics=ref<string[]>([])
let writableRequestVersion=0
async function loadWritableMetrics(){
  const version=++writableRequestVersion
  writableMetrics.value=[]
  if(mode.value!=='run'||!deviceKey.value||isForward.value)return
  try{
    const result=await api<{metrics:string[]}>(`/api/points/writable?deviceKey=${encodeURIComponent(deviceKey.value)}`)
    if(version===writableRequestVersion)writableMetrics.value=Array.isArray(result.metrics)?result.metrics:[]
  }catch{
    // A missing write permission or unavailable runtime must not expose a write action.
  }
}
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
const activeConnectionTest=computed(()=>connectionTest.value?.deviceKey===deviceKey.value?connectionTest.value:undefined)
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
const networkCollectProtocols=[['modbus-tcp','Modbus TCP'],['iec104','IEC 60870-5-104'],['siemens-s7','Siemens S7'],['opcua','OPC UA'],['opcda','OPC DA'],['iec61850','IEC 61850 MMS'],['iec61850-goose','IEC 61850 GOOSE']]
const serialCollectProtocols=[['modbus-rtu','Modbus RTU'],['dlt645','DL/T 645']]
const forwardProtocols=[['modbus-tcp-slave','Modbus TCP 从站'],['iec104-server','IEC104 服务端'],['iec61850-mms-server','IEC61850 MMS 服务端'],['iec61850-goose-publisher','IEC61850 GOOSE 发布']]
const modalProtocols=computed(()=>newChannelRole.value==='forward'?forwardProtocols:(resource.value?.type==='serial'?serialCollectProtocols:networkCollectProtocols))
const channelProtocols=computed(()=>isForward.value?forwardProtocols:(resource.value?.type==='serial'?serialCollectProtocols:networkCollectProtocols))
const channelProtocol=computed({get:()=>isForward.value?(channel.value?.forwardProtocol||channel.value?.protocol||'modbus-tcp-slave'):(channel.value?.protocol||'modbus-tcp'),set:(value:string)=>{if(!channel.value)return;if(isForward.value){channel.value.forwardProtocol=value;channel.value.protocol='none'}else channel.value.protocol=value;editChannelProtocol()}})
const dataTypes=['bool','int16','uint16','int32','uint32','float32','float64','string']
const modbusFunctions=[[1,'01 线圈'],[2,'02 离散量'],[3,'03 保持寄存器'],[4,'04 输入寄存器']]

function key(prefix:string, used:string[]){let i=1,next='';do{next=`${prefix}-${String(i++).padStart(3,'0')}`}while(used.includes(next));return next}
function selectResource(value:string){resourceKey.value=value;channelKey.value='';deviceKey.value='';pointMetric.value='';detailLevel.value='resource';emit('selectionChange',{resourceKey:value})}
function selectChannel(value:string){channelKey.value=value;deviceKey.value='';pointMetric.value='';detailLevel.value='channel';emit('selectionChange',{resourceKey:resourceKey.value,channelKey:value})}
function selectDevice(value:string){deviceKey.value=value;pointMetric.value=points.value[0]?.metric||'';detailLevel.value='device';emit('selectionChange',{resourceKey:resourceKey.value,channelKey:channelKey.value,deviceKey:value})}
function applySelection(value?:TreeSelection){if(value?.resourceKey)resourceKey.value=value.resourceKey;if(value?.deviceKey){channelKey.value=value.channelKey||'';deviceKey.value=value.deviceKey}else if(value?.channelKey){channelKey.value=value.channelKey;deviceKey.value='';pointMetric.value=''}else if(value?.resourceKey){channelKey.value='';deviceKey.value='';pointMetric.value=''}if(!resource.value)resourceKey.value=resources.value[0]?.resourceKey||'';detailLevel.value=value?.deviceKey?'device':value?.channelKey?'channel':value?.resourceKey?'resource':detailLevel.value;if(value?.action==='add-channel')addChannel(value.role);else if(value?.action==='add-device')addDeviceForChannel(value.channelKey)}
function ensureSelection(){if(!resource.value)resourceKey.value=resources.value[0]?.resourceKey||'';if(!resourceChannels.value.some(item=>item.channelKey===channelKey.value))channelKey.value=resourceChannels.value[0]?.channelKey||'';if(channel.value?.protocol==='iec104'&&!channel.value.iec104)channel.value.iec104={acquisitionMode:'auto',generalInterrogationOnStart:true,generalInterrogationIntervalSeconds:0,clockSyncOnStart:true,clockSyncIntervalSeconds:3600,counterInterrogationOnStart:false,counterInterrogationIntervalSeconds:0};if(!channelDevices.value.some(item=>item.deviceKey===deviceKey.value))deviceKey.value=channelDevices.value[0]?.deviceKey||'';if(!points.value.some(item=>item.metric===pointMetric.value))pointMetric.value=points.value[0]?.metric||''}
function addResource(){const value=key('resource',resources.value.map(item=>item.resourceKey));resources.value.push({resourceKey:value,name:`资源${resources.value.length+1}`,type:'network',enabled:true,network:{name:value,interface:'eth0',mode:'static',prefixLength:24,enabled:true}});selectResource(value);mode.value='config'}
async function removeResource(){if(!resource.value||!await confirmAction({title:'删除资源',message:`确认删除资源“${resource.value.name}”？`,detail:'该资源下的全部通道、设备和点位也会一并删除。',confirmText:'确认删除',danger:true}))return;const removeChannels=new Set(resourceChannels.value.map(item=>item.channelKey));cfg.value.channels=channels.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.devices=collectDevices.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.forwardDevices=forwardDevices.value.filter(item=>!removeChannels.has(item.channelKey));cfg.value.resources=resources.value.filter(item=>item.resourceKey!==resourceKey.value);resourceKey.value='';ensureSelection()}
function changeResourceType(){if(!resource.value)return;if(resource.value.type==='serial'){resource.value.network=undefined;resource.value.serial={name:resource.value.name||resource.value.resourceKey,port:'/dev/ttyS1',baudRate:9600,dataBits:8,stopBits:1,parity:'none',enabled:true}}else{resource.value.serial=undefined;resource.value.network={name:resource.value.name||resource.value.resourceKey,interface:'eth0',mode:'static',prefixLength:24,dns:[],enabled:true}}}
function addChannel(role:'collect'|'forward'='collect'){if(!resource.value){error.value='请先新增资源';return}newChannelRole.value=role;changeNewChannelRole();channelModal.value=true}
function changeNewChannelRole(){newChannelProtocol.value=newChannelRole.value==='forward'?'modbus-tcp-slave':resource.value?.type==='serial'?'modbus-rtu':'modbus-tcp'}
function confirmAddChannel(){if(!resource.value)return;const value=key('channel',channels.value.map(item=>item.channelKey));const forward=newChannelRole.value==='forward';channels.value.push({channelKey:value,resourceKey:resourceKey.value,name:`通道${resourceChannels.value.length+1}`,type:resource.value.type,role:newChannelRole.value,protocol:forward?'none':newChannelProtocol.value,forwardProtocol:forward?newChannelProtocol.value:'',collectIntervalSeconds:5,interfaceName:resource.value.name,enabled:true});if(forward)forwardSlave.value.enabled=true;channelModal.value=false;selectChannel(value);mode.value='config'}
async function removeChannel(){if(!channel.value||!await confirmAction({title:'删除通道',message:`确认删除通道“${channel.value.name||channel.value.channelKey}”？`,detail:'该通道下挂的全部设备和点位也会一并删除。',confirmText:'确认删除',danger:true}))return;const parentResourceKey=resourceKey.value,targetChannelKey=channelKey.value;cfg.value.devices=collectDevices.value.filter(item=>item.channelKey!==targetChannelKey);cfg.value.forwardDevices=forwardDevices.value.filter(item=>item.channelKey!==targetChannelKey);cfg.value.channels=channels.value.filter(item=>item.channelKey!==targetChannelKey);selectResource(parentResourceKey)}
function changeChannelRole(){if(!channel.value)return;if(channel.value.role==='forward'){channel.value.forwardProtocol=channel.value.protocol||'modbus-tcp';channel.value.protocol=channel.value.forwardProtocol}else channel.value.forwardProtocol='';deviceKey.value='';ensureSelection()}
function addDevice(){
  if(!channel.value){error.value='请先新增通道';return}
  const all=[...collectDevices.value,...forwardDevices.value,...(cfg.value.edgeComputing?.groups||[])].map(item=>item.deviceKey||item.groupKey)
  const value=key('device',all)
  if(isForward.value){
    const protocol=channel.value.forwardProtocol||'modbus-tcp-slave',index=channelDevices.value.length+1
    forwardDevices.value.push({deviceKey:value,channelKey:channelKey.value,name:`转发设备${index}`,protocol,enabled:true,unitId:1,commonAddress:1,iedName:'WEIKONG',logicalDevice:`LD${index}`,interfaceName:resource.value?.network?.interface||'eth0',goCbRef:`WEIKONGLD${index}/LLN0$GO$gcb01`,dataSetRef:`WEIKONGLD${index}/LLN0$Dataset01`,appId:4096,destinationMac:'01:0c:cd:01:00:01',vlanId:0,vlanPriority:4,timeAllowedToLiveMilliseconds:2000,confRev:1,points:[]})
    forwardSlave.value.enabled=true
  }else{
    const protocol=channel.value.protocol||'modbus-tcp',serialProtocol=['modbus-rtu','dlt645'].includes(protocol)
    collectDevices.value.push({deviceKey:value,channelKey:channelKey.value,name:`采集设备${channelDevices.value.length+1}`,protocol,interfaceType:resource.value?.type||'network',interfaceName:resource.value?.name||'',address:serialProtocol?(resource.value?.serial?.port||'/dev/ttyS1'):protocol==='iec61850-goose'?(resource.value?.network?.interface||'eth0'):protocol==='opcda'?'192.168.1.141':'192.168.1.100:502',slaveId:1,commonAddress:1,reconnectIntervalSeconds:30,dlt645:protocol==='dlt645'?{version:'auto',meterAddress:'000000000001',responseTimeoutMilliseconds:1500,retries:1,wakeupBytes:4,writePassword:'',operatorCode:''}:undefined,progId:'',clsid:'',domain:'',workstation:'EG100',securityMode:'packet_integrity',authentication:'ntlm',authenticationLevel:'packet_integrity',connectTimeoutMilliseconds:10000,requestTimeoutMilliseconds:10000,readMode:'sync',readSource:'device',batchSize:100,autoReconnect:true,groupName:'WK_OPCDA',requestedUpdateRateMilliseconds:1000,localeId:1033,localBindIp:'',goCbRef:'WEIKONGLD1/LLN0$GO$gcb01',dataSetRef:'WEIKONGLD1/LLN0$Dataset01',appId:4096,destinationMac:'01:0c:cd:01:00:01',vlanId:0,vlanPriority:4,points:[]})
  }
  selectDevice(value);mode.value='config'
}

async function changeOPCDASecurityMode(){
  if(!device.value||device.value.protocol!=='opcda')return
  if(device.value.securityMode==='legacy_connect'){
    const accepted=await confirmAction({title:'启用 Legacy Connect',message:'Legacy Connect 仅用于旧版 DCOM 系统，安全性较低。',detail:'Packet Integrity 不会在认证失败后自动降级。仅在隔离的旧系统环境中主动选择 Legacy。',confirmText:'继续使用 Legacy',danger:true})
    if(!accepted)device.value.securityMode='packet_integrity'
  }
  device.value.authenticationLevel=device.value.securityMode==='legacy_connect'?'connect':device.value.securityMode==='packet_privacy'?'packet_privacy':'packet_integrity'
}
function addDeviceForChannel(value?:string){let target=value||resourceChannels.value[0]?.channelKey;if(!target&&resource.value?.type==='serial'){target=key('channel',channels.value.map(item=>item.channelKey));channels.value.push({channelKey:target,resourceKey:resourceKey.value,name:resource.value.name||target,type:'serial',role:'collect',protocol:'modbus-rtu',collectIntervalSeconds:resource.value.collectIntervalSeconds||1,interfaceName:resource.value.name,enabled:true})}if(!target){error.value='当前资源尚未建立设备通道';return}selectChannel(target);addDevice()}
async function changeSerialResourceProtocol(protocol:SerialBusProtocol){
  if(!resource.value||resource.value.type!=='serial'||!isSerialBusProtocol(protocol))return
  let targetChannels=resourceChannels.value.filter(item=>item.role!=='forward')
  const targetChannelKeys=new Set(targetChannels.map(item=>item.channelKey))
  const affectedDevices=collectDevices.value.filter(item=>targetChannelKeys.has(item.channelKey))
  const current=serialBusProtocol(targetChannels)
  const changed=current!==protocol||affectedDevices.some(item=>String(item.protocol||'').toLowerCase()!==protocol)
  if(!changed)return
  const pointCount=affectedDevices.reduce((total,item)=>total+(item.points?.length||0),0)
  if(affectedDevices.length){
    const accepted=await confirmAction({title:'切换 RS485 采集协议',message:`确认将“${resource.value.name||resource.value.resourceKey}”下的全部采集设备切换为 ${protocol==='dlt645'?'DL/T 645':'Modbus RTU'}？`,detail:pointCount?`两种协议的点位参数不兼容，将清除 ${pointCount} 个原有点位；保存配置后生效。`:'该总线下的设备协议将同步更新，保存配置后生效。',confirmText:'确认切换',danger:pointCount>0})
    if(!accepted)return
  }
  if(!targetChannels.length){
    const value=key('channel',channels.value.map(item=>item.channelKey))
    const created={channelKey:value,resourceKey:resourceKey.value,name:resource.value.name||value,type:'serial',role:'collect',protocol,collectIntervalMilliseconds:resource.value.collectIntervalMilliseconds||1000,interfaceName:resource.value.name,enabled:true}
    channels.value.push(created)
    targetChannels=[created]
  }
  const result=applySerialBusProtocol(targetChannels,collectDevices.value,protocol,{name:resource.value.name,port:resource.value.serial?.port})
  message.value=`采集协议已统一为 ${protocol==='dlt645'?'DL/T 645':'Modbus RTU'}${result.removedPoints?`，已移除 ${result.removedPoints} 个不兼容点位`:''}；请保存配置`
  mode.value='config'
}
function selectDeviceFromResource(targetChannel:string,targetDevice:string){selectChannel(targetChannel);selectDevice(targetDevice)}
async function removeDevice(){
  if(!device.value||!await confirmAction({title:'删除设备',message:`确认删除设备“${device.value.name||device.value.deviceKey}”？`,detail:'该设备下的全部点位也会一并删除。',confirmText:'确认删除',danger:true}))return
  const target=isForward.value?'forwardDevices':'devices'
  const parentChannelKey=channelKey.value
  cfg.value[target]=(cfg.value[target]||[]).filter((item:any)=>item.deviceKey!==deviceKey.value)
  connectionTest.value=undefined
  selectChannel(parentChannelKey)
}
function nextMetric(){const used=[...collectDevices.value.flatMap(item=>item.points||[]),...forwardDevices.value.flatMap(item=>item.points||[]),...(cfg.value.edgeComputing?.groups||[]).flatMap((item:any)=>item.points||[])].map((item:any)=>item.metric);return key('point',used)}
function addPoint(){if(!device.value){error.value='请先新增设备';return}const metric=nextMetric();if(isForward.value)device.value.points.push({name:`转发点位${points.value.length+1}`,metric,sourceDeviceKey:'',sourceMetric:'',function:3,register:0,ioa:points.value.length+1,quantity:1,dataType:'uint16',byteOrder:'big',wordOrder:'big',scale:1});else device.value.points.push({deviceKey:device.value.deviceKey,channelKey:channelKey.value,name:`点位${points.value.length+1}`,metric,protocol:device.value.protocol,address:device.value.address,slaveId:device.value.slaveId||1,commonAddress:device.value.commonAddress||1,itemId:'',accessPath:'',access:'read',requestedDataType:'auto',active:true,enabled:true,function:3,register:0,ioa:points.value.length+1,quantity:1,dataType:device.value.protocol==='opcda'?'auto':device.value.protocol==='dlt645'?'float64':'uint16',dlt645:device.value.protocol==='dlt645'?{itemKey:'',dataIdentifier:'00000000',valueFormat:'bcd',dataLength:4,access:'read'}:undefined,byteOrder:'big',wordOrder:'big',scale:1,offset:0,decimals:0});pointMetric.value=metric;mode.value='config'}
function duplicatePoint(){if(!point.value)return;const copy=JSON.parse(JSON.stringify(point.value));copy.metric=nextMetric();copy.name=`${point.value.name||point.value.metric} 副本`;points.value.push(copy);pointMetric.value=copy.metric}
async function removePoint(){if(!point.value||!await confirmAction({title:'删除点位',message:`确认删除点位“${point.value.name||point.value.metric}”？`,detail:'删除后需保存配置才会正式生效。',confirmText:'确认删除',danger:true}))return;device.value.points=points.value.filter(item=>item!==point.value);pointMetric.value=device.value.points[0]?.metric||''}
function sourceDevices(){return collectDevices.value.filter(item=>item.deviceKey!==deviceKey.value)}
function sourcePoints(){const source=collectDevices.value.find(item=>item.deviceKey===point.value?.sourceDeviceKey);return source?.points||[]}
function syncForwardSource(){const source=sourcePoints().find((item:any)=>item.metric===point.value.sourceMetric);if(source){point.value.name=point.value.name||source.name;point.value.dataType=point.value.dataType||source.dataType;point.value.quantity=point.value.quantity||source.quantity||1}}
function changeDeviceProtocol(){
  if(!channel.value)return
  if(channel.value.protocol==='iec104'&&!channel.value.iec104)channel.value.iec104={acquisitionMode:'auto',generalInterrogationOnStart:true,generalInterrogationIntervalSeconds:0,clockSyncOnStart:true,clockSyncIntervalSeconds:3600,counterInterrogationOnStart:false,counterInterrogationIntervalSeconds:0}
  if(isForward.value){
    channel.value.protocol='none'
    for(const [index,item] of channelDevices.value.entries()){
      item.protocol=channel.value.forwardProtocol||'modbus-tcp-slave'
      if(item.protocol==='iec61850-mms-server'||item.protocol==='iec61850-goose-publisher'){item.iedName=item.iedName||'WEIKONG';item.logicalDevice=item.logicalDevice||`LD${index+1}`}
      if(item.protocol==='iec61850-goose-publisher'){item.interfaceName=item.interfaceName||resource.value?.network?.interface||'eth0';item.goCbRef=item.goCbRef||`${item.iedName}${item.logicalDevice}/LLN0$GO$gcb01`;item.dataSetRef=item.dataSetRef||`${item.iedName}${item.logicalDevice}/LLN0$Dataset01`;item.appId=item.appId||4096;item.destinationMac=item.destinationMac||'01:0c:cd:01:00:01';item.vlanPriority=item.vlanPriority??4;item.timeAllowedToLiveMilliseconds=item.timeAllowedToLiveMilliseconds||2000;item.confRev=item.confRev||1}
    }
    return
  }
  for(const item of channelDevices.value){
    item.protocol=channel.value.protocol
    if(item.protocol==='dlt645'){
      item.address=resource.value?.serial?.port||item.address||'/dev/ttyS1'
      item.dlt645=item.dlt645||{version:'auto',meterAddress:'000000000001',responseTimeoutMilliseconds:1500,retries:1,wakeupBytes:4,writePassword:'',operatorCode:''}
    }
    if(item.protocol==='iec61850-goose'){item.address=resource.value?.network?.interface||item.address||'eth0';item.goCbRef=item.goCbRef||'WEIKONGLD1/LLN0$GO$gcb01';item.dataSetRef=item.dataSetRef||'WEIKONGLD1/LLN0$Dataset01';item.appId=item.appId||4096;item.destinationMac=item.destinationMac||'01:0c:cd:01:00:01';item.vlanPriority=item.vlanPriority??4}
    for(const p of item.points||[]){p.protocol=item.protocol;p.address=item.address;p.goCbRef=item.goCbRef;p.dataSetRef=item.dataSetRef;p.appId=item.appId;p.destinationMac=item.destinationMac;p.vlanId=item.vlanId;p.vlanPriority=item.vlanPriority;if(item.protocol==='dlt645')p.dlt645=p.dlt645||{itemKey:'',dataIdentifier:'00000000',valueFormat:'bcd',dataLength:4,access:'read'}}
  }
}
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
  recordPointStatuses(points)
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
  recordPointStatuses(points)
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
function writePayloadValue(target:any,raw:string){
  // OPC DA keeps the user's text intact so the native layer can convert it
  // with the canonical VARIANT type returned by AddItems. This also preserves
  // uint64 values that cannot be represented exactly by JavaScript Number.
  if(['opcda','dlt645'].includes(String(target.protocol||device.value?.protocol||'').toLowerCase()))return raw
  if(target.dataType==='bool')return['1','true','on'].includes(raw.toLowerCase())
  return target.dataType==='string'?raw:Number(raw)
}
async function writePoint(){if(!device.value||!point.value||writeValue.value==='')return;try{const value=writePayloadValue(point.value,writeValue.value);await api('/api/points/write',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({deviceKey:device.value.deviceKey,metric:point.value.metric,value})});message.value=`点位 ${point.value.name||point.value.metric} 写入成功`;writeValue.value='';setTimeout(loadStatus,500)}catch(cause){error.value=cause instanceof Error?cause.message:'点位写入失败'}}
async function writeWorkspacePoint(target:any,raw:string,done?:(success:boolean,value?:unknown,failure?:{message:string;detail?:string;code?:string})=>void){if(!device.value){done?.(false,undefined,{message:'写入失败：未找到当前设备。'});return}try{const value=writePayloadValue(target,raw);await api('/api/points/write',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({deviceKey:device.value.deviceKey,metric:target.metric,value})});message.value=`点位 ${target.name||target.metric} 写入成功`;done?.(true,value);setTimeout(loadStatus,500)}catch(cause){const failure={message:cause instanceof Error?cause.message:'点位写入失败',detail:cause instanceof ApiError?cause.detail:undefined,code:cause instanceof ApiError?cause.code:undefined};error.value=failure.message;done?.(false,undefined,failure)}}
function connectionEndpoint(protocol:string,address:string){
  const value=String(address||'').trim()
  if(!value)return'-'
  if(value.includes('://')||value.includes(':'))return value
  if(protocol==='modbus-tcp')return`${value}:502`
  if(protocol==='iec104')return`${value}:2404`
  if(protocol==='siemens-s7'||protocol==='iec61850')return`${value}:102`
  return value
}
function connectionFailure(cause:unknown,fallbackEndpoint:string){
  const raw=cause instanceof Error?cause.message:'连接测试失败'
  try{
    const parsed=JSON.parse(raw)
    return{detail:String(parsed.message||raw),endpoint:String(parsed.address||fallbackEndpoint)}
  }catch{return{detail:raw,endpoint:fallbackEndpoint}}
}
async function testDeviceConnection(){
  if(!device.value)return
  const probePoint=(device.value.points||[]).find((point:any)=>[1,2,3,4].includes(Number(point.function)))
  const target={
    deviceKey:String(device.value.deviceKey),deviceName:String(device.value.name||device.value.deviceKey),
    protocol:String(device.value.protocol||''),address:String(device.value.address||''),
    baudRate:resource.value?.serial?.baudRate,dataBits:resource.value?.serial?.dataBits,stopBits:resource.value?.serial?.stopBits,parity:resource.value?.serial?.parity,
    dlt645:device.value.dlt645,
    slaveId:device.value.slaveId,commonAddress:device.value.commonAddress,rack:device.value.rack,slot:device.value.slot,
    function:probePoint?.function??3,register:probePoint?.register??0,
    localTsap:device.value.localTsap,remoteTsap:device.value.remoteTsap,username:device.value.username,password:device.value.password,
    progId:device.value.progId,clsid:device.value.clsid,domain:device.value.domain,workstation:device.value.workstation,
    securityMode:device.value.securityMode,authentication:device.value.authentication,
    authenticationLevel:device.value.authenticationLevel,connectTimeoutMilliseconds:device.value.connectTimeoutMilliseconds,
    requestTimeoutMilliseconds:device.value.requestTimeoutMilliseconds,groupName:device.value.groupName,
    requestedUpdateRateMilliseconds:device.value.requestedUpdateRateMilliseconds,deadband:device.value.deadband,
    localeId:device.value.localeId,localBindIp:device.value.localBindIp,
  }
  connectionTest.value=undefined;message.value='';error.value=''
  const fallbackEndpoint=connectionEndpoint(target.protocol,target.address)
  try{
    const result=await api<{message?:string;address?:string}>('/api/connection/test',{method:'POST',headers:{'Content-Type':'application/json'},timeoutMs:65000,body:JSON.stringify(target)})
    if(deviceKey.value!==target.deviceKey)return
    connectionTest.value={deviceKey:target.deviceKey,deviceName:target.deviceName,endpoint:result.address||fallbackEndpoint,detail:result.message||'连接成功',ok:true}
  }catch(cause){
    if(deviceKey.value!==target.deviceKey)return
    const failure=connectionFailure(cause,fallbackEndpoint)
    connectionTest.value={deviceKey:target.deviceKey,deviceName:target.deviceName,endpoint:failure.endpoint,detail:failure.detail,ok:false}
  }
}
async function readDLT645Address(){
  if(!device.value||device.value.protocol!=='dlt645')return
  if(!await confirmAction({title:'读取通信地址',message:'请确认当前串口总线上只连接了一只电表。',detail:'DL/T 645-2007 可广播读取通信地址；部分 1997 电表不支持此命令。',confirmText:'开始读取'}))return
  try{
    const result=await api<{meterAddress:string;message?:string}>('/api/dlt645/read-address',{method:'POST',headers:{'Content-Type':'application/json'},timeoutMs:15000,body:JSON.stringify({protocol:'dlt645',address:device.value.address,baudRate:resource.value?.serial?.baudRate,dataBits:resource.value?.serial?.dataBits,stopBits:resource.value?.serial?.stopBits,parity:resource.value?.serial?.parity,dlt645:device.value.dlt645})})
    device.value.dlt645.meterAddress=result.meterAddress;message.value=result.message||`读取到通信地址 ${result.meterAddress}`
  }catch(cause){error.value=cause instanceof Error?cause.message:'读取通信地址失败'}
}
function hasUnsavedChanges(){
  if(!savedSnapshot.value)return false
  const comparable=(value:any)=>JSON.stringify(value,(key,item)=>key==='_pointCount'?undefined:item)
  if(comparable(cfg.value)!==comparable(JSON.parse(savedSnapshot.value)))return true
  if(configView.value==='json'){
    try{return JSON.stringify(JSON.parse(jsonText.value))!==savedSnapshot.value}catch{return true}
  }
  return false
}
const requestLeave=usePageDraftGuard(hasUnsavedChanges,()=>{store.value=JSON.parse(savedSnapshot.value);jsonText.value=JSON.stringify(store.value,null,2)},saveBeforeNavigation,'.gateway-editor .device-tabs button, .gateway-editor .iec104-point-tabs button, .gateway-editor .point-groups button')
let navigating=false
async function navigateWithin(action:()=>void){
  if(navigating)return false
  navigating=true
  try{if(largeWorkspace.value&&!await largeWorkspace.value.requestLeave())return false;if(!await requestLeave())return false;action();return true}finally{navigating=false}
}
async function saveBeforeNavigation(){
  if(saving.value)return false
  if(largeWorkspace.value&&!await largeWorkspace.value.requestLeave())return false
  if(configView.value==='json'){await saveJSON();return !hasUnsavedChanges()}
  const changed=changedNetworkPorts()
  if(changed.length){
    if(!await confirmAction({title:'保存并应用网口配置',message:'网口参数有变化，应用后当前连接可能暂时中断。',detail:'确认后保存并应用网络配置；取消将保留草稿。',confirmText:'保存并应用',cancelText:'继续编辑'}))return false
    pendingNetworkPorts.value=changed
  }
  return await commitSave(changed.length>0,false)
}
function markSaved(){initializeConfigDraft(cfg.value);savedSnapshot.value=JSON.stringify(cfg.value)}
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
async function requestSave(){
  if(largeWorkspace.value&&!await largeWorkspace.value.savePendingRows())return
  const changed=changedNetworkPorts()
  if(changed.length){pendingNetworkPorts.value=changed;networkApplyOpen.value=true;return}
  void commitSave(false)
}
async function waitForOPCDASessionReload(deviceKeys:string[],startedAt:number){
  const targets=[...new Set(deviceKeys.filter(Boolean))]
  if(!targets.length)return true
  const sequence=++opcdaSessionReloadSequence
  opcdaSessionReload.value={deviceKeys:targets,visible:false,phase:'reconnecting',detail:'正在重新建立 DCOM 激活、OPC Group 和数据回调…'}
  opcdaSessionReloadTimer=window.setTimeout(()=>{
    if(opcdaSessionReloadSequence===sequence&&opcdaSessionReload.value)opcdaSessionReload.value.visible=true
  },350)
  const deadline=Date.now()+30000
  let lastDetail='等待目标设备返回新的采集结果'
  while(opcdaSessionReloadSequence===sequence&&Date.now()<deadline){
    try{
      const query=new URLSearchParams()
      targets.forEach(key=>query.append('deviceKey',key))
      const statuses=await api<PointStatus[]>(`/api/point-status?${query}`)
      const ready=targets.every(key=>{
        const target=collectDevices.value.find(item=>item.deviceKey===key)
        const configured=(target?.points||[]).filter((item:any)=>item.enabled!==false&&item.active!==false)
        if(!configured.length)return true
        const wanted=String(target?.readMode||'sync')==='subscription'?'subscription':'sync'
        const metrics=new Set(configured.map((item:any)=>String(item.metric)))
        return statuses.some(item=>item.deviceKey===key&&metrics.has(item.metric)&&item.collectionMode===wanted&&!item.error&&new Date(item.receiveTimestamp||item.updatedAt||0).getTime()>=startedAt-1000)
      })
      if(ready){
        window.clearTimeout(opcdaSessionReloadTimer)
        if(opcdaSessionReload.value){
          opcdaSessionReload.value={...opcdaSessionReload.value,visible:true,phase:'online',detail:'目标 OPC DA 设备会话已重新建立，采集已恢复。'}
        }
        await new Promise(resolve=>window.setTimeout(resolve,650))
        if(opcdaSessionReloadSequence===sequence)finishOPCDASessionReload()
        return true
      }
      const latestError=statuses.find(item=>targets.includes(item.deviceKey)&&item.error)?.error
      if(latestError)lastDetail=latestError
    }catch(cause){
      lastDetail=cause instanceof Error?cause.message:'读取 OPC DA 会话状态失败'
    }
    await new Promise(resolve=>window.setTimeout(resolve,500))
  }
  window.clearTimeout(opcdaSessionReloadTimer)
  if(opcdaSessionReloadSequence===sequence&&opcdaSessionReload.value){
    opcdaSessionReload.value={...opcdaSessionReload.value,visible:true,phase:'failed',detail:`会话仍在后台自动重连：${lastDetail}`}
  }
  return false
}
async function saveWithSessionHotReload(next:Record<string,any>=cfg.value){
  const startedAt=Date.now()
  let result=await store.save(next)
  if(result.restartConfirmationRequired){
    if(!await confirmAction({title:'保存配置并重启服务',message:result.message||'此配置需要重启网关服务后才能生效。',detail:'重启期间采集和当前连接会短暂中断；取消将保留草稿。',confirmText:'保存并重启',cancelText:'继续编辑'}))throw new Error('已取消保存，修改仍保留。')
    result=await store.save(next,true)
    if(result.restartConfirmationRequired)throw new Error('配置未保存，请检查重启确认状态。')
  }
  if(result.opcdaSessionReloaded){
    const ready=await waitForOPCDASessionReload(result.opcdaSessionDevices||[],startedAt)
    return {...result,opcdaSessionReady:ready}
  }
  return {...result,opcdaSessionReady:undefined}
}
async function commitSave(applyNetwork:boolean,runAfterSave=true){
  const networkChanges=applyNetwork?[...pendingNetworkPorts.value]:[]
  saving.value=true;error.value='';message.value=''
  try{
    const result=await saveWithSessionHotReload()
    if(result.restartRequired){
      markSaved();unsavedOpen.value=false;networkApplyOpen.value=false;if(runAfterSave)enterRunMode()
      message.value='配置已保存，网关服务正在重启，恢复后页面将自动刷新。'
      return false
    }
    await Promise.all([store.load(),loadStatus()])
    if(mode.value==='run')void loadWritableMetrics()
    markSaved();ensureSelection();unsavedOpen.value=false;networkApplyOpen.value=false;if(runAfterSave)enterRunMode()
    if(networkChanges.length){
      for(const item of networkChanges){
        const port={name:item.name,interface:item.interface,mode:item.mode,ipAddress:item.ipAddress,prefixLength:item.prefixLength,gateway:item.gateway,dns:item.dns,enabled:item.enabled}
        await api('/api/network/apply',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(port),timeoutMs:10000})
      }
      const changedAddress=networkChanges.find(networkAddressChanged)
      message.value=changedAddress&&changedAddress.mode==='static'
        ?`网口配置正在应用，页面可能短暂断开。请稍后使用 ${location.protocol}//${changedAddress.ipAddress}:${location.port||'80'} 重新访问；网卡未能获得目标地址时会自动回滚。`
        :'网口配置正在应用，DNS 与网络参数稍后生效；网卡应用失败时会自动回滚。'
    }else if(result.opcdaSessionReloaded&&result.opcdaSessionReady)message.value='配置已保存，OPC DA 设备会话已热重建并恢复采集'
    else if(result.opcdaSessionReloaded){error.value='配置已保存，但 OPC DA 设备会话仍在后台自动重连，请查看重连窗口中的诊断信息。'}
    else message.value='配置已保存并热加载'
    return true
  }catch(cause){finishOPCDASessionReload();error.value=cause instanceof Error?cause.message:'保存失败';return false}
  finally{saving.value=false;pendingNetworkPorts.value=[]}
}
async function discardAndRun(){unsavedOpen.value=false;await store.load();markSaved();ensureSelection();enterRunMode()}
function setConfigView(value:'form'|'json'){if(value===configView.value)return;void navigateWithin(()=>{configView.value=value;if(value==='json')jsonText.value=JSON.stringify(cfg.value,null,2);document.documentElement.dataset.configView=value})}
async function saveJSON(){saving.value=true;error.value='';try{const parsed=JSON.parse(jsonText.value);const result=await saveWithSessionHotReload(parsed);if(result.restartRequired){markSaved();jsonText.value=JSON.stringify(store.value,null,2);message.value='配置已保存，但该配置仍需要网关服务重启。';return}await store.load();markSaved();jsonText.value=JSON.stringify(store.value,null,2);if(result.opcdaSessionReloaded&&result.opcdaSessionReady)message.value='JSON 配置已保存，OPC DA 设备会话已热重建并恢复采集';else if(result.opcdaSessionReloaded)error.value='JSON 配置已保存，但 OPC DA 设备会话仍在后台自动重连。';else message.value='JSON 配置已保存并热加载'}catch(cause){finishOPCDASessionReload();error.value=cause instanceof Error?cause.message:'JSON 配置保存失败'}finally{saving.value=false}}
function openMonitor(){if(!device.value)return;const query=new URLSearchParams({deviceKey:device.value.deviceKey,deviceName:device.value.name||device.value.deviceKey,protocol:device.value.protocol||''});window.open(`/packet-monitor?${query}`,'_blank','noopener')}
async function projectImported(){if(largeWorkspace.value&&!await largeWorkspace.value.requestLeave())return;if(!await requestLeave())return;await Promise.all([store.load(),loadStatus()]);markSaved();ensureSelection();if(mode.value==='run')void loadWritableMetrics();message.value='工程配置已重新加载'}
function ensureOPCDAChannelDefaults(){
  if(channel.value?.protocol!=='opcda'||isForward.value)return
  for(const item of channelDevices.value){
    if(!item.address||String(item.address).includes(':')||String(item.address).includes('://'))item.address='192.168.1.141'
    item.workstation=item.workstation||'EG100'
    item.securityMode=!item.securityMode||item.securityMode==='modern'?'packet_integrity':item.securityMode
    item.authentication='ntlm'
    item.authenticationLevel=item.securityMode==='legacy_connect'?'connect':item.securityMode==='packet_privacy'?'packet_privacy':'packet_integrity'
    item.connectTimeoutMilliseconds=item.connectTimeoutMilliseconds||10000
    item.requestTimeoutMilliseconds=item.requestTimeoutMilliseconds||10000
    item.readMode=item.readMode||'sync'
    item.readSource=item.readSource||'device'
    item.batchSize=item.batchSize||100
    item.autoReconnect=item.autoReconnect??true
    item.groupName=item.groupName||'WK_OPCDA'
    item.requestedUpdateRateMilliseconds=item.requestedUpdateRateMilliseconds||1000
    item.localeId=item.localeId||1033
    item.reconnectIntervalSeconds=item.reconnectIntervalSeconds||30
    for(const point of item.points||[]){
      point.protocol='opcda';point.address=item.address;point.itemId=point.itemId||'';point.accessPath=point.accessPath||'';point.access=point.access||'read';point.requestedDataType=point.requestedDataType||'auto';point.active=point.active??true;point.enabled=point.enabled??true;point.dataType=point.dataType==='uint16'?'auto':point.dataType||'auto'
    }
  }
}
function editChannelProtocol(){changeDeviceProtocol();ensureOPCDAChannelDefaults()}
watch(()=>props.selection,(value)=>{
  const current={resourceKey:resourceKey.value,...(detailLevel.value!=='resource'?{channelKey:channelKey.value}:{}),...(detailLevel.value==='device'?{deviceKey:deviceKey.value}:{})}
  if(value?.resourceKey===current.resourceKey&&(value.channelKey||'')===(current.channelKey||'')&&(value.deviceKey||'')===(current.deviceKey||'')&&!value.action)return
  void navigateWithin(()=>applySelection(value)).then(allowed=>{if(!allowed)emit('selectionChange',current)})
},{deep:true})
// Selection/loading is not a protocol edit. Synchronize only explicit edits.
watch(deviceKey,(value,previous)=>{if(value!==previous)connectionTest.value=undefined})
watch(()=>[mode.value,deviceKey.value,isForward.value],()=>void loadWritableMetrics(),{immediate:true})
watch(()=>[deviceKey.value,isForward.value,points.value.length],rebuildLiveStatus)
watch(detailLevel,(value)=>{document.documentElement.dataset.gatewayDetail=value},{immediate:true})
watch(message,value=>{
  window.clearTimeout(messageTimer)
  if(!value)return
  if(error.value)error.value=''
  const current=value
  messageTimer=window.setTimeout(()=>{if(message.value===current)message.value=''},3500)
})
watch(error,value=>{
  window.clearTimeout(errorTimer)
  if(!value)return
  if(message.value)message.value=''
  const current=value
  errorTimer=window.setTimeout(()=>{if(error.value===current)error.value=''},6000)
})
watch(()=>[resourceKey.value,channelKey.value,deviceKey.value,detailLevel.value,configView.value,mode.value],(value,previous)=>{
  if(previous&&value.some((item,index)=>item!==previous[index]))clearNotices()
},{flush:'sync'})
watch(mode,async(value,previous)=>{
  if(suppressModeChange.value){suppressModeChange.value=false;return}
  if(value!=='run'||previous!=='config')return
  if(largeWorkspace.value?.hasPendingEdits()){
    suppressModeChange.value=true;mode.value='config'
    if(!await largeWorkspace.value.requestLeave())return
    if(hasUnsavedChanges())unsavedOpen.value=true
    else enterRunMode()
    return
  }
  if(hasUnsavedChanges()){suppressModeChange.value=true;mode.value='config';unsavedOpen.value=true}
})
watch(mode,value=>{
  if(value==='config')clearStatus()
  else void loadStatus()
})
onMounted(async()=>{document.documentElement.dataset.configView=configView.value;if(!store.value.gatewayKey)await store.load();markSaved();applySelection(props.selection);if(!props.selection?.resourceKey){resourceKey.value=resources.value[0]?.resourceKey||'';detailLevel.value='resource'}await Promise.all([loadStatus(),loadNetworkInterfaces()]);if(!store.largeMode)stopRealtime=openGatewayRealtime(handleRealtime)})
onBeforeUnmount(()=>{clearNotices();finishOPCDASessionReload();stopRealtime()})
</script>

<template><div class="view-stack gateway-editor"><div v-if="opcdaSessionReload?.visible" class="restart-modal opcda-session-reconnect"><div class="dialog-panel"><div class="session-reconnect-loader" :class="opcdaSessionReload.phase"><span/></div><small>OPC DA SESSION HOT RELOAD</small><h3>{{opcdaSessionReload.phase==='online'?'会话重连成功':opcdaSessionReload.phase==='failed'?'会话仍在重连':'OPC DA 会话重连中'}}</h3><p>{{opcdaSessionReload.detail}}</p><div class="session-reconnect-devices">{{opcdaSessionReload.deviceKeys.join('、')}}</div><div v-if="opcdaSessionReload.phase==='reconnecting'" class="restart-progress"><span/></div><button v-if="opcdaSessionReload.phase==='failed'" @click="finishOPCDASessionReload">关闭</button></div></div><p v-if="message" class="notice good">{{message}}</p><p v-if="error" class="notice bad">{{error}}</p>
<div v-if="activeConnectionTest" class="connection-test-result" :class="activeConnectionTest.ok?'success':'failed'"><i/><div><strong>{{activeConnectionTest.deviceName}}（{{activeConnectionTest.deviceKey}}）{{activeConnectionTest.ok?'连接测试成功':'连接测试失败'}}</strong><span>测试地址：<code>{{activeConnectionTest.endpoint}}</code></span><small>{{activeConnectionTest.detail}}</small></div><button aria-label="关闭连接测试结果" @click="connectionTest=undefined">×</button></div>
<section v-if="resource" class="content-card"><div class="section-head"><div><h2>资源配置</h2><p class="mono">{{resource.resourceKey}}</p></div><div class="button-row"><button :disabled="mode==='run'" @click="addChannel()">新增通道</button><button class="danger" :disabled="mode==='run'" @click="removeResource">删除资源</button></div></div><div class="config-grid"><label>资源名称<input v-model.trim="resource.name" :disabled="mode==='run'"/></label><label>资源类型<select v-model="resource.type" :disabled="mode==='run'" @change="changeResourceType"><option value="serial">串口</option><option value="network">网口</option></select></label><label>状态<select v-model="resource.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label><template v-if="resource.type==='serial'&&resource.serial"><label>设备文件<input v-model.trim="resource.serial.port" :disabled="mode==='run'"/></label><label>波特率<input v-model.number="resource.serial.baudRate" type="number" :disabled="mode==='run'"/></label><label>数据位<input v-model.number="resource.serial.dataBits" type="number" :disabled="mode==='run'"/></label><label>停止位<input v-model.number="resource.serial.stopBits" type="number" step="0.5" :disabled="mode==='run'"/></label><label>校验<select v-model="resource.serial.parity" :disabled="mode==='run'"><option value="none">无</option><option value="even">偶校验</option><option value="odd">奇校验</option></select></label></template><template v-if="resource.type==='network'&&resource.network"><label>系统接口<input v-model.trim="resource.network.interface" :disabled="mode==='run'"/></label><label>地址模式<select v-model="resource.network.mode" :disabled="mode==='run'"><option value="static">静态</option><option value="dhcp">DHCP</option></select></label><label>IP 地址<input v-model.trim="resource.network.ipAddress" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label><label>前缀长度<input v-model.number="resource.network.prefixLength" type="number" min="1" max="32" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label><label>网关<input v-model.trim="resource.network.gateway" :disabled="mode==='run'||resource.network.mode==='dhcp'"/></label></template></div></section>

<section v-if="channel" class="content-card"><div class="section-head"><div><h2>通道配置</h2><p class="mono">{{channel.channelKey}}</p></div><div class="button-row"><button :disabled="mode==='run'" @click="addDevice">新增设备</button><button class="danger" :disabled="mode==='run'" @click="removeChannel">删除通道</button></div></div><div class="config-grid"><label>通道名称<input v-model.trim="channel.name" :disabled="mode==='run'"/></label><label>用途<select v-model="channel.role" :disabled="mode==='run'" @change="changeChannelRole"><option value="collect">采集</option><option value="forward">转发</option></select></label><label>协议<select v-model="channelProtocol" :disabled="mode==='run'"><option v-for="item in channelProtocols" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label><label>状态<select v-model="channel.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label><label v-if="!isForward">采集周期（毫秒）<input v-model.number="channel.collectIntervalMilliseconds" type="number" min="10" step="10" :placeholder="String((channel.collectIntervalSeconds||1)*1000)" :disabled="mode==='run'"/></label><template v-if="channel.protocol==='iec104'&&!isForward"><label>采集模式<select v-model="channel.iec104.acquisitionMode" :disabled="mode==='run'"><option value="auto">自动兼容</option><option value="periodic">周期总召</option></select></label><label>启动总召<select v-model="channel.iec104.generalInterrogationOnStart" :disabled="mode==='run'"><option :value="true">开启</option><option :value="false">关闭</option></select></label><label>周期总召（秒）<input v-model.number="channel.iec104.generalInterrogationIntervalSeconds" type="number" min="0" :disabled="mode==='run'||channel.iec104.acquisitionMode==='auto'"/></label></template></div></section>

<section v-if="device" class="content-card">
  <div class="section-head"><div><h2>{{isForward?'转发设备':'采集设备'}}</h2><p class="mono">{{device.deviceKey}}</p></div><div class="button-row"><button class="danger" :disabled="mode==='run'" @click="removeDevice">删除设备</button></div></div>
  <div class="config-grid"><label>设备名称<input v-model.trim="device.name" :disabled="mode==='run'"/></label><label>设备标识<input v-model.trim="device.deviceKey" disabled/></label>
    <template v-if="isForward"><label>协议<input :value="device.protocol" disabled/></label><label v-if="device.protocol==='modbus-tcp-slave'">Unit ID<input v-model.number="device.unitId" type="number" min="1" max="247" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec104-server'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec61850-mms-server'">IED 名称<input v-model.trim="device.iedName" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec61850-mms-server'">逻辑设备<input v-model.trim="device.logicalDevice" :disabled="mode==='run'"/></label><label>状态<select v-model="device.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label></template>
    <template v-else><label>协议<input :value="device.protocol" disabled/></label><label>连接地址<input v-model.trim="device.address" :disabled="mode==='run'" :placeholder="device.protocol==='opcda'?'Windows OPC DA 服务器 IP':'IP:端口 或串口设备'"/></label><label v-if="device.protocol.startsWith('modbus')">从站 ID<input v-model.number="device.slaveId" type="number" min="1" max="247" :disabled="mode==='run'"/></label><label v-if="device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="mode==='run'"/></label><label v-if="device.protocol==='siemens-s7'">PLC 型号<select v-model="device.plcModel" :disabled="mode==='run'"><option value="s7-200-smart">S7-200 SMART</option><option value="s7-300">S7-300</option><option value="s7-1200">S7-1200</option><option value="s7-1500">S7-1500</option></select></label><label v-if="device.protocol==='siemens-s7'">Rack / Slot<input :value="`${device.rack||0} / ${device.slot||0}`" disabled/></label><label v-if="device.protocol==='iec61850'">IED 名称<input v-model.trim="device.iedName" :disabled="mode==='run'"/></label>
      <template v-if="device.protocol==='opcda'"><label>ProgID<input v-model.trim="device.progId" :disabled="mode==='run'" placeholder="Matrikon.OPC.Simulation.1"/></label><label>CLSID<input v-model.trim="device.clsid" :disabled="mode==='run'" placeholder="F8582CF2-88FB-11D0-B850-00C0F0104305"/></label><label>域/工作组<input v-model.trim="device.domain" :disabled="mode==='run'" placeholder="DESKTOP-XXXX"/></label><label>工作站<input v-model.trim="device.workstation" :disabled="mode==='run'" placeholder="EG100"/></label><label>认证方式<select v-model="device.authentication" :disabled="mode==='run'"><option value="ntlm">NTLMv2</option></select></label><label>DCOM 安全模式<select v-model="device.securityMode" :disabled="mode==='run'" @change="changeOPCDASecurityMode"><option value="packet_integrity">Packet Integrity（默认，签名校验，防篡改）</option><option value="packet_privacy">Packet Privacy（加密并签名，防窃听和篡改）</option><option value="legacy_connect">Legacy Connect（仅连接认证，兼容旧系统）</option></select></label><div v-if="device.securityMode==='legacy_connect'" class="span-2 form-hint warning">Legacy Connect 仅用于旧版 DCOM 系统；Packet Integrity 不会自动降级到此模式。</div><label>Group 名称<input v-model.trim="device.groupName" :disabled="mode==='run'"/></label><label>单批点数<input v-model.number="device.batchSize" type="number" min="1" max="10000" :disabled="mode==='run'"/></label><label>读取源<select v-model="device.readSource" :disabled="mode==='run'"><option value="device">DEVICE</option><option value="cache">CACHE</option></select></label><label>连接超时（毫秒）<input v-model.number="device.connectTimeoutMilliseconds" type="number" min="100" :disabled="mode==='run'"/></label><label>请求超时（毫秒）<input v-model.number="device.requestTimeoutMilliseconds" type="number" min="100" :disabled="mode==='run'"/></label><label>更新周期（毫秒）<input v-model.number="device.requestedUpdateRateMilliseconds" type="number" min="10" :disabled="mode==='run'"/></label><label>本地回调 IP<input v-model.trim="device.localBindIp" :disabled="mode==='run'" placeholder="订阅阶段使用"/></label></template>
      <label v-if="device.protocol==='opcua'||device.protocol==='opcda'">用户名<input v-model.trim="device.username" :disabled="mode==='run'"/></label><label v-if="device.protocol==='opcua'||device.protocol==='opcda'">密码<input v-model="device.password" type="password" autocomplete="new-password" :disabled="mode==='run'"/></label><label>重连间隔（秒）<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="mode==='run'"/></label>
    </template>
  </div>
</section>

<section v-if="device" class="content-card"><div class="section-head"><div><h2>点位配置</h2><p>{{points.length}} 个点位，标识符在全网关内唯一。</p></div><div class="button-row"><input v-model.trim="search" class="toolbar-search" placeholder="搜索名称或标识"/><button :disabled="mode==='run'" @click="addPoint">新增点位</button><button :disabled="mode==='run'||!point" @click="duplicatePoint">复制</button><button class="danger" :disabled="mode==='run'||!point" @click="removePoint">删除</button></div></div><div class="point-workspace"><div class="point-list"><button v-for="item in visiblePoints" :key="item.metric" :class="{active:item.metric===pointMetric}" @click="pointMetric=item.metric"><span>{{item.name||item.metric}}</span><code>{{item.metric}}</code><small :class="liveByMetric.get(item.metric)?.error?'bad-text':''">{{liveByMetric.get(item.metric)?.error||String(liveByMetric.get(item.metric)?.value??'--')}}</small></button><p v-if="!visiblePoints.length" class="empty-mini">暂无点位</p></div><div v-if="point" class="point-editor"><div class="config-grid"><label>点位名称<input v-model.trim="point.name" :disabled="mode==='run'"/></label><label>标识符 metric<input v-model.trim="point.metric" :disabled="mode==='run'" @change="pointMetric=point.metric"/></label><template v-if="isForward"><label>源设备<select v-model="point.sourceDeviceKey" :disabled="mode==='run'" @change="point.sourceMetric=''"><option value="">请选择</option><option v-for="item in sourceDevices()" :key="item.deviceKey" :value="item.deviceKey">{{item.name}}（{{item.deviceKey}}）</option></select></label><label>源点位<select v-model="point.sourceMetric" :disabled="mode==='run'||!point.sourceDeviceKey" @change="syncForwardSource"><option value="">请选择</option><option v-for="item in sourcePoints()" :key="item.metric" :value="item.metric">{{item.name}}（{{item.metric}}）</option></select></label></template><template v-else-if="device.protocol.startsWith('modbus')"><label>功能码<select v-model.number="point.function" :disabled="mode==='run'"><option v-for="item in modbusFunctions" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label><label>寄存器地址<input v-model.number="point.register" type="number" min="0" max="65535" :disabled="mode==='run'"/></label><label>寄存器数量<input v-model.number="point.quantity" type="number" min="1" max="125" :disabled="mode==='run'"/></label></template><template v-else-if="device.protocol==='iec104'"><label>IOA 地址<input v-model.number="point.ioa" type="number" min="1" max="16777215" :disabled="mode==='run'"/></label><label>点类型<select v-model="point.pointType" :disabled="mode==='run'"><option value="M_SP_NA_1">单点遥信</option><option value="M_DP_NA_1">双点遥信</option><option value="M_ME_NC_1">短浮点遥测</option><option value="M_IT_NA_1">累计量</option></select></label></template><template v-else-if="device.protocol==='siemens-s7'"><label>区域<select v-model="point.area" :disabled="mode==='run'"><option value="DB">DB</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></label><label>DB 编号<input v-model.number="point.dbNumber" type="number" min="0" :disabled="mode==='run'"/></label><label>字节地址<input v-model.number="point.register" type="number" min="0" :disabled="mode==='run'"/></label></template><template v-else-if="device.protocol==='opcua'"><label class="span-2">Node ID<input v-model.trim="point.nodeId" :disabled="mode==='run'" placeholder="ns=2;s=Demo.Static.Scalar.Float"/></label></template><template v-else-if="device.protocol==='opcda'"><label class="span-2">Item ID<input v-model.trim="point.itemId" :disabled="mode==='run'" placeholder="Channel1.Device1.Temperature"/></label><label>Access Path<input v-model.trim="point.accessPath" :disabled="mode==='run'"/></label><label>请求数据类型<input v-model.trim="point.requestedDataType" :disabled="mode==='run'" placeholder="auto"/></label><label>状态<select v-model="point.enabled" :disabled="mode==='run'"><option :value="true">启用</option><option :value="false">停用</option></select></label></template><template v-else-if="device.protocol==='iec61850'"><label class="span-2">对象引用<input v-model.trim="point.objectRef" :disabled="mode==='run'"/></label><label>功能约束 FC<input v-model.trim="point.fc" :disabled="mode==='run'" placeholder="MX / ST"/></label></template><label>数据类型<select v-model="point.dataType" :disabled="mode==='run'"><option v-for="item in dataTypes" :key="item">{{item}}</option></select></label><label>字节序<select v-model="point.byteOrder" :disabled="mode==='run'"><option value="big">大端</option><option value="little">小端</option></select></label><label>字序<select v-model="point.wordOrder" :disabled="mode==='run'"><option value="big">高字在前</option><option value="little">低字在前</option></select></label><label>倍率<input v-model.number="point.scale" type="number" step="any" :disabled="mode==='run'"/></label><label>偏移<input v-model.number="point.offset" type="number" step="any" :disabled="mode==='run'"/></label><label>单位<input v-model.trim="point.unit" :disabled="mode==='run'"/></label><label>小数位<input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="mode==='run'"/></label></div><div class="live-detail"><span>实时值</span><strong :class="liveByMetric.get(point.metric)?.error?'bad-text':''">{{liveByMetric.get(point.metric)?.error||String(liveByMetric.get(point.metric)?.value??'--')}}</strong><small>{{liveByMetric.get(point.metric)?.quality?`${liveByMetric.get(point.metric)!.quality} · `:''}}{{liveByMetric.get(point.metric)?.sourceTimestamp?new Date(liveByMetric.get(point.metric)!.sourceTimestamp!).toLocaleString():liveByMetric.get(point.metric)?.updatedAt?new Date(liveByMetric.get(point.metric)!.updatedAt!).toLocaleString():'尚无采集结果'}}</small></div></div></div></section>
<section v-if="device&&point&&mode==='run'" class="content-card"><div class="section-head"><div><h2>点位控制</h2><p>仅对协议和点位类型支持写入的点位生效，操作会立即下发到设备。</p></div></div><div class="form-row"><label>写入值<input v-model="writeValue" :placeholder="point.dataType==='bool'?'true / false':'请输入目标值'" @keydown.enter="writePoint" /></label><button class="primary" :disabled="writeValue===''" @click="writePoint">确认写入</button></div></section>
<section v-if="configView==='json'" class="content-card json-config-card"><textarea v-model="jsonText" spellcheck="false"/><div class="button-row"><button @click="jsonText=JSON.stringify(cfg,null,2)">格式化</button><button class="primary" :disabled="saving||mode==='run'" @click="saveJSON">保存 JSON 配置</button></div></section>
<ResourceWorkspace v-if="resource&&detailLevel==='resource'" :resource="resource" :channels="resourceChannels" :collect-devices="collectDevices" :forward-devices="forwardDevices" :readonly="mode==='run'" :saving="saving" @reload="projectImported" @save="requestSave" @add-channel="addChannel" @add-device="addDeviceForChannel" @select-channel="value=>navigateWithin(()=>selectChannel(value))" @select-device="(channel,device)=>navigateWithin(()=>selectDeviceFromResource(channel,device))" @serial-protocol-change="changeSerialResourceProtocol" />
<ChannelWorkspace v-if="resource&&channel&&detailLevel==='channel'" :resource="resource" :channel="channel" :devices="channelDevices" :forward-slave="forwardSlave" :readonly="mode==='run'" :saving="saving" @reload="projectImported" @save="requestSave" @add-device="addDevice" @remove-channel="removeChannel" @select-device="value=>navigateWithin(()=>selectDevice(value))" @protocol-change="editChannelProtocol" />
<LargeDevicePointWorkspace ref="largeWorkspace" v-if="device&&store.largeMode" :device="device" :channel-name="channel?.name" :readonly="mode==='run'" :is-forward="isForward" :saving="saving" :writable-metrics="writableMetrics" @back="navigateWithin(()=>{detailLevel='channel';deviceKey=''})" @reload="projectImported" @save="requestSave" @collect="collectNow" @monitor="openMonitor" @test="testDeviceConnection" @remove-device="removeDevice" @write="writeWorkspacePoint" @navigate="clearNotices" />
<DevicePointWorkspace v-else-if="device" :device="device" :channel-name="channel?.name" :interface-name="resource?.network?.interface||resource?.serial?.port||channel?.interfaceName||device?.interfaceName" :points="points" :all-points="allConfiguredPoints" :live-by-metric="liveByMetric" :readonly="mode==='run'" :is-forward="isForward" :saving="saving" :writable-metrics="writableMetrics" :source-devices="sourceDevices()" :source-channels="channels" :source-resources="resources" @back="navigateWithin(()=>{detailLevel='channel';deviceKey=''})" @reload="projectImported" @save="requestSave" @collect="collectNow" @monitor="openMonitor" @test="testDeviceConnection" @read-address="readDLT645Address" @remove-device="removeDevice" @write="writeWorkspacePoint" @navigate="clearNotices" />
<div v-if="channelModal" class="restart-modal"><div class="dialog-panel"><h3>新增{{newChannelRole==='forward'?'转发':'采集'}}通道</h3><p>将在当前资源下新增{{newChannelRole==='forward'?'转发':'采集'}}通道，请选择协议。</p><div class="channel-dialog-grid"><label>协议<select aria-label="协议" v-model="newChannelProtocol"><option v-for="item in modalProtocols" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label></div><div class="dialog-actions"><button @click="channelModal=false">取消</button><button class="primary" @click="confirmAddChannel">确定新增</button></div></div></div>
<div v-if="unsavedOpen" class="unsaved-config-modal"><div class="dialog-panel"><h3>配置尚未保存</h3><p>当前配置有修改，切换到运行模式前请选择处理方式。</p><div class="dialog-actions"><button @click="unsavedOpen=false">继续编辑</button><button class="danger" @click="discardAndRun">放弃修改并运行</button><button class="primary" :disabled="saving" @click="requestSave">保存修改并运行</button></div></div></div>
<div v-if="networkApplyOpen" class="restart-modal"><div class="dialog-panel network-apply-dialog"><h3>应用网口配置</h3><p>保存后将立即修改网关系统网络参数，页面、采集连接和云平台连接可能短暂中断。</p><div class="network-change-list"><div v-for="item in pendingNetworkPorts" :key="item.resourceKey"><strong>{{item.name}}（{{item.interface}}）</strong><span v-if="item.mode==='dhcp'">自动获取地址（DHCP）</span><span v-else>{{item.system?.ipv4Address||item.previous?.ipAddress||'未配置'}} → {{item.ipAddress}} / {{item.prefixLength}}</span><small>网关 {{item.gateway||'-'}} · DNS {{item.dns.join(', ')||'未配置'}}</small></div></div><p class="meta">若网卡未能启用或未获得目标地址，系统将在约 30 秒后恢复原网络配置。</p><div class="dialog-actions"><button :disabled="saving" @click="networkApplyOpen=false;pendingNetworkPorts=[]">取消</button><button class="primary" :disabled="saving" @click="commitSave(true)">{{saving?'正在保存':'保存并应用'}}</button></div></div></div>
</div></template>
