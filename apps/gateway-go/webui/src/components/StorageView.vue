<script setup lang="ts">
import {usePageDraftGuard} from '../composables/usePageDraftGuard.ts'
import RemoteStorageView from './RemoteStorageView.vue'
import {timeAxisFractions,contiguousSegments} from '../timeAxis.ts'
import { dataTypeLabel } from '../dataTypeLabels.ts'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { api } from '../api'
import { confirmAction } from '../confirm'
import { useConfigStore } from '../stores/config'

interface Device { device:string; mountPath:string; kind:string; totalBytes:number; freeBytes:number }
interface HistoryPointRef { deviceKey:string; metric:string }
interface HistoryTask {
  taskKey:string; name:string; enabled?:boolean; selectionMode:'all'|'selected'; points:HistoryPointRef[]
  saveMode:'interval'|'change'; saveIntervalSeconds:number; changeDeadband:number
  startAt?:string|null; endAt?:string|null
}
interface PointOption extends HistoryPointRef { name:string; dataType?:string; protocol?:string; unit?:string }
interface PointPage { offset:number; limit:number; total:number; points:PointOption[] }
interface HistoryTaskStat { taskKey:string; samples:number; estimatedBytes:number; lastWriteAt?:string }
interface HistoryExportJob { id:string; status:'running'|'completed'|'failed'; taskKey?:string; deviceKey?:string; metric?:string; start?:string; end?:string; fileName?:string; error?:string; createdAt:string; completedAt?:string }
interface AggregateStatus { enabled:boolean; hourlyRows:number; dailyRows:number; latestHourly?:string; latestDaily?:string; lastRunAt?:string }
interface BackupInfo { id:string; fileName:string; createdAt:string; bytes:number; sha256:string; encrypted:boolean; integrity:string }
interface IntegrityResult { mode:string; result:string; ok:boolean; checkedAt:string; durationMs:number }
interface HistoryMaintenanceStatus { database:StorageStatus; integrity:IntegrityResult; backups:BackupInfo[]; backupPath:string; backupEnabled:boolean; backupIntervalHours:number; backupKeep:number; encryptBackups:boolean; aggregates:AggregateStatus; integrityError?:string; backupError?:string; aggregateError?:string }
interface StorageStatus {
  pointGroups?:{deviceKey:string;name:string;protocol:string;count:number}[]
  configured:boolean; active:boolean; storagePath:string; storageType?:'local'|'removable'; storageError?:string; maxSizeMB:number; devices:Device[]; tasks?:HistoryTask[]
  databasePath?:string; bytes?:number; maxBytes?:number; samples?:number; legacyBytes?:number; legacyFiles?:number
  pendingSamples?:number; droppedSamples?:number; writable?:boolean; paused?:boolean; pauseReason?:string; totalBytes?:number; freeBytes?:number; minFreeBytes?:number; lastWriteAt?:string; lastError?:string; recoveryCount?:number; lastRecoveryPath?:string
  retentionDays?:number; taskStats?:HistoryTaskStat[]
  backupEnabled?:boolean; backupIntervalHours?:number; backupKeep?:number; backupPath?:string; encryptBackups?:boolean; aggregationEnabled?:boolean; aggregates?:AggregateStatus
}
interface HistoryRecord { id:number; time:string; deviceKey:string; metric:string; value:unknown; valueText:string; actualDataType?:string; quality?:string; sourceTimestamp?:string; receiveTimestamp?:string; collectionMode?:string; diagnostic?:string; resolution?:string; sampleCount?:number; minimum?:number; maximum?:number; average?:number }
interface HistoryQueryResult { offset:number; limit:number; total:number; resolution?:string; items:HistoryRecord[] }
interface HistoryChartPoint { at:string; value:number; x:number; y:number; item:HistoryRecord }

const props=defineProps<{canManage?:boolean}>()
const canManage=computed(()=>props.canManage===true)
const store=useConfigStore()
const savedHistoryDraft=ref('')
const requestHistoryLeave=usePageDraftGuard(()=>!!savedHistoryDraft.value&&(JSON.stringify(historyForm)!==savedHistoryDraft.value||taskEditorDirty()),()=>{Object.assign(historyForm,JSON.parse(savedHistoryDraft.value));closeDialog()},async()=>{if(taskEditorDirty()){commitTask();if(dialogOpen.value)return false}await saveHistory()},'#panel-history-storage .history-tabs button')
const history=ref<StorageStatus|null>(null)
const localHistoryPath='/userdata/weikong/data'
const historyForm=reactive<{enabled:boolean;maxSizeMB:number;storagePath:string;retentionDays:number;backupEnabled:boolean;backupIntervalHours:number;backupKeep:number;backupPath:string;encryptBackups:boolean;aggregationEnabled:boolean;tasks:HistoryTask[]}>({enabled:false,maxSizeMB:16,storagePath:localHistoryPath,retentionDays:0,backupEnabled:false,backupIntervalHours:24,backupKeep:3,backupPath:'current',encryptBackups:false,aggregationEnabled:false,tasks:[]})
const loading=ref(false), pointsLoading=ref(false), message=ref(''), dialogError=ref(''), dialogOpen=ref(false), editingIndex=ref(-1), nowTick=ref(Date.now())
const selectedDeviceKey=ref(''), pointSearch=ref('')
const pointPage=ref<PointPage>({offset:0,limit:100,total:0,points:[]})
const queryLoading=ref(false), queryError=ref(''), queryPointOptions=ref<PointOption[]>([])
const historyQuery=reactive({taskKey:'',deviceKey:'',metric:'',startLocal:'',endLocal:'',offset:0,limit:100})
const historyResult=ref<HistoryQueryResult>({offset:0,limit:100,total:0,items:[]})
const hoveredHistoryPoint=ref<HistoryChartPoint|null>(null)
const migrationTarget=ref(''), migrationLoading=ref(false), exportLoading=ref(false), exportJobs=ref<HistoryExportJob[]>([])
const maintenance=ref<HistoryMaintenanceStatus|null>(null), maintenanceLoading=ref(false), maintenanceAction=ref('')
type HistoryTab='tasks'|'query'|'exports'|'settings'|'maintenance'|'remote'
const remoteView=ref<InstanceType<typeof RemoteStorageView>|null>(null)
const activeHistoryTab=ref<HistoryTab>('tasks')
const draft=reactive<HistoryTask>({taskKey:'',name:'',enabled:true,selectionMode:'selected',points:[],saveMode:'interval',saveIntervalSeconds:1,changeDeadband:0,startAt:null,endAt:null})
const initialTaskEditor=ref('')
const taskEditorSnapshot=()=>JSON.stringify({draft,schedule})
const taskEditorDirty=()=>dialogOpen.value&&taskEditorSnapshot()!==initialTaskEditor.value
watch(dialogOpen,open=>{if(open)initialTaskEditor.value=taskEditorSnapshot()},{flush:'sync'})
const schedule=reactive<{startMode:'now'|'scheduled';startLocal:string;endMode:'never'|'at'|'duration';endLocal:string;durationHours:number}>({startMode:'now',startLocal:'',endMode:'never',endLocal:'',durationHours:24})
let statusTimer=0

const pointGroups=computed(()=>{
  if(history.value?.pointGroups)return history.value.pointGroups
  const cfg=store.value as any
  const physical=(cfg.devices||[]).map((device:any)=>({deviceKey:String(device.deviceKey||''),name:String(device.name||device.deviceKey||''),protocol:String(device.protocol||''),count:Number(device._pointCount??device.points?.length??store.pointCounts[device.deviceKey]??0)}))
  const derived=(cfg.edgeComputing?.groups||[]).map((group:any)=>({deviceKey:String(group.groupKey||''),name:String(group.name||group.groupKey||''),protocol:'edge-compute',count:Number(group.points?.length||0)}))
  const known=new Set([...physical,...derived].map(item=>item.deviceKey))
  const missing=draft.points.filter(point=>!known.has(point.deviceKey)).map(point=>({deviceKey:point.deviceKey,name:point.deviceKey,protocol:'',count:0}))
  return [...physical,...derived,...missing.filter((item,index,list)=>list.findIndex(other=>other.deviceKey===item.deviceKey)===index)].filter(item=>item.deviceKey)
})
const pointKey=(point:HistoryPointRef)=>`${point.deviceKey}\u0000${point.metric}`
const selectedKeys=computed(()=>new Set(draft.points.map(pointKey)))
const selectedForDevice=computed(()=>draft.points.filter(point=>point.deviceKey===selectedDeviceKey.value).length)
const pointPageNumber=computed(()=>Math.floor(pointPage.value.offset/pointPage.value.limit)+1)
const pointPageCount=computed(()=>Math.max(1,Math.ceil(pointPage.value.total/pointPage.value.limit)))
const taskSummary=computed(()=>({running:historyForm.tasks.filter(task=>taskStatus(task)==='running').length,pending:historyForm.tasks.filter(task=>taskStatus(task)==='pending').length,completed:historyForm.tasks.filter(task=>taskStatus(task)==='completed').length}))
const activeTasks=computed(()=>historyForm.tasks.map((task,index)=>({task,index})).filter(item=>taskStatus(item.task)!=='completed'))
const completedTasks=computed(()=>historyForm.tasks.map((task,index)=>({task,index})).filter(item=>taskStatus(item.task)==='completed'))
const taskStats=computed(()=>new Map((history.value?.taskStats||[]).map(item=>[item.taskKey,item])))
const migrationTargets=computed(()=>[
  {path:localHistoryPath,label:`本地存储 · ${localHistoryPath}`},
  ...(history.value?.devices||[]).map(device=>({path:device.mountPath,label:`${storageName(device)} · ${device.mountPath} · 可用 ${formatBytes(device.freeBytes)}`})),
].filter(item=>item.path!==historyForm.storagePath))
const historyPageNumber=computed(()=>Math.floor(historyResult.value.offset/Math.max(1,historyResult.value.limit))+1)
const historyPageCount=computed(()=>Math.max(1,Math.ceil(historyResult.value.total/Math.max(1,historyResult.value.limit))))
const selectedQueryPoint=computed(()=>queryPointOptions.value.find(point=>point.metric===historyQuery.metric))
const historyChart=computed(()=>{
  const ordered=[...historyResult.value.items].reverse()
  const samples=ordered.map(item=>({at:item.time,value:item.quality&&item.quality!=='GOOD'?undefined:historyNumericValue(item.value),item})).filter((sample):sample is {at:string;value:number;item:HistoryRecord}=>sample.value!==undefined&&Number.isFinite(Date.parse(sample.at)))
  if(!samples.length)return undefined
  let min=Math.min(...samples.map(item=>item.value)),max=Math.max(...samples.map(item=>item.value));if(min===max){const pad=Math.abs(min)*.05||1;min-=pad;max+=pad}
  const width=920,height=270,left=64,right=20,top=24,bottom=42,plotWidth=width-left-right,plotHeight=height-top-bottom,baseline=height-bottom
  const fractions=timeAxisFractions(ordered.map(item=>item.time))
  const usable=new Map(samples.map(item=>[item.item.id,item]))
  const candidates=ordered.map((record,index)=>{const item=usable.get(record.id);return !item||!Number.isFinite(fractions[index])?undefined:{x:left+fractions[index]*plotWidth,y:top+(max-item.value)/(max-min)*plotHeight,...item}})
  const points:HistoryChartPoint[]=candidates.filter((point):point is HistoryChartPoint=>!!point)
  const lines=contiguousSegments(candidates).map(segment=>segment.map(point=>`${point.x},${point.y}`).join(' '))
  const ticks=Array.from({length:5},(_,index)=>{const ratio=index/4;return{y:top+ratio*plotHeight,value:max-ratio*(max-min)}})
  const linePoints=points.map(point=>`${point.x},${point.y}`).join(' ')
  const areaPoints=lines.length===1&&points.length?`${points[0].x},${baseline} ${linePoints} ${points[points.length-1].x},${baseline}`:''
  return{width,height,left,right,top,bottom,baseline,points,ticks,linePoints,lines,areaPoints,start:samples[0]?.at,end:samples[samples.length-1]?.at}
})
const historyTooltipStyle=computed(()=>{const chart=historyChart.value,point=hoveredHistoryPoint.value;if(!chart||!point)return{};return{left:`${point.x/chart.width*100}%`,top:`${point.y/chart.height*100}%`}})
const historyTooltipFlipped=computed(()=>!!historyChart.value&&!!hoveredHistoryPoint.value&&hoveredHistoryPoint.value.x>historyChart.value.width*.72)
const formatBytes=(value:number|undefined)=>{const n=Number(value||0);if(n>=1073741824)return(n/1073741824).toFixed(2)+' GB';if(n>=1048576)return(n/1048576).toFixed(2)+' MB';if(n>=1024)return(n/1024).toFixed(1)+' KB';return n+' B'}
const isSelected=(point:HistoryPointRef)=>selectedKeys.value.has(pointKey(point))
const taskEnabled=(task:HistoryTask)=>task.enabled!==false
function storageName(device:Device){const source=`${device.kind||''} ${device.device||''}`.toLowerCase();if(/mmc|sd|tf/.test(source))return'TF卡';if(/usb|udisk/.test(source))return'U盘';return device.kind||'外部存储'}
function isLocalHistoryStorage(){return historyForm.storagePath===localHistoryPath}

function taskStatus(task:HistoryTask){
  const now=nowTick.value,start=task.startAt?new Date(task.startAt).getTime():0,end=task.endAt?new Date(task.endAt).getTime():0
  if(end&&now>=end)return'completed'
  if(!historyForm.enabled||!taskEnabled(task))return'disabled'
  if(start&&now<start)return'pending'
  return'running'
}
function statusText(task:HistoryTask){return{running:'运行中',pending:'待开始',completed:'已完成',disabled:'已停用'}[taskStatus(task)]}
function formatDate(value?:string|null){if(!value)return'永久';const date=new Date(value);return Number.isNaN(date.getTime())?'-':date.toLocaleString('zh-CN',{hour12:false})}
function toLocalInput(value?:string|null){if(!value)return'';const date=new Date(value);if(Number.isNaN(date.getTime()))return'';const local=new Date(date.getTime()-date.getTimezoneOffset()*60000);return local.toISOString().slice(0,16)}
function fromLocalInput(value:string){if(!value)return null;const date=new Date(value);return Number.isNaN(date.getTime())?null:date.toISOString()}
function newTaskKey(){return`history-${Date.now().toString(36)}-${Math.random().toString(36).slice(2,8)}`}
function historyNumericValue(value:unknown){if(typeof value==='boolean')return value?1:0;if(typeof value==='number'&&Number.isFinite(value))return value;return undefined}
function formatHistoryTime(value?:string){if(!value)return'-';const date=new Date(value);return Number.isNaN(date.getTime())?value:date.toLocaleString('zh-CN',{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,hour12:false})}
function formatChartTime(value?:string){if(!value)return'-';const date=new Date(value);return Number.isNaN(date.getTime())?value:date.toLocaleTimeString('zh-CN',{hour12:false,hour:'2-digit',minute:'2-digit',second:'2-digit'})}
function formatChartValue(value:number){return value.toLocaleString('zh-CN',{maximumFractionDigits:6})}
function historyMode(value?:string){return({poll:'同步读取',sync:'同步读取',subscription:'异步订阅',async_subscription:'异步订阅',sync_fallback:'同步回退',legacy:'兼容采集',aggregate_hour:'小时聚合',aggregate_day:'天聚合'} as Record<string,string>)[value||'']||value||'-'}
function taskError(task:HistoryTask){return taskStatus(task)==='running'&&history.value?.lastError?history.value.lastError:'正常'}
function updateHistoryChartHover(event:PointerEvent){
  const chart=historyChart.value,svg=event.currentTarget as SVGSVGElement
  if(!chart||!svg||!chart.points.length)return
  const rect=svg.getBoundingClientRect(),x=(event.clientX-rect.left)/Math.max(1,rect.width)*chart.width
  hoveredHistoryPoint.value=chart.points.reduce((nearest,point)=>Math.abs(point.x-x)<Math.abs(nearest.x-x)?point:nearest,chart.points[0])
}
function clearHistoryChartHover(){hoveredHistoryPoint.value=null}

async function load(){
  loading.value=true
  try{
    const data=await api<StorageStatus>('/api/history-storage')
    history.value=data
    Object.assign(historyForm,{enabled:data.configured,maxSizeMB:data.maxSizeMB||16,storagePath:data.storagePath||localHistoryPath,retentionDays:data.retentionDays||0,backupEnabled:data.backupEnabled===true,backupIntervalHours:data.backupIntervalHours||24,backupKeep:data.backupKeep||3,backupPath:data.backupPath||'current',encryptBackups:data.encryptBackups===true,aggregationEnabled:data.aggregationEnabled===true,tasks:(data.tasks||[]).map(task=>({...task,enabled:task.enabled!==false,points:[...(task.points||[])],selectionMode:task.selectionMode||'selected',saveMode:task.saveMode||'interval',saveIntervalSeconds:task.saveIntervalSeconds||1,changeDeadband:task.changeDeadband||0}))})
    savedHistoryDraft.value=JSON.stringify(historyForm)
    if(!migrationTargets.value.some(item=>item.path===migrationTarget.value))migrationTarget.value=migrationTargets.value[0]?.path||''
    message.value=''
  }catch(error){message.value=error instanceof Error?error.message:'历史存储状态读取失败'}finally{loading.value=false}
}
async function loadPointOptions(reset=true){
  if(!selectedDeviceKey.value){pointPage.value={offset:0,limit:100,total:0,points:[]};return}
  pointsLoading.value=true
  try{const offset=reset?0:pointPage.value.offset,query=new URLSearchParams({deviceKey:selectedDeviceKey.value,offset:String(offset),limit:String(pointPage.value.limit)});if(pointSearch.value.trim())query.set('search',pointSearch.value.trim());pointPage.value=await api<PointPage>('/api/history-storage/points?'+query)}
  catch(error){const text=error instanceof Error?error.message:'可选点位加载失败';if(dialogOpen.value)dialogError.value=text;else message.value=text}finally{pointsLoading.value=false}
}
async function loadQueryPointOptions(){
  queryPointOptions.value=[]
  if(!historyQuery.deviceKey){historyQuery.metric='';return}
  try{
    const query=new URLSearchParams({deviceKey:historyQuery.deviceKey,offset:'0',limit:'500'})
    const page=await api<PointPage>('/api/history-storage/points?'+query)
    queryPointOptions.value=page.points||[]
    if(!queryPointOptions.value.some(point=>point.metric===historyQuery.metric))historyQuery.metric=queryPointOptions.value[0]?.metric||''
  }catch(error){queryError.value=error instanceof Error?error.message:'查询点位加载失败'}
}
async function queryHistory(reset=true){
  if(!historyQuery.deviceKey||!historyQuery.metric){queryError.value='请选择设备和点位。';return}
  queryLoading.value=true;queryError.value='';hoveredHistoryPoint.value=null
  try{
    if(reset)historyQuery.offset=0
    const query=new URLSearchParams({deviceKey:historyQuery.deviceKey,metric:historyQuery.metric,offset:String(historyQuery.offset),limit:String(historyQuery.limit),resolution:'auto'});if(historyQuery.taskKey)query.set('taskKey',historyQuery.taskKey)
    const start=fromLocalInput(historyQuery.startLocal),end=fromLocalInput(historyQuery.endLocal);if(start)query.set('start',start);if(end)query.set('end',end)
    historyResult.value=await api<HistoryQueryResult>('/api/history-storage/query?'+query,{timeoutMs:30000})
  }catch(error){queryError.value=error instanceof Error?error.message:'历史数据查询失败'}finally{queryLoading.value=false}
}
function changeHistoryPage(direction:-1|1){const next=historyResult.value.offset+direction*historyResult.value.limit;if(next<0||next>=historyResult.value.total)return;historyQuery.offset=next;void queryHistory(false)}
function changePointPage(direction:-1|1){const next=pointPage.value.offset+direction*pointPage.value.limit;if(next<0||next>=pointPage.value.total)return;pointPage.value.offset=next;void loadPointOptions(false)}
function togglePoint(point:PointOption){const key=pointKey(point);draft.points=isSelected(point)?draft.points.filter(item=>pointKey(item)!==key):[...draft.points,{deviceKey:point.deviceKey,metric:point.metric}]}
function selectVisible(){const next=[...draft.points],known=new Set(next.map(pointKey));for(const point of pointPage.value.points)if(!known.has(pointKey(point))){next.push({deviceKey:point.deviceKey,metric:point.metric});known.add(pointKey(point))}draft.points=next}
function clearSelected(){draft.points=[]}

function resetDraft(){
  Object.assign(draft,{taskKey:newTaskKey(),name:`历史任务${historyForm.tasks.length+1}`,enabled:true,selectionMode:'selected',points:[],saveMode:'interval',saveIntervalSeconds:1,changeDeadband:0,startAt:null,endAt:null})
  Object.assign(schedule,{startMode:'now',startLocal:'',endMode:'never',endLocal:'',durationHours:24})
}
function openCreate(){editingIndex.value=-1;dialogError.value='';resetDraft();dialogOpen.value=true;if(pointGroups.value.length)selectedDeviceKey.value=pointGroups.value[0].deviceKey;void loadPointOptions(true)}
function openEdit(index:number){
  editingIndex.value=index
  dialogError.value=''
  const task=historyForm.tasks[index]
  Object.assign(draft,{...task,enabled:task.enabled!==false,points:[...(task.points||[])]})
  Object.assign(schedule,{startMode:task.startAt?'scheduled':'now',startLocal:toLocalInput(task.startAt),endMode:task.endAt?'at':'never',endLocal:toLocalInput(task.endAt),durationHours:24})
  dialogOpen.value=true
  selectedDeviceKey.value=draft.points[0]?.deviceKey||pointGroups.value[0]?.deviceKey||''
  void loadPointOptions(true)
}
function closeDialog(){dialogOpen.value=false;pointSearch.value='';pointPage.value={offset:0,limit:100,total:0,points:[]}}
function commitTask(){
  dialogError.value=''
  if(!draft.name.trim()){dialogError.value='请填写任务名称。';return}
  if(draft.selectionMode==='selected'&&!draft.points.length){dialogError.value='请至少选择一个历史点位。';return}
  const startAt=schedule.startMode==='scheduled'?fromLocalInput(schedule.startLocal):null
  if(schedule.startMode==='scheduled'&&!startAt){dialogError.value='请选择有效的开始时间。';return}
  let endAt:string|null=null
  if(schedule.endMode==='at')endAt=fromLocalInput(schedule.endLocal)
  if(schedule.endMode==='duration'){const hours=Number(schedule.durationHours);if(!Number.isFinite(hours)||hours<=0){dialogError.value='持续时长必须大于 0。';return}endAt=new Date((startAt?new Date(startAt).getTime():Date.now())+hours*3600000).toISOString()}
  if(schedule.endMode==='at'&&!endAt){dialogError.value='请选择有效的截止时间。';return}
  if(startAt&&endAt&&new Date(endAt).getTime()<=new Date(startAt).getTime()){dialogError.value='截止时间必须晚于开始时间。';return}
  const task:HistoryTask={...draft,name:draft.name.trim(),points:[...draft.points],startAt,endAt}
  if(editingIndex.value>=0)historyForm.tasks.splice(editingIndex.value,1,task);else historyForm.tasks.push(task)
  message.value='任务已更新，请点击“保存并应用”使配置生效。'
  closeDialog()
}
async function removeTask(index:number){const task=historyForm.tasks[index];if(!await confirmAction({title:'删除历史任务',message:`确认删除“${task.name}”？`,detail:'已保存的历史数据不会随任务删除，仍可导出或按容量策略淘汰。',confirmText:'确认删除',danger:true}))return;historyForm.tasks.splice(index,1);message.value='任务已删除，请保存配置。'}
async function saveHistory(){
  if(historyForm.enabled&&!historyForm.tasks.length){message.value='启用历史数据保存时，请至少创建一个任务。';return}
  for(const task of historyForm.tasks)if(task.selectionMode==='selected'&&!task.points.length){message.value=`任务“${task.name}”尚未选择点位。`;return}
  loading.value=true
  try{history.value=await api('/api/history-storage',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(historyForm)});message.value=historyForm.enabled?`历史数据配置已保存，共 ${historyForm.tasks.length} 个任务。`:'历史数据保存已关闭。';await load()}
  catch(error){message.value=error instanceof Error?error.message:'保存失败'}finally{loading.value=false}
}
async function clearHistory(){if(!await confirmAction({title:'清空历史数据',message:'确认清空全部历史数据？',detail:'该操作不会删除任务，但所有已保存采样不可恢复。',confirmText:'确认清空',danger:true}))return;try{await api('/api/history-storage',{method:'DELETE'});message.value='历史数据已清空。';await load()}catch(error){message.value=error instanceof Error?error.message:'清空失败'}}
async function migrateHistory(){
  if(!migrationTarget.value)return
  const target=migrationTargets.value.find(item=>item.path===migrationTarget.value)
  if(!await confirmAction({title:'迁移历史数据',message:`确认将历史数据库迁移到“${target?.label||migrationTarget.value}”？`,detail:'迁移期间会短暂停止历史写入，采集仍继续；完成后自动切换，原数据库保留为备份。',confirmText:'开始迁移'}))return
  migrationLoading.value=true
  try{const result=await api<{message?:string}>('/api/history-storage/migrate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({targetPath:migrationTarget.value}),timeoutMs:120000});message.value=result.message||'历史数据迁移完成。';await load()}catch(error){message.value=error instanceof Error?error.message:'迁移失败'}finally{migrationLoading.value=false}
}
async function loadExportJobs(){try{const result=await api<{jobs:HistoryExportJob[]}>('/api/history-storage/exports');exportJobs.value=result.jobs||[]}catch(error){message.value=error instanceof Error?error.message:'导出任务读取失败'}}
async function createHistoryExport(){
  exportLoading.value=true
  try{
    const start=fromLocalInput(historyQuery.startLocal),end=fromLocalInput(historyQuery.endLocal)
    await api('/api/history-storage/exports',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({taskKey:historyQuery.taskKey,deviceKey:historyQuery.deviceKey,metric:historyQuery.metric,start,end}),timeoutMs:30000})
    message.value='后台导出任务已创建，可继续使用页面。';await loadExportJobs()
  }catch(error){message.value=error instanceof Error?error.message:'创建导出任务失败'}finally{exportLoading.value=false}
}
function downloadExport(job:HistoryExportJob){window.location.assign('/api/history-storage/exports/download?id='+encodeURIComponent(job.id))}
async function loadMaintenance(){if(!historyForm.enabled)return;maintenanceLoading.value=true;try{maintenance.value=await api<HistoryMaintenanceStatus>('/api/history-storage/maintenance',{timeoutMs:120000})}catch(error){message.value=error instanceof Error?error.message:'数据库维护状态读取失败'}finally{maintenanceLoading.value=false}}
async function runMaintenance(action:'check'|'backup'|'aggregate'|'vacuum'|'restore'|'delete_backup',id='',full=false){
  const destructive=action==='restore'||action==='delete_backup'||action==='vacuum'
  if(destructive){const labels={restore:['恢复数据库','恢复会替换当前历史数据库，系统会先暂停历史写入并保留回滚文件。'],delete_backup:['删除备份','删除后无法通过该备份恢复。'],vacuum:['整理数据库','整理期间历史写入可能短暂停顿。']} as const;const [title,detail]=labels[action];if(!await confirmAction({title,message:title,detail,confirmText:'确认执行',danger:action!=='vacuum'}))return}
  maintenanceAction.value=action
  try{await api('/api/history-storage/maintenance',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action,id,full}),timeoutMs:180000});message.value=action==='restore'?'数据库恢复完成。':'数据库维护操作已完成。';await load();await loadMaintenance()}catch(error){message.value=error instanceof Error?error.message:'数据库维护操作失败'}finally{maintenanceAction.value=''}
}
function downloadBackup(item:BackupInfo){window.location.assign('/api/history-storage/backups/download?id='+encodeURIComponent(item.id))}
async function selectHistoryTab(tab:HistoryTab){
  if(activeHistoryTab.value==='remote'&&tab!=='remote'&&remoteView.value&&!(await remoteView.value.requestLeave()))return
  activeHistoryTab.value=tab
  if(tab==='exports')void loadExportJobs()
  if(tab==='maintenance')void loadMaintenance()
}

watch(selectedDeviceKey,()=>{pointSearch.value='';if(dialogOpen.value)void loadPointOptions(true)})
watch(()=>draft.selectionMode,mode=>{if(mode==='selected'&&!selectedDeviceKey.value&&pointGroups.value.length)selectedDeviceKey.value=pointGroups.value[0].deviceKey})
watch(()=>historyQuery.deviceKey,async()=>{await loadQueryPointOptions();historyResult.value={offset:0,limit:historyQuery.limit,total:0,items:[]}})
onMounted(async()=>{await load();const now=new Date(),before=new Date(now.getTime()-3600000);historyQuery.startLocal=toLocalInput(before.toISOString());historyQuery.endLocal=toLocalInput(now.toISOString());historyQuery.deviceKey=pointGroups.value[0]?.deviceKey||'';await loadQueryPointOptions();statusTimer=window.setInterval(()=>{nowTick.value=Date.now();if(exportJobs.value.some(job=>job.status==='running'))void loadExportJobs()},3000)})
onBeforeUnmount(()=>window.clearInterval(statusTimer))
async function guardedReload(){if(await requestHistoryLeave())await load()}
</script>

<template>
  <section id="panel-history-storage">
    <p v-if="message" class="notice">{{message}}</p>
    <p v-if="history?.droppedSamples" class="notice bad" role="alert">历史写入队列曾溢出，累计丢弃 {{history.droppedSamples}} 条采样；待写入 {{history.pendingSamples||0}} 条。请检查存储状态或降低保存频率。</p>
    <section class="card history-task-card" :class="{'history-disabled':!historyForm.enabled}">
      <div class="section-title history-page-head">
        <div><h2>历史数据</h2><p class="meta">按任务记录指定点位，查询、导出和数据库维护分别管理。</p></div>
        <div class="point-actions"><label class="edge-master-switch" :class="{checked:historyForm.enabled}" :aria-label="historyForm.enabled?'历史数据已启用':'历史数据已关闭'"><input :disabled="!canManage" v-model="historyForm.enabled" type="checkbox" role="switch" :aria-checked="historyForm.enabled"/><i class="edge-switch-track" aria-hidden="true"/><em>{{historyForm.enabled?'已启用':'已关闭'}}</em></label><button @click="guardedReload">重新加载</button><button v-if="canManage" class="primary" :disabled="loading" @click="saveHistory">{{loading?'保存中…':'保存并应用'}}</button><button v-if="canManage&&(historyForm.enabled&&activeHistoryTab==='tasks')" @click="openCreate">新增任务</button></div>
      </div>

      <nav class="history-tabs" role="tablist" aria-label="历史数据功能">
        <button role="tab" :aria-selected="activeHistoryTab==='tasks'" :class="{active:activeHistoryTab==='tasks'}" @click="selectHistoryTab('tasks')"><span>任务管理</span><b>{{activeTasks.length}}</b></button>
        <button role="tab" :aria-selected="activeHistoryTab==='query'" :class="{active:activeHistoryTab==='query'}" @click="selectHistoryTab('query')"><span>查询与趋势</span></button>
        <button role="tab" :aria-selected="activeHistoryTab==='exports'" :class="{active:activeHistoryTab==='exports'}" @click="selectHistoryTab('exports')"><span>导出任务</span><b v-if="exportJobs.length">{{exportJobs.length}}</b></button>
        <button v-if="canManage" role="tab" :aria-selected="activeHistoryTab==='settings'" :class="{active:activeHistoryTab==='settings'}" @click="selectHistoryTab('settings')"><span>存储设置</span></button>
        <button v-if="canManage" role="tab" :aria-selected="activeHistoryTab==='maintenance'" :class="{active:activeHistoryTab==='maintenance'}" @click="selectHistoryTab('maintenance')"><span>数据库维护</span></button>
        <button role="tab" :aria-selected="activeHistoryTab==='remote'" :class="{active:activeHistoryTab==='remote'}" @click="selectHistoryTab('remote')"><span>远程数据库</span></button>
      </nav>

      <template v-if="historyForm.enabled&&activeHistoryTab==='tasks'">
        <p v-if="history?.paused" class="history-storage-warning"><strong>历史写入已暂停</strong><span>{{history.pauseReason}}</span><small>采集不会停止；释放空间后历史写入会自动恢复。</small></p>
        <div class="history-summary"><div><span>任务总数</span><strong>{{historyForm.tasks.length}}</strong></div><div><span>运行中</span><strong class="running">{{taskSummary.running}}</strong></div><div><span>待开始</span><strong>{{taskSummary.pending}}</strong></div><div><span>已完成</span><strong>{{taskSummary.completed}}</strong></div></div>
        <div class="history-list-heading"><strong>当前任务</strong><span>{{activeTasks.length}} 个</span></div>
        <div class="history-task-table"><table><thead><tr><th>任务名称</th><th>点位范围</th><th>保存策略</th><th>已保存</th><th>占用空间</th><th>最后写入</th><th>状态/错误</th><th>操作</th><th>启用</th></tr></thead><tbody><tr v-if="!activeTasks.length"><td colspan="9" class="empty"><strong>尚无当前历史任务</strong><small>点击“新增任务”选择点位、保存策略和截止时间。</small></td></tr><tr v-for="item in activeTasks" :key="item.task.taskKey" :class="{disabled:!taskEnabled(item.task)}"><td><strong>{{item.task.name}}</strong><small class="mono">{{item.task.taskKey}}</small></td><td>{{item.task.selectionMode==='all'?'全部点位':`${item.task.points.length} 个指定点位`}}</td><td>{{item.task.saveMode==='change'?`变化保存 · 死区 ${item.task.changeDeadband}`:`每 ${item.task.saveIntervalSeconds} 秒`}}</td><td>{{taskStats.get(item.task.taskKey)?.samples||0}} 条</td><td>{{formatBytes(taskStats.get(item.task.taskKey)?.estimatedBytes)}}</td><td>{{formatHistoryTime(taskStats.get(item.task.taskKey)?.lastWriteAt)}}</td><td><span class="history-task-status" :class="taskStatus(item.task)">{{statusText(item.task)}}</span><small class="task-error" :class="{bad:taskError(item.task)!=='正常'}">{{taskError(item.task)}}</small></td><td><div class="table-actions"><button v-if="canManage" @click="openEdit(item.index)">编辑</button><button v-if="canManage" class="danger" @click="removeTask(item.index)">删除</button></div></td><td><label class="compact-switch" :class="{checked:taskEnabled(item.task)}"><input :disabled="!canManage" v-model="item.task.enabled" type="checkbox" role="switch"/><i/><span>{{taskEnabled(item.task)?'开':'关'}}</span></label></td></tr></tbody></table></div>
        <details v-if="completedTasks.length" class="history-archive"><summary>已完成任务 <b>{{completedTasks.length}}</b><span>截止时间到达后自动归档到此处</span></summary><div class="history-task-table"><table><thead><tr><th>任务名称</th><th>点位范围</th><th>保存策略</th><th>已保存</th><th>占用空间</th><th>最后写入</th><th>截止时间</th><th>操作</th></tr></thead><tbody><tr v-for="item in completedTasks" :key="item.task.taskKey"><td><strong>{{item.task.name}}</strong><small class="mono">{{item.task.taskKey}}</small></td><td>{{item.task.selectionMode==='all'?'全部点位':`${item.task.points.length} 个指定点位`}}</td><td>{{item.task.saveMode==='change'?'变化保存':`每 ${item.task.saveIntervalSeconds} 秒`}}</td><td>{{taskStats.get(item.task.taskKey)?.samples||0}} 条</td><td>{{formatBytes(taskStats.get(item.task.taskKey)?.estimatedBytes)}}</td><td>{{formatHistoryTime(taskStats.get(item.task.taskKey)?.lastWriteAt)}}</td><td>{{formatDate(item.task.endAt)}}</td><td><div class="table-actions"><button v-if="canManage" @click="openEdit(item.index)">查看/编辑</button><button v-if="canManage" class="danger" @click="removeTask(item.index)">删除</button></div></td></tr></tbody></table></div></details>
        <div class="history-quick-status"><div><span>SQLite 占用</span><strong>{{formatBytes(Math.max(0,(history?.bytes||0)-(history?.legacyBytes||0)))}}</strong><small>{{history?.samples||0}} 条采样</small></div><div><span>剩余空间</span><strong>{{history?.freeBytes?formatBytes(history.freeBytes):'-'}}</strong><small>保护线 {{formatBytes(history?.minFreeBytes)}}</small></div><div><span>最后写入</span><strong>{{formatHistoryTime(history?.lastWriteAt)}}</strong><small>{{history?.writable?'写入正常':history?.paused?'等待恢复':'当前不可写'}}</small></div><div><span>存储状态</span><strong>{{history?.active?'正常':'不可用'}}</strong><small>{{history?.storageType==='local'?'EG100 本地':'外部存储'}}</small></div></div>
      </template>
    </section>

    <RemoteStorageView v-if="activeHistoryTab==='remote'" ref="remoteView" :can-manage="canManage"/>
    <section v-if="historyForm.enabled&&activeHistoryTab==='settings'" class="card history-settings-card">
      <div class="section-title"><div><h2>存储设置</h2><p class="meta">设置容量、保存期限、存储位置、聚合和定时备份策略。</p></div></div>
      <div class="history-global-grid">
        <label>容量上限（MB）<input v-model.number="historyForm.maxSizeMB" type="number" min="16" max="32768"/><small>达到上限自动淘汰最旧数据。</small></label>
        <label>保存期限<select v-model.number="historyForm.retentionDays"><option :value="0">仅按容量淘汰</option><option :value="7">保留 7 天</option><option :value="30">保留 30 天</option><option :value="90">保留 90 天</option></select><small>期限和容量任一达到时，优先淘汰最旧数据。</small></label>
        <label><span>当前存储</span><select v-model="historyForm.storagePath" disabled><option :value="localHistoryPath">本地存储（默认） · {{localHistoryPath}}</option><option v-if="!isLocalHistoryStorage()&&!(history?.devices||[]).some(device=>device.mountPath===historyForm.storagePath)" :value="historyForm.storagePath">已配置外部存储（未挂载） · {{historyForm.storagePath}}</option><option v-for="device in history?.devices||[]" :key="device.mountPath" :value="device.mountPath">{{storageName(device)}} · {{device.mountPath}} · 可用 {{formatBytes(device.freeBytes)}}</option></select><small>使用“迁移数据库”安全切换位置。</small></label>
        <div class="history-migration"><label>迁移到<select v-model="migrationTarget" :disabled="!migrationTargets.length"><option value="">暂无其他可用存储</option><option v-for="target in migrationTargets" :key="target.path" :value="target.path">{{target.label}}</option></select></label><button v-if="canManage" :disabled="migrationLoading||!migrationTarget" @click="migrateHistory">{{migrationLoading?'迁移中…':'迁移数据库'}}</button></div>
        <div class="history-global-actions"><button v-if="canManage" class="danger" @click="clearHistory">清空历史</button></div>
      </div>
      <div class="history-feature-grid">
        <div class="feature-switch"><span><strong>小时/天聚合</strong><small>长时间趋势自动使用聚合数据</small></span><label class="compact-switch" :class="{checked:historyForm.aggregationEnabled}"><input v-model="historyForm.aggregationEnabled" type="checkbox" role="switch"/><i/><span>{{historyForm.aggregationEnabled?'开':'关'}}</span></label></div>
        <div class="feature-switch"><span><strong>定时备份</strong><small>一致性快照、校验并自动轮换</small></span><label class="compact-switch" :class="{checked:historyForm.backupEnabled}"><input v-model="historyForm.backupEnabled" type="checkbox" role="switch"/><i/><span>{{historyForm.backupEnabled?'开':'关'}}</span></label></div>
        <label>备份周期<select v-model.number="historyForm.backupIntervalHours" :disabled="!historyForm.backupEnabled"><option :value="6">每 6 小时</option><option :value="12">每 12 小时</option><option :value="24">每天</option><option :value="72">每 3 天</option><option :value="168">每 7 天</option></select></label>
        <label>保留份数<input v-model.number="historyForm.backupKeep" :disabled="!historyForm.backupEnabled" type="number" min="1" max="30"/></label>
        <label>备份位置<select v-model="historyForm.backupPath" :disabled="!historyForm.backupEnabled"><option value="current">当前历史存储</option><option :value="localHistoryPath">EG100 本地</option><option v-for="device in history?.devices||[]" :key="'backup-'+device.mountPath" :value="device.mountPath">{{storageName(device)}} · {{device.mountPath}}</option></select></label>
        <div class="feature-switch"><span><strong>加密备份</strong><small>AES-GCM 分块加密</small></span><label class="compact-switch" :class="{checked:historyForm.encryptBackups}"><input v-model="historyForm.encryptBackups" :disabled="!historyForm.backupEnabled" type="checkbox" role="switch"/><i/><span>{{historyForm.encryptBackups?'开':'关'}}</span></label></div>
      </div>
      <div class="history-storage-detail"><div><span>SQLite 数据库</span><code>{{history?.databasePath||'-'}}</code></div><div><span>容量策略</span><strong>{{history?.maxSizeMB||16}} MB · {{historyForm.retentionDays?`${historyForm.retentionDays} 天`:'仅容量淘汰'}}</strong></div><div><span>恢复记录</span><strong>{{history?.recoveryCount||0}} 次</strong><small :title="history?.lastRecoveryPath">{{history?.lastError||history?.lastRecoveryPath||'完整性正常'}}</small></div></div>
    </section>

    <section v-if="historyForm.enabled&&activeHistoryTab==='query'" class="card history-query-card">
      <div class="section-title"><div><h2>查询与趋势</h2><p class="meta">按任务、点位和时间范围查询 SQLite，并自动选择原始、小时或天聚合数据。</p></div><div class="point-actions"><button v-if="canManage" :disabled="exportLoading" @click="createHistoryExport">{{exportLoading?'创建中…':'后台导出当前条件'}}</button><button class="primary" :disabled="queryLoading" @click="queryHistory()">{{queryLoading?'查询中…':'查询'}}</button></div></div>
      <div class="history-query-filters"><label>任务<select v-model="historyQuery.taskKey"><option value="">全部任务</option><option v-for="task in historyForm.tasks" :key="task.taskKey" :value="task.taskKey">{{task.name}}</option></select></label><label>设备<select v-model="historyQuery.deviceKey"><option value="">请选择设备</option><option v-for="group in pointGroups" :key="group.deviceKey" :value="group.deviceKey">{{group.name}} · {{group.deviceKey}}</option></select></label><label>点位<input v-model.trim="historyQuery.metric" list="history-query-point-options" placeholder="选择或输入点位标识符"/><datalist id="history-query-point-options"><option v-for="point in queryPointOptions" :key="point.metric" :value="point.metric">{{point.name||point.metric}}</option></datalist></label><label>开始时间<input v-model="historyQuery.startLocal" type="datetime-local" step="1"/></label><label>结束时间<input v-model="historyQuery.endLocal" type="datetime-local" step="1"/></label></div>
      <p v-if="queryError" class="notice bad">{{queryError}}</p>
      <div class="history-query-chart">
        <div class="history-query-panel-head"><div><strong>变化趋势</strong><small>{{selectedQueryPoint?.name||historyQuery.metric||'未选择点位'}} · 当前页 {{historyResult.items.length}} 条</small></div><div class="history-chart-meta"><span v-if="selectedQueryPoint?.unit" class="history-unit-chip">{{selectedQueryPoint.unit}}</span><span>{{historyResult.resolution==='hour'?'小时聚合':historyResult.resolution==='day'?'天聚合':'原始数据'}} · 共 {{historyResult.total}} 条</span></div></div>
        <div v-if="historyChart" class="history-chart-canvas">
          <svg :viewBox="`0 0 ${historyChart.width} ${historyChart.height}`" preserveAspectRatio="none" role="img" :aria-label="`${selectedQueryPoint?.name||historyQuery.metric} 变化趋势`" @pointermove="updateHistoryChartHover" @pointerleave="clearHistoryChartHover">
            <g class="history-chart-grid" v-for="tick in historyChart.ticks" :key="tick.y"><line :x1="historyChart.left" :x2="historyChart.width-historyChart.right" :y1="tick.y" :y2="tick.y"/><text :x="historyChart.left-10" :y="tick.y+4">{{formatChartValue(tick.value)}}</text></g>
            <text v-if="selectedQueryPoint?.unit" :x="historyChart.left" :y="14" class="axis-unit">单位：{{selectedQueryPoint.unit}}</text>
            <polygon v-if="historyChart.areaPoints" class="history-chart-area" :points="historyChart.areaPoints"/>
            <polyline v-for="(line,index) in historyChart.lines" :key="index" class="history-chart-line-shadow" :points="line"/>
            <polyline v-for="(line,index) in historyChart.lines" :key="index" class="history-chart-line" :points="line"/>
            <circle v-for="point in historyChart.points" :key="point.at" class="history-chart-dot" :cx="point.x" :cy="point.y" r="2.5"/>
            <g v-if="hoveredHistoryPoint" class="history-chart-focus"><line class="vertical" :x1="hoveredHistoryPoint.x" :x2="hoveredHistoryPoint.x" :y1="historyChart.top" :y2="historyChart.baseline"/><line class="horizontal" :x1="historyChart.left" :x2="historyChart.width-historyChart.right" :y1="hoveredHistoryPoint.y" :y2="hoveredHistoryPoint.y"/><circle class="halo" :cx="hoveredHistoryPoint.x" :cy="hoveredHistoryPoint.y" r="8"/><circle class="core" :cx="hoveredHistoryPoint.x" :cy="hoveredHistoryPoint.y" r="4"/></g>
            <text :x="historyChart.left" :y="historyChart.height-12" class="time">{{formatChartTime(historyChart.start)}}</text><text :x="historyChart.width-historyChart.right" :y="historyChart.height-12" text-anchor="end" class="time">{{formatChartTime(historyChart.end)}}</text>
          </svg>
          <div v-if="hoveredHistoryPoint" class="history-chart-tooltip" :class="{flipped:historyTooltipFlipped}" :style="historyTooltipStyle">
            <div class="history-chart-tooltip-title"><strong>{{selectedQueryPoint?.name||historyQuery.metric}}</strong><code>{{historyQuery.metric}}</code></div>
            <div class="history-chart-tooltip-value"><strong>{{formatChartValue(hoveredHistoryPoint.value)}}</strong><span>{{selectedQueryPoint?.unit||'未配置单位'}}</span></div>
            <time>{{formatHistoryTime(hoveredHistoryPoint.at)}}</time>
          </div>
        </div>
        <div v-else class="history-query-empty">当前页没有可绘制的数值或布尔数据。</div>
      </div>
      <div class="history-query-table"><table><thead><tr><th>采集/时间桶</th><th>值/平均值</th><th>聚合统计</th><th>质量</th><th>实际类型</th><th>源时间</th><th>采集方式</th><th>诊断</th></tr></thead><tbody><tr v-if="!queryLoading&&!historyResult.items.length"><td colspan="8" class="empty">当前条件下暂无历史记录</td></tr><tr v-for="item in historyResult.items" :key="item.id||item.time"><td>{{formatHistoryTime(item.time)}}</td><td class="history-value" :title="item.valueText">{{item.valueText}}</td><td>{{item.resolution?`最小 ${item.minimum} · 最大 ${item.maximum} · ${item.sampleCount} 条`:'-'}}</td><td><span class="history-quality" :class="String(item.quality||'').toLowerCase()">{{item.quality||'-'}}</span></td><td :title="item.actualDataType">{{dataTypeLabel(item.actualDataType)}}</td><td>{{formatHistoryTime(item.sourceTimestamp)}}</td><td>{{historyMode(item.collectionMode)}}</td><td :title="item.diagnostic">{{item.diagnostic||'-'}}</td></tr></tbody></table></div>
      <div class="history-query-pagination"><span>第 {{historyPageNumber}} / {{historyPageCount}} 页 · 共 {{historyResult.total}} 条</span><button :disabled="historyResult.offset<=0||queryLoading" @click="changeHistoryPage(-1)">上一页</button><button :disabled="historyResult.offset+historyResult.limit>=historyResult.total||queryLoading" @click="changeHistoryPage(1)">下一页</button></div>
    </section>

    <section v-if="historyForm.enabled&&activeHistoryTab==='exports'" class="card history-export-card">
      <div class="section-title"><div><h2>导出任务</h2><p class="meta">后台生成大数据量文件，完成后可直接下载，不阻塞历史查询页面。</p></div><div class="point-actions"><button @click="selectHistoryTab('query')">新建导出</button><button :disabled="exportLoading" @click="loadExportJobs">刷新</button></div></div>
      <div v-if="exportJobs.length" class="history-export-jobs"><div v-for="job in exportJobs" :key="job.id" class="history-export-job"><div><strong>{{job.fileName||job.id}}</strong><small>{{job.taskKey?`任务：${historyForm.tasks.find(task=>task.taskKey===job.taskKey)?.name||job.taskKey}`:'全部任务'}} · {{formatHistoryTime(job.createdAt)}}</small></div><span class="history-task-status" :class="job.status==='completed'?'running':job.status==='failed'?'disabled':'pending'">{{job.status==='completed'?'已完成':job.status==='failed'?'失败':'生成中'}}</span><span class="export-error">{{job.error||''}}</span><button v-if="job.status==='completed'" @click="downloadExport(job)">下载</button></div></div>
      <div v-else class="history-tab-empty"><strong>暂无导出任务</strong><span>请先到“查询与趋势”设置范围，再创建后台导出。</span><button @click="selectHistoryTab('query')">前往查询</button></div>
    </section>

    <section v-if="historyForm.enabled&&activeHistoryTab==='maintenance'" class="card history-maintenance-card">
      <div class="section-title"><div><h2>数据库维护</h2><p class="meta">校验、备份、恢复和空间整理均使用一致性快照，不直接复制运行中的 SQLite 文件。</p></div><button :disabled="maintenanceLoading" @click="loadMaintenance">{{maintenanceLoading?'读取中…':'刷新'}}</button></div>
      <div class="maintenance-status-grid"><div><span>完整性</span><strong :class="maintenance?.integrity?.ok?'ok':'bad'">{{maintenance?.integrity?.ok?'正常':'待检查/异常'}}</strong><small>{{maintenance?.integrity?.mode||'quick'}} · {{maintenance?.integrity?.durationMs||0}} ms</small></div><div><span>备份目录</span><code>{{maintenance?.backupPath||'-'}}</code><small>{{maintenance?.backups?.length||0}} 份备份</small></div><div><span>小时聚合</span><strong>{{maintenance?.aggregates?.hourlyRows||0}} 条</strong><small>最新 {{formatHistoryTime(maintenance?.aggregates?.latestHourly)}}</small></div><div><span>天聚合</span><strong>{{maintenance?.aggregates?.dailyRows||0}} 条</strong><small>最新 {{formatHistoryTime(maintenance?.aggregates?.latestDaily)}}</small></div></div>
      <div class="maintenance-actions"><button v-if="canManage" :disabled="!!maintenanceAction" @click="runMaintenance('check')">快速校验</button><button v-if="canManage" :disabled="!!maintenanceAction" @click="runMaintenance('check','',true)">完整校验</button><button v-if="canManage" class="primary" :disabled="!!maintenanceAction" @click="runMaintenance('backup')">立即备份</button><button v-if="canManage" :disabled="!!maintenanceAction||!historyForm.aggregationEnabled" @click="runMaintenance('aggregate')">立即聚合</button><button v-if="canManage" :disabled="!!maintenanceAction" @click="runMaintenance('vacuum')">整理空间</button><span v-if="maintenanceAction">正在执行：{{maintenanceAction}}</span></div>
      <div class="backup-table"><table><thead><tr><th>创建时间</th><th>文件</th><th>大小</th><th>校验</th><th>加密</th><th>操作</th></tr></thead><tbody><tr v-if="!maintenance?.backups?.length"><td colspan="6" class="empty">暂无数据库备份</td></tr><tr v-for="item in maintenance?.backups||[]" :key="item.id"><td>{{formatHistoryTime(item.createdAt)}}</td><td class="mono">{{item.fileName}}</td><td>{{formatBytes(item.bytes)}}</td><td><span class="history-quality good">{{item.integrity}}</span><small class="backup-hash" :title="item.sha256">{{item.sha256.slice(0,12)}}…</small></td><td>{{item.encrypted?'AES-GCM':'否'}}</td><td><div class="table-actions"><button @click="downloadBackup(item)">下载</button><button v-if="canManage" @click="runMaintenance('restore',item.id)">恢复</button><button v-if="canManage" class="danger" @click="runMaintenance('delete_backup',item.id)">删除</button></div></td></tr></tbody></table></div>
    </section>

    <div v-if="dialogOpen" class="history-task-modal" @mousedown.self="closeDialog">
      <section class="dialog-panel" role="dialog" aria-modal="true" aria-labelledby="history-task-title">
        <div class="dialog-head"><div><h3 id="history-task-title">{{editingIndex>=0?'编辑历史任务':'新增历史任务'}}</h3><p>选择需要记录的点位、保存策略和任务运行时间。</p></div><button class="dialog-close" aria-label="关闭" @click="closeDialog">×</button></div>
        <div class="history-task-dialog-body">
          <p v-if="dialogError" class="notice bad history-dialog-error">{{dialogError}}</p>
          <div class="history-task-basics"><label>任务名称<input v-model.trim="draft.name" maxlength="64" placeholder="例如：OPC DA 压力测试"/></label><label>保存范围<select v-model="draft.selectionMode"><option value="selected">指定点位</option><option value="all">全部点位</option></select></label><label>保存策略<select v-model="draft.saveMode"><option value="interval">按周期保存</option><option value="change">变化时保存</option></select></label><label v-if="draft.saveMode==='interval'">保存周期（秒）<input v-model.number="draft.saveIntervalSeconds" type="number" min="1" max="86400"/></label><label v-else>数值变化死区<input v-model.number="draft.changeDeadband" type="number" min="0" step="any"/></label></div>

          <div class="history-schedule"><div><strong>开始时间</strong><label><input v-model="schedule.startMode" type="radio" value="now"/>保存后立即开始</label><label><input v-model="schedule.startMode" type="radio" value="scheduled"/>指定时间</label><input v-if="schedule.startMode==='scheduled'" v-model="schedule.startLocal" type="datetime-local"/></div><div><strong>截止时间</strong><label><input v-model="schedule.endMode" type="radio" value="never"/>永久运行</label><label><input v-model="schedule.endMode" type="radio" value="at"/>指定时间</label><label><input v-model="schedule.endMode" type="radio" value="duration"/>持续时长</label><input v-if="schedule.endMode==='at'" v-model="schedule.endLocal" type="datetime-local"/><label v-if="schedule.endMode==='duration'" class="duration-field"><input v-model.number="schedule.durationHours" type="number" min="0.1" step="0.5"/>小时</label></div></div>

          <div v-if="draft.selectionMode==='selected'" class="history-point-picker"><div class="history-point-picker-head"><div><strong>选择历史点位</strong><span>已选择 {{draft.points.length}} 个，任务之间重复的点位只保存一份。</span></div><div><button :disabled="!draft.points.length" @click="clearSelected">清空选择</button></div></div><div class="history-point-picker-body"><aside><button v-for="group in pointGroups" :key="group.deviceKey" :class="{active:selectedDeviceKey===group.deviceKey}" @click="selectedDeviceKey=group.deviceKey"><span>{{group.name}}</span><small>{{group.protocol||'采集设备'}} · {{group.count}} 个点位</small><b v-if="draft.points.some(point=>point.deviceKey===group.deviceKey)">{{draft.points.filter(point=>point.deviceKey===group.deviceKey).length}}</b></button><p v-if="!pointGroups.length" class="empty-mini">暂无可选设备</p></aside><div class="history-point-list"><div class="history-point-search"><input v-model.trim="pointSearch" placeholder="搜索点位名称或标识符" @keyup.enter="loadPointOptions(true)"/><button :disabled="pointsLoading" @click="loadPointOptions(true)">查询</button><button :disabled="!pointPage.points.length" @click="selectVisible">选择当前页</button><span>本设备已选 {{selectedForDevice}} 个</span></div><div class="history-point-table"><table><thead><tr><th></th><th>名称</th><th>标识符</th><th class="point-data-type-column">数据类型</th></tr></thead><tbody><tr v-if="!pointsLoading&&!pointPage.points.length"><td colspan="4" class="empty">暂无匹配点位</td></tr><tr v-for="point in pointPage.points" :key="pointKey(point)" :class="{selected:isSelected(point)}" @click="togglePoint(point)"><td><input type="checkbox" :checked="isSelected(point)" @click.stop @change="togglePoint(point)"/></td><td>{{point.name||point.metric}}</td><td class="mono">{{point.metric}}</td><td :title="point.dataType">{{dataTypeLabel(point.dataType)}}</td></tr></tbody></table></div><div class="history-point-pagination"><span>共 {{pointPage.total}} 个 · 第 {{pointPageNumber}} / {{pointPageCount}} 页</span><button :disabled="pointPage.offset<=0||pointsLoading" @click="changePointPage(-1)">上一页</button><button :disabled="pointPage.offset+pointPage.limit>=pointPage.total||pointsLoading" @click="changePointPage(1)">下一页</button></div></div></div></div>
        </div>
        <div class="dialog-actions"><label class="edge-master-switch" :class="{checked:draft.enabled}" :aria-label="draft.enabled?'任务已启用':'任务已关闭'"><input v-model="draft.enabled" type="checkbox" role="switch"/><i class="edge-switch-track" aria-hidden="true"/><em>{{draft.enabled?'已启用':'已关闭'}}</em></label><span/><button @click="closeDialog">取消</button><button class="primary" @click="commitTask">确定</button></div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.history-task-card{min-width:0}.history-page-head{margin-bottom:12px}.history-tabs{display:flex;align-items:center;gap:4px;margin:0 -18px -18px;padding:0 18px;border-top:1px solid #edf2f7;background:#fbfdff}.history-tabs button{display:inline-flex;height:44px;align-items:center;gap:7px;padding:0 15px;border:0;border-bottom:2px solid transparent;border-radius:0;color:#64748b;background:transparent;font-weight:700}.history-tabs button:hover{color:#1d4ed8;background:#f3f8ff}.history-tabs button.active{border-bottom-color:#1677ff;color:#1677ff;background:#fff}.history-tabs button b{min-width:20px;padding:2px 6px;border-radius:999px;color:#64748b;background:#e2e8f0;font-size:10px}.history-tabs button.active b{color:#1d4ed8;background:#dbeafe}.history-global-grid{display:grid;grid-template-columns:150px 180px minmax(260px,1fr) minmax(300px,1.1fr) auto;align-items:end;gap:10px;padding:13px;border:1px solid #e4ebf4;border-radius:11px;background:#f8fbff}.history-global-grid>label,.history-task-basics label,.history-migration label{display:grid;gap:5px;color:#64748b;font-size:12px;font-weight:700}.history-migration{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:6px}.history-global-actions{display:flex;justify-content:flex-end;gap:6px}.history-summary{display:grid;grid-template-columns:repeat(4,minmax(120px,1fr));gap:10px;margin:18px 0 12px}.history-summary>div{padding:12px 14px;border:1px solid #e7edf5;border-radius:10px;background:#fff}.history-summary span,.history-summary strong{display:block}.history-summary span{color:#64748b;font-size:11px}.history-summary strong{margin-top:5px;color:#0f172a;font-size:20px}.history-summary strong.running{color:#15803d}.history-list-heading{display:flex;align-items:center;gap:8px;margin:12px 0 7px}.history-list-heading>span{color:#64748b;font-size:11px}.history-list-heading>button{margin-left:auto}.history-task-table{overflow:auto;border:1px solid #e1e8f2;border-radius:11px}.history-task-table table{min-width:1280px}.history-task-table th,.history-task-table td{padding:9px 10px}.history-task-table td:first-child strong,.history-task-table td:first-child small{display:block}.history-task-table td:first-child small{margin-top:3px;color:#94a3b8}.history-task-table tbody tr.disabled td{color:#94a3b8;background:#f8fafc}.history-task-table .empty strong,.history-task-table .empty small{display:block}.history-task-table .empty small{margin-top:6px}.history-task-status{display:inline-flex;align-items:center;padding:3px 8px;border-radius:999px;font-size:11px;font-weight:700}.history-task-status.running{color:#15803d;background:#dcfce7}.history-task-status.pending{color:#1d4ed8;background:#dbeafe}.history-task-status.completed{color:#475569;background:#e2e8f0}.history-task-status.disabled{color:#64748b;background:#f1f5f9}.task-error{display:block;max-width:190px;margin-top:4px;overflow:hidden;color:#64748b;text-overflow:ellipsis;white-space:nowrap}.task-error.bad{color:#b91c1c}.history-archive{margin-top:12px;border:1px solid #e1e8f2;border-radius:11px;background:#f8fafc}.history-archive summary{display:flex;align-items:center;gap:8px;padding:11px 13px;cursor:pointer;font-weight:700}.history-archive summary b{min-width:22px;padding:2px 6px;border-radius:999px;color:#475569;background:#e2e8f0;text-align:center;font-size:10px}.history-archive summary span{margin-left:auto;color:#64748b;font-size:10px;font-weight:500}.history-archive .history-task-table{margin:0 8px 8px;background:#fff}.history-quick-status{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px;margin-top:12px}.history-quick-status>div{display:grid;gap:4px;padding:10px 12px;border:1px solid #e7edf5;border-radius:9px;background:#f8fafc}.history-quick-status span,.history-quick-status small{color:#64748b;font-size:10px}.history-quick-status strong{overflow:hidden;color:#0f172a;font-size:13px;text-overflow:ellipsis;white-space:nowrap}.history-settings-card,.history-export-card{min-width:0;margin-top:12px}.history-storage-detail{display:grid;grid-template-columns:2fr 1fr 1fr;gap:9px;margin-top:10px}.history-storage-detail>div{display:grid;gap:5px;padding:11px 12px;border:1px solid #e4ebf4;border-radius:10px;background:#f8fafc}.history-storage-detail span,.history-storage-detail small{color:#64748b;font-size:10px}.history-storage-detail code,.history-storage-detail strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.history-tab-empty{display:grid;min-height:230px;place-items:center;align-content:center;gap:8px;color:#64748b;text-align:center}.history-tab-empty strong{color:#334155;font-size:15px}.history-tab-empty span{font-size:11px}.history-tab-empty button{margin-top:4px}.table-actions{display:flex;gap:5px}.table-actions button{height:30px}.compact-switch{display:inline-flex;align-items:center;gap:5px;cursor:pointer}.compact-switch input{position:absolute;opacity:0;pointer-events:none}.compact-switch i{position:relative;width:34px;height:18px;border-radius:999px;background:#cbd5e1}.compact-switch i::after{position:absolute;top:3px;left:3px;width:12px;height:12px;border-radius:50%;background:#fff;box-shadow:0 1px 3px #64748b;content:"";transition:.18s}.compact-switch.checked i{background:#22c55e}.compact-switch.checked i::after{transform:translateX(16px)}.compact-switch span{font-size:11px}.history-task-modal{position:fixed;z-index:150;inset:0;display:grid;place-items:center;padding:22px;background:rgba(15,23,42,.45)}.history-task-modal .dialog-panel{display:grid;grid-template-rows:auto minmax(0,1fr) auto;width:min(1180px,96vw);max-height:92vh;overflow:hidden;border:1px solid #dbe3ee;border-radius:15px;background:#fff;box-shadow:0 28px 80px rgba(15,23,42,.28)}.history-task-modal .dialog-head{padding:16px 18px;border-bottom:1px solid #e7edf5}.dialog-close{width:34px;height:34px;padding:0;border:0;background:transparent;font-size:24px}.history-task-dialog-body{overflow:auto;padding:16px 18px}.history-task-basics{display:grid;grid-template-columns:1.4fr repeat(3,1fr);gap:10px}.history-schedule{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin:14px 0;padding:13px;border:1px solid #e5ebf3;border-radius:11px;background:#f8fafc}.history-schedule>div{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.history-schedule strong{width:100%;color:#334155;font-size:12px}.history-schedule label{display:flex;align-items:center;gap:5px;color:#475569;font-size:12px}.history-schedule label input[type=radio]{width:auto;min-width:0;height:auto}.history-schedule>div>input[type=datetime-local]{min-width:220px}.duration-field input{width:110px}.history-point-picker{overflow:hidden;border:1px solid #dbe5f1;border-radius:11px;background:#fff}.history-point-picker-head{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:11px 13px;border-bottom:1px solid #e7edf5;background:#f8fafc}.history-point-picker-head>div{display:flex;align-items:center;gap:10px}.history-point-picker-head strong{color:#0f172a;font-size:13px}.history-point-picker-head span{color:#64748b;font-size:11px}.history-point-picker-head button{height:30px}.history-point-picker-body{display:grid;grid-template-columns:230px minmax(0,1fr);min-height:300px}.history-point-picker aside{padding:8px;border-right:1px solid #e7edf5;background:#fbfdff}.history-point-picker aside button{position:relative;display:grid;width:100%;min-height:48px;margin-bottom:5px;padding:7px 38px 7px 9px;border-color:transparent;text-align:left}.history-point-picker aside button:hover,.history-point-picker aside button.active{border-color:#bfdbfe;background:#eff6ff}.history-point-picker aside button span,.history-point-picker aside button small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.history-point-picker aside button span{color:#334155;font-size:12px;font-weight:700}.history-point-picker aside button small{color:#94a3b8;font-size:10px}.history-point-picker aside button b{position:absolute;top:50%;right:9px;min-width:22px;padding:2px 5px;border-radius:999px;color:#1677ff;background:#dbeafe;text-align:center;font-size:10px;transform:translateY(-50%)}.history-point-list{display:grid;grid-template-rows:auto minmax(0,1fr) auto;min-width:0}.history-point-search{display:flex;align-items:center;gap:6px;padding:8px;border-bottom:1px solid #e7edf5}.history-point-search input{min-width:180px;flex:1}.history-point-search span{margin-left:auto;color:#64748b;font-size:11px}.history-point-table{max-height:310px;overflow:auto}.history-point-table table{width:100%;border-collapse:collapse;font-size:12px}.history-point-table th,.history-point-table td{height:34px;padding:0 9px;border-bottom:1px solid #edf2f7;text-align:left}.history-point-table th{position:sticky;z-index:1;top:0;color:#475569;background:#f8fafc}.history-point-table th:first-child,.history-point-table td:first-child{width:38px;text-align:center}.history-point-table tbody tr{cursor:pointer}.history-point-table tbody tr:hover{background:#f8fbff}.history-point-table tbody tr.selected{background:#eff6ff}.history-point-pagination{display:flex;align-items:center;justify-content:flex-end;gap:6px;padding:8px;border-top:1px solid #e7edf5;background:#fbfdff}.history-point-pagination span{margin-right:auto;color:#64748b;font-size:11px}.history-point-pagination button{height:29px}.history-task-modal .dialog-actions{display:grid;grid-template-columns:auto 1fr auto auto;gap:8px;padding:12px 18px;border-top:1px solid #e7edf5;background:#fbfdff}@media(max-width:1300px){.history-global-grid{grid-template-columns:150px 180px minmax(260px,1fr);}.history-migration{grid-column:1/3}.history-global-actions{grid-column:3}}@media(max-width:900px){.history-tabs{overflow-x:auto}.history-tabs button{flex:0 0 auto}.history-global-grid,.history-task-basics,.history-schedule{grid-template-columns:1fr}.history-migration,.history-global-actions{grid-column:auto}.history-global-actions{justify-content:flex-start}.history-summary,.history-quick-status{grid-template-columns:repeat(2,1fr)}.history-storage-detail{grid-template-columns:1fr}.history-point-picker-body{grid-template-columns:1fr}.history-point-picker aside{display:flex;overflow-x:auto;border-right:0;border-bottom:1px solid #e7edf5}.history-point-picker aside button{min-width:190px}.history-point-search{flex-wrap:wrap}.history-point-search input{width:100%;flex-basis:100%}.history-point-search span{margin-left:0}.history-task-modal{padding:8px}.history-task-modal .dialog-panel{width:100%;max-height:97vh}}
.history-global-grid>label small{color:#94a3b8;font-size:10px;font-weight:500}
.history-storage-warning{display:grid;grid-template-columns:auto 1fr;gap:4px 10px;margin:12px 0 0;padding:10px 12px;border:1px solid #fed7aa;border-radius:9px;color:#9a3412;background:#fff7ed}.history-storage-warning strong{font-size:12px}.history-storage-warning span{font-size:12px}.history-storage-warning small{grid-column:2;color:#c2410c}.history-query-card{min-width:0;margin-top:12px}.history-query-filters{display:grid;grid-template-columns:1fr 1.2fr 1.5fr 1fr 1fr;gap:10px;margin-bottom:12px}.history-query-filters label{display:grid;gap:5px;color:#64748b;font-size:11px;font-weight:700}.history-query-chart{overflow:hidden;margin-bottom:12px;border:1px solid #e1e8f2;border-radius:11px;background:#fff}.history-query-panel-head{display:flex;align-items:center;justify-content:space-between;padding:10px 12px;border-bottom:1px solid #edf2f7;background:#f8fafc}.history-query-panel-head>div{display:grid;gap:2px}.history-query-panel-head strong{color:#0f172a;font-size:13px}.history-query-panel-head small,.history-query-panel-head span{color:#64748b;font-size:10px}.history-query-chart svg{display:block;width:100%;height:250px}.history-query-chart line{stroke:#e5eaf1;stroke-width:1}.history-query-chart polyline{fill:none;stroke:#1677ff;stroke-width:2;vector-effect:non-scaling-stroke}.history-query-chart circle{fill:#1677ff}.history-query-chart text{fill:#64748b;font-size:10px}.history-query-chart text:not(.time){text-anchor:end}.history-query-empty{display:grid;height:180px;place-items:center;color:#94a3b8;font-size:12px}.history-query-table{overflow:auto;border:1px solid #e1e8f2;border-radius:10px}.history-query-table table{width:100%;min-width:1080px;border-collapse:collapse;font-size:11px}.history-query-table th,.history-query-table td{height:36px;padding:0 9px;border-bottom:1px solid #edf2f7;text-align:left;white-space:nowrap}.history-query-table th{color:#475569;background:#f8fafc}.history-query-table tbody tr:hover{background:#f8fbff}.history-query-table .history-value{max-width:260px;overflow:hidden;color:#0f172a;font-weight:700;text-overflow:ellipsis}.history-quality{display:inline-flex;min-width:50px;justify-content:center;padding:2px 7px;border-radius:999px;color:#475569;background:#f1f5f9;font-size:10px;font-weight:700}.history-quality.good{color:#15803d;background:#dcfce7}.history-quality.bad{color:#b91c1c;background:#fee2e2}.history-quality.uncertain{color:#a16207;background:#fef3c7}.history-query-pagination{display:flex;align-items:center;justify-content:flex-end;gap:6px;padding-top:9px}.history-query-pagination span{margin-right:auto;color:#64748b;font-size:11px}.history-query-pagination button{height:30px}.history-export-jobs{margin-top:14px;padding-top:2px;border-top:1px solid #e7edf5}.history-export-job{display:grid;grid-template-columns:minmax(220px,1fr) auto minmax(0,1fr) auto;align-items:center;gap:10px;padding:9px 10px;border-bottom:1px solid #edf2f7}.history-export-job>div{display:grid}.history-export-job small{margin-top:3px;color:#64748b;font-size:10px}.export-error{overflow:hidden;color:#b91c1c;font-size:11px;text-overflow:ellipsis;white-space:nowrap}
.history-feature-grid{display:grid;grid-template-columns:1.2fr 1.2fr 150px 110px minmax(210px,1fr) 1.1fr;align-items:end;gap:9px;margin-top:10px;padding:11px 12px;border:1px solid #e4ebf4;border-radius:11px;background:#fff}.history-feature-grid>label{display:grid;gap:5px;color:#64748b;font-size:11px;font-weight:700}.feature-switch{display:flex;min-height:42px;align-items:center;justify-content:space-between;gap:12px;padding:4px 8px;border-right:1px solid #edf2f7}.feature-switch>span{display:grid;gap:2px}.feature-switch strong{color:#334155;font-size:12px}.feature-switch small{color:#94a3b8;font-size:9px}.history-maintenance-card{min-width:0;margin-top:12px}.maintenance-status-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:9px}.maintenance-status-grid>div{display:grid;gap:5px;min-height:78px;padding:11px 12px;border:1px solid #e4ebf4;border-radius:10px;background:#f8fafc}.maintenance-status-grid span{color:#64748b;font-size:10px}.maintenance-status-grid strong,.maintenance-status-grid code{overflow:hidden;color:#0f172a;font-size:14px;text-overflow:ellipsis;white-space:nowrap}.maintenance-status-grid strong.ok{color:#15803d}.maintenance-status-grid strong.bad{color:#b91c1c}.maintenance-status-grid small{color:#94a3b8;font-size:10px}.maintenance-actions{display:flex;align-items:center;gap:7px;margin:11px 0}.maintenance-actions>span{margin-left:auto;color:#1d4ed8;font-size:11px}.backup-table{overflow:auto;border:1px solid #e1e8f2;border-radius:10px}.backup-table table{width:100%;min-width:900px;border-collapse:collapse;font-size:11px}.backup-table th,.backup-table td{height:38px;padding:0 10px;border-bottom:1px solid #edf2f7;text-align:left;white-space:nowrap}.backup-table th{color:#475569;background:#f8fafc}.backup-hash{margin-left:7px;color:#94a3b8;font-family:monospace}
@media(max-width:1300px){.history-feature-grid{grid-template-columns:repeat(3,1fr)}}
@media(max-width:1100px){.history-query-filters{grid-template-columns:1fr 1fr}.maintenance-status-grid{grid-template-columns:1fr 1fr}}
@media(max-width:700px){.history-query-filters,.history-feature-grid,.maintenance-status-grid{grid-template-columns:1fr}.history-storage-warning{grid-template-columns:1fr}.history-storage-warning small{grid-column:auto}.feature-switch{border-right:0;border-bottom:1px solid #edf2f7}.maintenance-actions{flex-wrap:wrap}}
.history-tabs{margin-bottom:0}
.history-chart-meta{display:flex;align-items:center;gap:8px}.history-unit-chip{display:inline-flex;min-width:30px;justify-content:center;padding:3px 8px;border:1px solid #bfdbfe;border-radius:999px;color:#1d4ed8!important;background:#eff6ff;font-weight:700}.history-chart-canvas{position:relative;overflow:hidden;background:#fff}.history-query-chart .history-chart-canvas svg{display:block;width:100%;height:290px;cursor:crosshair;touch-action:none}.history-query-chart .history-chart-grid line{stroke:#e6edf6;stroke-width:1;stroke-dasharray:3 4}.history-query-chart .history-chart-grid text{fill:#64748b;font-size:10px;font-weight:600}.history-query-chart .axis-unit{fill:#64748b;font-size:9px;font-weight:700;text-anchor:start}.history-query-chart .history-chart-area{fill:#dbeafe;fill-opacity:.48}.history-query-chart .history-chart-line-shadow{fill:none;stroke:#93c5fd;stroke-width:6;stroke-opacity:.18;vector-effect:non-scaling-stroke}.history-query-chart .history-chart-line{fill:none;stroke:#1677ff;stroke-width:2.25;stroke-linecap:round;stroke-linejoin:round;vector-effect:non-scaling-stroke}.history-query-chart .history-chart-dot{fill:#fff;stroke:#1677ff;stroke-width:1.8;vector-effect:non-scaling-stroke}.history-query-chart .history-chart-focus line{stroke:#64748b;stroke-width:1;stroke-dasharray:4 4;stroke-opacity:.65;vector-effect:non-scaling-stroke}.history-query-chart .history-chart-focus .halo{fill:#bfdbfe;fill-opacity:.55;stroke:none}.history-query-chart .history-chart-focus .core{fill:#1677ff;stroke:#fff;stroke-width:2;vector-effect:non-scaling-stroke}.history-query-chart .time{fill:#64748b;font-size:10px;font-weight:600}.history-chart-tooltip{position:absolute;z-index:3;min-width:190px;max-width:260px;padding:11px 12px;border:1px solid #dbeafe;border-radius:10px;color:#334155;background:#fff;box-shadow:0 12px 30px rgba(30,64,175,.18);pointer-events:none;transform:translate(12px,calc(-100% - 10px))}.history-chart-tooltip.flipped{transform:translate(calc(-100% - 12px),calc(-100% - 10px))}.history-chart-tooltip-title{display:grid;gap:2px;padding-bottom:8px;border-bottom:1px solid #edf2f7}.history-chart-tooltip-title strong{overflow:hidden;color:#0f172a;font-size:12px;text-overflow:ellipsis;white-space:nowrap}.history-chart-tooltip-title code{overflow:hidden;color:#64748b;font-size:9px;text-overflow:ellipsis;white-space:nowrap}.history-chart-tooltip-value{display:flex;align-items:baseline;gap:6px;padding-top:8px}.history-chart-tooltip-value strong{color:#1677ff;font-size:19px;line-height:1}.history-chart-tooltip-value span{color:#64748b;font-size:10px}.history-chart-tooltip time{display:block;margin-top:7px;color:#475569;font-size:10px;font-variant-numeric:tabular-nums}@media(max-width:700px){.history-query-chart .history-chart-canvas svg{height:240px}.history-chart-tooltip{min-width:160px}.history-chart-tooltip-value strong{font-size:16px}}
</style>
