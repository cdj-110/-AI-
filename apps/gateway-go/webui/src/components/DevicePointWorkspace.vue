<script setup lang="ts">
import { dataTypeLabel } from '../dataTypeLabels.ts'
import { formatPointValueDisplay } from '../pointValueDisplay.ts'
import {defaultPointGroup,iec104PointGroup} from '../pointGroupDefaults.ts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRaw, watch } from 'vue'
import { confirmAction } from '../confirm'
import { usePointTableInteractions } from '../composables/usePointTableInteractions.ts'
import { useResizableTableColumns } from '../composables/useResizableTableColumns.ts'
import DeviceConfigForm from './DeviceConfigForm.vue'
import PersistentHorizontalScrollbar from './PersistentHorizontalScrollbar.vue'
import PointValueDetailDialog from './PointValueDetailDialog.vue'
import ProtocolPointTools from './ProtocolPointTools.vue'
import PointMetricInput from './PointMetricInput.vue'
import DLT645PointSelect from './DLT645PointSelect.vue'
import { api } from '../api'
import type { DLT645Item } from '../dlt645PointOptions.ts'
import { dlt645ActualType, dlt645Version, dlt645Format } from '../dlt645Presentation.ts'

const props=defineProps<{device:any;channelName?:string;interfaceName?:string;points:any[];allPoints?:any[];liveByMetric:Map<string,any>;readonly:boolean;isForward:boolean;saving:boolean;writableMetrics?:string[];sourceDevices:any[];sourceChannels:any[];sourceResources:any[]} >()
type WriteFailure={message:string;detail?:string;code?:string}
type WriteRecord={success:boolean;value:string;at:string;message?:string;detail?:string;code?:string}
const emit=defineEmits<{back:[];reload:[];save:[];collect:[];monitor:[];test:[];readAddress:[];removeDevice:[];navigate:[];write:[point:any,value:string,done:(success:boolean,value?:unknown,failure?:WriteFailure)=>void]}>()
const isIEC104=computed(()=>['iec104','iec104-server'].includes(String(props.device?.protocol||'').toLowerCase()))
const isIEC61850=computed(()=>['iec61850','iec61850-mms-server'].includes(String(props.device?.protocol||'').toLowerCase()))
const isIEC61850Forward=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-mms-server')
const isGOOSECollect=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-goose')
const isGOOSEForward=computed(()=>String(props.device?.protocol||'').toLowerCase()==='iec61850-goose-publisher')
const isOPCDA=computed(()=>String(props.device?.protocol||'').toLowerCase()==='opcda')
const isDLT645=computed(()=>String(props.device?.protocol||'').toLowerCase()==='dlt645')
const defaultGroup=()=>defaultPointGroup(props.device?.protocol,props.points)
let groupSelectedManually=false
function selectPointGroup(value:string){groupSelectedManually=true;group.value=value}
const tab=ref<'device'|'points'>('device'), search=ref(''), group=ref(defaultGroup())
const dltItems=ref<DLT645Item[]>([]), dltDictionaryError=ref(''), dltDictionaryBusy=ref(false)
async function loadDLTItems(){
  if(dltDictionaryBusy.value)return
  dltDictionaryBusy.value=true;dltDictionaryError.value=''
  try{dltItems.value=(await api<{items:DLT645Item[]}>('/api/dlt645/dictionary')).items||[]}
  catch(error){dltDictionaryError.value=error instanceof Error?error.message:'标准数据项加载失败'}
  finally{dltDictionaryBusy.value=false}
}
watch(()=>isDLT645.value&&tab.value==='points',active=>{if(active&&!dltItems.value.length)void loadDLTItems()},{immediate:true})
function dltItemLabel(point:any){return dltItems.value.find(item=>item.key===point.dlt645?.itemKey)?.name||point.dlt645?.itemKey||dlt645Format(point.dlt645?.valueFormat,point.dlt645?.dataLength)}
const workspaceRef=ref<HTMLElement>(),valueDetailPoint=ref<any>()
usePointTableInteractions(workspaceRef,{onLiveValueClick:metric=>{valueDetailPoint.value=props.points.find(point=>point.metric===metric)}})
const VIRTUAL_ROW_HEIGHT=34, VIRTUAL_OVERSCAN=12, VIRTUAL_WINDOW_STEP=8, VIRTUAL_THRESHOLD=120, VIRTUAL_INITIAL_WINDOW=60
const virtualStart=ref(0), virtualEnd=ref(VIRTUAL_INITIAL_WINDOW)
let virtualScrollHost:HTMLElement|undefined, virtualFrame=0, virtualRowsTop=0, virtualViewportHeight=0, virtualGeometryDirty=true
// Compatibility bindings for the old inline pager markup. Infinity keeps that
// block unmounted while the table renders as one continuous list.
const page=ref(1), pageSize=ref(Number.POSITIVE_INFINITY)
const selected=ref<string[]>([]), batchOpen=ref(false), batchField=ref('name'), batchRange=ref('selected'), batchMode=ref('set'), batchValue=ref(''), batchStep=ref(1), batchPrefix=ref(''), batchSuffix=ref('')
const writePoint=ref<any>(), writeValue=ref(''), writeFailure=ref<WriteFailure>(), opcdaAdvancedOpen=ref(false)
const opcdaColumnCount=computed(()=>opcdaAdvancedOpen.value?24:16)
const {tableRef:opcdaTableRef,columnStyles:opcdaColumnStyles,tableStyle:opcdaTableStyle,onPointerMove:onColumnPointerMove,onPointerLeave:onColumnPointerLeave,onPointerDown:onColumnPointerDown,onDoubleClick:onColumnDoubleClick,onClick:onColumnClick,refreshColumnEffects}=useResizableTableColumns(opcdaColumnCount,{onLiveValueClick:metric=>{valueDetailPoint.value=props.points.find(point=>point.metric===metric)}})
const lastWrites=ref<Record<string,WriteRecord>>({})
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
  if(protocol==='opcda')return['auto','bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','datetime']
  if(protocol==='dlt645')return['float64','uint64','string','object']
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
  if(props.device?.protocol==='opcda')fields.splice(2,0,['access','读写类型'])
  if(props.device?.protocol==='dlt645')fields.splice(2,0,['access','读写类型'])
  return isDLT645.value?fields.filter(([key])=>key!=='dataType'):fields
})
const batchValueOptions=computed<[string,string][]>(()=>{
  if(batchField.value==='dataType')return dataTypes.value.map(type=>[type,dataTypeLabel(type)])
  if(batchField.value==='byteOrder')return[['big','ABCD'],['little','CDAB']]
  if(batchField.value==='fc')return[['ST','ST'],['MX','MX']]
  if(batchField.value==='access')return[['read','只读'],['read_write','读写'],['write','只写']]
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

const iec104Group=iec104PointGroup
function iec104Options(point:any){return iec104Types[iec104Group(point)]||iec104Types.yc}
const writableMetricSet=computed(()=>new Set(props.writableMetrics||[]))
function canWrite(point:any){return props.readonly&&!props.isForward&&writableMetricSet.value.has(String(point.metric))}
function writeDataType(point:any=writePoint.value){return String(live(point)?.actualDataType||point?.requestedDataType||point?.dataType||'').toLowerCase()}
function canSubmitWrite(){return !!writePoint.value&&(writeValue.value!==''||['string','bstr'].includes(writeDataType()))}

const visiblePoints=computed(()=>props.points.filter(point=>{
  const q=search.value.trim().toLowerCase()
  const grouped=props.device.protocol?.startsWith('modbus')?String(point.function||3)===group.value:isIEC104.value?iec104Group(point)===group.value:true
  return grouped&&(!q||`${point.name||''} ${point.metric||''}`.toLowerCase().includes(q))
}))
const visibleMetricSet=computed(()=>new Set(visiblePoints.value.map(point=>String(point.metric||''))))
const selectedMetricSet=computed(()=>new Set(selected.value))
const selectedDLTPoint=computed(()=>props.points.find(point=>point.metric===selected.value[0]))
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
watch(()=>[props.device,props.device?.protocol],()=>{
  groupSelectedManually=false
  search.value=''
  tab.value=props.points.length?'points':'device'
  group.value=defaultGroup()
  selected.value=[]
  opcdaAdvancedOpen.value=false
  writePoint.value=undefined
  writeValue.value=''
  writeFailure.value=undefined
  lastWrites.value={}
  virtualStart.value=0
  virtualEnd.value=VIRTUAL_INITIAL_WINDOW
  virtualGeometryDirty=true
},{immediate:true})
watch(()=>props.points.length,(count,previous)=>{
  if(count>0&&previous===0&&!groupSelectedManually){tab.value='points';group.value=defaultGroup()}
})
watch(tab,()=>emit('navigate'))
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
watch(opcdaAdvancedOpen,refreshColumnEffects)
watch(()=>pagedPoints.value.map(point=>point.metric),refreshColumnEffects,{flush:'post'})
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
      selected.value=selected.value.map(metric=>metric===previous?point.metric:metric)
    }
  }finally{normalizingMetric=false}
},{flush:'sync'})

function protocolMetricPrefix(){return{'modbus-tcp':'mbtcp','modbus-tcp-slave':'mbtcp','modbus-rtu':'mbrtu','dlt645':'dlt645','iec104':'iec104','iec104-server':'iec104','siemens-s7':'s7','opcua':'opcua','opcda':'opcda','iec61850':'iec61850','iec61850-mms-server':'iec61850','iec61850-goose':'goose','iec61850-goose-publisher':'goose'}[String(props.device.protocol||'').toLowerCase()]||'point'}
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
function addPoint(){if(props.isForward){forwardSelected.value=[];forwardSearch.value='';forwardPointRenderLimits.value={};forwardExpanded.value=forwardTree.value.map(resource=>resource.key);forwardOpen.value=true;return}const metric=nextMetric(),category=isIEC104.value?group.value:'',point={deviceKey:props.device.deviceKey,channelKey:props.device.channelKey,name:pointName(metric),metric,protocol:props.device.protocol,address:props.device.address,slaveId:props.device.slaveId||1,commonAddress:props.device.commonAddress||1,function:props.device.protocol?.startsWith('modbus')?Number(group.value):3,register:0,ioa:props.points.length+1,itemId:isOPCDA.value?'':'',accessPath:'',access:isOPCDA.value||isDLT645.value?'read':undefined,requestedDataType:isOPCDA.value?'auto':undefined,active:isOPCDA.value?true:undefined,enabled:true,dlt645:isDLT645.value?{itemKey:'',dataIdentifier:'00000000',valueFormat:'bcd',dataLength:4,access:'read'}:undefined,gooseIndex:isGOOSECollect.value?props.points.length:undefined,goCbRef:props.device.goCbRef,dataSetRef:props.device.dataSetRef,appId:props.device.appId,destinationMac:props.device.destinationMac,vlanId:props.device.vlanId,vlanPriority:props.device.vlanPriority,pointType:category?(iec104Types[category]||iec104Types.yc)[0][0]:undefined,quantity:1,dataType:isOPCDA.value?'auto':isDLT645.value?'float64':isGOOSECollect.value?'bool':category==='yx'?'bool':category==='yk'?'bool':category==='ym'?'uint32':'float32',byteOrder:'big',wordOrder:'big',scale:1,offset:0,decimals:0};knownPointMetrics.set(point,metric);appendPoints([point]);selected.value=[metric]}
function duplicate(point:any){const copy={...toRaw(point)};copy.metric=nextCopiedMetric(point.metric);copy.name=pointName(copy.metric);knownPointMetrics.set(copy,copy.metric);appendPoints([copy])}
function duplicateSelected(){const wanted=new Set(selected.value),copies:any[]=[];for(const point of props.points){if(!wanted.has(String(point.metric)))continue;const copy={...toRaw(point)},copyMetric=nextCopiedMetric(point.metric);copy.metric=copyMetric;copy.name=pointName(copyMetric);knownPointMetrics.set(copy,copyMetric);copies.push(copy)}appendPoints(copies);selected.value=copies.map(point=>point.metric)}
async function remove(point:any){if(!await confirmAction({title:'删除点位',message:`确认删除点位“${point.name||point.metric}”？`,detail:'删除后需保存配置才会正式生效。',confirmText:'确认删除',danger:true}))return;metricIndex.delete(String(point.metric||'').toLowerCase());props.device.points.splice(props.device.points.indexOf(point),1)}
async function removeSelected(){if(!selected.value.length||!await confirmAction({title:'删除所选点位',message:`确认删除选中的 ${selected.value.length} 个点位？`,detail:'该操作会一次移除全部所选点位，请确认选择范围。',confirmText:'确认删除',danger:true}))return;const removing=new Set(selected.value);props.device.points=props.points.filter(point=>!removing.has(point.metric));for(const metric of removing)metricIndex.delete(metric.toLowerCase());selected.value=[]}
function toggle(metric:string){const index=selected.value.indexOf(metric);if(index>=0)selected.value.splice(index,1);else selected.value.push(metric)}
function toggleAll(){const metrics=visiblePoints.value.map(item=>item.metric),visible=new Set(metrics);selected.value=allVisibleSelected.value?selected.value.filter(item=>!visible.has(item)):[...new Set([...selected.value,...metrics])]}
function live(point:any){return props.liveByMetric.get(point.metric)}
function formatPointValue(point:any){
  const state=live(point)
  if(state?.error||state?.value===undefined||state?.value===null)return '-'
  return formatPointValueDisplay(state.value,point.decimals,state.valueText)
}
function pointQuality(point:any){const state=live(point);return state?.quality||(state?.error?'OFFLINE':'-')}
function formatTimestamp(value:unknown){
	if(!value)return '-'
	const date=new Date(String(value))
	if(Number.isNaN(date.getTime()))return String(value)
	const pad=(part:number,width=2)=>String(part).padStart(width,'0')
	return `${date.getFullYear()}-${pad(date.getMonth()+1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(),3)}`
}
function pointSourceTime(point:any){return formatTimestamp(live(point)?.sourceTimestamp)}
function pointCollectionTime(point:any){return formatTimestamp(live(point)?.receiveTimestamp||live(point)?.updatedAt)}
function pointCollectionMode(point:any){
	const state=live(point),mode=String(state?.collectionMode||'')
	if(mode==='subscription')return'异步订阅'
	if(mode==='sync_fallback'||String(state?.diagnostic||'').includes('IOPCSyncIO fallback'))return'同步回退'
	if(mode==='sync')return'同步读取'
	return'-'
}
function pointDiagnostic(point:any){
  const state=live(point)
  if(state?.error)return state.error
  if(state?.diagnostic)return state.diagnostic
  if(state?.stale)return '数据陈旧'
  return state?.receiveTimestamp?'正常':'等待采集'
}
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
watch(()=>pagedPoints.value.map(point=>({point,dataType:String(point.dataType||'')})),(current,previous)=>{
  if(props.readonly||!isIEC61850Forward.value)return
  for(const entry of current){
    const before=previous?.find(item=>item.point===entry.point)
    if(before&&before.dataType!==entry.dataType)syncMMSPointShape(entry.point)
  }
},{flush:'sync'})
watch(()=>[props.device,String(props.device?.iedName||''),String(props.device?.logicalDevice||'')],(current,previous)=>{if(props.readonly||!isIEC61850Forward.value||!previous||current[0]!==previous[0])return;const prefix=`${String(props.device.iedName||'WEIKONG').replace(/[^A-Za-z0-9_]/g,'')||'WEIKONG'}${String(props.device.logicalDevice||'LD1').replace(/[^A-Za-z0-9_]/g,'')||'LD1'}`;for(const point of props.points){const value=String(point.objectRef||'');point.objectRef=value.includes('/')?`${prefix}/${value.slice(value.indexOf('/')+1)}`:mmsObjectRef(point.metric,point.dataType)}})
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
function openWrite(point:any){writePoint.value=point;writeValue.value=String(live(point)?.valueText??live(point)?.value??'');const previous=lastWrite(point);writeFailure.value=previous&&!previous.success?{message:previous.message||'写入失败',detail:previous.detail,code:previous.code}:undefined}
function openAlarm(point:any){window.dispatchEvent(new CustomEvent('gateway-open-alarm-rule',{detail:{deviceKey:props.device.deviceKey,metric:point.metric,name:point.name}}))}
function confirmWrite(){if(!canSubmitWrite())return;const point=writePoint.value,raw=writeValue.value;writeFailure.value=undefined;emit('write',point,raw,(success,value,failure)=>{const at=new Date().toISOString();if(!success){const current=failure||{message:'写入失败：设备未返回成功结果。'};writeFailure.value=current;lastWrites.value={...lastWrites.value,[point.metric]:{success:false,value:raw,at,...current}};return}lastWrites.value={...lastWrites.value,[point.metric]:{success:true,value:String(value??raw),at}};writeFailure.value=undefined;writePoint.value=undefined;writeValue.value=''})}
</script>

<template>
  <section ref="workspaceRef" class="card legacy-device-workspace" :class="{'is-running':readonly,'iec104-forward':device.protocol==='iec104-server','is-goose-collect':isGOOSECollect,'is-opcda':isOPCDA,'is-virtualized':isVirtualized,'is-virtual-offset':virtualStart>0}" :style="virtualRootStyle">
    <div class="device-workspace-head"><div><h3>{{device.name||device.deviceKey}}</h3><p class="meta">所属通道：{{channelName||device.channelKey||'-'}}</p></div><div class="contextual-config-actions"><button @click="emit('back')">返回通道</button><button @click="emit('reload')">重新加载</button><button class="primary" :disabled="saving||readonly" @click="emit('save')">{{saving?'保存中…':'保存配置'}}</button></div><div class="device-tabs"><button :class="{active:tab==='device'}" @click="tab='device'">设备配置</button><button :class="{active:tab==='points'}" @click="tab='points'">点位配置 ({{points.length}})</button></div></div>
    <template v-if="tab==='device'"><div class="device-config-grid legacy-device-config"><label>设备编号<input :value="device.deviceKey" disabled/></label><label>设备名称<input v-model.trim="device.name" :disabled="readonly"/></label><label>协议<input :value="device.protocol" disabled/></label><template v-if="isForward"><label v-if="device.protocol==='modbus-tcp-slave'||device.protocol==='modbus-tcp'">从站ID<input v-model.number="device.unitId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol==='iec104-server'||device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label><label v-if="isIEC61850Forward">IED 名称<input v-model.trim="device.iedName" :disabled="readonly"/></label><label v-if="isIEC61850Forward">逻辑设备<input v-model.trim="device.logicalDevice" :disabled="readonly"/></label></template><template v-else><label v-if="device.protocol==='modbus-rtu'">串口<input v-model.trim="device.address" :disabled="readonly"/></label><template v-else-if="['modbus-tcp','iec104','siemens-s7'].includes(device.protocol)"><label>设备地址<input v-model.trim="addressHost" :disabled="readonly"/></label><label>端口<input v-model="addressPort" inputmode="numeric" :disabled="readonly"/></label></template><label v-else>{{device.protocol==='opcua'?'Endpoint':'连接地址'}}<input v-model.trim="device.address" :disabled="readonly"/></label><label v-if="device.protocol.startsWith('modbus')">从站ID<input v-model.number="device.slaveId" type="number" min="1" max="247" :disabled="readonly"/></label><label v-if="device.protocol==='iec104'">公共地址<input v-model.number="device.commonAddress" type="number" min="1" :disabled="readonly"/></label><label v-if="device.protocol==='siemens-s7'">Rack<input v-model.number="device.rack" type="number" min="0" :disabled="readonly"/></label><label v-if="device.protocol==='siemens-s7'">Slot<input v-model.number="device.slot" type="number" min="0" :disabled="readonly"/></label><label>超时重连(s)<input v-model.number="device.reconnectIntervalSeconds" type="number" min="1" :disabled="readonly"/></label></template></div><div class="legacy-device-actions"><button v-if="!isForward" @click="emit('test')">测试连接</button><button class="danger" :disabled="readonly" @click="emit('removeDevice')">删除设备</button></div></template>
    <template v-else>
      <div class="point-management-head point-panel-head"><div class="point-panel-title"><h4>点位管理</h4><span class="point-mode-badge" :class="{run:readonly}">{{readonly?'运行中':'配置中 · 可编辑'}}</span></div></div>
      <div class="point-summary iec104-point-tabs">
        <button v-if="device.protocol.startsWith('modbus')" v-for="item in functions" :key="item[0]" :class="{active:group===String(item[0])}" @click="selectPointGroup(String(item[0]))"><span>{{item[1]}}</span><strong class="iec104-point-tab-count">{{groupCounts[String(item[0])]||0}}</strong></button>
        <button v-if="isIEC104" v-for="item in iec104Groups" :key="item.key" :class="{active:group===item.key}" @click="selectPointGroup(item.key)"><span>{{item.label}}</span><strong class="iec104-point-tab-count">{{groupCounts[item.key]||0}}</strong></button>
      </div>
      <div class="point-sheet"><div class="point-toolbar point-sheet-toolbar"><button :disabled="readonly" @click="addPoint">新增点位</button><button :disabled="readonly||!selected.length" @click="duplicateSelected">复制选中</button><ProtocolPointTools :device="device" :points="points" :all-points="allPoints" :name-prefix="pointNamePrefix()" :next-metric="nextMetric" :point-name="pointName" :readonly="readonly"/><button :disabled="readonly||!points.length" @click="openBatch()">批量修改</button><button class="danger" :disabled="readonly||!selected.length" @click="removeSelected">删除选中</button><span v-if="!readonly" class="selection-count muted">已选 {{selected.length}} / {{visiblePoints.length}}</span><div class="point-runtime-actions"><button v-if="readonly" class="collect-now-button" @click="emit('collect')">立即采集</button><button class="packet-monitor-button" @click="emit('monitor')">实时报文监控</button></div></div>
      <div v-if="isDLT645&&!readonly" class="dlt645-editor-hint form-hint">数据项 / 格式、值类型可直接下拉选择；标准项按字典匹配，修改手动值类型会联动解析格式。<span v-if="dltDictionaryBusy">正在加载标准数据项…</span><span v-if="dltDictionaryError" role="alert">标准数据项加载失败，仍可选择手动格式。<button @click="loadDLTItems">重试</button></span></div>
      <div v-if="isDLT645&&!readonly&&selectedDLTPoint&&!selectedDLTPoint.dlt645?.itemKey" class="point-batch-grid dlt645-manual-editor"><label>数据长度（字节）<input v-model.number="selectedDLTPoint.dlt645.dataLength" type="number" min="1" max="200"/></label><label>小数位<input v-model.number="selectedDLTPoint.decimals" type="number" min="0" max="9"/></label><span class="form-hint">{{selectedDLTPoint.metric}} · 手动 DI：2007 版填写 8 位十六进制，1997 版填写 4 位；自动模式按 DI 长度限定版本。</span></div>
      <div v-if="isDLT645" class="table-wrap point-table-wrap point-sheet-scroll dlt645-point-table">
        <table class="legacy-point-table point-sheet-table">
          <thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th>质量</th><th>版本</th><th>DI</th><th>数据项 / 格式</th><th class="dlt645-type-column">{{readonly?'实际类型':'值类型'}}</th><th class="dlt645-access-column">读写类型</th><th class="dlt645-scale-column" title="最终值 = 协议解析值 × 倍率 + 偏移">倍率</th><th class="dlt645-offset-column" title="最终值 = 协议解析值 × 倍率 + 偏移">偏移</th><th class="dlt645-unit-column">单位</th><th class="dlt645-status-column">状态</th><th class="point-sheet-actions">操作</th></tr></thead>
          <tbody><tr v-if="!visiblePoints.length"><td colspan="16" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric" class="point-table-row" :data-row-key="point.metric"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td>
            <td class="point-sheet-index">{{pointOffset+index+1}}</td>
            <td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td>
            <td class="point-metric-cell"><PointMetricInput v-model="point.metric" :disabled="readonly"/></td>
            <td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||formatPointValue(point)}}</span></td>
            <td>{{live(point)?.quality||'-'}}</td>
            <td>{{dlt645Version(live(point),device.dlt645?.version)}}</td>
            <td><input v-model.trim="point.dlt645.dataIdentifier" class="mono" :placeholder="point.dlt645.itemKey?'字典自动选择':'2007: 8位 / 1997: 4位'" :disabled="readonly||!!point.dlt645.itemKey"/></td>
            <td><span v-if="readonly" :title="dltItemLabel(point)">{{dltItemLabel(point)}}</span><DLT645PointSelect v-else :point="point" :items="dltItems" :version="device.dlt645?.version" kind="format"/></td>
            <td class="dlt645-type-column"><span v-if="readonly" class="dlt645-type-tag" :title="dlt645ActualType(live(point))">{{dataTypeLabel(dlt645ActualType(live(point)))}}</span><DLT645PointSelect v-else :point="point" :items="dltItems" kind="type"/></td>
            <td class="dlt645-access-column"><span v-if="readonly">{{({read:'只读',read_write:'读写',write:'只写'} as Record<string,string>)[point.dlt645.access]||'只读'}}</span><select v-else v-model="point.dlt645.access" aria-label="读写类型"><option value="read">只读</option><option value="read_write">读写</option><option value="write">只写</option></select></td>
            <td class="dlt645-scale-column"><span v-if="readonly">{{point.scale??1}}</span><input v-else v-model.number="point.scale" type="number" step="any" placeholder="1" aria-label="倍率"/></td>
            <td class="dlt645-offset-column"><span v-if="readonly">{{point.offset??0}}</span><input v-else v-model.number="point.offset" type="number" step="any" placeholder="0" aria-label="偏移"/></td>
            <td class="dlt645-unit-column"><span v-if="readonly" :title="point.unit">{{point.unit||'—'}}</span><input v-else v-model.trim="point.unit" aria-label="单位"/></td>
            <td class="dlt645-status-column">{{live(point)?.error||'正常'}}</td>
            <td class="row-actions point-sheet-actions"><button @click="openAlarm(point)">告警</button><button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button></td></tr></tbody>
        </table>
      </div>
      <div class="mobile-point-list">
        <p v-if="!visiblePoints.length" class="mobile-point-empty">暂无点位</p>
        <article v-for="(point,index) in pagedPoints" :key="`mobile-${point.metric}`" class="mobile-point-card" :class="{error:!!live(point)?.error}">
          <header>
            <label v-if="!readonly" class="mobile-point-check"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/><span>#{{pointOffset+index+1}}</span></label>
            <span v-else class="mobile-point-number">#{{pointOffset+index+1}}</span>
            <strong class="mobile-point-live" :class="{clickable:readonly}" :role="readonly?'button':undefined" :tabindex="readonly?0:undefined" @click="readonly&&(valueDetailPoint=point)" @keydown.enter="readonly&&(valueDetailPoint=point)" @keydown.space.prevent="readonly&&(valueDetailPoint=point)">{{readonly?(live(point)?.error||formatPointValue(point)):'—'}}</strong>
          </header>
          <div class="mobile-point-primary">
            <strong :title="point.name||point.metric">{{point.name||point.metric}}</strong>
            <code :title="point.metric">{{point.metric}}</code>
          </div>
          <details>
            <summary>{{readonly?'查看点位详情':'编辑点位参数'}}</summary>
            <div class="mobile-point-fields">
              <label v-if="!readonly" class="mobile-point-wide">名称<input v-model.trim="point.name"/></label>
              <label v-if="!readonly" class="mobile-point-wide">标识符<PointMetricInput v-model="point.metric" :disabled="readonly"/></label>
              <label v-if="isGOOSECollect">数据集索引<input v-model.number="point.gooseIndex" type="number" min="0" :disabled="readonly"/></label>
              <label v-if="device.protocol.startsWith('modbus')">寄存器<input v-model.number="point.register" type="number" :disabled="readonly"/></label>
              <label v-if="device.protocol.startsWith('modbus')">数量<input v-model.number="point.quantity" type="number" min="1" :disabled="readonly"/></label>
              <label v-if="device.protocol.startsWith('modbus')">字节序<select v-model="point.byteOrder" :disabled="readonly"><option value="big">ABCD</option><option value="little">CDAB</option></select></label>
              <label v-if="device.protocol==='iec104'||device.protocol==='iec104-server'">IOA<input v-model.number="point.ioa" type="number" min="1" max="16777215" :disabled="readonly"/></label>
              <label v-if="device.protocol==='iec104'||device.protocol==='iec104-server'">点类型<select v-model="point.pointType" :disabled="readonly"><option v-for="item in iec104Options(point)" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></label>
              <label v-if="device.protocol==='siemens-s7'">区域<select v-model="point.area" :disabled="readonly"><option value="DB">DB</option><option value="V">V</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></label>
              <label v-if="device.protocol==='siemens-s7'">DB号<input v-model.number="point.dbNumber" type="number" :disabled="readonly"/></label>
              <label v-if="device.protocol==='siemens-s7'">字节地址<input v-model.number="point.register" type="number" :disabled="readonly"/></label>
              <label v-if="device.protocol==='opcua'" class="mobile-point-wide">NodeId<input v-model.trim="point.nodeId" class="mono" :disabled="readonly"/></label>
              <template v-if="isOPCDA">
                <label class="mobile-point-wide">Item ID<input v-model.trim="point.itemId" class="mono" placeholder="例如 Random.Real8" :disabled="readonly"/></label>
                <label class="mobile-point-wide">Access Path<input v-model.trim="point.accessPath" class="mono" :disabled="readonly"/></label>
                <label>读写类型<select v-model="point.access" :disabled="readonly"><option value="read">只读</option><option value="read_write">读写</option><option value="write">只写</option></select></label>
                <label>请求数据类型<select :title="point.requestedDataType" class="point-data-type-select" v-model="point.requestedDataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></label>
                <label>Group 激活<select v-model="point.active" :disabled="readonly"><option :value="true">激活</option><option :value="false">不激活</option></select></label>
                <label>采集状态<select v-model="point.enabled" :disabled="readonly"><option :value="true">启用</option><option :value="false">停用</option></select></label>
              </template>
              <label v-if="isIEC61850" class="mobile-point-wide">对象引用<input v-model.trim="point.objectRef" class="mono" :disabled="readonly"/></label>
              <label v-if="isIEC61850">FC<input v-model.trim="point.fc" :disabled="readonly"/></label>
              <template v-if="isDLT645">
                <label class="mobile-point-wide">数据项 / 格式<span v-if="readonly">{{dltItemLabel(point)}}</span><DLT645PointSelect v-else :point="point" :items="dltItems" :version="device.dlt645?.version" kind="format"/></label>
                <label>{{readonly?'实际类型':'值类型'}}<span v-if="readonly" class="dlt645-type-tag" :title="dlt645ActualType(live(point))">{{dataTypeLabel(dlt645ActualType(live(point)))}}</span><DLT645PointSelect v-else :point="point" :items="dltItems" kind="type"/></label>
                <label>DI<input v-model.trim="point.dlt645.dataIdentifier" :disabled="readonly||!!point.dlt645.itemKey" placeholder="字典自动选择"/></label>
                <label>数据长度（字节）<input v-model.number="point.dlt645.dataLength" type="number" min="1" max="200" :disabled="readonly||!!point.dlt645.itemKey"/></label>
              </template>
              <label v-else>数据类型<select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></label>
              <label>倍率<input v-model.number="point.scale" type="number" step="any" :disabled="readonly"/></label>
              <label>偏移<input v-model.number="point.offset" type="number" step="any" :disabled="readonly"/></label>
              <label>单位<input v-model.trim="point.unit" :disabled="readonly"/></label>
              <label>小数位<input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></label>
            </div>
          </details>
          <footer>
            <small v-if="readonly&&lastWrite(point)" class="last-write-content" :class="{failed:!lastWrite(point).success}" :title="lastWrite(point).success?`上次下发成功：${lastWrite(point).value}`:lastWrite(point).message">{{lastWrite(point).success?`上次下发：${lastWrite(point).value}`:'上次下发失败'}}</small>
            <span/>
            <button @click="openAlarm(point)">告警</button>
            <button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button>
            <button v-if="!readonly" @click="duplicate(point)">复制</button>
            <button v-if="!readonly" class="danger" @click="remove(point)">删除</button>
          </footer>
        </article>
      </div>
      <div v-if="isGOOSECollect" class="table-wrap point-table-wrap point-sheet-scroll goose-point-table-wrap"><table class="legacy-point-table point-sheet-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th>数据集索引</th><th class="point-data-type-column">数据类型</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="10" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><PointMetricInput v-model="point.metric" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||formatPointValue(point)}}</span></td><td><input v-model.number="point.gooseIndex" type="number" min="0" :disabled="readonly"/></td><td class="point-data-type-column"><select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button @click="openAlarm(point)">告警</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button></td></tr></tbody></table></div>
      <div v-if="device.protocol==='iec104-server'" class="table-wrap point-table-wrap point-sheet-scroll iec104-forward-scroll"><table class="legacy-point-table point-sheet-table iec104-forward-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th>IOA</th><th>点类型</th><th class="point-data-type-column">数据类型</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="11" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><PointMetricInput v-model="point.metric" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||formatPointValue(point)}}</span></td><td><input v-model.number="point.ioa" type="number" min="1" max="16777215" :disabled="readonly"/></td><td><select v-model="point.pointType" :disabled="readonly"><option v-for="item in iec104Options(point)" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td class="point-data-type-column"><select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly" @change="syncMMSPointShape(point)"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button @click="openAlarm(point)">告警</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button></td></tr></tbody></table></div>
      <div class="table-wrap point-table-wrap point-sheet-scroll"><table class="legacy-point-table point-sheet-table"><thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th class="point-column-header" @contextmenu.prevent="openBatch('name')">名称</th><th class="point-column-header" @contextmenu.prevent="openBatch('metric')">标识符</th><th>实时值</th><th v-if="device.protocol.startsWith('modbus')">功能码</th><th v-if="device.protocol.startsWith('modbus')" class="point-column-header" @contextmenu.prevent="openBatch('register')">寄存器</th><th v-if="device.protocol==='iec104'" class="point-column-header" @contextmenu.prevent="openBatch('ioa')">IOA</th><th v-if="device.protocol==='iec104'">点类型</th><th v-if="device.protocol==='siemens-s7'">区域</th><th v-if="device.protocol==='siemens-s7'">DB号</th><th v-if="device.protocol==='siemens-s7'">字节地址</th><th v-if="device.protocol==='opcua'">NodeId</th><th v-if="isIEC61850" class="point-object-ref-column">对象引用</th><th v-if="isIEC61850" class="point-fc-column">FC</th><th v-if="device.protocol.startsWith('modbus')">数量</th><th v-if="device.protocol.startsWith('modbus')||device.protocol==='siemens-s7'">字节序</th><th class="point-data-type-column">数据类型</th><th>倍率</th><th>偏移</th><th>单位</th><th>小数位</th><th class="point-sheet-actions">操作</th></tr></thead><tbody><tr v-if="!visiblePoints.length"><td colspan="18" class="point-sheet-empty">暂无点位</td></tr><tr v-for="(point,index) in pagedPoints" :key="point.metric" class="point-table-row"><td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td><td class="point-sheet-index">{{pointOffset+index+1}}</td><td class="point-name-cell"><input v-model.trim="point.name" :disabled="readonly"/></td><td class="point-metric-cell"><PointMetricInput v-model="point.metric" :disabled="readonly"/></td><td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{live(point)?.error||formatPointValue(point)}}</span></td><td v-if="device.protocol.startsWith('modbus')"><select v-model.number="point.function" :disabled="readonly"><option v-for="item in functions" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td v-if="device.protocol.startsWith('modbus')"><input v-model.number="point.register" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='iec104'"><input v-model.number="point.ioa" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='iec104'"><select v-model="point.pointType" :disabled="readonly"><option v-for="item in iec104Options(point)" :key="item[0]" :value="item[0]">{{item[1]}}</option></select></td><td v-if="device.protocol==='siemens-s7'"><select v-model="point.area" :disabled="readonly"><option value="DB">DB</option><option value="V">V</option><option value="M">M</option><option value="I">I</option><option value="Q">Q</option></select></td><td v-if="device.protocol==='siemens-s7'"><input v-model.number="point.dbNumber" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='siemens-s7'"><input v-model.number="point.register" type="number" :disabled="readonly"/></td><td v-if="device.protocol==='opcua'" class="point-address-cell"><input v-model.trim="point.nodeId" class="mono" :disabled="readonly"/></td><td v-if="isIEC61850" class="point-address-cell point-object-ref-column"><input v-model.trim="point.objectRef" class="mono" :disabled="readonly"/></td><td v-if="isIEC61850" class="point-fc-column"><input v-model.trim="point.fc" :disabled="readonly"/></td><td v-if="device.protocol.startsWith('modbus')"><input v-model.number="point.quantity" type="number" min="1" :disabled="readonly"/></td><td v-if="device.protocol.startsWith('modbus')||device.protocol==='siemens-s7'"><select v-model="point.byteOrder" :disabled="readonly"><option value="big">ABCD</option><option value="little">CDAB</option></select></td><td class="point-data-type-column"><select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td><td><input v-model.number="point.scale" type="number" step="any" :disabled="readonly"/></td><td><input v-model.number="point.offset" type="number" step="any" :disabled="readonly"/></td><td><input v-model.trim="point.unit" :disabled="readonly"/></td><td><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td><td class="row-actions point-sheet-actions"><button @click="openAlarm(point)">告警</button><button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button><small v-if="readonly&&lastWrite(point)" class="last-write-content" :class="{failed:!lastWrite(point).success}" :title="lastWrite(point).success?`上次下发成功：${lastWrite(point).value} · ${new Date(lastWrite(point).at).toLocaleString()}`:lastWrite(point).message">{{lastWrite(point).success?`上次：${lastWrite(point).value}`:'失败'}}</small></td></tr></tbody></table></div><div v-if="visiblePoints.length>pageSize" class="point-pagination"><span>第 {{page}} / {{pageCount}} 页 · 共 {{visiblePoints.length}} 条</span><label>每页<select v-model.number="pageSize"><option :value="50">50</option><option :value="100">100</option><option :value="200">200</option></select></label><button :disabled="page<=1" @click="page--">上一页</button><button :disabled="page>=pageCount" @click="page++">下一页</button></div></div>
    </template>
    <div v-if="tab==='points'&&isOPCDA" class="table-wrap point-table-wrap point-sheet-scroll opcda-point-table-wrap" :class="{'opcda-advanced-open':opcdaAdvancedOpen}">
      <table ref="opcdaTableRef" class="legacy-point-table point-sheet-table resizable-opcda-table" :style="opcdaTableStyle" @pointermove="onColumnPointerMove" @pointerleave="onColumnPointerLeave" @pointerdown="onColumnPointerDown" @dblclick="onColumnDoubleClick" @click="onColumnClick">
        <colgroup><col v-for="(style,index) in opcdaColumnStyles" :key="index" :style="style"/></colgroup>
        <thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allVisibleSelected" @change="toggleAll"/></th><th class="point-sheet-index">#</th><th class="opcda-name-column">名称</th><th class="opcda-metric-column">标识符</th><th class="opcda-value-column">实时值</th><th class="opcda-quality-column">质量</th><th class="opcda-time-column">源时间</th><th class="opcda-collection-time-column">采集时间</th><th class="opcda-collection-mode-column">采集方式</th><th class="opcda-item-column">Item ID</th><th class="opcda-type-column">实际类型</th><th class="opcda-access-column">读写类型</th><th class="opcda-flag-column">启用</th><th class="opcda-diagnostic-column">状态/诊断</th><th class="point-sheet-actions">操作</th><th class="opcda-advanced-toggle-column"><button type="button" class="opcda-advanced-toggle" :aria-expanded="opcdaAdvancedOpen" :title="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" @click="opcdaAdvancedOpen=!opcdaAdvancedOpen"><span aria-hidden="true">{{opcdaAdvancedOpen?'◀':'▶'}}</span> 高级配置</button></th><template v-if="opcdaAdvancedOpen"><th class="opcda-access-path-column">Access Path</th><th class="opcda-request-type-column">请求类型</th><th class="opcda-output-type-column">输出类型</th><th class="opcda-active-column">激活</th><th class="opcda-scale-column">倍率</th><th class="opcda-offset-column">偏移</th><th class="opcda-unit-column">单位</th><th class="opcda-decimals-column">小数位</th></template></tr></thead>
        <tbody>
          <tr v-if="!visiblePoints.length"><td :colspan="opcdaAdvancedOpen?22:14" class="point-sheet-empty">暂无点位</td></tr>
          <tr v-for="(point,index) in pagedPoints" :key="point.metric" class="point-table-row" :data-row-key="`${device.deviceKey}::${point.metric}`">
            <td class="point-sheet-select"><input type="checkbox" :checked="selectedMetricSet.has(point.metric)" @change="toggle(point.metric)"/></td>
            <td class="point-sheet-index">{{pointOffset+index+1}}</td>
            <td class="point-name-cell"><input v-model.trim="point.name" :title="point.name" :disabled="readonly"/></td>
            <td class="point-metric-cell"><PointMetricInput v-model="point.metric" :disabled="readonly"/></td>
            <td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'" :title="String(live(point)?.value??'-')"><span>{{formatPointValue(point)}}</span></td>
            <td class="opcda-quality-column"><span class="opcda-quality" :class="String(pointQuality(point)).toLowerCase()" :title="`原始质量码：${live(point)?.rawQuality??'-'}`">{{pointQuality(point)}}</span></td>
            <td class="opcda-time-column mono" :title="String(live(point)?.sourceTimestamp||'-')">{{pointSourceTime(point)}}</td>
            <td class="opcda-collection-time-column mono" :title="String(live(point)?.receiveTimestamp||live(point)?.updatedAt||'-')">{{pointCollectionTime(point)}}</td>
            <td class="opcda-collection-mode-column"><span class="opcda-collection-mode" :class="String(live(point)?.collectionMode||'')">{{pointCollectionMode(point)}}</span></td>
            <td class="point-address-cell opcda-item-column"><input v-model.trim="point.itemId" class="mono" placeholder="Random.Real8" :disabled="readonly"/></td>
            <td class="opcda-type-column" :title="live(point)?.actualDataType"><span class="point-type-tag">{{dataTypeLabel(live(point)?.actualDataType)}}</span></td>
            <td class="opcda-access-column"><select v-model="point.access" :disabled="readonly"><option value="read">只读</option><option value="read_write">读写</option><option value="write">只写</option></select></td>
            <td class="opcda-flag-column"><select v-model="point.enabled" :disabled="readonly"><option :value="true">是</option><option :value="false">否</option></select></td>
            <td class="opcda-diagnostic-column" :class="{'bad-text':!!live(point)?.error}" :title="pointDiagnostic(point)">{{pointDiagnostic(point)}}</td>
            <td class="row-actions point-sheet-actions"><button @click="openAlarm(point)">告警</button><button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button><button :disabled="readonly" @click="duplicate(point)">复制</button><button class="danger" :disabled="readonly" @click="remove(point)">删除</button><small v-if="readonly&&lastWrite(point)" class="last-write-content" :class="{failed:!lastWrite(point).success}" :title="lastWrite(point).success?`上次下发成功：${lastWrite(point).value} · ${new Date(lastWrite(point).at).toLocaleString()}`:lastWrite(point).message">{{lastWrite(point).success?`上次：${lastWrite(point).value}`:'失败'}}</small></td>
            <td class="opcda-advanced-toggle-column"><button type="button" class="opcda-row-advanced-toggle" :aria-label="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" :title="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" @click="opcdaAdvancedOpen=!opcdaAdvancedOpen">...</button></td>
            <template v-if="opcdaAdvancedOpen">
              <td class="opcda-access-path-column"><input v-model.trim="point.accessPath" class="mono" :disabled="readonly"/></td>
              <td class="opcda-request-type-column"><select :title="point.requestedDataType" class="point-data-type-select" v-model="point.requestedDataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td>
              <td class="opcda-output-type-column"><select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly"><option v-for="type in dataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td>
              <td class="opcda-active-column"><select v-model="point.active" :disabled="readonly"><option :value="true">是</option><option :value="false">否</option></select></td>
              <td class="opcda-scale-column"><input v-model.number="point.scale" type="number" step="any" :disabled="readonly"/></td>
              <td class="opcda-offset-column"><input v-model.number="point.offset" type="number" step="any" :disabled="readonly"/></td>
              <td class="opcda-unit-column"><input v-model.trim="point.unit" :disabled="readonly"/></td>
              <td class="opcda-decimals-column"><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td>
            </template>
          </tr>
        </tbody>
      </table>
    </div>
    <PersistentHorizontalScrollbar v-if="tab==='points'"/>
    <DeviceConfigForm v-if="tab==='device'" :device="device" :readonly="readonly" :is-forward="isForward" @test="emit('test')" @read-address="emit('readAddress')" @remove="emit('removeDevice')" />
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
                      <code :title="point.dataType">{{dataTypeLabel(point.dataType)}}</code>
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
  <div v-if="writePoint" class="restart-modal"><div class="dialog-panel write-dialog-panel"><h3>点位下发</h3><p class="meta">{{writePoint.name||writePoint.metric}}</p><div v-if="writeDataType()==='bool'" class="bool-write-options"><label><input v-model="writeValue" type="radio" value="true"/><span>开启（true）</span></label><label><input v-model="writeValue" type="radio" value="false"/><span>关闭（false）</span></label></div><label v-else>目标值<input v-model="writeValue" @keydown.enter="confirmWrite"/></label><div v-if="writeFailure" class="point-write-result failed" role="alert"><strong>{{writeFailure.message}}</strong><small v-if="writeFailure.detail"><b>原始错误</b><code>{{writeFailure.detail}}</code></small></div><div class="dialog-actions"><button @click="writePoint=undefined;writeFailure=undefined">取消</button><button class="primary" :disabled="!canSubmitWrite()" @click="confirmWrite">确认下发</button></div></div></div>
  <PointValueDetailDialog v-if="valueDetailPoint" :point="valueDetailPoint" :status="live(valueDetailPoint)" @close="valueDetailPoint=undefined"/>
</template>
