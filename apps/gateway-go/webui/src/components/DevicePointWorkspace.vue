<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRaw, watch } from 'vue'
import { confirmAction } from '../confirm'
import DeviceConfigForm from './DeviceConfigForm.vue'
import ProtocolPointTools from './ProtocolPointTools.vue'

const props=defineProps<{device:any;channelName?:string;interfaceName?:string;points:any[];allPoints?:any[];liveByMetric:Map<string,any>;readonly:boolean;isForward:boolean;sourceDevices:any[];sourceChannels:any[];sourceResources:any[]} >()
const emit=defineEmits<{back:[];monitor:[];test:[];removeDevice:[];write:[point:any,value:string,done:(success:boolean,value?:unknown)=>void]}>()
const isIEC104=computed(()=>['iec104','iec104-server'].includes(String(props.device?.protocol||'').toLowerCase()))
const isIEC61850=computed(()=>['iec61850','iec61850-mms-server'].includes(String(props.device?.protocol||'').toLowerCase()))
const isIEC61850Forward=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-mms-server')
const isGOOSECollect=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-goose')
const isGOOSEForward=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-goose-publisher')
const defaultGroup=()=>props.device?.protocol?.startsWith('modbus')?'1':isIEC104.value?'yc':'all'
const tab=ref<'device'|'points'>('device'), search=ref(''), group=ref(defaultGroup())
const workspaceRef=ref<HTMLElement>()
const VIRTUAL_ROW_HEIGHT=34, VIRTUAL_OVERSCAN=12, VIRTUAL_WINDOW_STEP=8, VIRTUAL_THRESHOLD=120, VIRTUAL_INITIAL_WINDOW=60
const virtualStart=ref(0), virtualEnd=ref(VIRTUAL_INITIAL_WINDOW)
let virtualScrollHost:HTMLElement|undefined, virtualFrame=0, virtualRowsTop=0, virtualViewportHeight=0, virtualGeometryDirty=true
// Compatibility bindings for the old inline pager markup. Infinity keeps that
// block unmounted while the table renders as one continuous list.
const page=ref(1), pageSize=ref(Number.POSITIVE_INFINITY)
const selected=ref<string[]>([]), batchOpen=ref(false), batchField=ref('name'), batchRange=ref('selected'), batchMode=ref('set'), batchValue=ref(''), batchStep=ref(1), batchPrefix=ref(''), batchSuffix=ref('')
const writePoint=ref<any>(), writeValue=ref('')
const lastWrites=ref<Record<string,{value:string;at:string}>>({})
const forwardOpen=ref(false), forwardSearch=ref(''), forwardSelected=ref<string[]>([]), forwardExpanded=ref<string[]>([])
const forwardPointRenderLimits=ref<Record<string,number>>({})
const FORWARD_POINT_BATCH=160
const knownPointMetrics=new WeakMap<object,string>()
let metricIndex=new Set<string>(), nextGeneratedMetricIndex=1
const metricSuffixCursor=new Map<string,number>()
const metricSequenceCursor=new Map<string,number>()
const functions=[[1,'线圈 01'],[2,'离散输入 02'],[3,'保持寄存器 03'],[4,'输入寄存器 04']]
const allDataTypes=['auto','bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','datetime','quality','timestamp']
const dataTypes=computed(()=>{
  const protocol=String(props.device?.protocol||'').toLowerCase()
  if(protocol==='iec61850-mms-server')return['bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string']
  if(protocol==='iec61850-goose'||protocol==='iec61850-goose-publisher')return['bool','int8','uint8','int16','uint16','int32','uint32','float32','float64','string']
  if(props.isForward)return allDataTypes
  if(protocol==='modbus-tcp'||protocol==='modbus-rtu')return['1','2'].includes(group.value)?['bool']:['bool','uint16','int16','uint32','int32','float32']
  if(protocol==='iec104'||protocol==='iec104-server'){
    if(group.value==='yx'||group.value==='yk')return['bool']
    if(group.value==='ym')return['int32']
    return['int16','float32']
  }
  if(protocol==='siemens-s7')return['bool','uint16','int16','uint32','int32','float32']
  if(protocol==='opcua')return['auto','bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','datetime']
  if(protocol==='iec61850')return['auto','bool','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','quality','timestamp']
  return allDataTypes
})
const batchFields=computed(()=>{
  const fields=[['name','名称'],['metric','标识符'],['dataType','数据类型'],['scale','倍率'],['offset','偏移'],['unit','单位'],['decimals','小数位']]
  if(props.device?.protocol?.startsWith('modbus'))fields.splice(2,0,['register','寄存器'],['quantity','数量'],['byteOrder','字节序'])
  else if(props.device?.protocol==='siemens-s7')fields.splice(2,0,['byteOrder','字节序'])
  if(isIEC104.value)fields.splice(1,0,['ioa','IOA'],['pointType','点类型'])
  if(isIEC61850Forward.value)fields.splice(2,0,['objectRef','对象引用'],['fc','FC'])
  if(isGOOSECollect.value)fields.splice(2,0,['gooseIndex','数据集索引'])
  return fields
})
const batchValueOptions=computed<[string,string][]>(()=>{
  if(batchField.value==='dataType')return dataTypes.value.map(type=>[type,type])
  if(batchField.value==='byteOrder')return[['big','ABCD'],['little','CDAB']]
  if(batchField.value==='fc')return[['ST','ST'],['MX','MX']]
  return[]
})
const batchNumericFields=new Set(['register','quantity','scale','offset','decimals','ioa','gooseIndex'])
const batchFieldIsNumeric=computed(()=>batchNumericFields.has(batchField.value))
const batchSequenceEnabled=computed(()=>batchFieldIsNumeric.value||['name','metric'].includes(batchField.value))
const batchAffixesEnabled=computed(()=>!batchFieldIsNumeric.value&&!['dataType','pointType','byteOrder'].includes(batchField.value))
const iec104Groups=[{key:'yc',label:'遥测 YC'},{key:'yx',label:'遥信 YX'},{key:'yk',label:'遥控 YK'},{key:'yt',label:'遥调 YT'},{key:'ym',label:'遥脉 YM'}]
const iec104Types:Record<string,[string,string][]>= {
  yc:[['M_ME_NA_1','归一化遥测'],['M_ME_NB_1','标度化遥测'],['M_ME_NC_1','短浮点遥测']],
  yx:[['M_SP_NA_1','单点遥信'],['M_DP_NA_1','双点遥信']],
  yk:[['C_SC_NA_1','单点遥控'],['C_DC_NA_1','双点遥控']],
  yt:[['C_SE_NA_1','归一化遥调'],['C_SE_NB_1','标度化遥调'],['C_SE_NC_1','短浮点遥调']],
  ym:[['M_IT_NA_1','累计量/电度']]
}

function iec104Group(point:any){const type=String(point?.pointType||'').toUpperCase(),data=String(point?.dataType||'').toLowerCase();if(type.startsWith('M_SP')||type.startsWith('M_DP')||['single','double'].includes(data))return'yx';if(type.startsWith('C_SC')||type.startsWith('C_DC')||data.includes('command'))return'yk';if(type.startsWith('C_SE')||data.includes('setpoint'))return'yt';if(type.startsWith('M_IT')||data.includes('counter'))return'ym';return'yc'}
function iec104Options(point:any){return iec104Types[iec104Group(point)]||iec104Types.yc}
function canWrite(point:any){return !props.isForward&&['modbus-tcp','modbus-rtu'].includes(String(point.protocol||props.device.protocol))&&[1,3].includes(Number(point.function))}

const visiblePoints=computed(()=>props.points.filter(point=>{
  const q=search.value.trim().toLowerCase()
  const grouped=props.device.protocol?.startsWith('modbus')?String(point.function||3)===group.value:isIEC104.value?iec104Group(point)===group.value:true
  return grouped&&(!q||`${point.name||''} ${point.metric||''}`.toLowerCase().includes(q))
}))
const visibleMetricSet=computed(()=>new Set(visiblePoints.value.map(point=>String(point.metric||''))))
const selectedMetricSet=computed(()=>new Set(selected.value))
const selectedVisibleCount=computed(()=>selected.value.reduce((count,metric)=>count+(visibleMetricSet.value.has(metric)?1:0),0))
const allVisibleSelected=computed(()=>visiblePoints.value.length>0&&selectedVisibleCount.value===visiblePoints.value.length)
const groupCounts=computed(()=>{
  const counts:Record<string,number>={1:0,2:0,3:0,4:0,yc:0,yx:0,yk:0,yt:0,ym:0}
  for(const point of props.points){
    if(props.device.protocol?.startsWith('modbus')){
      const key=String(Number(point.function||3))
      counts[key]=(counts[key]||0)+1
    }else if(isIEC104.value){
      const key=iec104Group(point)
      counts[key]=(counts[key]||0)+1
    }
  }
  return counts
})
// Keep one continuous table and let the page own the only vertical scrollbar.
// WebSocket updates are already coalesced in realtime.ts, so the full list does
// not need pagination to avoid a render for every incoming point update.
// Never fall back to the full collection while the virtual window is being
// recalculated. A zero-sized group followed by a 20k-point group previously
// rendered all rows once before the post-render watcher could update the range.
const pagedPoints=computed(()=>{
  const end=Math.max(virtualStart.value+1,virtualEnd.value||VIRTUAL_INITIAL_WINDOW)
  return visiblePoints.value.slice(virtualStart.value,Math.min(visiblePoints.value.length,end))
})
const pointOffset=computed(()=>virtualStart.value)
const pageCount=computed(()=>1)
const virtualTopHeight=computed(()=>virtualStart.value*VIRTUAL_ROW_HEIGHT)
const isVirtualized=computed(()=>visiblePoints.value.length>VIRTUAL_THRESHOLD)
const virtualRootStyle=computed(()=>({
  '--virtual-total-height':`${visiblePoints.value.length*VIRTUAL_ROW_HEIGHT+VIRTUAL_ROW_HEIGHT}px`,
  '--virtual-top-height':`${virtualTopHeight.value}px`,
  '--virtual-bottom-height':`${Math.max(0,(visiblePoints.value.length-virtualEnd.value)*VIRTUAL_ROW_HEIGHT)}px`
}))
const allForwardCandidateMap=computed(()=>{
  const result=new Map<string,{device:any;point:any}>()
  for(const device of props.sourceDevices)for(const point of device.points||[])result.set(`${device.deviceKey}::${point.metric}`,{device,point})
  return result
})
const forwardSelectedSet=computed(()=>new Set(forwardSelected.value))
const forwardTree=computed(()=>{
  const q=forwardSearch.value.trim().toLowerCase()
  const channels=(props.sourceChannels||[]).filter(channel=>channel.role!=='forward')
  const buildChannel=(channel:any,ancestorMatch=false)=>{
    const channelMatch=ancestorMatch||Boolean(q&&`${channel.name||''} ${channel.channelKey||''} ${channel.protocol||''}`.toLowerCase().includes(q))
    const devices=props.sourceDevices.filter(device=>device.channelKey===channel.channelKey).map(device=>{
      const deviceMatch=channelMatch||Boolean(q&&`${device.name||''} ${device.deviceKey||''} ${device.protocol||''}`.toLowerCase().includes(q))
      const sourcePoints=Array.isArray(device.points)?device.points:[]
      const points=!q||deviceMatch?sourcePoints:sourcePoints.filter((point:any)=>`${point.name||''} ${point.metric||''}`.toLowerCase().includes(q))
      return{device,points,key:`device:${device.deviceKey}`,pointCount:points.length,keys:points.map((point:any)=>`${device.deviceKey}::${point.metric}`)}
    }).filter(item=>item.points.length||!q)
    return{channel,devices,key:`channel:${channel.channelKey}`,pointCount:devices.reduce((sum,item)=>sum+item.pointCount,0),keys:devices.flatMap(item=>item.keys)}
  }
  const resources=(props.sourceResources||[]).map(resource=>{
    const resourceMatch=Boolean(q&&`${resource.name||''} ${resource.resourceKey||''} ${resource.network?.interface||''} ${resource.serial?.port||''}`.toLowerCase().includes(q))
    const childChannels=channels.filter(channel=>channel.resourceKey===resource.resourceKey).map(channel=>buildChannel(channel,resourceMatch)).filter(item=>item.devices.length||!q)
    return{resource,channels:childChannels,key:`resource:${resource.resourceKey}`,pointCount:childChannels.reduce((sum,item)=>sum+item.pointCount,0),keys:childChannels.flatMap(item=>item.keys)}
  }).filter(item=>item.channels.some(channel=>channel.devices.length)||!q)
  const assignedResources=new Set((props.sourceResources||[]).map(resource=>resource.resourceKey))
  const unassignedChannels=channels.filter(channel=>!assignedResources.has(channel.resourceKey)).map(channel=>buildChannel(channel)).filter(item=>item.devices.length||!q)
  if(unassignedChannels.length)resources.push({resource:{resourceKey:'unassigned',name:'未归属资源',type:'network'},channels:unassignedChannels,key:'resource:unassigned',pointCount:unassignedChannels.reduce((sum,item)=>sum+item.pointCount,0),keys:unassignedChannels.flatMap(item=>item.keys)})
  const assignedChannels=new Set(channels.map(channel=>channel.channelKey))
  const unassigned=props.sourceDevices.filter(device=>!assignedChannels.has(device.channelKey)).map(device=>{
    const deviceMatch=`${device.name||''} ${device.deviceKey||''} ${device.protocol||''}`.toLowerCase().includes(q)
    const sourcePoints=Array.isArray(device.points)?device.points:[]
    const points=!q||deviceMatch?sourcePoints:sourcePoints.filter((point:any)=>`${point.name||''} ${point.metric||''}`.toLowerCase().includes(q))
    return{device,points,key:`device:${device.deviceKey}`,pointCount:points.length,keys:points.map((point:any)=>`${device.deviceKey}::${point.metric}`)}
  }).filter(item=>item.points.length||!q)
  if(unassigned.length){
    const channelNode={channel:{channelKey:'unassigned',name:'未归属通道',protocol:''},devices:unassigned,key:'channel:unassigned',pointCount:unassigned.reduce((sum,item)=>sum+item.pointCount,0),keys:unassigned.flatMap(item=>item.keys)}
    const resourceNode=resources.find(item=>item.key==='resource:unassigned')
    if(resourceNode){resourceNode.channels.push(channelNode);resourceNode.pointCount+=channelNode.pointCount;resourceNode.keys.push(...channelNode.keys)}
    else resources.push({resource:{resourceKey:'unassigned',name:'未归属资源',type:'network'},channels:[channelNode],key:'resource:unassigned',pointCount:channelNode.pointCount,keys:channelNode.keys})
  }
  return resources
})
const addressHost=computed({get:()=>{const value=String(props.device.address||'');if(value.includes('://'))return value;const index=value.lastIndexOf(':');return index>0?value.slice(0,index):value},set:(value:string)=>{props.device.address=addressPort.value?`${value}:${addressPort.value}`:value}})
const addressPort=computed({get:()=>{const value=String(props.device.address||'');if(value.includes('://'))return '';const index=value.lastIndexOf(':');return index>0?value.slice(index+1):''},set:(value:string|number)=>{props.device.address=value?`${addressHost.value}:${value}`:addressHost.value}})

function measureVirtualGeometry(){
  const root=workspaceRef.value,host=virtualScrollHost
  if(!root||!host)return false
  const tables=Array.from(root.querySelectorAll<HTMLElement>('.point-sheet-table'))
  const table=tables.find(item=>item.offsetParent!==null)
  if(!table)return false
  const hostRect=host.getBoundingClientRect(),tableRect=table.getBoundingClientRect()
  virtualRowsTop=tableRect.top-hostRect.top+host.scrollTop+VIRTUAL_ROW_HEIGHT
  virtualViewportHeight=host.clientHeight
  virtualGeometryDirty=false
  return true
}
function updateVirtualWindow(){
  const total=visiblePoints.value.length
  if(total<=VIRTUAL_THRESHOLD){virtualStart.value=0;virtualEnd.value=total;return}
  const host=virtualScrollHost
  if(!host||virtualGeometryDirty&&!measureVirtualGeometry()){virtualStart.value=0;virtualEnd.value=Math.min(total,VIRTUAL_INITIAL_WINDOW);return}
  const firstVisible=Math.max(0,Math.floor((host.scrollTop-virtualRowsTop)/VIRTUAL_ROW_HEIGHT))
  const rawStart=Math.max(0,firstVisible-VIRTUAL_OVERSCAN)
  const start=Math.floor(rawStart/VIRTUAL_WINDOW_STEP)*VIRTUAL_WINDOW_STEP
  const windowSize=Math.ceil(virtualViewportHeight/VIRTUAL_ROW_HEIGHT)+VIRTUAL_OVERSCAN*2+VIRTUAL_WINDOW_STEP
  const end=Math.min(total,start+windowSize)
  if(virtualStart.value!==start)virtualStart.value=start
  if(virtualEnd.value!==end)virtualEnd.value=end
}
function scheduleVirtualUpdate(){
  if(virtualFrame)return
  virtualFrame=requestAnimationFrame(()=>{virtualFrame=0;updateVirtualWindow()})
}
function invalidateVirtualGeometry(){
  virtualGeometryDirty=true
  scheduleVirtualUpdate()
}
function handleVirtualResize(){
  invalidateVirtualGeometry()
}
onMounted(()=>nextTick(()=>{
  virtualScrollHost=workspaceRef.value?.closest<HTMLElement>('.page')||undefined
  virtualScrollHost?.addEventListener('scroll',scheduleVirtualUpdate,{passive:true})
  window.addEventListener('resize',handleVirtualResize,{passive:true})
  updateVirtualWindow()
}))
onBeforeUnmount(()=>{
  virtualScrollHost?.removeEventListener('scroll',scheduleVirtualUpdate)
  window.removeEventListener('resize',handleVirtualResize)
  if(virtualFrame)cancelAnimationFrame(virtualFrame)
})
watch(()=>props.device?.deviceKey,()=>{
  tab.value=props.points.length?'points':'device'
  group.value=defaultGroup()
  selected.value=[]
  virtualStart.value=0
  virtualEnd.value=VIRTUAL_INITIAL_WINDOW
  virtualGeometryDirty=true
},{immediate:true})
watch(group,value=>{
  if(value==='all'&&props.device?.protocol?.startsWith('modbus'))group.value='1'
  virtualStart.value=0
  virtualEnd.value=VIRTUAL_INITIAL_WINDOW
  virtualGeometryDirty=true
})
watch(batchField,()=>{
  if(!batchSequenceEnabled.value&&batchMode.value!=='set')batchMode.value='set'
  const options=batchValueOptions.value
  if(options.length&&!options.some(item=>item[0]===batchValue.value))batchValue.value=options[0][0]
})
watch(()=>[props.device?.deviceKey,tab.value,group.value,search.value,visiblePoints.value.length],()=>nextTick(invalidateVirtualGeometry),{flush:'post'})
function rebuildPointIndexes(points:any[]){
  metricIndex=new Set()
  metricSuffixCursor.clear()
  metricSequenceCursor.clear()
  nextGeneratedMetricIndex=1
  const generatedPattern=new RegExp(`^${protocolMetricPrefix()}-P(\\d+)$`,'i')
  for(const point of props.allPoints||points){
    const metric=String(point.metric||'').trim()
    if(!metric)continue
    metricIndex.add(metric.toLowerCase())
    const match=metric.match(generatedPattern)
    if(match)nextGeneratedMetricIndex=Math.max(nextGeneratedMetricIndex,Number(match[1])+1)
  }
  for(const point of points){
    if(props.isForward&&point.sourceDeviceKey&&point.sourceMetric){
      const sourceDevice=props.sourceDevices.find(item=>item.deviceKey===point.sourceDeviceKey)
      const sourcePoint=(sourceDevice?.points||[]).find((item:any)=>item.metric===point.sourceMetric)
      if(sourcePoint){point.name=sourcePoint.name||point.sourceMetric;point.metric=point.sourceMetric}
    }
    if(point.decimals===undefined||point.decimals===null||point.decimals==='')point.decimals=0
    knownPointMetrics.set(point,String(point.metric||''))
  }
}
watch(()=>props.points,rebuildPointIndexes,{immediate:true})
let normalizingMetric=false
// Only visible rows can be edited directly. Watching all 20k metrics made every
// append/duplicate traverse the entire device twice; track the virtual window
// instead and let the add/copy paths reserve their metrics explicitly.
watch(()=>pagedPoints.value.map(point=>String(point.metric||'')),()=>{
  if(normalizingMetric)return
  normalizingMetric=true
  try{
    for(const point of pagedPoints.value){
      const current=String(point.metric||'').trim()
      const previous=knownPointMetrics.get(point)
      if(previous===undefined){knownPointMetrics.set(point,current);if(current)metricIndex.add(current.toLowerCase());continue}
      if(previous===current)continue
      if(previous)metricIndex.delete(previous.toLowerCase())
      point.metric=reserveMetric(current)
      knownPointMetrics.set(point,point.metric)
      syncPointName(point)
    }
  }finally{normalizingMetric=false}
},{flush:'sync'})

function protocolMetricPrefix(){return{'modbus-tcp':'mbtcp','modbus-tcp-slave':'mbtcp','modbus-rtu':'mbrtu','iec104':'iec104','iec104-server':'iec104','siemens-s7':'s7','opcua':'opcua','iec61850':'iec61850','iec61850-mms-server':'iec61850','iec61850-goose':'goose','iec61850-goose-publisher':'goose'}[String(props.device.protocol||'').toLowerCase()]||'point'}
function uniqueMetric(preferred=''){
  const requested=String(preferred||'').trim()
  if(requested&&!metricIndex.has(requested.toLowerCase()))return requested
  if(requested){
    const cursorKey=requested.toLowerCase()
    let index=metricSuffixCursor.get(cursorKey)||1,metric=''
    do metric=`${requested}-${String(index++).padStart(3,'0')}`;while(metricIndex.has(metric.toLowerCase()))
    metricSuffixCursor.set(cursorKey,index)
    return metric
  }
  const prefix=protocolMetricPrefix();let metric=''
  do metric=`${prefix}-P${String(nextGeneratedMetricIndex++).padStart(2,'0')}`;while(metricIndex.has(metric.toLowerCase()))
  return metric
}
function reserveMetric(preferred=''){const metric=uniqueMetric(preferred);metricIndex.add(metric.toLowerCase());return metric}
function nextMetric(preferred=''){return reserveMetric(preferred)}
function nextCopiedMetric(source=''){
  const requested=String(source||'').trim()
  const match=requested.match(/^(.*?)(\d+)$/)
  if(!match)return reserveMetric(requested)
  const prefix=match[1],width=match[2].length,cursorKey=`${prefix.toLowerCase()}#${width}`
  let index=Math.max(Number(match[2])+1,metricSequenceCursor.get(cursorKey)||0),metric=''
  do metric=`${prefix}${String(index++).padStart(width,'0')}`;while(metricIndex.has(metric.toLowerCase()))
  metricSequenceCursor.set(cursorKey,index)
  metricIndex.add(metric.toLowerCase())
  return metric
}
function pointNamePrefix(){return [props.interfaceName||props.device.interfaceName,props.channelName||props.device.channelKey,props.device.name||props.device.deviceKey].map(value=>String(value||'').trim()).filter(Boolean).join('_')||props.device.deviceKey||'device'}
function pointName(metric:string){return `${pointNamePrefix()}@${String(metric||'').trim()}`}
function syncPointName(point:any){point.name=pointName(point.metric)}
function appendPoints(points:any[]){if(points.length)props.device.points=[...props.points,...points]}
function addPoint(){if(props.isForward){forwardSelected.value=[];forwardSearch.value='';forwardPointRenderLimits.value={};forwardExpanded.value=forwardTree.value.map(resource=>resource.key);forwardOpen.value=true;return}const metric=nextMetric(),category=isIEC104.value?group.value:'',point={deviceKey:props.device.deviceKey,channelKey:props.device.channelKey,name:pointName(metric),metric,protocol:props.device.protocol,address:props.device.address,slaveId:props.device.slaveId||1,commonAddress:props.device.commonAddress||1,function:props.device.protocol?.startsWith('modbus')?Number(group.value):3,register:0,ioa:props.points.length+1,gooseIndex:isGOOSECollect.value?props.points.length:undefined,goCbRef:props.device.goCbRef,dataSetRef:props.device.dataSetRef,appId:props.device.appId,destinationMac:props.device.destinationMac,vlanId:props.device.vlanId,vlanPriority:props.device.vlanPriority,pointType:category?(iec104Types[category]||iec104Types.yc)[0][0]:undefined,quantity:1,dataType:isGOOSECollect.value?'bool':category==='yx'?'bool':category==='yk'?'bool':category==='ym'?'uint32':'float32',byteOrder:'big',wordOrder:'big',scale:1,offset:0,decimals:0};knownPointMetrics.set(point,metric);appendPoints([point]);selected.value=[metric]}
function duplicate(point:any){const copy={...toRaw(point)};copy.metric=nextCopiedMetric(point.metric);copy.name=pointName(copy.metric);knownPointMetrics.set(copy,copy.metric);appendPoints([copy])}
function duplicateSelected(){const wanted=new Set(selected.value),copies:any[]=[];for(const point of props.points){if(!wanted.has(String(point.metric)))continue;const copy={...toRaw(point)},copyMetric=nextCopiedMetric(point.metric);copy.metric=copyMetric;copy.name=pointName(copyMetric);knownPointMetrics.set(copy,copyMetric);copies.push(copy)}appendPoints(copies);selected.value=copies.map(point=>point.metric)}
async function remove(point:any){if(!await confirmAction({title:'删除点位',message:`确认删除点位“${point.name||point.metric}”？`,detail:'删除后需保存配置才会正式生效。',confirmText:'确认删除',danger:true}))return;metricIndex.delete(String(point.metric||'').toLowerCase());props.device.points.splice(props.device.points.indexOf(point),1)}
async function removeSelected(){if(!selected.value.length||!await confirmAction({title:'删除所选点位',message:`确认删除选中的 ${selected.value.length} 个点位？`,detail:'该操作会一次移除全部所选点位，请确认选择范围。',confirmText:'确认删除',danger:true}))return;const removing=new Set(selected.value);props.device.points=props.points.filter(point=>!removing.has(point.metric));for(const metric of removing)metricIndex.delete(metric.toLowerCase());selected.value=[]}
function toggle(metric:string){const index=selected.value.indexOf(metric);if(index>=0)selected.value.splice(index,1);else selected.value.push(metric)}
function toggleAll(){const metrics=visiblePoints.value.map(item=>item.metric),visible=new Set(metrics);selected.value=allVisibleSelected.value?selected.value.filter(item=>!visible.has(item)):[...new Set([...selected.value,...metrics])]}
function live(point:any){const state=props.liveByMetric.get(point.metric);if(state?.error)return{...state,value:undefined,error:'-'};return state}
function lastWrite(point:any){return lastWrites.value[point.metric]}
function openBatch(field='name'){batchField.value=field;if(!selected.value.length)batchRange.value='all';batchOpen.value=true}
function applyBatch(){
  const wanted=new Set(selected.value)
  const applies=(point:any)=>batchRange.value==='all'||wanted.has(point.metric)
  if(batchField.value==='metric'){
    for(const point of props.points)if(applies(point))metricIndex.delete(String(point.metric||'').toLowerCase())
  }
  let targetIndex=0
  const next=props.points.map(point=>{
    if(!applies(point))return point
    const copy={...toRaw(point)}
    const index=targetIndex++
    let value:any=batchValue.value
    if(batchFieldIsNumeric.value){
      const start=Number(batchValue.value||0)
      const step=Number(batchStep.value||1)
      value=start+(batchMode.value==='increment'?index*step:batchMode.value==='decrement'?-index*step:0)
    }else{
      if(batchMode.value==='increment'||batchMode.value==='decrement'){
        const start=Number(batchValue.value||0)
        const step=Number(batchStep.value||1)
        value=String(start+(batchMode.value==='increment'?index*step:-index*step))
      }else value=String(batchValue.value??'')
      if(batchAffixesEnabled.value&&batchPrefix.value)value=`${batchPrefix.value}${value}`
      if(batchAffixesEnabled.value&&batchSuffix.value)value=`${value}${batchSuffix.value}`
    }
    if(batchField.value==='metric'){
      value=reserveMetric(String(value||''))
      copy.metric=value
      copy.name=pointName(value)
    }else copy[batchField.value]=value
    knownPointMetrics.set(copy,String(copy.metric||''))
    return copy
  })
  props.device.points=next
  batchOpen.value=false
}
function toggleForward(key:string){const i=forwardSelected.value.indexOf(key);if(i>=0)forwardSelected.value.splice(i,1);else forwardSelected.value.push(key)}
function forwardGroupSelected(keys:string[]){return keys.length>0&&keys.every(key=>forwardSelectedSet.value.has(key))}
function forwardGroupPartial(keys:string[]){let selectedCount=0;for(const key of keys)if(forwardSelectedSet.value.has(key))selectedCount++;return selectedCount>0&&selectedCount<keys.length}
function toggleForwardGroup(keys:string[]){const keySet=new Set(keys);if(forwardGroupSelected(keys))forwardSelected.value=forwardSelected.value.filter(key=>!keySet.has(key));else forwardSelected.value=[...new Set([...forwardSelected.value,...keys])]}
function toggleForwardExpanded(key:string){const index=forwardExpanded.value.indexOf(key);if(index>=0)forwardExpanded.value.splice(index,1);else forwardExpanded.value.push(key)}
function expandForwardTree(){forwardExpanded.value=forwardTree.value.flatMap(resource=>[resource.key,...resource.channels.flatMap(channel=>[channel.key,...channel.devices.map(device=>device.key)])])}
function collapseForwardTree(){forwardExpanded.value=[]}
function renderedForwardPoints(deviceNode:any){return deviceNode.points.slice(0,forwardPointRenderLimits.value[deviceNode.key]||FORWARD_POINT_BATCH)}
function showMoreForwardPoints(deviceNode:any){forwardPointRenderLimits.value={...forwardPointRenderLimits.value,[deviceNode.key]:(forwardPointRenderLimits.value[deviceNode.key]||FORWARD_POINT_BATCH)+FORWARD_POINT_BATCH}}
watch(forwardSearch,()=>{forwardPointRenderLimits.value={}})
function nextForwardRegister(fn:number,quantity:number){let next=0;for(const point of props.points.filter(item=>Number(item.function||3)===fn))next=Math.max(next,Number(point.register||0)+Math.max(1,Number(point.quantity||1)));return next}
function nextForwardIOA(){return props.points.reduce((next,point)=>Math.max(next,Number(point.ioa||0)+1),1)}
function forwardIECType(point:any){const options=iec104Types[group.value]||iec104Types.yc,type=String(point.pointType||'').toUpperCase();return options.some(item=>item[0]===type)?type:options[0][0]}
function mmsObjectName(metric:string){const base=String(metric||'Point').replace(/[^A-Za-z0-9_]/g,'')||'Point',safe=/^[0-9]/.test(base)?`P${base}`:base,used=new Set(props.points.map(point=>String(point.objectRef||'').split('/').pop()?.split('.')[1]).filter(Boolean));if(!used.has(safe))return safe;let index=2;while(used.has(`${safe}${index}`))index++;return`${safe}${index}`}
function mmsObjectRef(metric:string,dataType:string){const ied=String(props.device.iedName||'WEIKONG').replace(/[^A-Za-z0-9_]/g,'')||'WEIKONG',ld=String(props.device.logicalDevice||'LD1').replace(/[^A-Za-z0-9_]/g,'')||'LD1',objectName=mmsObjectName(metric),leaf=['bool','string'].includes(String(dataType).toLowerCase())?'stVal':'mag.f';return`${ied}${ld}/GGIO1.${objectName}.${leaf}`}
function syncMMSPointShape(point:any){if(!isIEC61850Forward.value)return;const stateValue=['bool','string'].includes(String(point.dataType).toLowerCase());point.fc=stateValue?'ST':'MX';if(!point.objectRef)point.objectRef=mmsObjectRef(point.metric,point.dataType);else point.objectRef=String(point.objectRef).replace(/(?:stVal|mag\.f)$/i,stateValue?'stVal':'mag.f')}
watch(()=>pagedPoints.value.map(point=>String(point.dataType||'')),()=>{if(isIEC61850Forward.value)for(const point of pagedPoints.value)syncMMSPointShape(point)},{flush:'sync'})
watch(()=>[String(props.device?.iedName||''),String(props.device?.logicalDevice||'')],()=>{if(!isIEC61850Forward.value)return;const prefix=`${String(props.device.iedName||'WEIKONG').replace(/[^A-Za-z0-9_]/g,'')||'WEIKONG'}${String(props.device.logicalDevice||'LD1').replace(/[^A-Za-z0-9_]/g,'')||'LD1'}`;for(const point of props.points){const value=String(point.objectRef||'');point.objectRef=value.includes('/')?`${prefix}/${value.slice(value.indexOf('/')+1)}`:mmsObjectRef(point.metric,point.dataType)}})
function confirmForward(){
  if(!Array.isArray(props.device.points))props.device.points=[]
  const existingSources=new Set(props.device.points.map((item:any)=>`${item.sourceDeviceKey}::${item.sourceMetric}`))
  const registerCursor=new Map<number,number>()
  for(const fn of[1,2,3,4])registerCursor.set(fn,nextForwardRegister(fn,1))
  let ioaCursor=nextForwardIOA()
  const usedObjects=new Set(props.device.points.map((point:any)=>String(point.objectRef||'').split('/').pop()?.split('.')[1]).filter(Boolean))
  const additions:any[]=[]
  for(const key of forwardSelected.value){
    const candidate=allForwardCandidateMap.value.get(key)
    if(!candidate||existingSources.has(key))continue
    const {device:source,point}=candidate,deviceKey=source.deviceKey,sourceMetric=point.metric
    const metric=sourceMetric,dataType=point.dataType||'uint16',quantity=point.quantity||(['float32','uint32','int32'].includes(String(dataType).toLowerCase())?2:1),name=point.name||sourceMetric
    if(isIEC104.value){
      const pointType=forwardIECType(point)
      additions.push({name,metric,sourceDeviceKey:deviceKey,sourceMetric,pointType,ioa:ioaCursor++,quantity:1,dataType:pointType.startsWith('M_SP')||pointType.startsWith('M_DP')?'bool':dataType,byteOrder:'big',wordOrder:'big',scale:1,decimals:point.decimals??0})
    }else if(isGOOSEForward.value){
      const normalizedType=String(dataType).toLowerCase()==='auto'?'bool':dataType
      additions.push({name,metric,sourceDeviceKey:deviceKey,sourceMetric,dataType:normalizedType,scale:1,offset:0,unit:point.unit||'',decimals:point.decimals??0})
    }else if(isIEC61850Forward.value){
      const normalizedType=String(dataType).toLowerCase()==='auto'?'float32':dataType,fc=['bool','string'].includes(String(normalizedType).toLowerCase())?'ST':'MX'
      const base=String(metric||'Point').replace(/[^A-Za-z0-9_]/g,'')||'Point',safe=/^[0-9]/.test(base)?`P${base}`:base
      let objectName=safe,index=2
      while(usedObjects.has(objectName))objectName=`${safe}${index++}`
      usedObjects.add(objectName)
      const ied=String(props.device.iedName||'WEIKONG').replace(/[^A-Za-z0-9_]/g,'')||'WEIKONG',ld=String(props.device.logicalDevice||'LD1').replace(/[^A-Za-z0-9_]/g,'')||'LD1',leaf=fc==='ST'?'stVal':'mag.f'
      additions.push({name,metric,sourceDeviceKey:deviceKey,sourceMetric,dataType:normalizedType,objectRef:`${ied}${ld}/GGIO1.${objectName}.${leaf}`,fc,scale:1,offset:0,unit:point.unit||'',decimals:point.decimals??0})
    }else{
      const functionCode=[1,2,3,4].includes(Number(point.function))?Number(point.function):(String(dataType).toLowerCase()==='bool'?1:3),register=registerCursor.get(functionCode)||0
      registerCursor.set(functionCode,register+Math.max(1,Number(quantity)))
      additions.push({name,metric,sourceDeviceKey:deviceKey,sourceMetric,function:functionCode,register,ioa:ioaCursor++,quantity,dataType,byteOrder:'big',wordOrder:'big',scale:1,decimals:point.decimals??0})
    }
    existingSources.add(key)
  }
  if(additions.length)props.device.points=[...props.device.points,...additions]
  forwardSelected.value=[]
  forwardOpen.value=false
}
function openWrite(point:any){writePoint.value=point;writeValue.value=String(live(point)?.value??'')}
function confirmWrite(){if(!writePoint.value)return;const point=writePoint.value,raw=writeValue.value;emit('write',point,raw,(success,value)=>{if(!success)return;lastWrites.value={...lastWrites.value,[point.metric]:{value:String(value??raw),at:new Date().toISOString()}};writePoint.value=undefined;writeValue.value=''})}
</script>

<template>
  <section ref="workspaceRef" class="card legacy-device-workspace" :class="{'is-running':readonly,'iec104-forward':device.protocol==='iec104-server','is-goose-collect':isGOOSECollect,'is-virtualized':isVirtualized,'is-virtual-offset':virtualStart>0}" :style="virtualRootStyle">
    <div class="device-workspace-head"><div><h3>{{device.name||device.deviceKey}}</h3><p class="meta">所属通道：{{channelName||device.channelKey||'-'}}</p></div><button @click="emit('back')">返回通道</button><div class="device-tabs"><button :class="{active:tab==='device'}" @click="tab='device'">设备配置</button><button :class="{active:tab==='points'}" @click="tab='points'">点位配置 ({{points.length}})</button></div></div>
    <template v-if="tab==='device'"><div class="device-config-grid legacy-device-config"><label>设备编号<input :value="device.deviceKey" disabled/></label><label>设备名称<input v-model.trim="device.name" :disabled="readonly"/></label><label>协议<input :value="device.protocol" disabled/></label><template v-if="isForward"><label v-if="device.protocol==='modbus-tcp-slave'||device.protocol==='modbus-tcp'">从站ID<input v-model.number="device.unitId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol==='iec104-server'||device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label><label v-if="isIEC61850Forward">IED 名称<input v-model.trim="device.iedName" :disabled="readonly"/></label><label v-if="isIEC61850Forward">逻辑设备<input v-model.trim="device.logicalDevice" :disabled="readonly"/></label></template><template v-else><label v-if="device.protocol==='modbus-rtu'">串口<input v-model.trim="device.address" :disabled="readonly"/></label><template v-else-if="['modbus-tcp','iec104','siemens-s7'].includes(device.protocol)"><label>设备地址<input v-model.trim="addressHost" :disabled="readonly"/></label><label>端口<input v-model="addressPort" inputmode="numeric" :disabled="readonly"/></label></template><label v-else>{{device.protocol==='opcua'?'Endpoint':'连接地址'}}<input v-model.trim="device.address" :disabled="readonly"/></label><label v-if="device.protocol.startsWith('modbus')">从站ID<input v-model.number="device.slaveId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label><label v-if="device.protocol==='siemens-s7'">Rack<input v-model.number="device.rack" type="number" min="0" :disabled="readonly"/></label><label v-if="device.protocol==='siemens-s7'">Slot<input v-model.number="device.slot" type="number" min="0" :disabled="readonly"/></label><label>超时重连(s)<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="readonly"/></label></template></div><div class="legacy-device-actions"><button v-if="!isForward" @click="emit('test')">测试连接</button><button class="danger" :disabled="readonly" @click="emit('removeDevice')">删除设备</button></div></template>
    <template v-else>
      <div class="point-management-head point-panel-head"><div class="point-panel-title"><h4>点位管理</h4><span class="point-mode-badge" :class="{run:readonly}">{{readonly?'运行中':'配置中 · 可编辑'}}</span></div></div>
      <div class="point-summary iec104-point-tabs">
        <button v-if="device.protocol.startsWith('modbus')" v-for="item in functions" :key="item[0]" :class="{active:group===String(item[0])}" @click="group=String(item[0])"><span>{{item[1]}}</span><strong class="iec104-point-tab-count">{{groupCounts[String(item[0])]||0}}</strong></button>
        <button v-if="isIEC104" v-for="item in iec104Groups" :key="item.key" :class="{active:group===item.key}" @click="group=item.key"><span>{{item.label}}</span><strong class="iec104-point-tab-count">{{groupCounts[item.key]||0}}</strong></button>
      </div>
      <div class="point-sheet"><div class="point-toolbar point-sheet-toolbar"><button :disabled="readonly" @click="addPoint">新增点位</button><button :disabled="readonly||!selected.length" @click="duplicateSelected">复制选中</button><ProtocolPointTools :device="device" :points="points" :all-points="allPoints" :name-prefix="pointNamePrefix()" :readonly="readonly"/><button :disabled="readonly||!points.length" @click="openBatch()">批量修改</button><button class="danger" :disabled="readonly||!selected.length" @click="removeSelected">删除选中</button><span class="selection-count muted">已选 {{selected.length}} / {{visiblePoints.length}}</span><button class="packet-monitor-button" @click="emit('monitor')">实时报文监控</button></div>
      <div v-if="isGOOSECollect" class="table-wrap point-table-wrap point-sheet-scroll goose-point-table-wrap"><table class="legacy-point-table point-sheet-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th>数据集索引</th><th>数据类型</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="10" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><input v-model.trim="point.metric" class="mono" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||String(live(point)?.value??'-')}}</span></td><td><input v-model.number="point.gooseIndex" type="number" min="0" :disabled="readonly"/></td><td><select v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type">{{type}}</option></select></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button></td></tr></tbody></table></div>
      <div v-if="device.protocol==='iec104-server'" class="table-wrap point-table-wrap point-sheet-scroll iec104-forward-scroll"><table class="legacy-point-table point-sheet-table iec104-forward-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th>IOA</th><th>点类型</th><th>数据类型</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="11" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><input v-model.trim="point.metric" class="mono" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||String(live(point)?.value??'-')}}</span></td><td><input v-model.number="point.ioa" type="number" min="1" max="16777215" :disabled="readonly"/></td><td><select v-model="point.pointType" :disabled="readonly"><option v-for="item in iec104Options(point)" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td><select v-model="point.dataType" :disabled="readonly" @change="syncMMSPointShape(point)"><option v-for="type in dataTypes" :key="type">{{type}}</option></select></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button></td></tr></tbody></table></div>
      <div class="table-wrap point-table-wrap point-sheet-scroll"><table class="legacy-point-table point-sheet-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th class="point-column-header" @contextmenu.prevent="openBatch('name')">名称</th><th class="point-column-header" @contextmenu.prevent="openBatch('metric')">标识符</th><th>实时值</th><th v-if="device.protocol.startsWith('modbus')">功能码</th><th v-if="device.protocol.startsWith('modbus')" class="point-column-header" @contextmenu.prevent="openBatch('register')">寄存器</th><th v-if="device.protocol==='iec104'" class="point-column-header" @contextmenu.prevent="openBatch('ioa')">IOA</th><th v-if="device.protocol==='iec104'">点类型</th><th v-if="device.protocol==='siemens-s7'">区域</th><th v-if="device.protocol==='siemens-s7'">DB号</th><th v-if="device.protocol==='siemens-s7'">字节地址</th><th v-if="device.protocol==='opcua'">NodeId</th><th v-if="isIEC61850" class="point-object-ref-column">对象引用</th><th v-if="isIEC61850" class="point-fc-column">FC</th><th v-if="device.protocol.startsWith('modbus')">数量</th><th v-if="device.protocol.startsWith('modbus')||device.protocol==='siemens-s7'">字节序</th><th>数据类型</th><th>倍率</th><th>偏移</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="18" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric" class="point-table-row"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><input v-model.trim="point.metric" class="mono" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||String(live(point)?.value??'-')}}</span></td><td v-if="device.protocol.startsWith('modbus')"><select v-model.number="point.function" :disabled="readonly"><option v-for="item in functions" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td v-if="device.protocol.startsWith('modbus')"><input v-model.number="point.register" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='iec104'"><input v-model.number="point.ioa" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='iec104'"><select v-model="point.pointType" :disabled="readonly"><option v-for="item in iec104Options(point)" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td v-if="device.protocol==='siemens-s7'"><select v-model="point.area" :disabled="readonly"><option value="DB">DB</option><option value="V">V</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></td><td v-if="device.protocol==='siemens-s7'"><input v-model.number="point.dbNumber" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='siemens-s7'"><input v-model.number="point.register" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='opcua'" class="point-address-cell"><input v-model.trim="point.nodeId" class="mono" :disabled="readonly"/></td><td v-if="isIEC61850" class="point-address-cell point-object-ref-column"><input v-model.trim="point.objectRef" class="mono" :disabled="readonly"/></td><td v-if="isIEC61850" class="point-fc-column"><input v-model.trim="point.fc" :disabled="readonly"/></td><td v-if="device.protocol.startsWith('modbus')"><input v-model.number="point.quantity" type="number" min="1" :disabled="readonly"/></td><td v-if="device.protocol.startsWith('modbus')||device.protocol==='siemens-s7'"><select v-model="point.byteOrder" :disabled="readonly"><option value="big">ABCD</option><option value="little">CDAB</option></select></td><td><select v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type">{{type}}</option></select></td><td><input v-model.number="point.scale" type="number" step="any" :disabled="readonly"/></td><td><input v-model.number="point.offset" type="number" step="any" :disabled="readonly"/></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button><small v-if="readonly&&lastWrite(point)" class="last-write-content" :title="`上次下发：${lastWrite(point).value} · ${new Date(lastWrite(point).at).toLocaleString()}`">上次：{{lastWrite(point).value}}</small></td></tr></tbody></table></div><div v-if="visiblePoints.length>pageSize" class="point-pagination"><span>第 {{page}} / {{pageCount}} 页 · 共 {{visiblePoints.length}} 条</span><label>每页<select v-model.number="pageSize"><option :value="50">50</option><option :value="100">100</option><option :value="200">200</option></select></label><button :disabled="page<=1" @click="page--">上一页</button><button :disabled="page>=pageCount" @click="page++">下一页</button></div></div>
    </template>
    <DeviceConfigForm v-if="tab==='device'" :device="device" :readonly="readonly" :is-forward="isForward" @test="emit('test')" @remove="emit('removeDevice')" />
  </section>
  <div v-if="batchOpen" class="point-batch-modal"><div class="point-batch-dialog"><div><h3>批量修改</h3><p class="meta">名称、标识符和数值字段支持递增或递减；固定开头、固定结尾留空时不生效。</p></div><div class="point-batch-grid"><label>字段<select v-model="batchField"><option v-for="item in batchFields" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label><label>范围<select v-model="batchRange"><option value="selected" :disabled="!selected.length">仅选中行</option><option value="all">当前设备全部点位</option></select></label><label>方式<select v-model="batchMode"><option value="set">统一设置</option><option value="increment" :disabled="!batchSequenceEnabled">递增</option><option value="decrement" :disabled="!batchSequenceEnabled">递减</option></select></label><label>{{batchMode==='set'?'统一值':'起始值'}}<select v-if="batchValueOptions.length" v-model="batchValue"><option v-for="item in batchValueOptions" :key="item[0]" :value="item[0]">{{item[1]}}</option></select><input v-else v-model="batchValue"/></label><label v-if="batchMode!=='set'">步长<input v-model.number="batchStep" type="number"/></label><label>固定开头（可选）<input v-model="batchPrefix" :disabled="!batchAffixesEnabled" placeholder="留空不生效"/></label><label>固定结尾（可选）<input v-model="batchSuffix" :disabled="!batchAffixesEnabled" placeholder="留空不生效"/></label></div><div class="dialog-actions"><button @click="batchOpen=false">取消</button><button class="primary" @click="applyBatch">应用</button></div></div></div>
  <div v-if="forwardOpen" class="restart-modal">
    <div class="forward-point-select-dialog">
      <div>
        <h3>选择已有实时点位</h3>
        <p class="meta">按网口、采集通道、设备、点位逐级展开；大量点位按批次渲染，勾选上级可直接批量选择。</p>
      </div>
      <div class="forward-point-select-toolbar">
        <input v-model.trim="forwardSearch" type="search" placeholder="搜索网口、通道、设备、点位名称或标识符"/>
        <button @click="expandForwardTree">全部展开</button>
        <button @click="collapseForwardTree">全部折叠</button>
        <button :disabled="!forwardSelected.length" @click="forwardSelected=[]">清空选择</button>
      </div>
      <div class="forward-source-tree">
        <section v-for="resourceNode in forwardTree" :key="resourceNode.key" class="forward-tree-resource">
          <div class="forward-tree-row resource">
            <button class="forward-tree-expand" :class="{expanded:forwardExpanded.includes(resourceNode.key)||!!forwardSearch.trim()}" @click="toggleForwardExpanded(resourceNode.key)"><span/></button>
            <input type="checkbox" :checked="forwardGroupSelected(resourceNode.keys)" :indeterminate="forwardGroupPartial(resourceNode.keys)" @change="toggleForwardGroup(resourceNode.keys)"/>
            <div>
              <strong>{{resourceNode.resource.name||resourceNode.resource.resourceKey}}</strong>
              <small>{{resourceNode.resource.type==='serial'?'串口资源':'网口资源'}} · {{resourceNode.resource.network?.interface||resourceNode.resource.serial?.port||resourceNode.resource.resourceKey}}</small>
            </div>
            <em>{{resourceNode.pointCount}} 个点位</em>
          </div>
          <div v-if="forwardExpanded.includes(resourceNode.key)||forwardSearch.trim()" class="forward-tree-resource-children">
            <section v-for="channelNode in resourceNode.channels" :key="channelNode.key" class="forward-tree-channel">
              <div class="forward-tree-row channel">
                <button class="forward-tree-expand" :class="{expanded:forwardExpanded.includes(channelNode.key)||!!forwardSearch.trim()}" @click="toggleForwardExpanded(channelNode.key)"><span/></button>
                <input type="checkbox" :checked="forwardGroupSelected(channelNode.keys)" :indeterminate="forwardGroupPartial(channelNode.keys)" @change="toggleForwardGroup(channelNode.keys)"/>
                <div><strong>{{channelNode.channel.name||channelNode.channel.channelKey}}</strong><small>{{channelNode.channel.protocol||'采集通道'}} · {{channelNode.devices.length}} 台设备</small></div>
                <em>{{channelNode.pointCount}} 个点位</em>
              </div>
              <div v-if="forwardExpanded.includes(channelNode.key)||forwardSearch.trim()" class="forward-tree-children">
                <section v-for="deviceNode in channelNode.devices" :key="deviceNode.key" class="forward-tree-device">
                  <div class="forward-tree-row device">
                    <button class="forward-tree-expand" :class="{expanded:forwardExpanded.includes(deviceNode.key)||!!forwardSearch.trim()}" @click="toggleForwardExpanded(deviceNode.key)"><span/></button>
                    <input type="checkbox" :checked="forwardGroupSelected(deviceNode.keys)" :indeterminate="forwardGroupPartial(deviceNode.keys)" @change="toggleForwardGroup(deviceNode.keys)"/>
                    <div><strong>{{deviceNode.device.name||deviceNode.device.deviceKey}}</strong><small>{{deviceNode.device.deviceKey}} · {{deviceNode.device.protocol||'-'}}</small></div>
                    <em>{{deviceNode.pointCount}} 个点位</em>
                  </div>
                  <div v-if="forwardExpanded.includes(deviceNode.key)||forwardSearch.trim()" class="forward-tree-points">
                    <label v-for="point in renderedForwardPoints(deviceNode)" :key="point.metric">
                      <input type="checkbox" :checked="forwardSelectedSet.has(`${deviceNode.device.deviceKey}::${point.metric}`)" @change="toggleForward(`${deviceNode.device.deviceKey}::${point.metric}`)"/>
                      <span><strong>{{point.name||point.metric}}</strong><small>{{point.metric}}</small></span>
                      <code>{{point.dataType||'-'}}</code>
                    </label>
                    <button v-if="renderedForwardPoints(deviceNode).length<deviceNode.pointCount" class="forward-tree-load-more" @click="showMoreForwardPoints(deviceNode)">继续加载（已显示 {{renderedForwardPoints(deviceNode).length}} / {{deviceNode.pointCount}}）</button>
                    <p v-if="!deviceNode.pointCount" class="forward-tree-empty">该设备暂无可选点位</p>
                  </div>
                </section>
              </div>
            </section>
          </div>
        </section>
        <p v-if="!forwardTree.length" class="forward-tree-empty">没有匹配的采集点位</p>
      </div>
      <div class="dialog-actions"><span class="muted">已选择 {{forwardSelected.length}} 个点位</span><button @click="forwardOpen=false">取消</button><button class="primary" :disabled="!forwardSelected.length" @click="confirmForward">添加所选点位</button></div>
    </div>
  </div>
  <div v-if="writePoint" class="restart-modal"><div class="dialog-panel"><h3>点位下发</h3><p class="meta">{{writePoint.name||writePoint.metric}}</p><div v-if="String(writePoint.dataType||'').toLowerCase()==='bool'" class="bool-write-options"><label><input v-model="writeValue" type="radio" value="true"/><span>开启（true）</span></label><label><input v-model="writeValue" type="radio" value="false"/><span>关闭（false）</span></label></div><label v-else>目标值<input v-model="writeValue" @keydown.enter="confirmWrite"/></label><div class="dialog-actions"><button @click="writePoint=undefined">取消</button><button class="primary" @click="confirmWrite">确认下发</button></div></div></div>
</template>
