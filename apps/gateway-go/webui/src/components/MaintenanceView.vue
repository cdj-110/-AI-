<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { IconChevronDown } from '@tabler/icons-vue'
import { api } from '../api'
import {useFormDraftGuard} from '../composables/useFormDraftGuard.ts'
import type { AuditEvent } from '../types'
import DeviceLicensePanel from './DeviceLicensePanel.vue'

type MaintenancePanel='time'|'license'|'diagnostics'|'remote'|'audit'|'operations'
type SSHAccessStatus={supported:boolean;enabled:boolean;running:boolean;service?:string;initScript?:string;port:number;message?:string}
const activePanel=ref<MaintenancePanel>('time')
type CurrentSession={username:string;permissions:string[];mfaEnabled?:boolean}
type ApplicationVersion={version:string;release?:number;buildTime?:string;commit?:string;goVersion?:string}
const currentSession=ref<CurrentSession>()
const applicationVersion=ref<ApplicationVersion>({version:'V0.2'})
const legacyMaintenancePermissions=['maintenance.view','maintenance.diagnose','maintenance.time','maintenance.remote','maintenance.restart','maintenance.factory_reset','maintenance.audit']
function can(permission:string){const granted=currentSession.value?.permissions;if(!granted)return true;return granted.includes('*')||granted.includes(permission)||(granted.includes('maintenance.run')&&legacyMaintenancePermissions.includes(permission))}
const canViewLicense=computed(()=>!!currentSession.value&&(can('maintenance.view')||can('cloud.manage')))
const target = ref('www.baidu.com')
const count = ref(4)
const pinging = ref(false)
const pingOutput = ref('暂无结果')
const events = ref<AuditEvent[]>([])
const auditError = ref('')
const modal = ref<''|'restart'|'reboot'|'ssh-enable'|'ssh-disable'|'factory-confirm'>('')
const verification=reactive({currentPassword:'',totpCode:''})
const actionError=ref('')
const verificationReady=computed(()=>verification.currentPassword.length>0&&(!currentSession.value?.mfaEnabled||verification.totpCode.trim().length===6))
function resetVerification(){verification.currentPassword='';verification.totpCode='';actionError.value=''}
function openSensitiveModal(value:'restart'|'reboot'|'ssh-enable'|'ssh-disable'|'factory-confirm'){resetVerification();modal.value=value}
type RestartKind='service'|'gateway'
const restartState=reactive({active:false,kind:'service' as RestartKind,phase:'sending' as 'sending'|'offline'|'waiting'|'online'|'timeout',elapsed:0,error:''})
let restartProbeTimer=0
let restartClockTimer=0
let restartStartedAt=0
let restartSawOffline=false
let restartHealthyCount=0
let restartPreviousBootID=''
type TimeSyncStatus={
  config:{enabled:boolean;timezone:string;primaryServer:string;secondaryServer:string}
  systemTime:string
  synchronized:boolean
  supported:boolean
  stratum?:number
  reference?:string
  lastOffsetSeconds?:number
  systemOffsetSeconds?:number
  leapStatus?:string
  message?:string
}
const timeStatus=ref<TimeSyncStatus>()
const timeForm=reactive({enabled:true,timezone:'Asia/Shanghai',primaryServer:'pool.ntp.org',secondaryServer:''})
const timeDraft=useFormDraftGuard(()=>timeForm,value=>Object.assign(timeForm,value),saveTime,'#panel-maintenance .maintenance-tabs button')
const timeBusy=ref(false)
const timeMessage=ref('')
const timeError=ref('')
const sshStatus=ref<SSHAccessStatus>()
const sshBusy=ref(false)
const sshMessage=ref('')
const sshError=ref('')
const sshDetailsOpen=ref(false)
const browserNow=ref(new Date())
const clockOffsetMs=ref(0)
let clockTimer=0
const gatewayNow=computed(()=>new Date(browserNow.value.getTime()+clockOffsetMs.value))
const clockDifference=computed(()=>{
  const seconds=Math.round(clockOffsetMs.value/1000)
  if(Math.abs(seconds)<1)return'时间基本一致'
  return seconds>0?`网关快 ${seconds} 秒`:`网关慢 ${Math.abs(seconds)} 秒`
})
const auditSearch=ref('')
const auditResult=ref('all')
const auditPage=ref(1)
const auditPageSize=20
const filteredEvents=computed(()=>events.value.filter(event=>{
  const search=auditSearch.value.trim().toLowerCase()
  const matchesResult=auditResult.value==='all'||event.result===auditResult.value
  const haystack=`${event.username||''} ${event.remoteIp||''} ${event.method} ${event.path} ${event.detail||''}`.toLowerCase()
  return matchesResult&&(!search||haystack.includes(search))
}))
const auditPages=computed(()=>Math.max(1,Math.ceil(filteredEvents.value.length/auditPageSize)))
const visibleEvents=computed(()=>filteredEvents.value.slice((auditPage.value-1)*auditPageSize,auditPage.value*auditPageSize))
watch([auditSearch,auditResult],()=>auditPage.value=1)
function formatClock(value:Date){return value.toLocaleString('zh-CN',{hour12:false})}
function formatAuditTime(value:string){const date=new Date(value);return Number.isNaN(date.getTime())?value:date.toLocaleString('zh-CN',{hour12:false})}
function auditResultText(value:string){return value==='success'?'成功':value==='failed'?'失败':value||'-'}

async function loadTime(){
  try{
    const result=await api<TimeSyncStatus>('/api/maintenance/time')
    timeStatus.value=result
    Object.assign(timeForm,result.config)
    timeDraft.markSaved()
    clockOffsetMs.value=new Date(result.systemTime).getTime()-Date.now()
    timeError.value=''
  }catch(error){timeError.value=error instanceof Error?error.message:'时间同步状态读取失败'}
}
async function saveTime(){
  timeBusy.value=true;timeMessage.value='';timeError.value=''
  try{
    const result=await api<TimeSyncStatus>('/api/maintenance/time',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(timeForm),timeoutMs:30000})
    timeStatus.value=result;clockOffsetMs.value=new Date(result.systemTime).getTime()-Date.now();timeMessage.value='日期与时间配置已保存并应用。'
    timeDraft.markSaved()
  }catch(error){timeError.value=error instanceof Error?error.message:'时间配置保存失败'}
  finally{timeBusy.value=false}
}
async function timeAction(action:'test'|'sync'){
  timeBusy.value=true;timeMessage.value='';timeError.value=''
  try{
    const result=await api<TimeSyncStatus>('/api/maintenance/time',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({action,primaryServer:timeForm.primaryServer,secondaryServer:timeForm.secondaryServer}),timeoutMs:action==='sync'?30000:20000})
    timeStatus.value=result;clockOffsetMs.value=new Date(result.systemTime).getTime()-Date.now();timeMessage.value=action==='test'?'NTP 服务器响应正常。':'网关时间已同步。'
  }catch(error){timeError.value=error instanceof Error?error.message:(action==='test'?'NTP 服务器检测失败':'立即校时失败')}
  finally{timeBusy.value=false}
}

async function loadSSH(){
  try{sshStatus.value=await api<SSHAccessStatus>('/api/maintenance/ssh');sshError.value=''}
  catch(error){sshError.value=error instanceof Error?error.message:'SSH 状态读取失败'}
}
async function loadApplicationVersion(){
  try{applicationVersion.value=await api<ApplicationVersion>('/api/system/version')}
  catch{/* The compiled UI version remains visible when the detail request fails. */}
}
async function toggleSSH(){
  if(!sshStatus.value?.supported||sshBusy.value)return
  const enabled=!sshStatus.value.enabled
  openSensitiveModal(enabled?'ssh-enable':'ssh-disable')
}
async function applySSH(enabled:boolean){
  if(!verificationReady.value)return
  sshBusy.value=true;sshMessage.value='';sshError.value='';actionError.value=''
  try{
    sshStatus.value=await api<SSHAccessStatus>('/api/maintenance/ssh',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({enabled,...verification}),timeoutMs:20000})
    sshMessage.value=enabled?'SSH 已启用并设置为开机启动。':'SSH 已关闭并取消开机启动。'
    modal.value='';resetVerification()
  }catch(error){sshError.value=error instanceof Error?error.message:(enabled?'SSH 启用失败':'SSH 关闭失败')}
  finally{sshBusy.value=false}
}

async function ping() {
  pinging.value = true
  pingOutput.value = '执行中...'
  try {
    const result = await api<{ ok: boolean; output: string }>('/api/maintenance/ping', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ target: target.value, count: count.value }), timeoutMs: 40000 })
    pingOutput.value = result.output || (result.ok ? '目标可达' : '目标不可达')
  } catch (error) {
    pingOutput.value = error instanceof Error ? error.message : 'Ping 执行失败'
  } finally { pinging.value = false; if(can('maintenance.audit'))await loadAudit() }
}

async function loadAudit() {
  try {
    events.value = (await api<{ events: AuditEvent[] }>('/api/maintenance/audit-log?limit=500')).events || []
    auditError.value = ''
  } catch (error) { auditError.value = error instanceof Error ? error.message : '审计日志读取失败' }
}

const restartTitle=computed(()=>restartState.kind==='service'?'网关服务正在重启':'网关设备正在重启')
const restartDescription=computed(()=>{
  if(restartState.phase==='online')return restartState.kind==='service'?'服务已恢复，正在刷新管理页面（可能需要重新登录）…':'网关已重新上线，正在刷新管理页面（可能需要重新登录）…'
  if(restartState.phase==='timeout')return'暂未检测到恢复，请确认网关供电和网络连接后继续重试。'
  if(restartState.phase==='sending')return restartState.kind==='service'?'正在提交服务重启命令…':'正在提交整机重启命令…'
  if(restartState.phase==='offline')return restartState.kind==='service'?'服务已断开，正在等待进程重新启动…':'设备已离线，正在等待系统完成启动…'
  return restartState.kind==='service'?'正在检测服务是否恢复，请稍候…':'系统启动通常需要 30–90 秒，请保持设备供电。'
})
function stopRestartTimers(){window.clearTimeout(restartProbeTimer);window.clearInterval(restartClockTimer)}
async function probeRestartRecovery(){
  if(!restartState.active||restartState.phase==='online')return
  restartState.elapsed=Math.floor((Date.now()-restartStartedAt)/1000)
  try{
    // The public readiness endpoint exposes only lifecycle state and a boot ID.
    // It remains available after in-memory sessions are cleared by a restart.
    const response=await fetch(`/api/readyz?restart_probe=${Date.now()}`,{cache:'no-store',signal:AbortSignal.timeout(2500)})
    const status=await response.json() as {ready?:boolean;bootId?:string}
    if(!response.ok||!status.ready||!status.bootId||(restartPreviousBootID&&status.bootId===restartPreviousBootID))throw new Error('not ready')
    restartHealthyCount++
    const minimum=restartState.kind==='service'?4:12
    const canRecover=restartState.kind==='service'||restartSawOffline
    if(restartState.elapsed>=minimum&&canRecover&&restartHealthyCount>=2){
      restartState.phase='online'
      window.clearInterval(restartClockTimer)
      restartProbeTimer=window.setTimeout(()=>window.location.reload(),1200)
      return
    }
    restartState.phase=restartSawOffline?'waiting':'sending'
  }catch{
    restartSawOffline=true
    restartHealthyCount=0
    restartState.phase='offline'
  }
  if(restartState.elapsed>=180){restartState.phase='timeout';window.clearInterval(restartClockTimer)}
  else restartProbeTimer=window.setTimeout(probeRestartRecovery,1500)
}
async function beginRestart(kind:RestartKind,path:string) {
  if(!verificationReady.value)return
  modal.value=''
  stopRestartTimers()
  Object.assign(restartState,{active:true,kind,phase:'sending',elapsed:0,error:''})
  restartStartedAt=Date.now();restartSawOffline=false;restartHealthyCount=0
  restartClockTimer=window.setInterval(()=>restartState.elapsed=Math.floor((Date.now()-restartStartedAt)/1000),1000)
  // Paint the waiting screen before the request can take the gateway offline.
  // Two animation frames are intentional: one applies Vue's DOM update and the
  // second lets the browser composite the loading layer on slower EG100 CPUs.
  await nextTick()
  await new Promise<void>(resolve=>{
    const fallback=window.setTimeout(resolve,150)
    window.requestAnimationFrame(()=>window.requestAnimationFrame(()=>{window.clearTimeout(fallback);resolve()}))
  })
  try {
    try{const ready=await fetch(`/api/readyz?before=${Date.now()}`,{cache:'no-store'});const body=await ready.json() as {bootId?:string};restartPreviousBootID=body.bootId||''}catch{restartPreviousBootID=''}
    await api(path, { method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(verification),timeoutMs:6000 })
  } catch (error) {
    if(!(error instanceof TypeError)&&!(error instanceof DOMException&&error.name==='AbortError')){
      stopRestartTimers();restartState.active=false
      actionError.value=error instanceof Error?error.message:'重启命令提交失败'
      modal.value=kind==='service'?'restart':'reboot'
      return
    }
  }
  restartProbeTimer=window.setTimeout(probeRestartRecovery,1200)
}

async function factoryReset() {
  if(!verificationReady.value)return
  try {
    await api('/api/maintenance/factory-reset', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({verification}) })
    modal.value=''
    window.location.reload()
  } catch (error) { actionError.value=error instanceof Error ? error.message : '恢复出厂设置失败' }
}

function panelAllowed(panel:MaintenancePanel){if(panel==='license')return canViewLicense.value;if(panel==='time')return can('maintenance.time');if(panel==='diagnostics')return can('maintenance.diagnose');if(panel==='remote')return can('maintenance.remote');if(panel==='audit')return can('maintenance.audit');return can('maintenance.restart')||can('maintenance.factory_reset')}
async function loadSession(){try{currentSession.value=await api<CurrentSession>('/api/session');if(!panelAllowed(activePanel.value))activePanel.value=(['time','license','diagnostics','remote','audit','operations'] as MaintenancePanel[]).find(panelAllowed)||'time';if(can('maintenance.time'))void loadTime();if(can('maintenance.remote'))void loadSSH()}catch{/* route-level authorization remains authoritative */}}
watch(activePanel,panel=>{if(panel==='audit')void loadAudit()})
onMounted(()=>{void loadSession();void loadApplicationVersion();clockTimer=window.setInterval(()=>browserNow.value=new Date(),1000)})
onBeforeUnmount(()=>{window.clearInterval(clockTimer);stopRestartTimers()})
</script>

<template>
  <section id="panel-maintenance"><section class="card"><div class="section-title maintenance-page-title"><div><h2>系统维护</h2><p class="meta">集中管理设备授权、网关时间、诊断工具、远程访问和维护操作。</p></div><div class="maintenance-app-version" aria-label="网关应用版本"><span>应用版本</span><strong>{{applicationVersion.version}}</strong><i v-if="applicationVersion.release" aria-hidden="true"></i><small v-if="applicationVersion.release">发布序号 {{applicationVersion.release}}</small></div></div>
    <div class="sub-tabs maintenance-tabs" role="tablist" aria-label="系统维护功能">
      <button v-if="can('maintenance.time')" :class="{active:activePanel==='time'}" role="tab" :aria-selected="activePanel==='time'" @click="activePanel='time'">日期与时间</button>
      <button v-if="canViewLicense" :class="{active:activePanel==='license'}" role="tab" :aria-selected="activePanel==='license'" @click="activePanel='license'">设备授权</button>
      <button v-if="can('maintenance.diagnose')" :class="{active:activePanel==='diagnostics'}" role="tab" :aria-selected="activePanel==='diagnostics'" @click="activePanel='diagnostics'">诊断工具</button>
      <button v-if="can('maintenance.remote')" :class="{active:activePanel==='remote'}" role="tab" :aria-selected="activePanel==='remote'" @click="activePanel='remote'">远程访问</button>
      <button v-if="can('maintenance.audit')" :class="{active:activePanel==='audit'}" role="tab" :aria-selected="activePanel==='audit'" @click="activePanel='audit'">审计日志</button>
      <button v-if="can('maintenance.restart')||can('maintenance.factory_reset')" :class="{active:activePanel==='operations'}" role="tab" :aria-selected="activePanel==='operations'" @click="activePanel='operations'">高级操作</button>
    </div>
    <DeviceLicensePanel v-if="activePanel==='license'&&canViewLicense"/>
    <section v-else-if="activePanel==='time'" class="wireless-panel maintenance-tab-panel time-sync-panel"><div class="section-title maintenance-subtitle"><div><h3>日期与时间</h3><p class="meta">通过 NTP 保持网关时间准确。工业现场可填写局域网时间服务器。</p></div><span class="time-sync-state" :class="{online:timeStatus?.synchronized,offline:timeStatus&&!timeStatus.synchronized}">{{timeStatus?.synchronized?'已同步':timeStatus?.supported===false?'当前系统不支持':'未同步'}}</span></div>
      <div class="time-clock-grid"><div><span>网关时间</span><strong>{{formatClock(gatewayNow)}}</strong></div><div><span>浏览器时间</span><strong>{{formatClock(browserNow)}}</strong></div><div><span>时间偏差</span><strong :class="{'bad-text':Math.abs(clockOffsetMs)>=5000}">{{clockDifference}}</strong></div><div><span>同步层级</span><strong>{{timeStatus?.stratum||'-'}}</strong></div></div>
      <div class="time-config-grid"><label>自动同步<select v-model="timeForm.enabled"><option :value="true">启用</option><option :value="false">停用</option></select></label><label>时区<input v-model.trim="timeForm.timezone" list="gateway-timezones"/><datalist id="gateway-timezones"><option value="Asia/Shanghai"/><option value="Asia/Hong_Kong"/><option value="Asia/Tokyo"/><option value="UTC"/><option value="Europe/London"/><option value="America/New_York"/></datalist></label><label>主 NTP 服务器<input v-model.trim="timeForm.primaryServer" :disabled="!timeForm.enabled" placeholder="例如 ntp.example.com"/></label><label>备用 NTP 服务器<input v-model.trim="timeForm.secondaryServer" :disabled="!timeForm.enabled" placeholder="可选"/></label></div>
      <div class="time-sync-footer"><div><p v-if="timeMessage" class="inline-result good">{{timeMessage}}</p><p v-else-if="timeError" class="inline-result bad">{{timeError}}</p><p v-else class="meta">{{timeStatus?.message||'正在读取时间同步状态...'}}</p></div><div class="point-actions"><button :disabled="timeBusy||!timeForm.enabled" @click="timeAction('test')">检测服务器</button><button :disabled="timeBusy||!timeForm.enabled||timeStatus?.supported===false" @click="timeAction('sync')">立即校时</button><button class="primary" :disabled="timeBusy" @click="saveTime">保存并应用</button></div></div>
    </section>
    <section v-else-if="activePanel==='diagnostics'" class="wireless-panel maintenance-tab-panel"><div class="section-title maintenance-subtitle"><div><h3>诊断工具</h3><p class="meta">检查网络连通性，或下载已脱敏的诊断包用于离线排查。</p></div><a class="button-link secondary-link" href="/api/maintenance/diagnostic-bundle">下载诊断包</a></div><div class="form-row"><label>目标地址<input v-model="target" @keydown.enter="ping"/></label><label>次数<input v-model.number="count" type="number" min="1" max="10"/></label><button class="primary" :disabled="pinging" @click="ping">{{pinging?'执行中':'开始 Ping'}}</button></div><pre>{{pingOutput}}</pre></section>
    <section v-else-if="activePanel==='remote'" class="maintenance-tab-panel remote-access-panel">
      <div class="remote-access-row" :class="{unsupported:sshStatus&&!sshStatus.supported}">
        <div class="remote-access-summary"><span class="state-tag" :class="sshStatus?.running?'ok':sshStatus?.enabled?'warn':'bad'">{{!sshStatus?'读取中':!sshStatus.supported?'不可用':sshStatus.running?'运行中':sshStatus.enabled?'未运行':'已关闭'}}</span><div><h3>SSH 远程登录</h3><p>用于命令行运维、日志排查和程序部署；不需要时建议保持关闭。</p></div></div>
        <div class="remote-access-actions"><button type="button" class="security-detail-toggle" :class="{expanded:sshDetailsOpen}" :aria-expanded="sshDetailsOpen" @click="sshDetailsOpen=!sshDetailsOpen">详情 <IconChevronDown aria-hidden="true"/></button><label class="edge-master-switch" :class="{checked:sshStatus?.enabled,disabled:sshBusy||!sshStatus?.supported}" :aria-label="sshStatus?.enabled?'SSH 已启用':'SSH 已关闭'"><input type="checkbox" role="switch" :checked="sshStatus?.enabled" :aria-checked="!!sshStatus?.enabled" :disabled="sshBusy||!sshStatus?.supported" @click.prevent="toggleSSH"/><i class="edge-switch-track" aria-hidden="true"/><em>{{sshBusy?'处理中':sshStatus?.enabled?'已启用':'已关闭'}}</em></label></div>
      </div>
      <div v-if="sshDetailsOpen" class="remote-access-details"><p>{{sshStatus?.message||'正在读取 SSH 服务状态…'}}</p><dl><div><dt>运行状态</dt><dd>{{sshStatus?.running?'正在运行':'未运行'}}</dd></div><div><dt>开机启动</dt><dd>{{sshStatus?.enabled?'已启用':'已关闭'}}</dd></div><div><dt>SSH 服务</dt><dd>{{sshStatus?.service||'-'}}</dd></div><div><dt>默认端口</dt><dd>{{sshStatus?.port||22}}</dd></div><div v-if="sshStatus?.initScript"><dt>启动脚本</dt><dd class="mono">{{sshStatus.initScript}}</dd></div></dl></div>
      <p v-if="sshMessage" class="inline-result good">{{sshMessage}}</p><p v-else-if="sshError" class="inline-result bad">{{sshError}}</p>
    </section>
    <section v-else-if="activePanel==='audit'" class="maintenance-tab-panel audit-panel"><div class="audit-toolbar"><label>检索<input v-model.trim="auditSearch" placeholder="账号、IP、请求路径或说明"/></label><label>结果<select v-model="auditResult"><option value="all">全部结果</option><option value="success">成功</option><option value="failed">失败</option></select></label><button @click="loadAudit">刷新</button></div><p v-if="auditError" class="inline-result bad">{{auditError}}</p><div class="audit-table-wrap"><table><thead><tr><th>时间</th><th>账号 / 来源</th><th>操作</th><th>结果</th><th>耗时</th><th>说明</th></tr></thead><tbody><tr v-for="(event,index) in visibleEvents" :key="`${event.timestamp}-${index}`"><td>{{formatAuditTime(event.timestamp)}}</td><td><strong>{{event.username||'匿名'}}</strong><small>{{event.remoteIp||'-'}}</small></td><td><code>{{event.method}}</code> {{event.path}}</td><td><span class="audit-result" :class="event.result">{{auditResultText(event.result)}} · {{event.status}}</span></td><td>{{event.durationMs===undefined?'-':`${event.durationMs} ms`}}</td><td class="audit-detail">{{event.detail||'-'}}</td></tr><tr v-if="!visibleEvents.length"><td colspan="6" class="empty">暂无匹配的审计记录</td></tr></tbody></table></div><div class="audit-pagination"><span>共 {{filteredEvents.length}} 条</span><div><button :disabled="auditPage<=1" @click="auditPage--">上一页</button><span>{{auditPage}} / {{auditPages}}</span><button :disabled="auditPage>=auditPages" @click="auditPage++">下一页</button></div></div></section>
    <section v-else-if="activePanel==='operations'" class="maintenance-tab-panel"><section class="maintenance-actions-panel"><div><h3>维护操作</h3><p class="meta">重启和恢复出厂设置需要再次验证当前账号身份。</p></div><div class="point-actions"><button v-if="can('maintenance.factory_reset')" class="danger" @click="openSensitiveModal('factory-confirm')">恢复出厂设置</button><button v-if="can('maintenance.restart')" @click="openSensitiveModal('restart')">重启服务</button><button v-if="can('maintenance.restart')" class="danger" @click="openSensitiveModal('reboot')">重启网关</button></div></section><p v-if="actionError" class="inline-result bad">{{actionError}}</p></section>
    <section v-else class="maintenance-empty"><p>当前账号没有可操作的系统维护权限。</p></section>
  </section>
  <div v-if="modal" class="restart-modal"><div class="dialog-panel sensitive-dialog"><h3>{{modal==='restart'?'重启服务':modal==='reboot'?'重启网关':modal==='ssh-enable'?'启用 SSH':modal==='ssh-disable'?'关闭 SSH':'恢复出厂设置'}}</h3><p v-if="modal==='restart'">服务会短暂中断，真实就绪后页面将自动刷新。</p><p v-else-if="modal==='reboot'">采集、转发和云连接会中断，设备真实就绪后页面将自动刷新。</p><p v-else-if="modal==='ssh-enable'">SSH 将立即启动并设为开机启动，请确认系统账号已配置安全密码或密钥。</p><p v-else-if="modal==='ssh-disable'">关闭后现有 SSH 连接可能立即断开，并取消开机启动。</p><p v-else>系统会先自动生成完整备份，失败时不会继续；新配置应用失败时会自动回滚。设备、通道、点位和业务配置将被清空，账号与 Web 安全配置保留。</p><div class="sensitive-fields"><label>当前账号密码 <span>*</span><input v-model="verification.currentPassword" type="password" autocomplete="current-password" placeholder="请输入当前登录密码" @keydown.enter="modal==='restart'?beginRestart('service','/api/maintenance/restart'):modal==='reboot'?beginRestart('gateway','/api/maintenance/reboot'):modal==='ssh-enable'?applySSH(true):modal==='ssh-disable'?applySSH(false):factoryReset()"/></label><label v-if="currentSession?.mfaEnabled">身份验证器验证码 <span>*</span><input v-model.trim="verification.totpCode" inputmode="numeric" maxlength="6" autocomplete="one-time-code" placeholder="6 位动态验证码"/></label></div><p v-if="actionError" class="inline-result bad" role="alert">{{actionError}}</p><div class="dialog-actions"><button @click="modal='';resetVerification()">取消</button><button :class="modal==='restart'||modal==='ssh-enable'||modal==='ssh-disable'?'primary':'danger'" :disabled="!verificationReady||sshBusy" @click="modal==='restart'?beginRestart('service','/api/maintenance/restart'):modal==='reboot'?beginRestart('gateway','/api/maintenance/reboot'):modal==='ssh-enable'?applySSH(true):modal==='ssh-disable'?applySSH(false):factoryReset()">{{modal==='restart'?'确认重启':modal==='reboot'?'确认重启网关':modal==='ssh-enable'?'确认启用':modal==='ssh-disable'?'确认关闭':'备份并恢复'}}</button></div></div></div>
  <div v-if="restartState.active" class="restart-loading-page" aria-live="polite"><div class="restart-loading-card"><div class="restart-loader" :class="{online:restartState.phase==='online'}"><span></span></div><div><p class="restart-kicker">{{restartState.kind==='service'?'SERVICE RESTART':'GATEWAY REBOOT'}}</p><h2>{{restartTitle}}</h2><p>{{restartDescription}}</p></div><div class="restart-progress"><span></span></div><div class="restart-wait"><span>已等待 {{restartState.elapsed}} 秒</span><span>{{restartState.phase==='online'?'连接已恢复':'自动检测中'}}</span></div><button v-if="restartState.phase==='timeout'" class="primary" @click="restartState.phase='waiting';restartStartedAt=Date.now();restartSawOffline=true;restartHealthyCount=0;probeRestartRecovery()">重新检测</button></div></div>
  </section>
</template>
