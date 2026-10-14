<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { Download, Setting } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';
import { historyFilterBounds, validTimeBounds, boundedFilterRange, disabledFilterDate, disabledTimeUnits } from '../utils/history-filter-time';

const props=defineProps<{taskId?:string}>();
const emit=defineEmits<{saved:[id:string]}>();
const editing=ref(!props.taskId),saving=ref(false),taskName=ref('历史数据任务');
interface Task {id:string;name:string;startAt:string;endAt:string;pairs:Array<{value1:string;value2:string}>;devices:Device[]}
interface Device { id:string; name:string; deviceKey:string }
interface Metric { deviceId?:string; deviceName?:string; isGatewayMetric?:boolean; identifier:string; name:string; unit?:string; enabled?:boolean; ignored?:boolean }
interface Retention { label:string; queryEarliestAt:string; graceEndsAt?:string }
interface HistoryResult { dataset:{dimensions:Array<{title:string;key:string}>;source:Array<Record<string,unknown>>};total:number;page:number;pageSize:number }
const devices=ref<Device[]>([]),deviceIds=ref<string[]>([]),metrics=reactive<Record<string,Metric[]>>({}),selected=reactive<Record<string,string[]>>({});
const metricErrors=reactive<Record<string,string>>({}),deviceLoading=ref(false),retention=ref<Retention>();
const range=ref<[Date,Date]|null>([new Date(Date.now()-3600000),new Date()]);
const loading=ref(false),exporting=ref(false),result=ref<HistoryResult>(),page=ref(1),pageSize=ref(20),queryError=ref('');
const filters=reactive({keyword:'',deviceId:'',pointKey:'',valueMin:'',valueMax:''});
const filterRange=ref<[Date,Date]|null>(null),appliedFilters=ref<Record<string,string>>({});
const filterBounds=computed(()=>range.value?historyFilterBounds(range.value[0],range.value[1],retention.value?.queryEarliestAt):{min:NaN,max:NaN});
const canFilterTime=computed(()=>validTimeBounds(filterBounds.value));
function normalizeFilterRange(){const next=boundedFilterRange(filterRange.value,filterBounds.value);if(JSON.stringify(next)!==JSON.stringify(filterRange.value))filterRange.value=next;}
watch([filterRange,filterBounds],normalizeFilterRange,{deep:true});
const filterDisabledDate=(value:Date)=>disabledFilterDate(value,filterBounds.value);
type PickerDate={toDate:()=>Date};
const spinnerDate=(role:string,value?:PickerDate)=>value?.toDate()||filterRange.value?.[role==='end'?1:0]||new Date(filterBounds.value.min);
const filterDisabledHours=(role:string,value?:PickerDate)=>disabledTimeUnits(spinnerDate(role,value),filterBounds.value,'hour');
const filterDisabledMinutes=(hour:number,role:string,value?:PickerDate)=>disabledTimeUnits(spinnerDate(role,value),filterBounds.value,'minute',hour);
const filterDisabledSeconds=(hour:number,minute:number,role:string,value?:PickerDate)=>disabledTimeUnits(spinnerDate(role,value),filterBounds.value,'second',hour,minute);
const pointOptions=computed(()=>pairs.value.filter(p=>!filters.deviceId||p.value1===filters.deviceId).map(p=>({key:JSON.stringify([p.value1,p.value2]),label:`${deviceName(p.value1)} / ${p.value2}`})));
watch(()=>filters.deviceId,()=>{filters.pointKey='';filters.valueMin='';filters.valueMax='';});
watch(()=>filters.pointKey,()=>{filters.valueMin='';filters.valueMax='';});
function filterPayload(){const value:Record<string,string>={};for(const [key,v] of Object.entries(filters))if(v.trim())value[key]=v.trim();if(filterRange.value){value.start=filterRange.value[0].toISOString();value.end=filterRange.value[1].toISOString();}return value;}
async function applyFilters(){if(!canFilterTime.value)return;normalizeFilterRange();appliedFilters.value=filterPayload();await query(1);}
async function resetFilters(){Object.assign(filters,{keyword:'',deviceId:'',pointKey:'',valueMin:'',valueMax:''});filterRange.value=boundedFilterRange(null,filterBounds.value);appliedFilters.value=filterPayload();if(canFilterTime.value)await query(1);}
let deviceGeneration=0,pointGeneration=0,queryGeneration=0,disposed=false;
const metricKey=(id:string,m:Metric)=>JSON.stringify([m.deviceId||id,m.identifier]);
const pairs=computed(()=>[...new Map(deviceIds.value.flatMap(id=>(selected[id]||[]).map(key=>{const [owner,metric]=JSON.parse(key) as [string,string];return {value1:owner,value2:metric};})).map(p=>[JSON.stringify([p.value1,p.value2]),p])).values()]);
const columns=computed(()=>result.value?.dataset.dimensions||[]);
const availableMetrics=(id:string)=>(metrics[id]||[]).filter(m=>m.enabled!==false&&!m.ignored);
const metricGroups=(id:string)=>[...new Set(availableMetrics(id).map(m=>m.deviceId||id))].map(owner=>({id:owner,label:owner===id?'设备自身点位':`${availableMetrics(id).find(m=>m.deviceId===owner)?.deviceName||owner} · 子设备`,items:availableMetrics(id).filter(m=>(m.deviceId||id)===owner)}));
const date=(v?:string)=>v?new Date(v).toLocaleString('zh-CN',{timeZone:'Asia/Shanghai'}):'—';
const deviceName=(id:string)=>devices.value.find(d=>d.id===id)?.name||id;
async function searchDevices(keyword='') {
  const generation=++deviceGeneration;deviceLoading.value=true;
  try {
    const data=await apiRequest<{items:Device[]}>({url:'/api/devices',method:'GET',params:{page:1,pageSize:100,keyword}});
    if(disposed||generation!==deviceGeneration)return;
    devices.value=[...new Map([...devices.value.filter(d=>deviceIds.value.includes(d.id)),...data.items].map(d=>[d.id,d])).values()];
  } finally {if(generation===deviceGeneration)deviceLoading.value=false;}
}
watch(deviceIds,async ids=>{
  const generation=++pointGeneration;
  for(const id of Object.keys(selected))if(!ids.includes(id))delete selected[id];
  await Promise.all(ids.filter(id=>!metrics[id]||metricErrors[id]).map(async id=>{
    metricErrors[id]='';
    try{const items=await apiRequest<Metric[]>({url:`/api/devices/${id}/metrics`,method:'GET'});if(!disposed&&generation===pointGeneration&&deviceIds.value.includes(id))metrics[id]=items;}
    catch{if(generation===pointGeneration)metricErrors[id]='点位加载失败，请移除设备后重新选择';}
  }));
},{deep:true});
watch(()=>JSON.stringify([pairs.value,range.value]),()=>{queryGeneration++;result.value=undefined;page.value=1;queryError.value='';});
function selectAll(id:string){const keys=availableMetrics(id).map(m=>metricKey(id,m));const all=new Set([...deviceIds.value.filter(other=>other!==id).flatMap(other=>selected[other]||[]),...keys]);if(all.size>20){ElMessage.warning('单次最多选择20个点位，请按需勾选');return;}selected[id]=keys;}
const disabledDate=(value:Date)=>value.getTime()>Date.now()||(!!retention.value&&value.getTime()+86400000<=new Date(retention.value.queryEarliestAt).getTime());
function quickRange(hours:number){const now=new Date(),cutoff=retention.value?new Date(retention.value.queryEarliestAt).getTime():0;range.value=[new Date(Math.max(now.getTime()-hours*3600000,cutoff+1000)),now];}
async function payload(){
  if(!pairs.value.length)throw Error('请至少选择一个设备点位');
  if(pairs.value.length>20)throw Error('单次最多选择20个点位');
  if(!range.value||range.value[0]>=range.value[1])throw Error('请选择有效时间范围');
  retention.value=await apiRequest<Retention>({url:'/api/devices/data/retention',method:'GET'});
  const [start,end]=range.value;
  if(start.getTime()<new Date(retention.value.queryEarliestAt).getTime())throw Error(`超出保存期限，请选择${date(retention.value.queryEarliestAt)}之后的时间`);
  if(end.getTime()>Date.now()+1000)throw Error('不能查询未来时间');
  if(end.getTime()-start.getTime()>31*86400000)throw Error('单次查询跨度不能超过31天，请分段查询或导出');
  return {pairs:pairs.value.map(p=>({...p})),start:start.toISOString(),end:end.toISOString()};
}
async function query(nextPage=1){
  if(!props.taskId)return;
  const generation=++queryGeneration;loading.value=true;queryError.value='';
  try{const value=await apiRequest<HistoryResult>({url:`/api/history-data-tasks/${props.taskId}/records`,method:'GET',params:{...appliedFilters.value,page:nextPage,pageSize:pageSize.value},timeout:30000});if(!disposed&&generation===queryGeneration){result.value=value;page.value=value.page;}}
  catch(error:any){if(generation===queryGeneration){result.value=undefined;queryError.value=error.response?.data?.message||error.message||'查询失败，请重试';}}
  finally{loading.value=false;}
}
async function loadTask(){
  if(!props.taskId)return;
  const task=await apiRequest<Task>({url:`/api/history-data-tasks/${props.taskId}`,method:'GET'});
  if(disposed)return;
  taskName.value=task.name;devices.value=[...new Map([...devices.value,...task.devices].map(d=>[d.id,d])).values()];
  for(const id of Object.keys(selected))delete selected[id];
  for(const pair of task.pairs)(selected[pair.value1]||=([])).push(JSON.stringify([pair.value1,pair.value2]));
  deviceIds.value=[...new Set(task.pairs.map(p=>p.value1))];range.value=[new Date(task.startAt),new Date(task.endAt)];
  Object.assign(filters,{keyword:'',deviceId:'',pointKey:'',valueMin:'',valueMax:''});filterRange.value=null;appliedFilters.value={};
  normalizeFilterRange();appliedFilters.value=filterPayload();
}
async function saveTask(){
  saving.value=true;queryError.value='';
  try{if(!taskName.value.trim())throw Error('请填写任务名称');const data=await payload();const task=await apiRequest<{id:string}>({url:props.taskId?`/api/history-data-tasks/${props.taskId}`:'/api/history-data-tasks',method:props.taskId?'PATCH':'POST',data:{...data,name:taskName.value.trim()},timeout:30000});ElMessage.success('历史数据任务已保存');emit('saved',task.id);if(props.taskId){editing.value=false;await loadTask();await query();}}
  catch(error:any){queryError.value=error.response?.data?.message||error.message||'保存失败，请重试';}
  finally{saving.value=false;}
}
async function cancelEdit(){await loadTask();editing.value=false;await query();}

async function exportData(){
  exporting.value=true;
  try{
    if((result.value?.total||0)>2000)await ElMessageBox.confirm(`当前结果共${result.value!.total.toLocaleString()}条，单次最多导出最新2000条。可缩小时间范围分批导出，是否继续？`,'导出范围提示',{type:'warning',confirmButtonText:'导出最新2000条',cancelButtonText:'取消'});
    if(!props.taskId)throw Error('请先保存任务');
    const value=await apiRequest<{csv:string;total:number;exported:number;limited:boolean}>({url:`/api/history-data-tasks/${props.taskId}/export`,method:'POST',data:appliedFilters.value,timeout:60000});
    const url=URL.createObjectURL(new Blob([value.csv],{type:'text/csv;charset=utf-8'})),link=document.createElement('a');link.href=url;link.download=`历史数据-${new Date().toISOString().slice(0,10)}.csv`;link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
    if(value.limited)ElMessage.warning(`共${value.total}条，本次导出最新${value.exported}条，请缩小范围分批导出`);else ElMessage.success(`已导出${value.exported}条历史记录`);
  }catch(error:any){if(error!=='cancel'&&error!=='close')ElMessage.warning(error.response?.data?.message||error.message||'导出失败，请重试');}
  finally{exporting.value=false;}
}
onMounted(async()=>{await Promise.all([searchDevices(),apiRequest<Retention>({url:'/api/devices/data/retention',method:'GET'}).then(value=>{retention.value=value;})]).then(async()=>{if(props.taskId){await loadTask();await query();}}).catch(error=>{queryError.value=error.response?.data?.message||'任务加载失败，请返回列表重试';});});
onBeforeUnmount(()=>{disposed=true;deviceGeneration++;pointGeneration++;queryGeneration++;});
</script>

<template>
  <div class="data-center">
    <div v-if="props.taskId" class="page-actions"><div><h1 class="page-title">{{taskName}}</h1><p class="page-description">历史查询任务 · 查看时按保存条件读取数据，不作为永久归档</p></div><div><el-button :icon="Setting" :disabled="loading||exporting||saving" @click="editing=true">编辑任务</el-button><el-button :icon="Download" :disabled="loading||editing" :loading="exporting" @click="exportData">导出 CSV</el-button></div></div>
    <div class="retention-note"><b>历史保存：{{retention?.label||'读取中'}}</b><span>可查询自 {{date(retention?.queryEarliestAt)}}<template v-if="retention?.graceEndsAt"> · 清理宽限至 {{date(retention.graceEndsAt)}}</template></span></div>
    <section v-if="editing" class="table-panel filter-panel">
      <el-form-item label="任务名称" required><el-input v-model="taskName" maxlength="80" show-word-limit placeholder="例如：噪声设备每日历史查询" :disabled="saving"/></el-form-item>
      <div class="section-heading"><h2>选择数据源</h2><el-tag effect="plain">已选 {{pairs.length}} / 20 个点位</el-tag></div>
      <el-select v-model="deviceIds" multiple filterable remote reserve-keyword :remote-method="searchDevices" :loading="deviceLoading" :disabled="loading||exporting||saving" placeholder="搜索并选择设备，可多选" class="device-select" collapse-tags collapse-tags-tooltip><el-option v-for="d in devices" :key="d.id" :label="`${d.name} · ${d.deviceKey}`" :value="d.id"/></el-select>
      <p class="hint">一次最多展示100个设备候选，可输入名称搜索。网关汇总有权限访问的子设备点位，保存时按实际所属设备记录；重复点位只计一次。</p>
      <div v-for="id in deviceIds" :key="id" class="point-row"><div class="point-label"><b>{{deviceName(id)}}</b><el-button link type="primary" :disabled="loading||exporting||saving" @click="selectAll(id)">全选点位</el-button></div><el-select v-model="selected[id]" multiple filterable :disabled="loading||exporting||saving" placeholder="选择需要查询或导出的点位" collapse-tags collapse-tags-tooltip><el-option-group v-for="group in metricGroups(id)" :key="group.id" :label="group.label"><el-option v-for="m in group.items" :key="metricKey(id,m)" :label="`${m.deviceName||deviceName(id)} / ${m.name||m.identifier} · ${m.identifier}${m.unit?' ('+m.unit+')':''}`" :value="metricKey(id,m)" :disabled="pairs.length>=20&&!pairs.some(p=>p.value1===(m.deviceId||id)&&p.value2===m.identifier)"/></el-option-group></el-select><span v-if="metricErrors[id]" class="error-text">{{metricErrors[id]}}</span><span v-else-if="metrics[id]&&!availableMetrics(id).length" class="hint">没有可查询的云端点位</span></div>
      <div class="time-section"><label>时间范围</label><el-date-picker v-model="range" type="datetimerange" :disabled="loading||exporting||saving" :disabled-date="disabledDate" start-placeholder="开始时间" end-placeholder="结束时间" format="YYYY-MM-DD HH:mm:ss"/><div class="shortcuts"><el-button v-for="item in [{hours:1,label:'近1小时'},{hours:24,label:'近24小时'},{hours:168,label:'近7天'}]" :key="item.hours" size="small" :disabled="loading||exporting||saving" @click="quickRange(item.hours)">{{item.label}}</el-button></div></div>
      <el-alert v-if="queryError" type="warning" :closable="false" :title="queryError" show-icon/><div class="filter-footer"><span class="hint">保存查询条件，单次跨度最多31天；导出单次最多2000条。</span><div><el-button v-if="props.taskId" :disabled="saving" @click="cancelEdit">取消修改</el-button><el-button type="primary" :loading="saving" :disabled="!pairs.length" @click="saveTask">保存任务</el-button></div></div>
    </section>
    <div v-if="props.taskId&&!editing" class="retention-note"><span>设备：{{deviceIds.map(deviceName).join('、')}} · {{pairs.length}}个点位</span><span>时间：{{date(range?.[0]?.toISOString())}} 至 {{date(range?.[1]?.toISOString())}}</span></div>
    <section v-if="props.taskId&&!editing" class="table-panel record-filters">
      <h2>筛选历史记录</h2>
      <div class="record-filter-fields">
        <el-select v-model="filters.deviceId" clearable placeholder="全部设备" aria-label="筛选设备" :disabled="loading||exporting"><el-option v-for="id in deviceIds" :key="id" :value="id" :label="deviceName(id)"/></el-select>
        <el-select v-model="filters.pointKey" clearable filterable placeholder="全部点位" aria-label="筛选点位" :disabled="loading||exporting"><el-option v-for="point in pointOptions" :key="point.key" :value="point.key" :label="point.label"/></el-select>
        <el-input v-model="filters.keyword" clearable placeholder="搜索设备名称或点位值" :disabled="loading||exporting" @keyup.enter="applyFilters"/>
        <el-input v-model="filters.valueMin" placeholder="数值下限（含）" :disabled="!filters.pointKey||loading||exporting" @keyup.enter="applyFilters"/>
        <el-input v-model="filters.valueMax" placeholder="数值上限（含）" :disabled="!filters.pointKey||loading||exporting" @keyup.enter="applyFilters"/>
      </div>
      <div class="time-section"><label>筛选时间</label><el-date-picker v-model="filterRange" type="datetimerange" start-placeholder="任务开始时间" end-placeholder="任务结束时间" format="YYYY-MM-DD HH:mm:ss" :disabled="loading||exporting||!canFilterTime" :disabled-date="filterDisabledDate" :disabled-hours="filterDisabledHours" :disabled-minutes="filterDisabledMinutes" :disabled-seconds="filterDisabledSeconds" :default-time="range||undefined" :default-value="range||undefined" :clearable="false" @change="normalizeFilterRange"/><el-button type="primary" :loading="loading" :disabled="exporting||!canFilterTime" @click="applyFilters">筛选</el-button><el-button :disabled="loading||exporting||!canFilterTime" @click="resetFilters">重置筛选</el-button></div>
      <p v-if="!canFilterTime" class="error-text">此任务已无可筛选的有效时间，请编辑任务调整时间范围。</p>
      <p class="hint">在任务保存的时间范围内筛选；选择具体点位后可设置数值范围。点击筛选后生效，分页和导出均使用已应用条件，不修改任务配置。</p>
    </section>
    <section v-if="props.taskId&&!editing" class="table-panel results-panel"><div class="section-heading"><h2>历史记录</h2><span class="hint">{{result?'共 '+result.total.toLocaleString()+' 条':'请先选择点位和时间范围'}}</span></div><el-alert v-if="queryError" type="warning" :closable="false" :title="queryError" show-icon/><el-table :data="result?.dataset.source||[]" v-loading="loading" stripe border max-height="520" :empty-text="result?'该时段没有云端历史记录，请检查设备是否上报并保存数据':'选择数据源后点击查询'"><el-table-column v-for="c in columns" :key="c.key" :prop="c.key" :label="c.title" :min-width="c.key==='timestamp'?185:150" show-overflow-tooltip><template #default="{row}">{{row[c.key]??'—'}}</template></el-table-column></el-table><div class="result-footer"><span class="hint">按设备上报记录展示；未上报的点位显示“—”，不补0。</span><el-pagination v-if="result" :current-page="page" :page-size="pageSize" :total="result.total" :page-sizes="[20,50,100]" :disabled="loading||exporting||saving" layout="total, sizes, prev, pager, next" @current-change="query" @size-change="(value: number)=>{pageSize=value;query();}"/></div></section>
  </div>
</template>

<style scoped>
.record-filters h2{font-size:16px;margin:0 0 16px}.record-filter-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.record-filters .time-section{margin:16px 0 8px}@media(max-width:700px){.record-filter-fields{grid-template-columns:1fr}}
.retention-note{display:flex;align-items:center;gap:20px;flex-wrap:wrap;padding:15px 20px;margin-bottom:18px;border:1px solid #dce9fc;border-radius:12px;background:#edf5ff;color:#335b8f;font-size:13px}.retention-note b{color:#1769d2}.section-heading,.filter-footer,.result-footer{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}.section-heading{margin-bottom:18px}.section-heading h2{margin:0;font-size:17px;color:#203754}.device-select{width:100%}.hint{font-size:12px;color:#7f8da2;line-height:1.65}.point-row{display:grid;grid-template-columns:180px minmax(0,1fr);gap:12px;align-items:center;margin-top:14px;padding:12px 14px;border-radius:9px;background:#f7f9fc}.point-label{display:flex;align-items:center;justify-content:space-between;gap:8px;font-size:13px}.error-text{grid-column:2;color:#c45656;font-size:12px}.time-section{display:flex;gap:12px;align-items:center;flex-wrap:wrap;margin:24px 0 18px}.time-section label{font-size:13px;color:#52677e}.filter-footer{padding-top:16px;border-top:1px solid #eaf0f8}.results-panel{margin-top:20px}.result-footer{margin-top:16px}@media(max-width:700px){.point-row{grid-template-columns:1fr}.error-text{grid-column:1}.filter-footer>div{width:100%;display:flex}.filter-footer>div .el-button{flex:1}.time-section :deep(.el-date-editor){width:100%}.result-footer :deep(.el-pagination){max-width:100%;overflow:auto}}
</style>
