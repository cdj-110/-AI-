<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useConfigStore } from '../stores/config'
import { confirmAction } from '../confirm'
import type { PointStatus } from '../types'
import DeviceConfigForm from './DeviceConfigForm.vue'

const props=defineProps<{device:any;channelName?:string;readonly:boolean;isForward:boolean}>()
const emit=defineEmits<{back:[];monitor:[];test:[];removeDevice:[]}>()
const store=useConfigStore()
const root=ref<HTMLElement>(), tab=ref<'device'|'points'>('points')
const search=ref(''), group=ref('1'), rows=ref<any[]>([]), total=ref(0), offset=ref(0)
const loading=ref(false), loadError=ref(''), liveByMetric=ref(new Map<string,PointStatus>())
const selected=ref<string[]>([]), mutating=ref(false), batchOpen=ref(false), deleteOpen=ref(false), deleteScope=ref<'filtered'|'device'>('filtered')
const editingMetric=ref(''), metricDraft=ref('')
const batchField=ref('name'), batchRange=ref('selected'), batchMode=ref('set'), batchValue=ref(''), batchStep=ref(1), batchPrefix=ref(''), batchSuffix=ref('')
const ROW_HEIGHT=34, WINDOW=160, ALIGN=80
let pageHost:HTMLElement|undefined, frame=0, requestVersion=0, searchTimer=0, statusTimer=0
const protocol=computed(()=>String(props.device?.protocol||'').toLowerCase())
const isModbus=computed(()=>protocol.value.startsWith('modbus'))
const isIEC104=computed(()=>['iec104','iec104-server'].includes(protocol.value))
const pointCount=computed(()=>Number(props.device?._pointCount||store.pointCounts[props.device?.deviceKey]||0))
const functionCode=computed(()=>isModbus.value?Number(group.value||1):0)
const topHeight=computed(()=>offset.value*ROW_HEIGHT)
const bottomHeight=computed(()=>Math.max(0,(total.value-offset.value-rows.value.length)*ROW_HEIGHT))
const selectedSet=computed(()=>new Set(selected.value))
const allWindowSelected=computed(()=>rows.value.length>0&&rows.value.every(point=>selectedSet.value.has(point.metric)))
const pointGroups=computed(()=>{
  if(isModbus.value)return[[1,'线圈 01'],[2,'离散输入 02'],[3,'保持寄存器 03'],[4,'输入寄存器 04']]
  return[]
})

async function loadWindow(nextOffset=0){
  const version=++requestVersion
  loading.value=true
  try{
    const result=await store.loadDevicePoints(props.device.deviceKey,nextOffset,WINDOW,search.value,functionCode.value)
    if(version!==requestVersion)return
    rows.value=result.points||[]
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
  mutating.value=true;loadError.value=''
  try{
    await store.mutateDevicePoints({deviceKey:props.device.deviceKey,action,metrics:selected.value,...extra})
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
    liveByMetric.value=new Map(points.map(point=>[point.metric,point]))
  }catch{}
}
function live(point:any){
  if(!props.readonly)return undefined
  const status=liveByMetric.value.get(point.metric)
  if(status?.error)return{...status,error:'-'}
  return status
}
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
  window.clearTimeout(searchTimer)
  searchTimer=window.setTimeout(()=>{offset.value=0;void loadWindow(0)},220)
}
function formatLive(point:any){const state=live(point);return state?.error||String(state?.value??'-')}

watch(()=>props.device?.deviceKey,()=>{group.value=isModbus.value?'1':'';offset.value=0;selected.value=[];cancelMetricEdit();void loadWindow(0)},{immediate:true})
watch(group,()=>{offset.value=0;void loadWindow(0)})
watch(search,queueSearch)
watch(()=>props.readonly,()=>void loadVisibleStatus())
onMounted(()=>nextTick(()=>{
  pageHost=root.value?.closest<HTMLElement>('.page')||undefined
  pageHost?.addEventListener('scroll',scheduleWindow,{passive:true})
  statusTimer=window.setInterval(loadVisibleStatus,1000)
}))
onBeforeUnmount(()=>{
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
      <button @click="emit('back')">返回通道</button>
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
        <button v-for="item in pointGroups" :key="item[0]" :class="{active:Number(group)===item[0]}" @click="group=String(item[0])">{{item[1]}}</button>
      </div>
      <div class="point-sheet">
        <div class="point-toolbar point-sheet-toolbar">
          <button :disabled="readonly||mutating" @click="mutate('add',{function:functionCode})">新增点位</button>
          <button :disabled="readonly||mutating||!selected.length" @click="duplicateSelected">复制选中</button>
          <button :disabled="readonly||mutating||!total" @click="openBatch">批量修改</button>
          <button class="danger" :disabled="readonly||mutating||!selected.length" @click="removeSelected">删除选中</button>
          <button class="danger" :disabled="readonly||mutating||!pointCount" @click="openDeleteAll">批量删除</button>
          <input v-model.trim="search" type="search" placeholder="搜索名称或标识符"/>
          <span class="selection-count muted">{{loading||mutating?'处理中…':`已选 ${selected.length} · 当前筛选 ${total.toLocaleString()} 条`}}</span>
          <button class="packet-monitor-button" @click="emit('monitor')">实时报文监控</button>
        </div>
        <p v-if="loadError" class="notice bad">{{loadError}}</p>
        <div class="table-wrap point-table-wrap point-sheet-scroll">
          <table class="legacy-point-table point-sheet-table large-point-table">
            <thead><tr><th class="point-sheet-select"><input type="checkbox" :checked="allWindowSelected" @change="toggleWindow"/></th><th class="point-sheet-index">#</th><th>名称</th><th>标识符</th><th>实时值</th><th v-if="isModbus">寄存器</th><th v-if="isIEC104">IOA</th><th>数据类型</th><th>倍率</th><th>偏移</th><th>单位</th><th>小数位</th></tr></thead>
            <tbody>
              <tr v-if="topHeight" class="large-point-spacer"><td :colspan="12" :style="{height:`${topHeight}px`}"></td></tr>
              <tr v-for="(point,index) in rows" :key="`${offset+index}:${point.metric}`" class="point-table-row">
                <td class="point-sheet-select"><input type="checkbox" :checked="selectedSet.has(point.metric)" @change="toggle(point.metric)"/></td>
                <td class="point-sheet-index">{{offset+index+1}}</td>
                <td class="point-name-cell"><input v-model.trim="point.name" :disabled="true"/></td>
                <td class="point-metric-cell">
                  <div v-if="editingMetric===point.metric" class="metric-inline-editor">
                    <input v-model="metricDraft" class="mono" :disabled="mutating" @keydown.enter.prevent="saveMetricEdit" @keydown.esc.prevent="cancelMetricEdit"/>
                    <button class="primary" :disabled="mutating" title="保存标识符" @click="saveMetricEdit">确定</button>
                    <button :disabled="mutating" title="取消修改" @click="cancelMetricEdit">取消</button>
                  </div>
                  <button v-else class="metric-inline-trigger mono" :disabled="readonly||mutating" title="点击修改标识符" @click="beginMetricEdit(point)">
                    <span>{{point.metric}}</span><i aria-hidden="true">✎</i>
                  </button>
                </td>
                <td class="point-live-cell point-live-value" :class="live(point)?.error?'bad-text':'ok-text'"><span>{{formatLive(point)}}</span></td>
                <td v-if="isModbus"><input :value="point.register??0" disabled/></td>
                <td v-if="isIEC104"><input :value="point.ioa??0" disabled/></td>
                <td><input :value="point.dataType||'-'" disabled/></td>
                <td><input :value="point.scale??1" disabled/></td>
                <td><input :value="point.offset??0" disabled/></td>
                <td><input :value="point.unit||''" disabled/></td>
                <td><input :value="point.decimals??0" disabled/></td>
              </tr>
              <tr v-if="bottomHeight" class="large-point-spacer"><td :colspan="12" :style="{height:`${bottomHeight}px`}"></td></tr>
              <tr v-if="!loading&&!rows.length"><td colspan="12" class="point-sheet-empty">暂无点位</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
    <DeviceConfigForm v-else :device="device" :readonly="readonly" :is-forward="isForward" @test="emit('test')" @remove="emit('removeDevice')"/>
  </section>
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
  <div v-if="deleteOpen" class="restart-modal"><div class="dialog-panel point-batch-dialog">
    <div><h3>批量删除点位</h3><p class="meta">由网关端直接删除，不会把全部点位加载到浏览器。删除后会立即保存并重新加载配置。</p></div>
    <div class="point-batch-grid">
      <label>删除范围
        <select v-model="deleteScope">
          <option value="filtered">当前筛选全部（{{total.toLocaleString()}} 个）</option>
          <option value="device">当前设备全部（{{pointCount.toLocaleString()}} 个）</option>
        </select>
      </label>
    </div>
    <p class="notice bad">此操作不可撤销，请确认删除范围。</p>
    <div class="dialog-actions"><button @click="deleteOpen=false">取消</button><button class="danger" @click="applyDeleteAll">确认删除</button></div>
  </div></div>
</template>
