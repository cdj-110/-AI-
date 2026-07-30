<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import type { AuditEvent } from '../types'

const target = ref('www.baidu.com')
const count = ref(4)
const pinging = ref(false)
const pingOutput = ref('暂无结果')
const events = ref<AuditEvent[]>([])
const auditError = ref('')
const resetPassword = ref('')
const modal = ref<''|'restart'|'reboot'|'factory-backup'|'factory-confirm'>('')
type RestartKind='service'|'gateway'
const restartState=reactive({active:false,kind:'service' as RestartKind,phase:'sending' as 'sending'|'offline'|'waiting'|'online'|'timeout',elapsed:0,error:''})
let restartProbeTimer=0
let restartClockTimer=0
let restartStartedAt=0
let restartSawOffline=false
let restartHealthyCount=0
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
const timeBusy=ref(false)
const timeMessage=ref('')
const timeError=ref('')
const browserNow=ref(new Date())
const clockOffsetMs=ref(0)
let clockTimer=0
const gatewayNow=computed(()=>new Date(browserNow.value.getTime()+clockOffsetMs.value))
const clockDifference=computed(()=>{
  const seconds=Math.round(clockOffsetMs.value/1000)
  if(Math.abs(seconds)<1)return'时间基本一致'
  return seconds>0?`网关快 ${seconds} 秒`:`网关慢 ${Math.abs(seconds)} 秒`
})
function formatClock(value:Date){return value.toLocaleString('zh-CN',{hour12:false})}

async function loadTime(){
  try{
    const result=await api<TimeSyncStatus>('/api/maintenance/time')
    timeStatus.value=result
    Object.assign(timeForm,result.config)
    clockOffsetMs.value=new Date(result.systemTime).getTime()-Date.now()
    timeError.value=''
  }catch(error){timeError.value=error instanceof Error?error.message:'时间同步状态读取失败'}
}
async function saveTime(){
  timeBusy.value=true;timeMessage.value='';timeError.value=''
  try{
    const result=await api<TimeSyncStatus>('/api/maintenance/time',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(timeForm),timeoutMs:30000})
    timeStatus.value=result;clockOffsetMs.value=new Date(result.systemTime).getTime()-Date.now();timeMessage.value='日期与时间配置已保存并应用。'
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

async function ping() {
  pinging.value = true
  pingOutput.value = '执行中...'
  try {
    const result = await api<{ ok: boolean; output: string }>('/api/maintenance/ping', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ target: target.value, count: count.value }), timeoutMs: 40000 })
    pingOutput.value = result.output || (result.ok ? '目标可达' : '目标不可达')
  } catch (error) {
    pingOutput.value = error instanceof Error ? error.message : 'Ping 执行失败'
  } finally { pinging.value = false; await loadAudit() }
}

async function loadAudit() {
  try {
    events.value = (await api<{ events: AuditEvent[] }>('/api/maintenance/audit-log?limit=100')).events || []
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
    // Login is intentionally unauthenticated. In-memory sessions are cleared by
    // a service restart, so an authenticated health endpoint cannot be used here.
    const response=await fetch(`/login?restart_probe=${Date.now()}`,{cache:'no-store',redirect:'follow',signal:AbortSignal.timeout(2500)})
    if(!response.ok)throw new Error('not ready')
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
  modal.value=''
  stopRestartTimers()
  Object.assign(restartState,{active:true,kind,phase:'sending',elapsed:0,error:''})
  restartStartedAt=Date.now();restartSawOffline=false;restartHealthyCount=0
  restartClockTimer=window.setInterval(()=>restartState.elapsed=Math.floor((Date.now()-restartStartedAt)/1000),1000)
  try {
    await api(path, { method:'POST',headers:{'Content-Type':'application/json'},body:'{}',timeoutMs:6000 })
  } catch (error) {
    if(!(error instanceof TypeError)&&!(error instanceof DOMException&&error.name==='AbortError')){
      stopRestartTimers();restartState.active=false
      pingOutput.value=error instanceof Error?error.message:'重启命令提交失败'
      return
    }
  }
  restartProbeTimer=window.setTimeout(probeRestartRecovery,1200)
}

async function factoryReset() {
  try {
    await api('/api/maintenance/factory-reset', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({ adminPassword:resetPassword.value }) })
    modal.value=''
    window.location.reload()
  } catch (error) { pingOutput.value=error instanceof Error ? error.message : '恢复出厂设置失败' }
}

onMounted(()=>{void loadAudit();void loadTime();clockTimer=window.setInterval(()=>browserNow.value=new Date(),1000)})
onBeforeUnmount(()=>{window.clearInterval(clockTimer);stopRestartTimers()})
</script>

<template>
  <section id="panel-maintenance"><section class="card"><div class="section-title"><div><h2>系统维护</h2><p class="meta">执行时间配置、网络诊断和高级维护操作。</p></div></div>
    <section class="wireless-panel time-sync-panel"><div class="section-title maintenance-subtitle"><div><h3>日期与时间</h3><p class="meta">通过 NTP 保持网关时间准确。工业现场可填写局域网时间服务器。</p></div><span class="time-sync-state" :class="{online:timeStatus?.synchronized,offline:timeStatus&&!timeStatus.synchronized}">{{timeStatus?.synchronized?'已同步':timeStatus?.supported===false?'当前系统不支持':'未同步'}}</span></div>
      <div class="time-clock-grid"><div><span>网关时间</span><strong>{{formatClock(gatewayNow)}}</strong></div><div><span>浏览器时间</span><strong>{{formatClock(browserNow)}}</strong></div><div><span>时间偏差</span><strong :class="{'bad-text':Math.abs(clockOffsetMs)>=5000}">{{clockDifference}}</strong></div><div><span>同步层级</span><strong>{{timeStatus?.stratum||'-'}}</strong></div></div>
      <div class="time-config-grid"><label>自动同步<select v-model="timeForm.enabled"><option :value="true">启用</option><option :value="false">停用</option></select></label><label>时区<input v-model.trim="timeForm.timezone" list="gateway-timezones"/><datalist id="gateway-timezones"><option value="Asia/Shanghai"/><option value="Asia/Hong_Kong"/><option value="Asia/Tokyo"/><option value="UTC"/><option value="Europe/London"/><option value="America/New_York"/></datalist></label><label>主 NTP 服务器<input v-model.trim="timeForm.primaryServer" :disabled="!timeForm.enabled" placeholder="例如 ntp.example.com"/></label><label>备用 NTP 服务器<input v-model.trim="timeForm.secondaryServer" :disabled="!timeForm.enabled" placeholder="可选"/></label></div>
      <div class="time-sync-footer"><div><p v-if="timeMessage" class="inline-result good">{{timeMessage}}</p><p v-else-if="timeError" class="inline-result bad">{{timeError}}</p><p v-else class="meta">{{timeStatus?.message||'正在读取时间同步状态...'}}</p></div><div class="point-actions"><button :disabled="timeBusy||!timeForm.enabled" @click="timeAction('test')">检测服务器</button><button :disabled="timeBusy||!timeForm.enabled||timeStatus?.supported===false" @click="timeAction('sync')">立即校时</button><button class="primary" :disabled="timeBusy" @click="saveTime">保存并应用</button></div></div>
    </section>
    <section class="wireless-panel"><div class="section-title maintenance-subtitle"><div><h3>网络诊断</h3><p class="meta">从网关本机发起 Ping，用于检查 DNS、路由和外网连通性。</p></div></div><div class="form-row"><label>目标地址<input v-model="target" @keydown.enter="ping"/></label><label>次数<input v-model.number="count" type="number" min="1" max="10"/></label><button class="primary" :disabled="pinging" @click="ping">{{pinging?'执行中':'开始 Ping'}}</button></div><pre>{{pingOutput}}</pre></section><section class="maintenance-actions-panel"><div><h3>维护操作</h3><p class="meta">维护命令会短暂影响采集和页面访问，请谨慎操作。</p></div><div class="point-actions"><button class="danger" @click="modal='factory-backup'">恢复出厂设置</button><button @click="modal='restart'">重启服务</button><button class="danger" @click="modal='reboot'">重启网关</button></div></section></section>
  <div v-if="modal" class="restart-modal"><div class="dialog-panel" v-if="modal==='restart'"><h3>重启服务</h3><p>确认重启当前网关服务？重启过程中页面会短暂断开，恢复后页面将自动刷新。</p><div class="dialog-actions"><button @click="modal=''">取消</button><button class="primary" @click="beginRestart('service','/api/maintenance/restart')">确认重启</button></div></div><div class="dialog-panel" v-else-if="modal==='reboot'"><h3>重启网关</h3><p>确认重启整台网关设备？设备重启期间采集、转发和云平台连接都会中断，重新上线后页面将自动刷新。</p><div class="dialog-actions"><button @click="modal=''">取消</button><button class="danger" @click="beginRestart('gateway','/api/maintenance/reboot')">确认重启</button></div></div><div class="dialog-panel" v-else-if="modal==='factory-backup'"><h3>恢复出厂设置</h3><p>恢复出厂设置会清空当前工程配置。建议先导出当前工程备份，也可以跳过备份继续下一步。</p><div class="dialog-actions"><button @click="modal='factory-confirm'">跳过备份</button><a class="button-link" href="/api/project/export" @click="modal='factory-confirm'">导出备份</a></div></div><div class="dialog-panel" v-else><h3>二次确认</h3><p>恢复后，设备、通道、点位、云平台连接、无线网络和转发配置都会被清空，仅保留登录账号和 Web 访问配置。</p><label>超级管理员密码<input v-model="resetPassword" type="password" autocomplete="current-password" placeholder="请输入超级管理员密码"/></label><div class="dialog-actions"><button @click="modal=''">取消</button><button class="danger" @click="factoryReset">确认恢复</button></div></div></div>
  <div v-if="restartState.active" class="restart-loading-page" aria-live="polite"><div class="restart-loading-card"><div class="restart-loader" :class="{online:restartState.phase==='online'}"><span></span></div><div><p class="restart-kicker">{{restartState.kind==='service'?'SERVICE RESTART':'GATEWAY REBOOT'}}</p><h2>{{restartTitle}}</h2><p>{{restartDescription}}</p></div><div class="restart-progress"><span></span></div><div class="restart-wait"><span>已等待 {{restartState.elapsed}} 秒</span><span>{{restartState.phase==='online'?'连接已恢复':'自动检测中'}}</span></div><button v-if="restartState.phase==='timeout'" class="primary" @click="restartState.phase='waiting';restartStartedAt=Date.now();restartSawOffline=true;restartHealthyCount=0;probeRestartRecovery()">重新检测</button></div></div>
  </section>
</template>
