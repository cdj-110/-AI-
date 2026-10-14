<script setup lang="ts">
import {computed,onMounted,onBeforeUnmount,ref} from 'vue'
import {IconSitemap} from '@tabler/icons-vue'
import {api} from '../api'
import {useConfigStore} from '../stores/config'
import type {GatewayStatus} from '../types'
import {deviceSelection,type CollectionOverview,type TopologySelection} from '../topology'
const props=defineProps<{canConfig?:boolean;canIncidents?:boolean;canTopology?:boolean}>()
const emit=defineEmits<{navigate:[key:'config'|'collection-incidents'|'topology'];openDevice:[selection:TopologySelection]}>()
const configStore=useConfigStore()
const status=ref<GatewayStatus|null>(null),overview=ref<CollectionOverview|null>(null)
const loading=ref(true),error=ref(''),refreshedAt=ref<Date>()
let timer=0,busy=false,disposed=false
const uptime=computed(()=>{const s=status.value?.uptimeSeconds||0;return (s>=86400?Math.floor(s/86400)+'天 ':'')+(s>=3600?Math.floor(s%86400/3600)+'小时 ':'')+Math.floor(s%3600/60)+'分钟'})
const collectIntervalText=computed(()=>status.value?.collectMilliseconds?status.value.collectMilliseconds+'ms':(status.value?.collectSeconds??'-')+'s')
function metric(name:'cpu'|'memory'|'storage'){return status.value?.systemMetrics?.[name]||{usedPercent:0,usedBytes:0,totalBytes:0}}
function percent(name:'cpu'|'memory'|'storage'){return Math.max(0,Math.min(100,metric(name).usedPercent||0))}
function formatBytes(value?:number){if(!value)return '-';const u=['B','KB','MB','GB','TB'];let i=0;while(value>=1024&&i<u.length-1){value/=1024;i++}return value.toFixed(i>1?1:0)+' '+u[i]}
function formatTime(value?:string){return value?new Date(value).toLocaleString():'-'}
function metricMeta(name:'cpu'|'memory'|'storage'){const m=metric(name);return name==='cpu'?'CPU':m.totalBytes?formatBytes(m.usedBytes)+' / '+formatBytes(m.totalBytes):name==='memory'?'内存':'系统分区'}
function deviceName(key:string){return (configStore.value.devices||[]).find((d:any)=>d.deviceKey===key)?.name||key}
function openDevice(key:string){const selection=deviceSelection(configStore.value,key);if(selection)emit('openDevice',selection);else emit('navigate','config')}
async function refresh(){if(busy)return;busy=true;try{const [s,o]=await Promise.all([api<GatewayStatus>('/api/status?compact=1',{timeoutMs:10000}),api<CollectionOverview>('/api/status/overview',{timeoutMs:10000})]);if(disposed)return;status.value=s;overview.value=o;refreshedAt.value=new Date();error.value=''}catch(e){if(!disposed)error.value=e instanceof Error?e.message:'状态读取失败'}finally{busy=false;if(!disposed)loading.value=false}}
onMounted(()=>{void refresh();timer=window.setInterval(refresh,3000)})
onBeforeUnmount(()=>{disposed=true;window.clearInterval(timer)})
</script>
<template>
  <section id="panel-status">
    <p v-if="error" class="notice bad" role="alert">{{error}}；下方可能为上次成功读取的状态。</p>
    <section class="hero">
      <article class="card hero-main">
        <div class="gateway-summary-heading"><span class="label">网关编号</span><button v-if="canTopology" type="button" class="topology-entry" aria-haspopup="dialog" @click="emit('navigate','topology')"><IconSitemap :size="17" aria-hidden="true"/>设备拓扑</button></div>
        <strong class="big">{{status?.gatewayKey||'-'}}</strong>
        <p class="meta">运行时长 {{uptime}} · 采集周期 {{collectIntervalText}}</p>
        <p class="meta">最近采集：{{formatTime(status?.lastCollectAt)}}</p>
        <p class="meta">最近上报：{{formatTime(status?.lastPublishAt)}}</p>
        <p class="meta">硬件唯一 ID：{{status?.hardwareIdentity?.available?`${status.hardwareIdentity.id} · 来源：${status.hardwareIdentity.source}`:`未读取到 · ${status?.hardwareIdentity?.message||'当前系统未开放硬件标识'}`}}</p>
        <p class="meta">授权芯片 USID：{{status?.secureChipIdentity?.available?`${status.secureChipIdentity.usid} · RJGT102 已认证`:`未读取到 · ${status?.secureChipIdentity?.message||'当前版本未启用芯片认证'}`}}</p>
      </article>
      <article class="card load-card"><span class="label">设备负载</span><div class="load-rings">
        <div v-for="item in ([['cpu','CPU','#f97316'],['memory','内存','#8b5cf6'],['storage','系统分区','#0ea5e9']] as const)" :key="item[0]" class="load-ring" :title="item[0]==='storage'?'网关系统分区（/），不包含外接 U 盘；历史存储空间请在历史数据中查看。':''"><div class="ring-chart" :style="{'--value':percent(item[0]),'--color':percent(item[0])>=90?'#dc2626':item[2]}"><span class="ring-inner"><strong>{{percent(item[0]).toFixed(0)}}%</strong><span>{{item[1]}}</span></span></div><span class="ring-meta">{{metricMeta(item[0])}}</span></div>
      </div></article>
    </section>
    <section class="stats">
      <article class="card"><span class="label">点位总数</span><strong>{{status?.pointCount??0}}</strong></article>
      <article class="card"><span class="label">正常点位</span><strong>{{overview?overview.devices.reduce((n,d)=>n+d.healthy,0):status?.healthyCount??0}}</strong></article>
      <article class="card"><span class="label">异常点位</span><button class="status-count-link" :disabled="!canIncidents" @click="emit('navigate','collection-incidents')">{{overview?.affectedPoints??status?.errorCount??0}}</button></article>
      <article class="card"><span class="label">软件内存占用</span><strong>{{formatBytes(status?.processMetrics?.memoryBytes)}}</strong></article>
      <article class="card" :title="status?.processMetrics?.diskPath||''"><span class="label">软件磁盘占用</span><strong>{{formatBytes(status?.processMetrics?.diskBytes)}}</strong></article>
    </section>

    <p v-if="percent('storage')>=90" class="status-storage-warning" role="status">系统分区已使用 {{percent('storage').toFixed(0)}}%，请检查可用空间，避免影响日志、缓存和配置保存。此处不包含外接 U 盘。</p>
    <section class="card status-issue-card">
      <div class="section-title"><div><h2>当前采集异常 <small v-if="overview?.issueGroups">{{overview.issueGroups}} 组 · {{overview.affectedPoints}} 个点位</small></h2><p class="meta">按设备和原因归并，最多展示 5 组 · 每 3 秒刷新{{refreshedAt?' · '+refreshedAt.toLocaleTimeString():''}}</p></div><div class="status-shortcuts"><button v-if="canConfig" @click="emit('navigate','config')">查看全部点位</button><button v-if="canIncidents" @click="emit('navigate','collection-incidents')">查看异常记录</button></div></div>
      <p v-if="loading" class="status-issue-empty">正在读取采集状态…</p>
      <p v-else-if="!overview" class="status-issue-empty">暂无法确认采集状态，请稍后重试。</p>
      <p v-else-if="!overview.affectedPoints" class="status-issue-empty" :class="{healthy:status?.pointCount&&!status?.pendingCount}">{{!status?.pointCount?'尚未配置采集点位':status?.pendingCount?'暂无采集异常，仍有 '+status.pendingCount+' 个点位等待采集':'采集正常，暂无异常点位'}}</p>
      <div v-else class="status-issue-list"><article v-for="issue in overview.issues" :key="issue.deviceKey+'-'+issue.reason" class="status-issue-row"><div><strong>{{deviceName(issue.deviceKey)}}</strong><small>{{issue.deviceKey}} · {{issue.protocol}} · {{issue.address}}</small></div><div class="status-issue-reason"><span>{{issue.reason}}</span><details><summary>原始诊断</summary><p>{{issue.detail}}</p></details></div><span class="status-issue-count">{{issue.count}} 个点位</span><button v-if="canConfig" @click="openDevice(issue.deviceKey)">查看设备</button></article></div>
    </section>
    <details class="card status-log-details"><summary>最近运行错误 <span>{{status?.errors?.length||0}} 条</span></summary><div class="events"><div v-for="event in status?.errors?.slice(0,20)" :key="event.time+event.message" class="event"><strong>{{formatTime(event.time)}}</strong><div>{{event.message}}</div></div><p v-if="!status?.errors?.length" class="meta">暂无运行错误</p></div></details>
  </section>
</template>
<style scoped>
.gateway-summary-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:34px;margin-top:-5px}
.topology-entry{display:inline-flex;align-items:center;gap:6px;flex-shrink:0;color:#3577b9;background:#f5f9ff;border-color:#e2ecf9;box-shadow:none}
.topology-entry:focus-visible{outline:2px solid #1677ff;outline-offset:2px}
.status-count-link{display:block;padding:0;border:0;background:transparent;color:#dc2626;font-size:26px;font-weight:700;cursor:pointer}.status-count-link:disabled{opacity:1;cursor:default}
.status-storage-warning{padding:10px 14px;border:1px solid #fed7aa;background:#fff7ed;color:#9a3412;border-radius:8px;font-size:12px}
.status-issue-card h2{font-size:17px}.status-issue-card h2 small{font-size:12px;font-weight:400;margin-left:10px;color:#64748b}.status-shortcuts{display:flex;flex-wrap:wrap;gap:7px}
.status-issue-row{display:grid;grid-template-columns:minmax(180px,1fr) minmax(180px,1.3fr) 90px auto;align-items:start;gap:18px;padding:13px 0;border-top:1px solid #edf1f6;font-size:12px}
.status-issue-row strong,.status-issue-row small{display:block}.status-issue-row small{margin-top:5px;color:#64748b;overflow-wrap:anywhere}.status-issue-reason>span{color:#b91c1c}.status-issue-reason details{margin-top:5px;color:#64748b}.status-issue-reason summary{cursor:pointer;font-size:11px}.status-issue-reason p{overflow-wrap:anywhere;line-height:1.6}.status-issue-count{color:#b91c1c;background:#fff1f2;border-radius:5px;padding:4px 7px;text-align:center}
.status-issue-empty{padding:22px;text-align:center;color:#64748b;background:#f8fafc;border-radius:8px}.status-issue-empty.healthy{color:#15803d;background:#f0fdf4}
.status-log-details{margin-top:16px}.status-log-details>summary{font-size:13px;cursor:pointer;color:#64748b}.status-log-details>summary span{margin-left:8px}.status-log-details .events{margin-top:14px}
@media(max-width:800px){.status-issue-card .section-title{flex-wrap:wrap}.status-issue-row{grid-template-columns:minmax(0,1fr) auto;gap:12px}.status-issue-reason{grid-column:1/-1;grid-row:2}.status-issue-row small{font-size:10px}.status-issue-card h2 small{display:block;margin:5px 0}}
</style>
