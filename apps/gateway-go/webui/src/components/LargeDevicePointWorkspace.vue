<script setup lang="ts">
import { formatPointValueDisplay } from '../pointValueDisplay.ts'
import {firstPopulatedPointGroup} from '../pointGroupDefaults.ts'
import { dataTypeLabel } from '../dataTypeLabels.ts'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useConfigStore } from '../stores/config'
import { confirmAction } from '../confirm'
import {usePageDraftGuard} from '../composables/usePageDraftGuard.ts'
import { usePointTableInteractions } from '../composables/usePointTableInteractions.ts'
import { useResizableTableColumns } from '../composables/useResizableTableColumns.ts'
import type { PointStatus } from '../types'
import { recordPointStatuses } from '../pointHistory'
import DeviceConfigForm from './DeviceConfigForm.vue'
import PersistentHorizontalScrollbar from './PersistentHorizontalScrollbar.vue'
import PointValueDetailDialog from './PointValueDetailDialog.vue'
import ConfirmationDialog from './ConfirmationDialog.vue'

const props=defineProps<{device:any;channelName?:string;readonly:boolean;isForward:boolean;saving:boolean;writableMetrics?:string[]}>()
type WriteFailure={message:string;detail?:string;code?:string}
type WriteRecord={success:boolean;value:string;at:string;message?:string;detail?:string;code?:string}
const emit=defineEmits<{back:[];reload:[];save:[];collect:[];monitor:[];test:[];removeDevice:[];navigate:[];write:[point:any,value:string,done:(success:boolean,value?:unknown,failure?:WriteFailure)=>void]}>()
const store=useConfigStore()
const root=ref<HTMLElement>(), tab=ref<'device'|'points'>('points'),valueDetailPoint=ref<any>()
usePointTableInteractions(root,{onLiveValueClick:metric=>{valueDetailPoint.value=rows.value.find(point=>point.metric===metric)}})
const search=ref(''), group=ref('1'), rows=ref<any[]>([]), total=ref(0), offset=ref(0)
const loading=ref(false), loadError=ref(''), liveByMetric=ref(new Map<string,PointStatus>())
const selected=ref<string[]>([]), mutating=ref(false), batchOpen=ref(false), deleteOpen=ref(false), deleteScope=ref<'filtered'|'device'>('filtered')
const addOpen=ref(false), addItemId=ref(''), addAccessPath=ref(''), addAccess=ref('read'), addRequestedDataType=ref('auto')
const opcdaAdvancedOpen=ref(false)
const writePoint=ref<any>(), writeValue=ref(''), writeFailure=ref<WriteFailure>(), lastWrites=ref<Record<string,WriteRecord>>({})
const editingMetric=ref(''), metricDraft=ref('')
const batchField=ref('name'), batchRange=ref('selected'), batchMode=ref('set'), batchValue=ref(''), batchStep=ref(1), batchPrefix=ref(''), batchSuffix=ref('')
const rowBaselines=new WeakMap<object,string>()
const dirtyRows=()=>rows.value.filter(point=>rowBaselines.has(point)&&JSON.stringify(point)!==rowBaselines.get(point))
const hasPendingEdits=()=>dirtyRows().length>0||!!editingMetric.value&&metricDraft.value.trim()!==editingMetric.value
function discardPointEdits(){
  for(const point of dirtyRows()){const saved=JSON.parse(rowBaselines.get(point)!);for(const key of Object.keys(point))delete point[key];Object.assign(point,saved)}
  cancelMetricEdit()
}
async function savePendingRows(){
  if(mutating.value)return false
  mutating.value=true;loadError.value=''
  try{
    for(const point of dirtyRows()){
      const snapshot=JSON.stringify(point)
      await store.mutateDevicePoints({deviceKey:props.device.deviceKey,action:'update',point:JSON.parse(snapshot)})
      rowBaselines.set(point,snapshot)
    }
  }catch(cause){loadError.value=cause instanceof Error?cause.message:'点位保存失败';return false}
  finally{mutating.value=false}
  if(editingMetric.value&&metricDraft.value.trim()!==editingMetric.value)await saveMetricEdit()
  return !hasPendingEdits()
}
const requestPointLeave=usePageDraftGuard(hasPendingEdits,discardPointEdits,savePendingRows,'.large-device-workspace .device-tabs button, .large-device-workspace .point-groups button')
defineExpose({requestLeave:requestPointLeave,savePendingRows,hasPendingEdits})
const ROW_HEIGHT=34, WINDOW=160, ALIGN=80
let pageHost:HTMLElement|undefined, frame=0, requestVersion=0, searchTimer=0, statusTimer=0
let defaultSelectionVersion=0, initializingDefaultGroup=false
const protocol=computed(()=>String(props.device?.protocol||'').toLowerCase())
const isModbus=computed(()=>protocol.value.startsWith('modbus'))
const isIEC104=computed(()=>['iec104','iec104-server'].includes(protocol.value))
const isOPCDA=computed(()=>protocol.value==='opcda')
const pointCount=computed(()=>Number(props.device?._pointCount||store.pointCounts[props.device?.deviceKey]||0))
const functionCode=computed(()=>isModbus.value?Number(group.value||1):0)
const topHeight=computed(()=>offset.value*ROW_HEIGHT)
const bottomHeight=computed(()=>Math.max(0,(total.value-offset.value-rows.value.length)*ROW_HEIGHT))
const selectedSet=computed(()=>new Set(selected.value))
const allWindowSelected=computed(()=>rows.value.length>0&&rows.value.every(point=>selectedSet.value.has(point.metric)))
const opcdaColumnCount=computed(()=>opcdaAdvancedOpen.value?24:16)
const standardColumnCount=computed(()=>10+(isModbus.value||isIEC104.value?1:0)+(props.readonly?1:0))
const writableMetricSet=computed(()=>new Set(props.writableMetrics||[]))
const opcdaDataTypes=['auto','bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','datetime']
const {tableRef:opcdaTableRef,columnStyles:opcdaColumnStyles,tableStyle:opcdaTableStyle,onPointerMove:onColumnPointerMove,onPointerLeave:onColumnPointerLeave,onPointerDown:onColumnPointerDown,onDoubleClick:onColumnDoubleClick,onClick:onColumnClick,refreshColumnEffects}=useResizableTableColumns(opcdaColumnCount,{onLiveValueClick:metric=>{valueDetailPoint.value=rows.value.find(point=>point.metric===metric)}})
const pointGroups=computed(()=>{
  if(isModbus.value)return[[1,'线圈 01'],[2,'离散输入 02'],[3,'保持寄存器 03'],[4,'输入寄存器 04']]
  return[]
})

async function loadWindow(nextOffset=0){
  if(hasPendingEdits()&&!await requestPointLeave())return
  const version=++requestVersion
  loading.value=true
  try{
    const result=await store.loadDevicePoints(props.device.deviceKey,nextOffset,WINDOW,search.value,functionCode.value)
    if(version!==requestVersion)return
    rows.value=result.points||[]
    for(const point of rows.value)rowBaselines.set(point,JSON.stringify(point))
    offset.value=result.offset||0
    total.value=result.total||0
    loadError.value=''
    await loadVisibleStatus()
  }catch(cause){
    if(version===requestVersion)loadError.value=cause instanceof Error?cause.message:'点位读取失败'
  }finally{if(version===requestVersion)loading.value=false}
}
async function mutate(action:string,extra:Record<string,any>={}){
  if(mutating.value)return
  if(action!=='update'&&hasPendingEdits()&&!await requestPointLeave())return
  const savedPoint=extra.point?JSON.stringify(extra.point):''
  mutating.value=true;loadError.value=''
  try{
    await store.mutateDevicePoints({deviceKey:props.device.deviceKey,action,metrics:selected.value,...extra})
    if(savedPoint){const updated=rows.value.find(point=>point.metric===extra.point.metric);if(updated)rowBaselines.set(updated,savedPoint)}
    selected.value=[]
    await loadWindow(Math.min(offset.value,Math.max(0,pointCount.value-WINDOW)))
  }catch(cause){loadError.value=cause instanceof Error?cause.message:'点位操作失败'}
  finally{mutating.value=false}
}
function beginMetricEdit(point:any){
  if(props.readonly||mutating.value)return
  editingMetric.value=String(point.metric||'')
  metricDraft.value=editingMetric.value
  nextTick(()=>root.value?.querySelector<HTMLInputElement>('.metric-inline-editor input')?.select())
}
function cancelMetricEdit(){editingMetric.value='';metricDraft.value=''}
async function saveMetricEdit(){
  const oldMetric=editingMetric.value,newMetric=metricDraft.value.trim()
  if(!oldMetric)return
  if(!newMetric){loadError.value='标识符不能为空';return}
  if(newMetric===oldMetric){cancelMetricEdit();return}
  mutating.value=true;loadError.value=''
  try{
    await store.mutateDevicePoints({deviceKey:props.device.deviceKey,action:'rename',oldMetric,newMetric})
    selected.value=selected.value.map(metric=>metric===oldMetric?newMetric:metric)
    cancelMetricEdit()
    await loadWindow(offset.value)
  }catch(cause){loadError.value=cause instanceof Error?cause.message:'标识符修改失败'}
  finally{mutating.value=false}
}
function toggle(metric:string){selected.value=selectedSet.value.has(metric)?selected.value.filter(item=>item!==metric):[...selected.value,metric]}
function toggleWindow(){const metrics=rows.value.map(point=>point.metric);selected.value=allWindowSelected.value?selected.value.filter(item=>!metrics.includes(item)):[...new Set([...selected.value,...metrics])]}
async function removeSelected(){if(selected.value.length&&await confirmAction({title:'删除所选点位',message:`确认删除选中的 ${selected.value.length} 个点位？`,detail:'删除由网关端批量执行，操作完成后无法撤销。',confirmText:'确认删除',danger:true}))void mutate('delete')}
function duplicateSelected(){if(selected.value.length)void mutate('duplicate')}
function openAdd(){if(!isOPCDA.value){void mutate('add',{function:functionCode.value});return}addItemId.value='';addAccessPath.value='';addAccess.value='read';addRequestedDataType.value='auto';addOpen.value=true}
function applyAdd(){if(!addItemId.value.trim()){loadError.value='OPC DA Item ID 不能为空';return}addOpen.value=false;void mutate('add',{itemId:addItemId.value.trim(),accessPath:addAccessPath.value.trim(),access:addAccess.value,requestedDataType:addRequestedDataType.value})}
function saveAdvanced(point:any){if(!props.readonly&&!mutating.value)void mutate('update',{point:{...point}})}
function openDeleteAll(){deleteScope.value='filtered';deleteOpen.value=true}
function applyDeleteAll(){
  const extra=deleteScope.value==='device'
    ?{range:'all',search:'',function:0}
    :{range:'all',search:search.value,function:functionCode.value}
  deleteOpen.value=false
  void mutate('delete',extra)
}
function openBatch(){batchRange.value=selected.value.length?'selected':'all';batchOpen.value=true}
function applyBatch(){void mutate('batch',{range:batchRange.value,search:search.value,function:functionCode.value,field:batchField.value,mode:batchMode.value,value:batchValue.value,step:batchStep.value,prefix:batchPrefix.value,suffix:batchSuffix.value});batchOpen.value=false}
async function loadVisibleStatus(){
  if(!props.readonly||!rows.value.length){liveByMetric.value=new Map();return}
  const query=new URLSearchParams({deviceKey:props.device.deviceKey})
  for(const point of rows.value)if(point.metric)query.append('metric',point.metric)
  try{
    const response=await fetch('/api/point-status?'+query)
    if(!response.ok)return
    const points=await response.json() as PointStatus[]
    recordPointStatuses(points)
    liveByMetric.value=new Map(points.map(point=>[point.metric,point]))
  }catch{}
}
function live(point:any){
  if(!props.readonly)return undefined
  return liveByMetric.value.get(point.metric)
}
function formatLive(point:any){const state=live(point);if(state?.error||state?.value===undefined||state?.value===null)return'-';return formatPointValueDisplay(state.value,point.decimals,state.valueText)}
function pointQuality(point:any){const state=live(point);return state?.quality||(state?.error?'OFFLINE':'-')}
function formatTimestamp(value:unknown){
  if(!value)return'-'
  const date=new Date(String(value))
  if(Number.isNaN(date.getTime()))return String(value)
  const pad=(part:number,width=2)=>String(part).padStart(width,'0')
  return `${date.getFullYear()}-${pad(date.getMonth()+1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(),3)}`
}
function pointSourceTime(point:any){return formatTimestamp(live(point)?.sourceTimestamp)}
function pointCollectionTime(point:any){return formatTimestamp(live(point)?.receiveTimestamp||live(point)?.updatedAt)}
function pointCollectionMode(point:any){const state=live(point),mode=String(state?.collectionMode||'');if(mode==='subscription')return'异步订阅';if(mode==='sync_fallback'||String(state?.diagnostic||'').includes('IOPCSyncIO fallback'))return'同步回退';if(mode==='sync')return'同步读取';return'-'}
function pointDiagnostic(point:any){const state=live(point);if(state?.error)return state.error;if(state?.diagnostic)return state.diagnostic;if(state?.stale)return'数据陈旧';return state?.receiveTimestamp?'正常':'等待采集'}
function canWrite(point:any){return props.readonly&&!props.isForward&&writableMetricSet.value.has(String(point.metric))}
function writeDataType(point:any=writePoint.value){return String(live(point)?.actualDataType||point?.requestedDataType||point?.dataType||'').toLowerCase()}
function canSubmitWrite(){return !!writePoint.value&&(writeValue.value!==''||['string','bstr'].includes(writeDataType()))}
function lastWrite(point:any){return lastWrites.value[point.metric]}
function openWrite(point:any){writePoint.value=point;writeValue.value=String(live(point)?.valueText??live(point)?.value??'');const previous=lastWrite(point);writeFailure.value=previous&&!previous.success?{message:previous.message||'写入失败',detail:previous.detail,code:previous.code}:undefined}
function confirmWrite(){if(!canSubmitWrite())return;const point=writePoint.value,raw=writeValue.value;writeFailure.value=undefined;emit('write',point,raw,(success,value,failure)=>{const at=new Date().toISOString();if(!success){const current=failure||{message:'写入失败：设备未返回成功结果。'};writeFailure.value=current;lastWrites.value={...lastWrites.value,[point.metric]:{success:false,value:raw,at,...current}};return}lastWrites.value={...lastWrites.value,[point.metric]:{success:true,value:String(value??raw),at}};writeFailure.value=undefined;writePoint.value=undefined;writeValue.value='';void loadVisibleStatus()})}
function scheduleWindow(){
  if(frame)return
  frame=requestAnimationFrame(()=>{
    frame=0
    if(!pageHost||!root.value||tab.value!=='points')return
    const table=root.value.querySelector<HTMLElement>('.large-point-table')
    if(!table)return
    const tableTop=table.getBoundingClientRect().top-pageHost.getBoundingClientRect().top+pageHost.scrollTop+ROW_HEIGHT
    const visible=Math.max(0,Math.floor((pageHost.scrollTop-tableTop)/ROW_HEIGHT))
    const wanted=Math.max(0,Math.floor(Math.max(0,visible-30)/ALIGN)*ALIGN)
    if(wanted!==offset.value&&wanted<total.value)void loadWindow(wanted)
  })
}
function queueSearch(){
  if(initializingDefaultGroup)return
  window.clearTimeout(searchTimer)
  searchTimer=window.setTimeout(()=>{offset.value=0;void loadWindow(0)},220)
}
async function initializeDeviceView(){
  const version=++defaultSelectionVersion,deviceKey=props.device.deviceKey
  ++requestVersion
  initializingDefaultGroup=true;loading.value=true;loadError.value=''
  window.clearTimeout(searchTimer)
  search.value='';group.value=isModbus.value?'1':''
  tab.value=pointCount.value>0?'points':'device'
  rows.value=[];total.value=0;offset.value=0;selected.value=[];liveByMetric.value=new Map()
  opcdaAdvancedOpen.value=false;writePoint.value=undefined;writeValue.value='';writeFailure.value=undefined;lastWrites.value={};cancelMetricEdit()
  try{
    let initialGroup=group.value
    if(isModbus.value&&pointCount.value>0){
      // Query complete filtered totals; the first visible window may omit 01.
      const counts:Record<string,number>={}
      for(const key of ['1','2','3','4']){
        const result=await store.loadDevicePoints(deviceKey,0,1,'',Number(key))
        if(version!==defaultSelectionVersion)return
        counts[key]=result.total
        if(result.total>0)break
      }
      initialGroup=firstPopulatedPointGroup(['1','2','3','4'],counts)
    }
    if(version!==defaultSelectionVersion)return
    group.value=initialGroup
    initializingDefaultGroup=false
    await loadWindow(0)
  }catch(cause){
    if(version===defaultSelectionVersion){initializingDefaultGroup=false;loading.value=false;loadError.value=cause instanceof Error?cause.message:'点位读取失败'}
  }
}
function selectPointGroup(value:string){
  ++defaultSelectionVersion;initializingDefaultGroup=false
  if(group.value===value)void loadWindow(0)
  else group.value=value
}
watch(()=>[props.device,protocol.value],()=>void initializeDeviceView(),{immediate:true})
watch(tab,()=>emit('navigate'))
watch(group,()=>{if(!initializingDefaultGroup){offset.value=0;void loadWindow(0)}},{flush:'sync'})
watch(search,queueSearch)
watch(()=>props.readonly,()=>void loadVisibleStatus())
watch(opcdaAdvancedOpen,refreshColumnEffects)
watch(rows,refreshColumnEffects,{flush:'post'})
onMounted(()=>nextTick(()=>{
  pageHost=root.value?.closest<HTMLElement>('.page')||undefined
  pageHost?.addEventListener('scroll',scheduleWindow,{passive:true})
  statusTimer=window.setInterval(loadVisibleStatus,1000)
}))
onBeforeUnmount(()=>{
  ++defaultSelectionVersion;++requestVersion
  pageHost?.removeEventListener('scroll',scheduleWindow)
  if(frame)cancelAnimationFrame(frame)
  window.clearTimeout(searchTimer)
  window.clearInterval(statusTimer)
})
</script>

<template>
  <section ref="root" class="card legacy-device-workspace large-device-workspace" :class="{'is-running':readonly}">
    <div class="device-workspace-head">
      <div><h3>{{device.name||device.deviceKey}}</h3><p class="meta">所属通道：{{channelName||device.channelKey||'-'}} · 大数据分段模式</p></div>
      <div class="contextual-config-actions">
        <button @click="emit('back')">返回通道</button>
        <button @click="emit('reload')">重新加载</button>
        <button class="primary" :disabled="saving||readonly" @click="emit('save')">{{saving?'保存中…':'保存配置'}}</button>
      </div>
      <div class="device-tabs">
        <button :class="{active:tab==='device'}" @click="tab='device'">设备配置</button>
        <button :class="{active:tab==='points'}" @click="tab='points'">点位配置 ({{pointCount.toLocaleString()}})</button>
      </div>
    </div>
    <template v-if="tab==='points'">
      <div class="point-management-head">
        <div><h3>点位管理</h3><p class="meta">只加载当前可见区间，{{pointCount.toLocaleString()}} 个点位不会一次进入浏览器内存。</p></div>
      </div>
      <div v-if="pointGroups.length" class="point-groups">
        <button v-for="item in pointGroups" :key="item[0]" :class="{active:Number(group)===item[0]}" @click="selectPointGroup(String(item[0]))">{{item[1]}}</button>
      </div>
      <div class="point-sheet">
        <div class="point-toolbar point-sheet-toolbar">
          <button :disabled="readonly||mutating" @click="openAdd">新增点位</button>
          <button :disabled="readonly||mutating||!selected.length" @click="duplicateSelected">复制选中</button>
          <button :disabled="readonly||mutating||!total" @click="openBatch">批量修改</button>
          <button class="danger" :disabled="readonly||mutating||!selected.length" @click="removeSelected">删除选中</button>
          <button class="danger" :disabled="readonly||mutating||!pointCount" @click="openDeleteAll">批量删除</button>
          <input v-model.trim="search" type="search" placeholder="搜索名称或标识符"/>
          <span class="selection-count muted">{{loading||mutating?'处理中…':`已选 ${selected.length} · 当前筛选 ${total.toLocaleString()} 条`}}</span>
          <div class="point-runtime-actions">
            <button v-if="readonly" class="collect-now-button" @click="emit('collect')">立即采集</button>
            <button class="packet-monitor-button" @click="emit('monitor')">实时报文监控</button>
          </div>
        </div>
        <p v-if="loadError" class="notice bad">{{loadError}}</p>
        <div class="table-wrap point-table-wrap point-sheet-scroll" :class="{'opcda-point-table-wrap':isOPCDA,'opcda-advanced-open':isOPCDA&&opcdaAdvancedOpen}">
          <table ref="opcdaTableRef" class="legacy-point-table point-sheet-table large-point-table" :class="{'resizable-opcda-table':isOPCDA}" :style="isOPCDA?opcdaTableStyle:undefined" @pointermove="isOPCDA&&onColumnPointerMove($event)" @pointerleave="isOPCDA&&onColumnPointerLeave()" @pointerdown="isOPCDA&&onColumnPointerDown($event)" @dblclick="isOPCDA&&onColumnDoubleClick($event)" @click="isOPCDA&&onColumnClick($event)">
            <colgroup v-if="isOPCDA"><col v-for="(style,index) in opcdaColumnStyles" :key="index" :style="style"/></colgroup>
            <thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allWindowSelected" @change="toggleWindow"/></th><th class="point-sheet-index">#</th><th :class="{'opcda-name-column':isOPCDA}">名称</th><th :class="{'opcda-metric-column':isOPCDA}">标识符</th><th :class="{'opcda-value-column':isOPCDA}">实时值</th><th v-if="isOPCDA" class="opcda-quality-column">质量</th><th v-if="isOPCDA" class="opcda-time-column">源时间</th><th v-if="isOPCDA" class="opcda-collection-time-column">采集时间</th><th v-if="isOPCDA" class="opcda-collection-mode-column">采集方式</th><th v-if="isModbus">寄存器</th><th v-if="isIEC104">IOA</th><th v-if="isOPCDA" class="opcda-item-column">Item ID</th><th v-if="isOPCDA" class="opcda-type-column">实际类型</th><th v-if="isOPCDA" class="opcda-access-column">读写类型</th><th v-if="isOPCDA" class="opcda-flag-column">启用</th><th v-if="isOPCDA" class="opcda-diagnostic-column">状态/诊断</th><th v-if="isOPCDA" class="point-sheet-actions">操作</th><th v-if="isOPCDA" class="opcda-advanced-toggle-column"><button type="button" class="opcda-advanced-toggle" :aria-expanded="opcdaAdvancedOpen" :title="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" @click="opcdaAdvancedOpen=!opcdaAdvancedOpen"><span aria-hidden="true">{{opcdaAdvancedOpen?'◀':'▶'}}</span> 高级配置</button></th><template v-if="isOPCDA&&opcdaAdvancedOpen"><th class="opcda-access-path-column">Access Path</th><th class="opcda-request-type-column">请求类型</th><th class="opcda-output-type-column">输出类型</th><th class="opcda-active-column">激活</th><th class="opcda-scale-column">倍率</th><th class="opcda-offset-column">偏移</th><th class="opcda-unit-column">单位</th><th class="opcda-decimals-column">小数位</th></template><th v-if="!isOPCDA" class="point-data-type-column">数据类型</th><th v-if="!isOPCDA">倍率</th><th v-if="!isOPCDA">偏移</th><th v-if="!isOPCDA">单位</th><th v-if="!isOPCDA">小数位</th><th v-if="!isOPCDA&&readonly" class="point-sheet-actions">操作</th></tr></thead>
            <tbody>
              <tr v-if="topHeight" class="large-point-spacer"><td :colspan="isOPCDA?opcdaColumnCount:standardColumnCount" :style="{height:`${topHeight}px`}"></td></tr>
              <tr v-for="(point,index) in rows" :key="`${offset+index}:${point.metric}`" class="point-table-row" :data-row-key="`${device.deviceKey}::${point.metric}`">
                <td class="point-sheet-select"><input type="checkbox" :checked="selectedSet.has(point.metric)" @change="toggle(point.metric)"/></td>
                <td class="point-sheet-index">{{offset+index+1}}</td>
                <td class="point-name-cell" :class="{'opcda-name-column':isOPCDA}"><input v-model.trim="point.name" :title="point.name" :disabled="true"/></td>
                <td class="point-metric-cell" :class="{'opcda-metric-column':isOPCDA}">
                  <div v-if="editingMetric===point.metric" class="metric-inline-editor">
                    <input v-model="metricDraft" class="mono" :disabled="mutating" @keydown.enter.prevent="saveMetricEdit" @keydown.esc.prevent="cancelMetricEdit"/>
                    <button class="primary" :disabled="mutating" title="保存标识符" @click="saveMetricEdit">确定</button>
                    <button :disabled="mutating" title="取消修改" @click="cancelMetricEdit">取消</button>
                  </div>
                  <button v-else class="metric-inline-trigger mono" :disabled="readonly||mutating" :title="`标识符：${point.metric}（点击修改）`" @click="beginMetricEdit(point)">
                    <span>{{point.metric}}</span><i aria-hidden="true">✎</i>
                  </button>
                </td>
                <td class="point-live-cell point-live-value" :class="[live(point)?.error?'bad-text':'ok-text',{'opcda-value-column':isOPCDA}]"><span>{{formatLive(point)}}</span></td>
                <td v-if="isOPCDA" class="opcda-quality-column"><span class="opcda-quality" :class="String(pointQuality(point)).toLowerCase()">{{pointQuality(point)}}</span></td>
                <td v-if="isOPCDA" class="opcda-time-column mono" :title="String(live(point)?.sourceTimestamp||'-')">{{pointSourceTime(point)}}</td>
                <td v-if="isOPCDA" class="opcda-collection-time-column mono" :title="String(live(point)?.receiveTimestamp||live(point)?.updatedAt||'-')">{{pointCollectionTime(point)}}</td>
                <td v-if="isOPCDA" class="opcda-collection-mode-column"><span class="opcda-collection-mode" :class="String(live(point)?.collectionMode||'')">{{pointCollectionMode(point)}}</span></td>
                <td v-if="isModbus"><input :value="point.register??0" disabled/></td>
                <td v-if="isIEC104"><input :value="point.ioa??0" disabled/></td>
                <td v-if="isOPCDA" class="opcda-item-column"><input :value="point.itemId||''" class="mono" disabled/></td>
                <td v-if="isOPCDA" class="opcda-type-column" :title="live(point)?.actualDataType"><span class="point-type-tag">{{dataTypeLabel(live(point)?.actualDataType)}}</span></td>
                <td v-if="isOPCDA" class="opcda-access-column"><select v-model="point.access" :disabled="readonly"><option value="read">只读</option><option value="read_write">读写</option><option value="write">只写</option></select></td>
                <td v-if="isOPCDA" class="opcda-flag-column">{{point.enabled===false?'否':'是'}}</td>
                <td v-if="isOPCDA" class="opcda-diagnostic-column" :class="{'bad-text':!!live(point)?.error}" :title="pointDiagnostic(point)">{{pointDiagnostic(point)}}</td>
                <td v-if="isOPCDA" class="row-actions point-sheet-actions"><button v-if="readonly&&canWrite(point)" class="primary" @click="openWrite(point)">下发</button><button v-else-if="!readonly&&opcdaAdvancedOpen" :disabled="mutating" @click="saveAdvanced(point)">保存</button><span v-else class="muted">—</span><small v-if="readonly&&lastWrite(point)" class="last-write-content" :class="{failed:!lastWrite(point).success}" :title="lastWrite(point).success?`上次下发成功：${lastWrite(point).value}`:lastWrite(point).message">{{lastWrite(point).success?'成功':'失败'}}</small></td>
                <td v-if="isOPCDA" class="opcda-advanced-toggle-column"><button type="button" class="opcda-row-advanced-toggle" :aria-label="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" :title="opcdaAdvancedOpen?'收起高级配置':'展开高级配置'" @click="opcdaAdvancedOpen=!opcdaAdvancedOpen">...</button></td>
                <template v-if="isOPCDA&&opcdaAdvancedOpen">
                  <td class="opcda-access-path-column"><input v-model.trim="point.accessPath" class="mono" :disabled="readonly"/></td>
                  <td class="opcda-request-type-column"><select :title="point.requestedDataType" class="point-data-type-select" v-model="point.requestedDataType" :disabled="readonly"><option v-for="type in opcdaDataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td>
                  <td class="opcda-output-type-column"><select :title="point.dataType" class="point-data-type-select" v-model="point.dataType" :disabled="readonly"><option v-for="type in opcdaDataTypes" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></td>
                  <td class="opcda-active-column"><select v-model="point.active" :disabled="readonly"><option :value="true">是</option><option :value="false">否</option></select></td>
                  <td class="opcda-scale-column"><input v-model.number="point.scale" type="number" step="any" :disabled="readonly"/></td>
                  <td class="opcda-offset-column"><input v-model.number="point.offset" type="number" step="any" :disabled="readonly"/></td>
                  <td class="opcda-unit-column"><input v-model.trim="point.unit" :disabled="readonly"/></td>
                  <td class="opcda-decimals-column"><input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="readonly"/></td>
                </template>
                <td v-if="!isOPCDA" class="point-data-type-column"><input :value="dataTypeLabel(point.dataType)" :title="point.dataType" disabled/></td>
                <td v-if="!isOPCDA"><input :value="point.scale??1" disabled/></td>
                <td v-if="!isOPCDA"><input :value="point.offset??0" disabled/></td>
                <td v-if="!isOPCDA"><input :value="point.unit||''" disabled/></td>
                <td v-if="!isOPCDA"><input :value="point.decimals??0" disabled/></td>
                <td v-if="!isOPCDA&&readonly" class="row-actions point-sheet-actions"><button v-if="canWrite(point)" class="primary" @click="openWrite(point)">下发</button><span v-else class="muted">—</span><small v-if="lastWrite(point)" class="last-write-content" :class="{failed:!lastWrite(point).success}" :title="lastWrite(point).success?`上次下发成功：${lastWrite(point).value}`:lastWrite(point).message">{{lastWrite(point).success?'成功':'失败'}}</small></td>
              </tr>
              <tr v-if="bottomHeight" class="large-point-spacer"><td :colspan="isOPCDA?opcdaColumnCount:standardColumnCount" :style="{height:`${bottomHeight}px`}"></td></tr>
              <tr v-if="!loading&&!rows.length"><td :colspan="isOPCDA?opcdaColumnCount:standardColumnCount" class="point-sheet-empty">暂无点位</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
    <DeviceConfigForm v-else :device="device" :readonly="readonly" :is-forward="isForward" @test="emit('test')" @remove="emit('removeDevice')"/>
    <PersistentHorizontalScrollbar v-if="tab==='points'"/>
  </section>
  <div v-if="addOpen" class="point-batch-modal"><div class="point-batch-dialog">
    <div><h3>新增 OPC DA 点位</h3><p class="meta">Item ID 是 OPC DA 服务器内的项目标识；保存后由网关端直接加入当前大点表设备。</p></div>
    <div class="point-batch-grid"><label>Item ID<input v-model.trim="addItemId" class="mono" placeholder="例如 Random.Real8" @keydown.enter.prevent="applyAdd"/></label><label>Access Path<input v-model.trim="addAccessPath" class="mono"/></label><label>读写类型<select v-model="addAccess"><option value="read">只读</option><option value="read_write">读写</option><option value="write">只写</option></select></label><label>请求数据类型<select :title="addRequestedDataType" class="point-data-type-select" v-model="addRequestedDataType"><option v-for="type in ['auto','bool','int8','uint8','int16','uint16','int32','uint32','int64','uint64','float32','float64','string','datetime']" :key="type" :value="type" :title="type">{{dataTypeLabel(type)}}</option></select></label></div>
    <div class="dialog-actions"><button @click="addOpen=false">取消</button><button class="primary" @click="applyAdd">确认新增</button></div>
  </div></div>
  <div v-if="writePoint" class="restart-modal"><div class="dialog-panel write-dialog-panel"><h3>点位下发</h3><p class="meta">{{writePoint.name||writePoint.metric}}<template v-if="writePoint.name&&writePoint.name!==writePoint.metric"> · {{writePoint.metric}}</template><template v-if="writePoint.itemId"> · {{writePoint.itemId}}</template></p><div v-if="writeDataType()==='bool'" class="bool-write-options"><label><input v-model="writeValue" type="radio" value="true"/><span>开启（true）</span></label><label><input v-model="writeValue" type="radio" value="false"/><span>关闭（false）</span></label></div><label v-else>目标值<input v-model="writeValue" autofocus @keydown.enter="confirmWrite"/></label><div v-if="writeFailure" class="point-write-result failed" role="alert"><strong>{{writeFailure.message}}</strong><small v-if="writeFailure.detail"><b>原始错误</b><code>{{writeFailure.detail}}</code></small></div><div class="dialog-actions"><button @click="writePoint=undefined;writeFailure=undefined">取消</button><button class="primary" :disabled="!canSubmitWrite()" @click="confirmWrite">确认下发</button></div></div></div>
  <div v-if="batchOpen" class="point-batch-modal"><div class="point-batch-dialog">
    <div><h3>批量修改</h3><p class="meta">操作由网关端直接处理，不会把全部点位加载到浏览器。</p></div>
    <div class="point-batch-grid">
      <label>字段<select v-model="batchField"><option value="name">名称</option><option value="metric">标识符</option><option v-if="isModbus" value="register">寄存器</option><option value="dataType">数据类型</option><option v-if="isModbus" value="byteOrder">字节序</option><option value="scale">倍率</option><option value="offset">偏移</option><option value="unit">单位</option><option value="decimals">小数位</option><option v-if="isIEC104" value="ioa">IOA</option></select></label>
      <label>范围<select v-model="batchRange"><option value="selected" :disabled="!selected.length">仅选中点位</option><option value="all">当前筛选全部点位</option></select></label>
      <label>方式<select v-model="batchMode"><option value="set">统一设置</option><option value="increment">递增</option><option value="decrement">递减</option></select></label>
      <label>值<input v-model="batchValue"/></label><label v-if="batchMode!=='set'">步长<input v-model.number="batchStep" type="number"/></label>
      <label>固定开头（可选）<input v-model="batchPrefix"/></label><label>固定结尾（可选）<input v-model="batchSuffix"/></label>
    </div>
    <p v-if="batchField==='metric'" class="metric-batch-hint">标识符按全局唯一规则生成；递增示例：固定开头 <code>mbtcp-P</code>、起始值 <code>1</code>、步长 <code>1</code>。</p>
    <div class="dialog-actions"><button @click="batchOpen=false">取消</button><button class="primary" @click="applyBatch">应用</button></div>
  </div></div>
  <ConfirmationDialog
    :open="deleteOpen"
    title="批量删除点位"
    message="请选择需要删除的点位范围。"
    detail="由网关端直接删除，删除后会立即保存并重新加载配置，此操作不可撤销。"
    confirm-text="确认删除"
    danger
    panel-class="point-batch-dialog"
    @cancel="deleteOpen=false"
    @confirm="applyDeleteAll"
  >
    <div class="point-batch-grid">
      <label>删除范围
        <select v-model="deleteScope">
          <option value="filtered">当前筛选全部（{{total.toLocaleString()}} 个）</option>
          <option value="device">当前设备全部（{{pointCount.toLocaleString()}} 个）</option>
        </select>
      </label>
    </div>
  </ConfirmationDialog>
  <PointValueDetailDialog v-if="valueDetailPoint" :point="valueDetailPoint" :status="live(valueDetailPoint)" @close="valueDetailPoint=undefined"/>
</template>
