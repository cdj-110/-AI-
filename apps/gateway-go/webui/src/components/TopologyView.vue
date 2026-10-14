<script setup lang="ts">
import {computed,onMounted,onBeforeUnmount,ref} from 'vue'
import {IconPlugConnected,IconNetwork,IconDeviceDesktop,IconSearch,IconChevronDown,IconChevronsDown,IconChevronsUp,IconCloud,IconArrowsUpDown,IconBolt,IconGauge,IconCpu,IconArrowDown} from '@tabler/icons-vue'
import {api} from '../api'
import {buildTopology,channelProtocol,protocolLabel,stateLabels,type CollectionOverview,type TopologySelection} from '../topology'
import {useConfigStore} from '../stores/config'
const emit=defineEmits<{openDevice:[selection:TopologySelection]}>()
const props=defineProps<{status?:any}>()
const configStore=useConfigStore()
const config=computed(()=>configStore.value),counts=computed(()=>configStore.pointCounts),overview=ref<CollectionOverview|null>(null)
const error=ref(''),statusError=ref(''),loaded=ref(false),loading=ref(false),search=ref(''),filter=ref('all'),updated=ref('')
const folded=ref(new Set<string>())
const communication=computed(()=>props.status)
const cloudLabel=computed(()=>statusError.value||!communication.value?'状态未知':!communication.value.mqttEnabled?'未启用云连接':communication.value.mqttConnected?'云连接正常':'云连接未建立')
const cloudState=computed(()=>statusError.value||!communication.value?'pending':!communication.value.mqttEnabled?'disabled':communication.value.mqttConnected?'healthy':'failed')
function deviceIcon(d:any,c:any){const name=String(d.name||'');return /电表|电能/.test(name)||c.protocol==='dlt645'?IconBolt:/噪|压力|温|传感/.test(name)?IconGauge:/plc/i.test(name)||c.protocol==='siemens-s7'?IconCpu:IconDeviceDesktop}
let timer=0,polling=false,disposed=false
const resources=computed(()=>buildTopology(config.value,overview.value?.devices||[]))
const deviceCount=computed(()=>resources.value.reduce((n:number,r:any)=>n+r.channels.reduce((m:number,c:any)=>m+c.devices.length,0),0))
const serialCount=computed(()=>resources.value.filter((r:any)=>r.type==='serial').length)
const networkCount=computed(()=>resources.value.filter((r:any)=>r.type==='network').length)
const visible=computed(()=>resources.value.filter((r:any)=>filter.value==='all'||r.type===filter.value).map((r:any)=>{
 const q=search.value.trim().toLowerCase(),resourceMatch=matches(q,r.name,r.resourceKey,r.serial?.port,r.network?.interface)
 return {...r,channels:r.channels.map((c:any)=>({...c,devices:resourceMatch||matches(q,c.name,c.protocol,c.forwardProtocol)?c.devices:c.devices.filter((d:any)=>matches(q,d.name,d.deviceKey,d.address))})).filter((c:any)=>!q||resourceMatch||c.devices.length||matches(q,c.name,c.protocol,c.forwardProtocol))}
}).filter((r:any)=>!search.value.trim()||r.channels.length||matches(search.value.toLowerCase(),r.name,r.resourceKey,r.serial?.port,r.network?.interface)))
function matches(q:string,...values:unknown[]){return values.some(v=>String(v||'').toLowerCase().includes(q))}
function toggle(key:string){const next=new Set(folded.value);next.has(key)?next.delete(key):next.add(key);folded.value=next}
function resourceDisabled(r:any){return r.enabled===false||r.serial?.enabled===false||r.network?.enabled===false}
function pointCount(d:any){return counts.value[d.deviceKey]??d._pointCount??d.points?.length??0}
function deviceStateLabel(d:any){return statusError.value?'状态未知':stateLabels[d.state as keyof typeof stateLabels]||'状态未知'}
function deviceDetails(r:any,c:any,d:any){
 const address=d.address||r.serial?.port||'未配置地址'
 const station=d.dlt645?.meterAddress?' · 表 '+d.dlt645.meterAddress:d.slaveId?' · 从站 '+d.slaveId:''
 const health=d.health&&c.role==='collect'&&!statusError.value?` · 正常 ${d.health.healthy} · 异常 ${d.health.failed+d.health.stale}`:''
 return `${d.name||d.deviceKey} · ${deviceStateLabel(d)}\n${d.deviceKey} · ${protocolLabel(channelProtocol(c))}\n${address}${station}\n${pointCount(d)} 个点位${health}${c.role==='forward'?' · 不判定远端在线':''}\n点击查看设备配置`
}
function open(r:any,c?:any,d?:any){if(r.resourceKey==='__unassigned')return;emit('openDevice',{resourceKey:r.resourceKey,channelKey:c?.channelKey,deviceKey:d?.deviceKey})}
async function poll(){if(polling)return;polling=true;try{const o=await api<CollectionOverview>('/api/status/overview',{timeoutMs:10000});if(disposed)return;overview.value=o;statusError.value='';updated.value=new Date().toLocaleTimeString()}catch(e){if(!disposed)statusError.value=e instanceof Error?e.message:'状态刷新失败'}finally{polling=false}}
async function load(){if(loading.value)return;loading.value=true;try{await configStore.load();if(disposed)return;error.value='';loaded.value=true;await poll()}catch(e){if(!disposed)error.value=e instanceof Error?e.message:'拓扑配置加载失败'}finally{loading.value=false}}
onMounted(()=>{void load();timer=window.setInterval(poll,3000)})
onBeforeUnmount(()=>{disposed=true;window.clearInterval(timer)})
</script>
<template>
 <section class="topology-page card">
  <div class="topology-heading"><div class="topology-heading-title"><span class="topology-heading-icon"><IconNetwork :size="23" aria-hidden="true"/></span><div><h2>设备拓扑</h2><p>云端互联 · 网关接入 · 设备状态一览</p></div></div><div class="topology-heading-actions"><button :disabled="loading" @click="load">{{loading?'读取中…':'重新加载'}}</button><slot name="actions"/></div></div>
  <p v-if="error" class="notice bad" role="alert">{{error}}</p><p v-if="statusError" class="notice bad" role="alert">实时状态读取失败：{{statusError}}。下方状态暂不可确认。</p>
  <div class="topology-toolbar"><label class="topology-search"><IconSearch :size="16" aria-hidden="true"/><input v-model="search" aria-label="搜索拓扑" placeholder="搜索端口、设备、地址或协议"/></label><select v-model="filter" aria-label="筛选资源类型"><option value="all">全部资源</option><option value="serial">串口资源</option><option value="network">网口资源</option></select><div class="topology-fold-actions" role="group" aria-label="拓扑展开与折叠"><button type="button" @click="folded=new Set()"><IconChevronsDown :size="15" aria-hidden="true"/>全部展开</button><button type="button" @click="folded=new Set(resources.map((r:any)=>r.resourceKey))"><IconChevronsUp :size="15" aria-hidden="true"/>全部折叠</button></div><span class="topology-updated">{{updated?'状态更新 '+updated+' · 每 3 秒刷新':'正在获取状态'}}</span></div>
  <div class="topology-legend"><span class="node-status healthy"><i/>采集正常</span><span class="node-status partial"><i/>部分异常</span><span class="node-status failed"><i/>采集异常</span><span class="node-status pending"><i/>待采集 / 未知</span><span class="node-status disabled"><i/>已停用</span><span class="node-status forward"><i/>转发配置（不判定远端在线）</span></div>
  <div class="topology-canvas" tabindex="0" aria-label="设备连接拓扑，可横向滚动查看所有端口">
  <div class="topology-diagram" :style="{'--ports':Math.max(visible.length,1)}">
  <div class="topology-cloud">
   <IconCloud class="cloud-symbol" :size="76" stroke="1.3" aria-hidden="true"/>
   <div class="cloud-caption"><strong>云平台</strong><p>远程管理 · 数据服务</p><span class="node-status cloud-status" :class="cloudState"><i/>{{cloudLabel}}</span></div>
  </div>
  <div class="cloud-uplink"><IconArrowsUpDown :size="20" aria-hidden="true"/><span>数据上报 / 远程管理</span></div>
  <div class="topology-gateway">
   <div class="gateway-identity"><span class="gateway-icon"><IconCpu :size="30" stroke="1.5" aria-hidden="true"/></span><div class="gateway-label"><strong>边缘网关</strong><span class="gateway-key" :title="config.gatewayKey||'网关编号未配置'">{{config.gatewayKey||'网关编号未配置'}}</span></div></div>
   <div class="gateway-capacity" aria-label="网关接入规模"><span><IconPlugConnected :size="13" aria-hidden="true"/><b>{{serialCount}}</b>串口</span><span><IconNetwork :size="13" aria-hidden="true"/><b>{{networkCount}}</b>网口</span><span><IconDeviceDesktop :size="13" aria-hidden="true"/><b>{{deviceCount}}</b>设备</span></div>
  </div>
  <p v-if="!loaded&&!error" class="topology-empty">正在加载拓扑…</p>
  <p v-else-if="loaded&&!visible.length" class="topology-empty">{{resources.length?'没有匹配的资源或设备':'尚未配置资源，请在智能网关中添加串口或网口。'}}</p>
  <div v-if="visible.length" class="topology-branches">
   <section v-for="r in visible" :key="r.resourceKey" class="topology-resource">
    <div class="resource-node"><IconArrowDown class="resource-arrow" :size="16" aria-hidden="true"/><span class="resource-icon" :class="r.type"><component :is="r.type==='serial'?IconPlugConnected:IconNetwork" :size="26" stroke="1.5" aria-hidden="true"/></span><button class="resource-name" :disabled="r.resourceKey==='__unassigned'" :title="'查看资源：'+(r.name||r.resourceKey)" @click="open(r)"><strong>{{r.name||r.resourceKey}}</strong><span>{{r.type==='serial'?r.serial?.port:r.network?.interface||'未绑定'}}<template v-if="r.type==='serial'"> · {{r.serial?.baudRate||'—'}} bps</template></span></button><span class="resource-summary">{{r.channels.reduce((n:number,c:any)=>n+c.devices.length,0)}} 个设备<span v-if="resourceDisabled(r)"> · 已停用</span></span><button type="button" class="resource-fold" :aria-expanded="!folded.has(r.resourceKey)" :aria-label="(folded.has(r.resourceKey)?'展开':'折叠')+r.name" :title="(folded.has(r.resourceKey)?'展开':'折叠')+r.name" @click="toggle(r.resourceKey)"><IconChevronDown :size="17" stroke="1.8" aria-hidden="true"/></button></div>
    <div v-if="!folded.has(r.resourceKey)" class="topology-channels">
     <p v-if="!r.channels.length" class="topology-empty resource-empty">空闲资源 · 尚未配置通道和设备</p>
     <section v-for="role in (['collect','forward'] as const)" v-show="r.channels.some((c:any)=>c.role===role)" :key="role" class="topology-role" :class="role"><h3>{{role==='collect'?'采集':'转发'}}</h3>
      <div v-for="c in r.channels.filter((c:any)=>c.role===role)" :key="c.channelKey" class="topology-channel">
       <div class="channel-node"><strong>{{c.name||c.channelKey}}</strong><span>{{protocolLabel(channelProtocol(c))}}</span><small v-if="c.enabled===false">通道已停用</small></div>
       <div class="topology-devices"><span v-if="!c.devices.length" class="topology-empty">尚未添加设备</span><button v-for="d in c.devices" :key="d.deviceKey" class="topology-device" :class="statusError?'pending':d.state" :title="deviceDetails(r,c,d)" :aria-label="deviceDetails(r,c,d)" :disabled="r.resourceKey==='__unassigned'" @click="open(r,c,d)"><span class="device-icon"><component :is="deviceIcon(d,c)" :size="21" stroke="1.5" aria-hidden="true"/></span><strong>{{d.name||d.deviceKey}}</strong><span class="node-status device-led" :class="statusError?'pending':d.state" aria-hidden="true"><i/></span></button></div>
      </div>
     </section>
    </div>
   </section>
  </div>
  </div></div>
  <div class="topology-footnote"><span>悬浮设备查看详情 · 点击进入配置 · 支持搜索与折叠</span><span>连接关系来自已保存配置，非物理接线自动探测。</span></div>
 </section>
</template>
<style scoped>
.topology-page{min-width:0}
.topology-heading{display:flex;justify-content:space-between;gap:16px;align-items:center}
.topology-heading-title{display:flex;align-items:center;gap:12px}
.topology-heading-icon{display:grid;place-items:center;width:44px;height:44px;border-radius:12px;background:#edf4ff;color:#2675c9;flex-shrink:0}
.topology-heading h2{font-size:20px;margin:0;color:#203653}
.topology-heading p{font-size:12px;color:#64748b;margin:5px 0 0;line-height:1.6}
.topology-heading-actions{display:flex;align-items:center;justify-content:flex-end;gap:8px;flex-shrink:0;white-space:nowrap}
@media(max-width:650px){.topology-heading{flex-wrap:wrap}.topology-heading-actions{margin-left:auto}}
.topology-toolbar{display:flex;flex-wrap:wrap;align-items:center;gap:8px;margin:18px 0 12px}
.topology-search{display:flex;align-items:center;gap:8px;flex:1;max-width:370px;min-width:200px;color:#708198}
.topology-search input{width:100%}
.topology-toolbar>select{width:145px;flex:0 0 145px}
.topology-updated{margin-left:auto;font-size:11px;color:#64748b}
.topology-legend{display:flex;gap:18px;flex-wrap:wrap;padding:10px 14px;margin-bottom:12px;border:1px solid #e8eef5;background:#f8fafd;border-radius:9px}
.node-status{display:inline-flex;align-items:center;gap:6px;font-size:11px;font-weight:400;color:#64748b;white-space:nowrap}
.node-status i{box-sizing:border-box;width:7px;height:7px;background:currentColor;border-radius:50%}
.node-status.healthy{color:#16804a}.node-status.partial{color:#bb7a12}.node-status.failed{color:#d34444}
.node-status.disabled{color:#94a3b8}.node-status.forward{color:#497cc4}
.node-status.pending i{background:transparent;border:1.5px solid currentColor}
.topology-canvas{overflow:auto;border:1px solid #e1e9f3;border-radius:14px;background:#f7f9fc;max-width:100%;scrollbar-width:thin}
.topology-canvas:focus-visible{outline:2px solid #1677ff;outline-offset:2px}
.topology-diagram{padding:22px 20px 24px;min-width:calc(var(--ports)*245px);position:relative}
.topology-cloud{display:flex;align-items:center;justify-content:center;gap:17px;width:270px;max-width:100%;margin:4px auto;background:#fff;border:1px solid #e4edf8;border-radius:28px;padding:13px 20px;box-shadow:0 8px 24px #3d76af0c}
.cloud-symbol{flex:0 0 76px;color:#3c8ce8;filter:drop-shadow(0 5px 4px #3c8ce817)}
.cloud-symbol :deep(path:not([stroke="none"])){fill:#e9f3ff}
.cloud-caption{min-width:0}
.cloud-caption strong{display:block;font-size:18px;font-weight:600;line-height:1.4;color:#264f7c;letter-spacing:1px}
.cloud-caption p{font-size:10px;line-height:1.5;color:#738ba6;margin:5px 0 7px;white-space:nowrap}
.cloud-status{font-size:10px;line-height:16px;gap:5px;padding:2px 7px;border-radius:12px;background:#f2f5f9;white-space:nowrap}
.cloud-status.healthy{color:#218358;background:#eaf6ef}.cloud-status.failed{color:#c13e4c;background:#fff0f1}.cloud-status.pending,.cloud-status.disabled{color:#6d7d90;background:#eff3f7}
@media(max-width:650px){.topology-cloud{width:260px;padding:12px 16px;gap:14px}}
.cloud-uplink{height:36px;position:relative;display:flex;align-items:center;justify-content:center;color:#86a1c3}
.cloud-uplink::before{content:'';position:absolute;height:100%;border-left:1px solid #bacde4;left:50%}
.cloud-uplink svg{z-index:1;background:#f7f9fc}.cloud-uplink span{position:absolute;left:calc(50% + 20px);font-size:10px;white-space:nowrap;color:#73839a}
.topology-gateway{width:300px;max-width:100%;margin:0 auto;background:#213d60;border:1px solid #304f74;border-radius:14px;color:#f5f9ff;box-shadow:0 8px 20px #1734511a;overflow:hidden}
.gateway-identity{display:flex;align-items:center;gap:13px;padding:15px 18px}
.gateway-icon{display:grid;place-items:center;width:44px;height:44px;border-radius:11px;background:#304f74;border:1px solid #496b94;color:#b9dbff;flex-shrink:0}
.gateway-label{min-width:0}.gateway-label>strong{display:block;font-size:17px;line-height:1.4;font-weight:600;letter-spacing:.5px;color:#fff}
.gateway-key{display:block;font-size:11px;line-height:1.5;letter-spacing:.4px;color:#b7c9df;margin-top:4px;overflow-wrap:anywhere;font-variant-numeric:tabular-nums}
.gateway-capacity{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));padding:10px 8px;border-top:1px solid #3e5979;background:#1c3554;gap:0}
.gateway-capacity>span{display:flex;align-items:center;justify-content:center;gap:5px;white-space:nowrap;color:#b7c9df;font-size:10px;line-height:16px}.gateway-capacity>span+span{border-left:1px solid #3e5979}
.gateway-capacity svg{color:#92b9e2;flex-shrink:0}.gateway-capacity b{font-weight:600;font-size:12px;color:#f5f9ff;font-variant-numeric:tabular-nums}
.topology-branches{display:flex;position:relative;padding-top:30px}
.topology-branches::before{content:'';position:absolute;top:0;left:50%;height:30px;border-left:1px solid #b2c6df}
.topology-resource{position:relative;flex:1;min-width:0;padding:24px 12px 0}
.topology-resource::before{content:'';position:absolute;left:0;right:0;top:0;border-top:1px solid #b2c6df}
.topology-resource:first-child::before{left:50%}.topology-resource:last-child::before{right:50%}.topology-resource:only-child::before{display:none}
.resource-node{display:grid;position:relative;grid-template-columns:38px minmax(0,1fr) 32px;grid-template-rows:auto auto;column-gap:10px;row-gap:4px;align-items:center;min-height:70px;text-align:left;margin:0 auto 16px;padding:10px 12px;max-width:270px;background:#fff;border:1px solid #dce6f2;border-radius:11px;box-shadow:0 3px 10px #254c7905}
.resource-node::before{content:'';position:absolute;top:-25px;height:24px;left:50%;border-left:1px solid #b2c6df}
.resource-arrow{position:absolute;top:-12px;left:calc(50% - 8px);color:#8ea7c8}
.resource-icon{display:grid;place-items:center;grid-row:1 / 3;background:#ebf4ff;color:#2675c9;border:1px solid #d6e8fd;border-radius:9px;width:38px;height:38px}
.resource-icon.network{color:#616bb7;background:#f0f1fc;border-color:#e0e3f6}
.topology-page .resource-name{grid-column:2;height:auto!important;min-height:0;padding:0!important;display:grid;gap:3px;border:0;background:transparent;text-align:left;box-shadow:none;max-width:100%;white-space:normal}
.resource-name strong{font-size:13px;color:#253c5d}.resource-name:hover strong{color:#1677ff}
.resource-name span,.resource-summary{font-size:10px;color:#708198;overflow-wrap:anywhere}.resource-summary{grid-column:2}
.topology-page .resource-fold{grid-column:3;grid-row:1 / 3;display:grid;place-items:center;padding:0!important;min-width:32px!important;width:32px!important;min-height:32px!important;height:32px!important;border:0!important;border-radius:50%;background:transparent;color:#71849e;box-shadow:none!important;transition:background .15s,color .15s}
.topology-page .resource-fold:hover{background:#edf4fd;color:#2675c9}
.topology-page .resource-fold:active{background:#dceafb}
.resource-fold svg{transition:transform .18s ease}
.resource-fold[aria-expanded="false"] svg{transform:rotate(-90deg)}
.topology-page .resource-fold[aria-expanded="false"]{color:#2675c9;background:#f0f5fc}
.topology-page .resource-fold:focus-visible{outline:2px solid #2675c9;outline-offset:2px}
.topology-fold-actions{display:flex;align-items:center;gap:2px;padding:3px;border-radius:9px;background:#f0f4f9}
.topology-page .topology-fold-actions button{display:inline-flex;align-items:center;justify-content:center;gap:5px;height:30px!important;min-height:30px!important;padding:0 10px!important;white-space:nowrap;border:0!important;border-radius:6px;background:transparent;color:#536b89;box-shadow:none!important;font-size:11px}
.topology-page .topology-fold-actions button:hover{background:#fff;color:#2675c9}
.topology-page .topology-fold-actions button:active{background:#e0ebf9}
.topology-page .topology-fold-actions button:focus-visible{outline:2px solid #2675c9;outline-offset:1px}
@media(prefers-reduced-motion:reduce){.resource-fold svg{transition:none}}
.topology-channels{position:relative;min-width:0;max-width:270px;margin:0 auto}
.topology-channels::before{content:'';position:absolute;left:50%;top:-16px;bottom:24px;border-left:1px solid #c9d7e8}
.topology-role h3{position:relative;display:flex;align-items:center;justify-content:center;gap:8px;font-size:10px;letter-spacing:1px;color:#387abb;text-align:center;margin:12px 0 8px;background:#f7f9fc}
.topology-role h3::before,.topology-role h3::after{content:'';width:24px;height:1px;background:#d5e2f2}
.topology-role.forward h3{color:#996c2c}.topology-role.forward h3::before,.topology-role.forward h3::after{background:#e5d6bf}
.topology-channel{position:relative;margin:8px 0 12px}
.channel-node{position:relative;display:flex;flex-wrap:wrap;justify-content:center;align-items:center;column-gap:6px;row-gap:4px;min-width:0;text-align:center;margin:0 0 7px;padding:3px 4px;background:#f7f9fc}
.channel-node strong{font-size:11px;font-weight:500;color:#536b89;overflow-wrap:anywhere}
.channel-node span,.channel-node small{font-size:10px;color:#7c8da3;overflow-wrap:anywhere}
.topology-devices{display:grid;grid-template-columns:minmax(0,1fr);gap:10px;position:relative}
.topology-devices::before{content:'';position:absolute;top:-8px;bottom:20px;left:50%;border-left:1px solid #c9d7e8}
.topology-page .topology-device{position:relative;display:flex;align-items:center;gap:10px;width:100%;height:56px!important;min-height:56px;padding:10px 12px!important;text-align:left;white-space:normal;border:1px solid #dde6f0;border-radius:10px;background:#fff;color:#334155;box-shadow:0 2px 8px #18385b07;min-width:0;transition:box-shadow .15s,border-color .15s}
.topology-page .topology-device:hover{background:#fff!important;box-shadow:0 5px 16px #18385b12!important;border-color:#8fb6e5}
.topology-device:focus-visible,.resource-name:focus-visible{outline:2px solid #1677ff;outline-offset:2px}
.device-icon{display:grid;place-items:center;flex:0 0 32px;height:32px;background:#f0f5fb;color:#6280a3;border-radius:8px}
.topology-device strong{display:block;min-width:0;flex:1;font-size:12px;font-weight:600;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}
.device-led{flex-shrink:0;padding:4px}.device-led i{width:8px;height:8px;box-shadow:0 0 0 3px color-mix(in srgb,currentColor 10%,transparent)}
.topology-empty{position:relative;font-size:12px;color:#8292a7;padding:16px;text-align:center;background:#f7f9fc}
.resource-empty{font-size:11px;border:1px dashed #d5e0ee;border-radius:9px;line-height:1.7}
.topology-footnote{display:flex;flex-wrap:wrap;justify-content:space-between;gap:6px 16px;font-size:11px;color:#7c8da3;margin-top:12px;line-height:1.6}
@media(max-width:650px){.topology-heading{align-items:start}.topology-heading-icon{width:36px;height:36px;border-radius:10px}.topology-heading-title{gap:8px}.topology-heading h2{font-size:18px}.topology-heading p{font-size:11px}.topology-updated{margin-left:0;width:100%}.topology-search{min-width:100%;max-width:none}.topology-toolbar{gap:6px}.topology-legend{gap:10px;padding:10px}.topology-diagram{padding:18px 8px}.topology-gateway{width:280px}.gateway-identity{padding:13px 16px}.gateway-label>strong{font-size:16px}.topology-page{padding:12px!important}.topology-resource{padding-left:8px;padding-right:8px}}
</style>
