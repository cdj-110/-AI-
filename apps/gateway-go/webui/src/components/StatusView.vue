<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import { mergePointDiff, openGatewayRealtime } from '../realtime'
import { useConfigStore } from '../stores/config'
import type { GatewayStatus } from '../types'

const configStore=useConfigStore()
const status = ref<GatewayStatus | null>(null)
const loading = ref(true)
const error = ref('')
const refreshedAt = ref<Date>()
const realtimeConnected = ref(false)
const statusRoot=ref<HTMLElement>()
const statusVirtualStart=ref(0),statusVirtualEnd=ref(0)
const STATUS_ROW_HEIGHT=53,STATUS_OVERSCAN=12,STATUS_VIRTUAL_THRESHOLD=150
let statusScrollHost:HTMLElement|undefined,statusVirtualFrame=0
// The status table is continuous; these keep the legacy pager branch false.
const pointPage=ref(1), pointPageSize=Number.POSITIVE_INFINITY
let stopRealtime=()=>{}, compactTimer=0

const uptime = computed(() => {
  const seconds = status.value?.uptimeSeconds ?? 0
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${days ? days + '天 ' : ''}${hours ? hours + '小时 ' : ''}${minutes}分钟`
})
const collectIntervalText = computed(() => status.value?.collectMilliseconds ? `${status.value.collectMilliseconds}ms` : `${status.value?.collectSeconds ?? '-'}s`)
const allStatusPoints=computed(()=>status.value?.points??[])
const statusPointCount=computed(()=>allStatusPoints.value.length)
const statusPoints=computed(()=>allStatusPoints.value.slice(statusVirtualStart.value,statusVirtualEnd.value||allStatusPoints.value.length))
const statusPageCount=computed(()=>1)
const statusVirtualized=computed(()=>statusPointCount.value>STATUS_VIRTUAL_THRESHOLD)
const statusVirtualTop=computed(()=>statusVirtualStart.value*STATUS_ROW_HEIGHT)
const statusVirtualStyle=computed(()=>({
  '--status-virtual-height':`${statusPointCount.value*STATUS_ROW_HEIGHT+STATUS_ROW_HEIGHT}px`,
  '--status-virtual-top':`${statusVirtualTop.value}px`,
  '--status-virtual-bottom':`${Math.max(0,(statusPointCount.value-statusVirtualEnd.value)*STATUS_ROW_HEIGHT)}px`
}))

function metric(name: 'cpu'|'memory'|'storage') { return status.value?.systemMetrics?.[name] || { usedPercent:0, usedBytes:0, totalBytes:0 } }
function percent(name: 'cpu'|'memory'|'storage') { return Math.max(0, Math.min(100, metric(name).usedPercent || 0)) }
function formatBytes(value?: number) {
  if (!value) return '-'
  const units = ['B','KB','MB','GB','TB']; let current = value, index = 0
  while (current >= 1024 && index < units.length - 1) { current /= 1024; index++ }
  return `${current.toFixed(index > 1 ? 1 : 0)} ${units[index]}`
}
function formatTime(value?: string) { return value ? new Date(value).toLocaleString() : '-' }
function metricMeta(name:'cpu'|'memory'|'storage') {
  const value=metric(name)
  return name==='cpu' ? 'CPU' : value.totalBytes ? `${formatBytes(value.usedBytes)} / ${formatBytes(value.totalBytes)}` : (name==='memory'?'内存':'硬盘')
}

async function refresh() {
  try { status.value = await api<GatewayStatus>(configStore.largeMode?'/api/status?compact=1':'/api/status', { timeoutMs:10000 }); refreshedAt.value=new Date(); error.value='' }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '状态读取失败' }
  finally { loading.value=false }
}

function handleRealtime(message:any) {
  const payload=message.payload||{}
  if(message.type==='snapshot')status.value=payload
  else if(message.type==='gateway.status'&&status.value)status.value={...status.value,...payload,points:status.value.points,errors:status.value.errors}
  else if(message.type==='points.diff'&&status.value)status.value={...status.value,points:mergePointDiff(status.value.points,payload)}
  else if(message.type==='communications.diff'&&status.value)status.value={...status.value,mqttEnabled:Boolean(payload.mqttEnabled),mqttConnected:Boolean(payload.mqttConnected),mqttChannels:payload.mqttChannels||{}}
  else if(message.type==='logs.diff'&&status.value){const entries=Array.isArray(payload.entries)?payload.entries:[];status.value={...status.value,errors:payload.replace?entries:[...entries,...status.value.errors].slice(0,200)}}
  else return
  loading.value=false
  error.value=''
  refreshedAt.value=new Date()
}

function updateStatusVirtualWindow(){
  const total=statusPointCount.value
  if(total<=STATUS_VIRTUAL_THRESHOLD){statusVirtualStart.value=0;statusVirtualEnd.value=total;return}
  const host=statusScrollHost,table=statusRoot.value?.querySelector<HTMLElement>('.table-wrap table')
  if(!host||!table){statusVirtualStart.value=0;statusVirtualEnd.value=Math.min(total,50);return}
  const hostRect=host.getBoundingClientRect(),tableRect=table.getBoundingClientRect()
  const rowsTop=tableRect.top-hostRect.top+host.scrollTop+STATUS_ROW_HEIGHT
  const firstVisible=Math.floor((host.scrollTop-rowsTop)/STATUS_ROW_HEIGHT)
  const start=Math.max(0,firstVisible-STATUS_OVERSCAN)
  const windowSize=Math.ceil(host.clientHeight/STATUS_ROW_HEIGHT)+STATUS_OVERSCAN*2
  statusVirtualStart.value=start
  statusVirtualEnd.value=Math.min(total,start+windowSize)
}
function scheduleStatusVirtualUpdate(){
  if(statusVirtualFrame)return
  statusVirtualFrame=requestAnimationFrame(()=>{statusVirtualFrame=0;updateStatusVirtualWindow()})
}
watch(statusPointCount,()=>nextTick(scheduleStatusVirtualUpdate),{flush:'post'})
onMounted(async()=>{
  await refresh()
  await nextTick()
  statusScrollHost=statusRoot.value?.closest<HTMLElement>('.page')||undefined
  statusScrollHost?.addEventListener('scroll',scheduleStatusVirtualUpdate,{passive:true})
  window.addEventListener('resize',scheduleStatusVirtualUpdate,{passive:true})
  updateStatusVirtualWindow()
  if(configStore.largeMode){
    realtimeConnected.value=true
    compactTimer=window.setInterval(refresh,1000)
  }else stopRealtime=openGatewayRealtime(handleRealtime,connected=>realtimeConnected.value=connected)
})
onBeforeUnmount(()=>{
  stopRealtime()
  window.clearInterval(compactTimer)
  statusScrollHost?.removeEventListener('scroll',scheduleStatusVirtualUpdate)
  window.removeEventListener('resize',scheduleStatusVirtualUpdate)
  if(statusVirtualFrame)cancelAnimationFrame(statusVirtualFrame)
})
</script>

<template>
  <section id="panel-status" ref="statusRoot" :class="{'status-virtualized':statusVirtualized,'status-virtual-offset':statusVirtualStart>0}" :style="statusVirtualStyle">
    <p v-if="error" class="notice bad">{{error}}</p>
    <section class="hero">
      <article class="card hero-main">
        <span class="label">网关编号</span>
        <strong class="big">{{status?.gatewayKey||'-'}}</strong>
        <p class="meta">运行时长 {{uptime}} · 采集周期 {{collectIntervalText}}</p>
        <p class="meta">最近采集：{{formatTime(status?.lastCollectAt)}}</p>
        <p class="meta">最近上报：{{formatTime(status?.lastPublishAt)}}</p>
        <p class="meta">硬件唯一 ID：{{status?.hardwareIdentity?.available?`${status.hardwareIdentity.id} · 来源：${status.hardwareIdentity.source}`:`未读取到 · ${status?.hardwareIdentity?.message||'当前系统未开放硬件标识'}`}}</p>
      </article>
      <article class="card load-card"><span class="label">设备负载</span><div class="load-rings">
        <div v-for="item in ([['cpu','CPU','#f97316'],['memory','内存','#8b5cf6'],['storage','硬盘','#0ea5e9']] as const)" :key="item[0]" class="load-ring"><div class="ring-chart" :style="{'--value':percent(item[0]),'--color':item[2]}"><span class="ring-inner"><strong>{{percent(item[0]).toFixed(0)}}%</strong><span>{{item[1]}}</span></span></div><span class="ring-meta">{{metricMeta(item[0])}}</span></div>
      </div></article>
    </section>
    <section class="stats">
      <article class="card"><span class="label">点位总数</span><strong>{{status?.pointCount??0}}</strong></article>
      <article class="card"><span class="label">正常点位</span><strong>{{status?.healthyCount??0}}</strong></article>
      <article class="card"><span class="label">异常点位</span><strong>{{status?.errorCount??0}}</strong></article>
      <article class="card"><span class="label">软件内存占用</span><strong>{{formatBytes(status?.processMetrics?.memoryBytes)}}</strong></article>
      <article class="card" :title="status?.processMetrics?.diskPath||''"><span class="label">软件磁盘占用</span><strong>{{formatBytes(status?.processMetrics?.diskBytes)}}</strong></article>
    </section>
    <section class="card">
      <div class="section-title"><h2>点位状态</h2><span class="muted">{{realtimeConnected?'WebSocket 实时更新':'连接恢复中'}}{{refreshedAt?` · ${refreshedAt.toLocaleTimeString()}`:''}}</span></div>
      <div class="table-wrap"><table><thead><tr><th>子设备</th><th>点位</th><th>协议</th><th>地址</th><th>当前值</th><th>更新时间</th><th>状态</th></tr></thead><tbody>
        <tr v-if="loading"><td colspan="7" class="empty">正在加载...</td></tr><tr v-else-if="!status?.points.length"><td colspan="7" class="empty">暂无点位</td></tr>
        <tr v-for="point in statusPoints" :key="`${point.deviceKey}:${point.metric}`"><td><code>{{point.deviceKey}}</code></td><td>{{point.name||point.metric}}<div class="muted">{{point.metric}}</div></td><td>{{point.protocol}}</td><td>{{point.address}}</td><td>{{point.value??'-'}}</td><td>{{formatTime(point.updatedAt)}}</td><td :class="point.error?'bad-text':'ok-text'">{{point.error||'正常'}}</td></tr>
      </tbody></table></div>
      <div v-if="statusPointCount>pointPageSize" class="point-pagination"><span>第 {{pointPage}} / {{statusPageCount}} 页 · 共 {{statusPointCount}} 条</span><button :disabled="pointPage<=1" @click="pointPage--">上一页</button><button :disabled="pointPage>=statusPageCount" @click="pointPage++">下一页</button></div>
    </section>
    <section class="card status-errors-card">
      <div class="section-title"><h2>最近错误</h2><span class="muted">最多显示 50 条</span></div>
      <div class="events"><div v-for="event in status?.errors?.slice(0,50)" :key="`${event.time}:${event.message}`" class="event"><strong>{{formatTime(event.time)}}</strong><div>{{event.message}}</div></div><div v-if="!status?.errors?.length" class="muted">暂无运行日志</div></div>
    </section>
  </section>
</template>
