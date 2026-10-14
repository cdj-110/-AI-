<script setup lang="ts">
import {computed,onMounted,onBeforeUnmount,ref} from 'vue'
import {IconDatabase,IconPlus,IconCloudUpload,IconInfoCircle} from '@tabler/icons-vue'
import {api} from '../api'
import {confirmAction} from '../confirm'
import {usePageDraftGuard} from '../composables/usePageDraftGuard'
import {changeRemoteDriver,driverNames,newRemoteTarget,pointKey,validateRemoteTarget,type RemoteConfig,type RemoteTarget,type PointRef} from '../remoteStorage'
const props=defineProps<{canManage:boolean}>()
interface Status{id:string;enabled:boolean;cacheReady:boolean;pending:number;bytes:number;sent:number;dropped:number;connected:boolean;lastSuccess?:string;lastError?:string;cacheError?:string;nextRetry?:string}
interface Volume{device:string;mountPath:string;uuid:string;freeBytes:number;readOnly?:boolean}
interface Group{deviceKey:string;name:string;count:number}
interface Point extends PointRef{name:string;unit?:string}
interface Reply{config:RemoteConfig;revision:string;statuses:Status[];devices:Volume[];pointGroups:Group[]}
const localPath='/userdata/weikong/data'
const config=ref<RemoteConfig>({storagePath:localPath,storageUUID:'',maxSizeMB:16,targets:[]})
const saved=ref(''),revision=ref(''),statuses=ref<Status[]>([]),volumes=ref<Volume[]>([]),groups=ref<Group[]>([])
const message=ref(''),error=ref(''),pollError=ref(''),busy=ref(false),testing=ref(false),loaded=ref(false)
const editor=ref<RemoteTarget|null>(null),editorInitial=ref(''),editorIndex=ref(-1),editorError=ref(''),testMessage=ref('')
const device=ref(''),search=ref(''),offset=ref(0),total=ref(0),points=ref<Point[]>([]),pointsBusy=ref(false)
let timer=0,pointRequest=0,disposed=false,polling=false
const clone=<T,>(value:T):T=>JSON.parse(JSON.stringify(value))
const configDirty=computed(()=>!!saved.value&&JSON.stringify(config.value)!==saved.value)
const dirty=computed(()=>configDirty.value||(editor.value!==null&&JSON.stringify(editor.value)!==editorInitial.value))
const statusMap=computed(()=>new Map(statuses.value.map(s=>[s.id,s])))
const totalPending=computed(()=>config.value.targets.reduce((sum,t)=>sum+(statusMap.value.get(t.id)?.pending||0),0))
const healthyCount=computed(()=>config.value.targets.filter(t=>stateTone(t)==='healthy').length)
function stateTone(t:RemoteTarget){const s=statusMap.value.get(t.id);if(!s||!s.enabled)return 'idle';if(s.cacheError)return 'failed';if(s.lastError)return 'warning';return s.cacheReady&&s.connected?'healthy':'idle'}
function cachePercent(t:RemoteTarget){return Math.min(100,Math.max(0,(statusMap.value.get(t.id)?.bytes||0)/(Math.max(4,config.value.maxSizeMB)*1024*1024/2)*100))}
const chosen=computed(()=>new Set(editor.value?.points.map(pointKey)||[]))
function discard(){if(saved.value)config.value=JSON.parse(saved.value);editor.value=null;error.value='';message.value='已放弃未保存的修改。'}
const requestLeave=usePageDraftGuard(()=>dirty.value,discard,async()=>{if(editor.value&&JSON.stringify(editor.value)!==editorInitial.value){applyEditor();if(editor.value)return false}await save()})
defineExpose({requestLeave})
async function load(){
  if(!(await requestLeave()))return
  busy.value=true;error.value=''
  try{const r=await api<Reply>('/api/remote-storage');if(disposed)return;config.value={...r.config,storageUUID:r.config.storageUUID||'',targets:r.config.targets||[]};saved.value=JSON.stringify(config.value);revision.value=r.revision;statuses.value=r.statuses||[];volumes.value=r.devices||[];groups.value=r.pointGroups||[];loaded.value=true}
  catch(e){error.value=String(e instanceof Error?e.message:e)}finally{busy.value=false}
}
async function poll(){
  if(polling||busy.value||disposed)return
  polling=true
  try{const r=await api<{statuses:Status[]}>('/api/remote-storage/status');if(disposed)return;statuses.value=r.statuses||[];pollError.value=''}catch(e){pollError.value='状态刷新失败：'+String(e instanceof Error?e.message:e)}finally{polling=false}
}
function selectVolume(){config.value.storageUUID=volumes.value.find(v=>v.mountPath===config.value.storagePath)?.uuid||''}
async function save(){
  if(!props.canManage||busy.value)return
  error.value='';message.value=''
  for(const t of config.value.targets){const problem=validateRemoteTarget(t);if(problem){error.value=t.name+'：'+problem;return}}
  if(config.value.targets.some(t=>t.enabled&&t.tlsMode==='disabled')){
    if(!(await confirmAction({title:'存在未加密的数据库连接',message:'关闭 TLS 的任务会通过未加密连接传输数据。',detail:'仅在可信隔离网络中使用；推荐配置服务器证书并开启 TLS。',confirmText:'保存设置',cancelText:'返回检查',danger:true})))return
  }
  busy.value=true
  try{const r=await api<Reply>('/api/remote-storage',{method:'PUT',headers:{'Content-Type':'application/json','If-Match':revision.value},body:JSON.stringify(config.value),timeoutMs:90000});config.value={...r.config,storageUUID:r.config.storageUUID||'',targets:r.config.targets||[]};saved.value=JSON.stringify(config.value);revision.value=r.revision;statuses.value=r.statuses||[];message.value='已保存并应用。已启用任务会独立采样并写入远程数据库。'}catch(e){error.value=String(e instanceof Error?e.message:e)}finally{busy.value=false}
}
function edit(index:number){
  if(index<0&&config.value.targets.length>=4){error.value='最多配置 4 个远程存储任务。';return}
  editorIndex.value=index;editor.value=index<0?newRemoteTarget():clone(config.value.targets[index]);editorInitial.value=JSON.stringify(editor.value);editorError.value='';testMessage.value='';search.value='';device.value=groups.value[0]?.deviceKey||'';void loadPoints(true)
}
async function closeEditor(){
  if(testing.value)return
  if(editor.value&&JSON.stringify(editor.value)!==editorInitial.value&&!(await confirmAction({title:'放弃当前任务编辑？',message:'当前任务的修改尚未加入页面草稿。',confirmText:'放弃编辑',cancelText:'继续编辑',danger:true})))return
  editor.value=null;pointRequest++
}
function applyEditor(){
  if(!editor.value||!props.canManage)return
  const problem=validateRemoteTarget(editor.value);if(problem){editorError.value=problem;return}
  if(editorIndex.value<0)config.value.targets.push(clone(editor.value));else config.value.targets[editorIndex.value]=clone(editor.value)
  editor.value=null;message.value='任务已加入草稿，请点击“保存设置”生效。'
}
async function remove(index:number){if(await confirmAction({title:'删除远程存储任务？',message:'仅删除任务配置，不会删除远程数据库中的数据。',detail:'已保存的任务需先补传完成并停用，再删除。',confirmText:'加入删除草稿',cancelText:'取消',danger:true}))config.value.targets.splice(index,1)}
async function test(){
  if(!editor.value)return
  const problem=validateRemoteTarget(editor.value,false);if(problem){editorError.value=problem;return}
  testing.value=true;testMessage.value='正在检查连接和专用表…';editorError.value=''
  try{const r=await api<{ok:boolean;message:string;durationMs:number}>('/api/remote-storage/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(editor.value),timeoutMs:(editor.value.timeoutSeconds+5)*1000});testMessage.value=r.message+'（'+r.durationMs+' ms）'}catch(e){testMessage.value='测试失败：'+String(e instanceof Error?e.message:e)}finally{testing.value=false}
}
async function loadPoints(reset=false){
  const request=++pointRequest;if(reset)offset.value=0;points.value=[]
  if(!device.value){total.value=0;return}
  pointsBusy.value=true
  try{const q=new URLSearchParams({deviceKey:device.value,search:search.value,offset:String(offset.value),limit:'100'});const r=await api<{total:number;points:Point[]}>('/api/history-storage/points?'+q);if(request!==pointRequest||disposed)return;points.value=r.points||[];total.value=r.total}catch(e){if(request===pointRequest)editorError.value='点位加载失败：'+String(e instanceof Error?e.message:e)}finally{if(request===pointRequest)pointsBusy.value=false}
}
function toggle(p:PointRef){if(!editor.value)return;const key=pointKey(p),i=editor.value.points.findIndex(x=>pointKey(x)===key);if(i>=0)editor.value.points.splice(i,1);else if(editor.value.points.length<2000)editor.value.points.push({deviceKey:p.deviceKey,metric:p.metric});else editorError.value='每个任务最多 2000 个点位。'}
function selectPage(){for(const p of points.value)if(!chosen.value.has(pointKey(p)))toggle(p)}
function page(delta:number){offset.value+=delta*100;void loadPoints()}
function time(value?:string){return value?new Date(value).toLocaleString():'—'}
function mb(bytes?:number){return ((bytes||0)/1024/1024).toFixed(2)+' MB'}
function stateLabel(t:RemoteTarget){const s=statusMap.value.get(t.id);if(!s)return '尚未应用';if(!s.enabled)return '已停用';if(s.cacheError)return '缓存异常';if(s.lastError)return '等待补传';if(!s.cacheReady)return '初始化';return s.connected?'写入正常':'等待采样'}
onMounted(()=>{void load();timer=window.setInterval(poll,5000)})
onBeforeUnmount(()=>{disposed=true;pointRequest++;window.clearInterval(timer)})
</script>

<template>
  <section class="card remote-storage">
    <header class="remote-head remote-page-head"><div class="remote-title"><span class="remote-title-icon"><IconDatabase :size="24" aria-hidden="true"/></span><div><h2>远程数据库存储</h2><p>定时写入第三方数据库，断网自动缓存，恢复后补传。</p></div></div><div class="actions"><span v-if="configDirty" class="draft-indicator">有未保存的修改</span><button :disabled="busy" @click="load">重新加载</button><template v-if="canManage"><button :disabled="!dirty||busy" @click="discard">放弃修改</button><button class="primary" :disabled="!configDirty||busy||!loaded" @click="save">{{busy?'处理中…':'保存设置'}}</button></template></div></header>
    <p v-if="error" class="notice bad" role="alert">{{error}}</p><p v-if="message" class="notice" role="status">{{message}}</p><p v-if="pollError" class="notice bad" role="alert">{{pollError}}</p>
    <section class="cache-settings" aria-label="断网缓存设置">
      <div class="cache-heading"><IconCloudUpload :size="18" aria-hidden="true"/><strong>断网缓存</strong><span>连接中断时保留待补传数据</span></div>
      <div class="cache-grid"><label>缓存位置<select v-model="config.storagePath" :disabled="!canManage||busy||!loaded" @change="selectVolume"><option :value="localPath">网关本地分区（空间有限）</option><option v-for="v in volumes.filter(v=>v.uuid&&!v.readOnly)" :key="v.uuid+v.mountPath" :value="v.mountPath">{{v.mountPath}} · 可用 {{mb(v.freeBytes)}}</option><option v-if="config.storagePath!==localPath&&!volumes.some(v=>v.mountPath===config.storagePath)" :value="config.storagePath">{{config.storagePath}}（当前未挂载）</option></select></label><label>每任务缓存上限（MB）<input v-model.number="config.maxSizeMB" :disabled="!canManage||busy||!loaded" type="number" min="4" max="1024" step="1"/></label><details class="cache-note"><summary><IconInfoCircle :size="15" aria-hidden="true"/>缓存保护说明</summary><p>缓存满或空间不足时停止接收新采样并记录数量，保留已有待补传数据。外部卷按 UUID 绑定，掉线不回退到系统分区。本功能不受本地历史记录开关影响。</p></details></div>
    </section>
    <div class="remote-head list-head"><div class="task-list-heading"><h3>存储任务 <small>{{config.targets.length}} / 4</small></h3><span v-if="loaded" class="task-overview"><i/>{{healthyCount}} 个写入正常<span class="overview-divider">/</span>待补传 {{totalPending}} 条</span></div><button v-if="canManage" class="add-task" :disabled="!loaded||busy||config.targets.length>=4" @click="edit(-1)"><IconPlus :size="15" aria-hidden="true"/>新增数据库任务</button></div>
    <div v-if="loaded&&!config.targets.length" class="remote-empty"><strong>暂无远程存储任务</strong><p>支持 MySQL、SQL Server 和 PostgreSQL。添加目标、选择点位后保存并启用。</p></div>
    <article v-for="(t,index) in config.targets" :key="t.id" class="remote-task" :class="'task-'+stateTone(t)">
      <header class="remote-head task-head"><div class="task-identity"><span class="database-icon" :class="'driver-'+t.driver"><IconDatabase :size="22" aria-hidden="true"/></span><div class="task-description"><div class="task-title-row"><h4>{{t.name}}</h4><span class="driver-badge">{{driverNames[t.driver]}}</span></div><div class="task-meta"><span class="task-destination" :title="t.host+':'+t.port+' / '+t.database+' / '+(t.schema?t.schema+'.':'')+t.table">{{t.host}}:{{t.port}} / {{t.database}} / {{t.schema?t.schema+'.':''}}{{t.table}}</span><span>{{t.points.length}} 个点位</span><span>每 {{t.intervalSeconds}} 秒存储</span></div></div></div><div class="task-controls"><span class="state-badge" :class="stateTone(t)"><i/>{{stateLabel(t)}}</span><div v-if="canManage" class="actions task-actions"><label class="task-switch"><input v-model="t.enabled" :disabled="busy" type="checkbox" role="switch" :aria-label="t.name+' 启用任务'"/><span class="switch-track" aria-hidden="true"/><span>{{t.enabled?'启用':'停用'}}</span></label><button :disabled="busy" @click="edit(index)">编辑</button><button class="danger" :disabled="busy" @click="remove(index)">删除</button></div></div></header>
      <div class="stats"><div><small>待补传</small><strong :class="{pending:statusMap.get(t.id)?.pending}">{{statusMap.get(t.id)?.pending??'—'}}<em>条</em></strong></div><div class="cache-stat"><small>缓存载荷</small><strong>{{mb(statusMap.get(t.id)?.bytes)}}</strong><div class="cache-meter" role="meter" aria-label="缓存载荷额度使用比例" :aria-valuenow="cachePercent(t)" :aria-valuemin="0" :aria-valuemax="100" :title="'载荷额度约 '+config.maxSizeMB/2+' MB，另预留数据库与日志空间'"><span :style="{width:cachePercent(t)+'%'}"/></div></div><div><small>本次运行已发送</small><strong>{{statusMap.get(t.id)?.sent??'—'}}<em>条</em></strong></div><div><small>本次运行未入队</small><strong :class="{bad:statusMap.get(t.id)?.dropped}">{{statusMap.get(t.id)?.dropped??'—'}}<em>条</em></strong></div><div class="last-write"><small>最后成功写入</small><strong>{{time(statusMap.get(t.id)?.lastSuccess)}}</strong></div></div>
      <p v-if="statusMap.get(t.id)?.cacheError" class="error-text">{{statusMap.get(t.id)?.cacheError}}</p><p v-if="statusMap.get(t.id)?.lastError" class="error-text">{{statusMap.get(t.id)?.lastError}}<small> 下次重试：{{time(statusMap.get(t.id)?.nextRetry)}}</small></p>
    </article>
    <details class="help"><summary>存储与兼容性说明</summary><p>使用独立专用表，一行记录一个点位的一次存储采样，包括网关、设备、标识符、采样/接收/源时间、原始值、实际类型、质量和诊断。64 位整数用精确十进制文本保存，避免丢失精度。</p><p>重试使用同一记录编号去重。存储周期并非采集周期；每次保存当时最新的点位状态，不保证记录两次存储之间的全部变化。</p><p>更换数据库、表或缓存卷前，请先完成补传并单独保存为停用状态，再修改。停止任务会同时暂停采样和补传。缓存上限包含数据库文件，载荷额度约为上限的一半，另需预留日志及系统空间。</p><p>现阶段为三种数据库的专用表适配，不执行自定义 SQL，也不会迁移已有历史库。国产数据库需要增加对应驱动、SQL 方言及实库兼容测试，不能仅凭协议兼容认定已支持。</p></details>
    <Teleport to="body"><div v-if="editor" class="remote-modal" @keydown.esc.stop.prevent="closeEditor"><form class="remote-dialog" role="dialog" aria-modal="true" aria-labelledby="remote-dialog-title" @submit.prevent="applyEditor"><header class="remote-head"><h2 id="remote-dialog-title">{{editorIndex<0?'新增':'编辑'}}数据库任务</h2><button type="button" :disabled="testing" aria-label="关闭任务编辑" @click="closeEditor">×</button></header><div class="remote-body">
      <p v-if="editorError" class="notice bad" role="alert">{{editorError}}</p>
      <fieldset :disabled="testing||!canManage" class="remote-fields"><label>任务名称<input v-model="editor.name" required maxlength="80"/></label><label>数据库类型<select v-model="editor.driver" @change="changeRemoteDriver(editor)"><option v-for="(name,key) in driverNames" :key="key" :value="key">{{name}}</option></select></label><label>主机 / IP<input v-model.trim="editor.host" required placeholder="例如 db.example.com"/></label><label>端口<input v-model.number="editor.port" required type="number" min="1" max="65535"/></label><label>数据库名称<input v-model.trim="editor.database" required maxlength="128"/></label><label v-if="editor.driver!=='mysql'">模式名<input v-model.trim="editor.schema" required maxlength="48"/></label><label>专用表名<input v-model.trim="editor.table" required maxlength="48"/></label><label>数据库账号<input v-model.trim="editor.username" required autocomplete="off"/></label><label>数据库密码<input v-model="editor.password" type="password" autocomplete="new-password" placeholder="*** 表示保留已保存密码"/></label><label>存储周期（秒）<input v-model.number="editor.intervalSeconds" type="number" min="1" max="86400" required/></label><label>每批最多记录数<input v-model.number="editor.batchSize" type="number" min="1" max="500" required/></label><label>数据库请求超时（秒）<input v-model.number="editor.timeoutSeconds" type="number" min="2" max="60" required/></label><label>传输安全<select v-model="editor.tlsMode"><option value="verify-full">TLS · 验证服务器证书</option><option value="disabled">不加密 · 仅可信隔离网络</option></select></label><label class="wide">CA 证书路径（可选）<input v-model.trim="editor.caFile" placeholder="网关上的绝对路径；留空使用驱动默认信任库"/></label></fieldset>
      <div class="remote-options"><label><input v-model="editor.enabled" :disabled="testing" type="checkbox"/>保存后启用任务</label><label><input v-model="editor.createTable" :disabled="testing" type="checkbox"/>启用后自动创建专用表（需要建表权限）</label></div>
      <div class="picker"><header class="remote-head"><strong>选择点位 · 已选 {{editor.points.length}} / 2000</strong><button type="button" :disabled="testing||!editor.points.length" @click="editor.points=[]">清空选择</button></header><div class="point-filter"><select v-model="device" aria-label="筛选设备" @change="loadPoints(true)"><option value="">选择设备</option><option v-for="g in groups" :key="g.deviceKey" :value="g.deviceKey">{{g.name||g.deviceKey}}（{{g.count}}）</option></select><input v-model="search" placeholder="搜索名称或标识符" aria-label="搜索点位" @keydown.enter.prevent="loadPoints(true)"/><button type="button" @click="loadPoints(true)">搜索</button><button type="button" :disabled="testing||pointsBusy" @click="selectPage">选择本页</button></div><div class="point-list"><label v-for="p in points" :key="pointKey(p)"><input :checked="chosen.has(pointKey(p))" :disabled="testing" type="checkbox" @change="toggle(p)"/><span>{{p.name||p.metric}}<small>{{p.metric}} {{p.unit}}</small></span></label><p v-if="!points.length">{{pointsBusy?'正在加载…':'该设备没有匹配点位'}}</p></div><footer class="actions"><small>共 {{total}} 项，当前 {{Math.floor(offset/100)+1}} 页</small><button type="button" :disabled="offset===0||pointsBusy" @click="page(-1)">上一页</button><button type="button" :disabled="offset+100>=total||pointsBusy" @click="page(1)">下一页</button></footer><details v-if="editor.points.length"><summary>查看全部已选点位（跨设备）</summary><div class="selected-points"><span v-for="p in editor.points" :key="pointKey(p)">{{p.deviceKey}} / {{p.metric}}<button type="button" :disabled="testing" :aria-label="'移除 '+p.metric" @click="toggle(p)">×</button></span></div></details></div>
      <p class="test-note">测试连接只验证连接和已存在的专用表，不建表、不写数据。首次未建表时可能提示表不存在；自动建表仅在保存且启用任务后执行。</p><p v-if="testMessage" class="notice" role="status">{{testMessage}}</p>
    </div><footer class="remote-head"><button type="button" :disabled="testing" @click="test">{{testing?'测试中…':'测试连接（只读）'}}</button><div class="actions"><button type="button" :disabled="testing" @click="closeEditor">取消</button><button class="primary" :disabled="testing" type="submit">加入草稿</button></div></footer></form></div></Teleport>
  </section>
</template>

<style scoped>
.remote-storage button,.remote-dialog button{height:34px;padding:0 12px;border:1px solid #d8e3f3;border-radius:7px;background:#fff;color:#244267;font:inherit;font-size:12px;cursor:pointer;white-space:nowrap}.remote-storage button.primary,.remote-dialog button.primary{background:#1677ff;border-color:#1677ff;color:#fff}.remote-storage button.danger{color:#c24141}.remote-storage button:disabled,.remote-dialog button:disabled{opacity:.5;cursor:not-allowed}.remote-storage input:not([type=checkbox]),.remote-storage select,.remote-dialog input:not([type=checkbox]),.remote-dialog select{height:34px;padding:0 10px;border:1px solid #d8e3f3;border-radius:7px;background:#fff;color:#244267;font:inherit;font-size:12px;box-shadow:none}.remote-storage input:focus-visible,.remote-storage select:focus-visible,.remote-dialog input:focus-visible,.remote-dialog select:focus-visible,.remote-dialog button:focus-visible{outline:2px solid #1677ff;outline-offset:2px}.remote-storage input:disabled,.remote-storage select:disabled{background:#f1f5f9}.remote-dialog h2{font-size:18px}
.remote-storage{margin-top:12px;min-width:0}.remote-head{display:flex;align-items:center;justify-content:space-between;gap:14px}.remote-head h2{margin:0}.remote-head p,.cache-grid p,.help p,.test-note{color:#64748b;font-size:12px;line-height:1.6;margin:6px 0}.actions{display:flex;align-items:center;gap:8px;flex-shrink:0}.cache-grid{display:grid;grid-template-columns:minmax(220px,1.2fr) 190px minmax(240px,1.4fr);gap:16px;align-items:end;padding:16px;margin:18px 0;border:1px solid #e1e8f3;border-radius:10px;background:#f8faff}.cache-grid label,.remote-fields label{display:grid;gap:6px;font-size:12px;color:#475569}.cache-grid input,.cache-grid select,.remote-fields input,.remote-fields select{width:100%;min-width:0;box-sizing:border-box}.list-head{margin:14px 0}.list-head small{color:#64748b;margin-left:8px}.remote-empty{text-align:center;padding:42px 18px;color:#64748b;border:1px dashed #d6e0ef;border-radius:10px}.remote-task{padding:16px;border:1px solid #dce5f1;border-radius:10px;margin:10px 0}.badge{display:inline-block;padding:3px 8px;margin-left:8px;font-size:11px;color:#1d4ed8;background:#edf5ff;border-radius:5px}.stats{display:grid;grid-template-columns:repeat(4,1fr) 1.6fr;gap:8px;margin-top:12px;padding-top:12px;border-top:1px solid #edf2f7}.stats small,.stats strong{display:block}.stats small{font-size:11px;color:#64748b;margin-bottom:4px}.stats strong{font-size:14px;color:#1e293b}.bad,.stats .bad,.error-text{color:#b91c1c!important}.error-text{font-size:12px;overflow-wrap:anywhere}.enable,.remote-options label{display:flex;align-items:center;gap:7px;font-size:12px}.enable input,.remote-options input,.point-list input{width:15px;min-width:15px;height:15px;padding:0;margin:0}.help{margin-top:18px;font-size:12px}.help summary,.picker summary{cursor:pointer;color:#476182}.remote-modal{position:fixed;inset:0;z-index:180;background:#0f172a70;display:grid;place-items:center;padding:20px}.remote-dialog{display:grid;grid-template-rows:auto minmax(0,1fr) auto;width:min(1040px,96vw);max-height:94vh;overflow:hidden;background:#fff;border-radius:14px;border:1px solid #d6e0ef;box-shadow:0 22px 65px #0f172a40}.remote-dialog>header,.remote-dialog>footer{padding:14px 20px;background:#f8faff}.remote-dialog>header{border-bottom:1px solid #e1e8f3}.remote-dialog>footer{border-top:1px solid #e1e8f3}.remote-body{padding:18px 20px;overflow:auto}.remote-fields{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;border:0;padding:0;margin:0}.remote-fields .wide{grid-column:span 3}.remote-options{display:flex;gap:22px;flex-wrap:wrap;margin:16px 0}.picker{border:1px solid #dce5f1;border-radius:9px;overflow:hidden}.picker>header,.picker>footer,.picker>details{padding:10px 12px;background:#f8faff;font-size:12px}.point-filter{display:flex;gap:8px;padding:10px}.point-filter select{width:35%;min-width:0}.point-filter input{flex:1;min-width:0}.point-list{max-height:220px;overflow:auto;display:grid;grid-template-columns:1fr 1fr;padding:0 10px 10px;gap:4px}.point-list label{display:flex;align-items:center;gap:8px;padding:8px;border-radius:5px;font-size:12px}.point-list label:hover{background:#eff6ff}.point-list span{overflow:hidden;overflow-wrap:anywhere}.point-list small{display:block;color:#94a3b8;font-size:10px;margin-top:3px}.picker>footer{justify-content:flex-end}.picker>footer small{margin-right:auto;color:#64748b}.selected-points{max-height:160px;overflow:auto;display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}.selected-points span{display:flex;align-items:center;gap:4px;padding:3px 6px;border:1px solid #e1e8f3;border-radius:4px}.selected-points button{padding:0 5px;min-height:24px;height:24px}.test-note{margin-top:12px}@media(max-width:1000px){.cache-grid{grid-template-columns:1fr 180px}.cache-grid p{grid-column:1/-1}.remote-head{flex-wrap:wrap}.stats{grid-template-columns:repeat(3,1fr)}}@media(max-width:700px){.remote-fields{grid-template-columns:repeat(2,minmax(0,1fr))}.remote-fields .wide{grid-column:span 2}.point-filter{flex-wrap:wrap}.point-filter select{width:100%}.point-list{grid-template-columns:1fr}.cache-grid{grid-template-columns:1fr}.stats{grid-template-columns:repeat(2,1fr)}.remote-modal{padding:8px}.remote-dialog{width:100%;max-height:97vh}.remote-body{padding:12px}}
</style>


<style scoped>
/* Keep nested card headers independent of the application's sticky top bar. */
.remote-storage .remote-head,.remote-dialog .remote-head{position:static;top:auto;z-index:auto;height:auto;min-height:0;backdrop-filter:none;box-shadow:none}
.remote-page-head{padding:0 0 18px;background:transparent;border-bottom:1px solid #e7edf5}
.remote-title{display:flex;align-items:center;gap:12px;min-width:0}
.remote-title-icon{display:grid;place-items:center;flex-shrink:0;width:44px;height:44px;border-radius:12px;color:#1677ff;background:#edf5ff}
.remote-page-head h2{font-size:21px;letter-spacing:-.4px}.remote-page-head p{margin:5px 0 0}
.draft-indicator{font-size:12px;color:#a16207}
.cache-settings{display:grid;grid-template-columns:145px minmax(0,1fr);align-items:center;gap:16px;margin:16px 0 20px;padding:12px 16px;background:#f8faff;border:1px solid #e3ebf7;border-radius:8px}
.cache-heading{display:flex;align-items:center;flex-wrap:wrap;gap:6px;color:#385b85;font-size:13px}.cache-heading span{flex-basis:100%;font-size:11px;font-weight:400;color:#77869b;line-height:1.5}
.cache-grid{grid-template-columns:minmax(200px,1fr) 170px minmax(130px,.6fr);margin:0;padding:0;border:0;background:transparent;gap:14px;align-items:start}
.cache-note{align-self:center;color:#64748b;font-size:12px}.cache-note summary{display:flex;align-items:center;gap:5px;cursor:pointer;list-style:none}.cache-note summary::after{content:'展开';margin-left:3px;font-size:11px;color:#1677ff}.cache-note[open] summary::after{content:'收起'}.cache-note p{margin:8px 0 0}
.task-list-heading{display:flex;align-items:center;flex-wrap:wrap;gap:16px}.task-list-heading h3{margin:0;font-size:15px}.task-list-heading h3 small{padding:3px 7px;background:#f1f5fa;border-radius:5px;font-size:11px;font-weight:500}
.task-overview{display:inline-flex;align-items:center;gap:6px;font-size:12px;color:#64748b}.task-overview i{width:6px;height:6px;border-radius:50%;background:#94a3b8}.overview-divider{margin:0 5px;color:#cbd5e1}
.add-task{display:inline-flex;align-items:center;gap:5px}
.remote-task{padding:0;margin:10px 0;border-color:#e0e8f2;border-radius:8px;overflow:hidden;background:#fff;box-shadow:none}
.remote-task.task-failed{border-color:#fecaca}.remote-task.task-warning{border-color:#f6d8a9}
.remote-task .task-head{padding:12px 16px 10px;border:0;background:transparent;gap:16px}
.task-identity{display:flex;align-items:center;gap:10px;min-width:0;flex:1}.database-icon{display:grid;place-items:center;width:24px;height:32px;flex-shrink:0;color:#3675b3}
.database-icon.driver-mysql{color:#a36c13}.database-icon.driver-sqlserver{color:#7661b1}
.task-controls{display:flex;align-items:center;gap:16px;flex-shrink:0}.task-controls>.state-badge{min-width:80px;justify-content:center}
.task-actions{padding-left:16px;border-left:1px solid #e8edf4}
.task-description{min-width:0;flex:1}.task-title-row{display:flex;align-items:center;flex-wrap:wrap;gap:8px}.task-title-row h4{margin:0;font-size:14px;font-weight:650;color:#1e293b;overflow-wrap:anywhere}
.driver-badge{padding:2px 6px;border-radius:4px;color:#65768b;background:#f1f5f9;font-size:10px;font-weight:600}
.state-badge{display:inline-flex;align-items:center;gap:5px;padding:3px 7px;border-radius:5px;font-size:11px;white-space:nowrap;color:#64748b;background:#f1f5f9}.state-badge i{width:5px;height:5px;border-radius:50%;background:currentColor}
.state-badge.healthy{color:#16814b;background:#eaf8f0}.state-badge.warning{color:#a16207;background:#fff5df}.state-badge.failed{color:#b91c1c;background:#fef0f0}
.task-meta{display:flex;align-items:center;gap:12px;min-width:0;margin-top:7px;font-size:11px;color:#738399}.task-meta>span{white-space:nowrap}.task-destination{overflow:hidden;text-overflow:ellipsis}.task-meta>span+span{border-left:1px solid #dfe7f0;padding-left:12px}
.task-switch{display:inline-flex;align-items:center;gap:7px;position:relative;cursor:pointer;font-size:12px;color:#64748b;margin-right:5px}.task-switch input{position:absolute;opacity:0;width:32px;height:22px;inset:0;margin:0;cursor:pointer}.switch-track{width:30px;height:17px;border-radius:20px;background:#b7c5d6;position:relative;transition:background .15s}.switch-track::after{content:'';position:absolute;left:2px;top:2px;width:13px;height:13px;border-radius:50%;background:#fff;box-shadow:0 1px 2px #0002;transition:transform .15s}.task-switch input:checked+.switch-track{background:#1677ff}.task-switch input:checked+.switch-track::after{transform:translateX(13px)}.task-switch input:focus-visible+.switch-track{outline:2px solid #1677ff;outline-offset:3px}.task-switch input:disabled~span{opacity:.5;cursor:not-allowed}
.stats{grid-template-columns:.7fr .9fr 1fr 1fr 1.5fr;gap:12px;margin:0 16px;padding:10px 0 12px;background:transparent;border-top:1px solid #edf2f8}
.stats>div{display:flex;align-items:center;gap:9px;min-width:0;padding:0;border:0}.stats small{font-size:11px;margin:0;white-space:nowrap}.stats strong{font-size:14px;font-weight:650;font-variant-numeric:tabular-nums;white-space:nowrap}.stats strong em{font-size:10px;font-style:normal;font-weight:400;margin-left:4px;color:#94a3b8}.stats .pending{color:#b77916}.stats .last-write strong{font-size:11px;line-height:22px;white-space:normal;overflow-wrap:anywhere}
.cache-stat{position:relative;flex-wrap:wrap;row-gap:0!important}.cache-meter{height:2px;width:100%;max-width:100px;background:#edf2f8;border-radius:5px;overflow:hidden}.cache-meter span{display:block;height:100%;background:#78a8e8;border-radius:5px}
.remote-task .error-text{margin:0;padding:10px 16px;background:#fff8f5;border-top:1px solid #fde6da;line-height:1.6}
.task-switch .switch-track{pointer-events:none}
.help{padding:8px 0;margin-top:12px;background:transparent;border:0}.help summary{font-weight:500}
@media(max-width:1100px){.task-meta{flex-wrap:wrap;gap:7px}.task-destination{flex-basis:100%}.task-meta>span:nth-child(2){border:0;padding:0}.remote-page-head{flex-wrap:wrap}.cache-grid{grid-template-columns:minmax(0,1fr) 190px}.cache-note{grid-column:1/-1}.task-overview{font-size:11px}}
@media(max-width:700px){.remote-title-icon{width:36px;height:36px}.remote-page-head h2{font-size:18px}.remote-page-head>.actions{flex-wrap:wrap;flex-shrink:1}.cache-grid{grid-template-columns:1fr}.cache-heading{flex-wrap:wrap}.cache-heading span{flex-basis:100%;padding-left:26px}.task-head{flex-wrap:wrap}.task-identity{flex-basis:100%}.task-actions{margin-left:auto}.stats{grid-template-columns:repeat(2,minmax(0,1fr));gap:14px 0}.stats>div:nth-child(odd){border-left:0;padding-left:0}.stats .last-write{grid-column:1/-1}.task-meta>span{white-space:normal;overflow-wrap:anywhere}.task-meta .task-destination{word-break:break-word}.list-head{align-items:center}.task-list-heading{gap:6px}.task-overview{flex-basis:100%}}
</style>
<style scoped>
@media(max-width:1200px){
  .stats{grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}
  .stats .last-write{grid-column:1/-1}
  .task-controls{gap:10px}.task-actions{padding-left:10px}
}
@media(max-width:900px){
  .cache-settings{grid-template-columns:1fr;gap:12px}
  .cache-heading span{flex-basis:auto;margin-left:4px}
  .task-head{flex-wrap:wrap}.task-identity{flex-basis:100%}
  .task-controls{width:100%;justify-content:space-between}
}
@media(max-width:700px){
  .stats{grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}
  .stats>div{display:block}.stats small{margin-bottom:5px}
  .stats .last-write{display:flex;flex-wrap:wrap;gap:5px}
  .stats .last-write small{margin:0}.cache-meter{margin-top:4px}
  .cache-settings .cache-heading span{padding-left:0}
  .task-controls{gap:8px}.task-actions{gap:6px}
}
</style>
