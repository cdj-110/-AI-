<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, provide, ref } from 'vue'
import StatusView from './components/StatusView.vue'
import MaintenanceView from './components/MaintenanceView.vue'
import WiFiView from './components/WiFiView.vue'
import CellularView from './components/CellularView.vue'
import StorageView from './components/StorageView.vue'
import CloudView from './components/CloudView.vue'
import UserManagementView from './components/UserManagementView.vue'
import SmartGatewayView from './components/SmartGatewayView.vue'
import EdgeComputeView from './components/EdgeComputeView.vue'
import AiAssistantView from './components/AiAssistantView.vue'
import GlobalConfirmDialog from './components/GlobalConfirmDialog.vue'
import { useConfigStore } from './stores/config'
import { api } from './api'
import { openGatewayRealtime } from './realtime'
import type { GatewayStatus, PointStatus } from './types'
import brandLogo from '../../internal/web/frontend/brand-logo.png'

type NavKey = 'status'|'config'|'edge'|'ai'|'wifi'|'cellular'|'offline-cache'|'cloud'|'users'|'maintenance'
const active = ref<NavKey>('status')
const gatewayMode = ref<'config'|'run'>('run')
const edgeMode = ref<'config'|'run'>('run')
const treeSelection = ref<{resourceKey?:string;channelKey?:string;deviceKey?:string;action?:'add-channel'|'add-device';nonce:number}>({nonce:0})
const collapsed = ref(localStorage.getItem('gateway-vue-nav-collapsed') === 'true')
const resourceFolded = ref<Record<string,boolean>>({})
const channelFolded = ref<Record<string,boolean>>({})
const moreOpen = ref(false)
const logoutOpen = ref(false)
const cloudUnsavedOpen = ref(false)
const cloudDirty = ref(false)
const pendingNav = ref<NavKey | null>(null)
const cloudView = ref<InstanceType<typeof CloudView>>()
const projectFile = ref<HTMLInputElement>()
const configStore = useConfigStore()
const config = computed(() => configStore.value)
const gatewayStatus = ref<GatewayStatus | null>(null)
provide('gatewayRealtimeStatus', gatewayStatus)
const gatewayPointIndex = new Map<string,number>()
const wifiStatus = ref<any>(null)
const cellularStatus = ref<any>(null)
let stopRealtime = () => {}
let navStatusTimer = 0
const navItems: Array<{key:NavKey; label:string; icon:string; description:string}> = [
  { key:'status', label:'运行状态', icon:'⌁', description:'网关、点位及连接的实时状态' },
  { key:'config', label:'智能网关', icon:'▣', description:'资源、通道、设备和点位配置' },
  { key:'edge', label:'边缘计算', icon:'⌘', description:'计算组、表达式和输出点位' },
  { key:'ai', label:'AI 助手', icon:'✦', description:'配置诊断与 PDF 智能生成' },
  { key:'wifi', label:'WiFi', icon:'⌁', description:'无线网络连接与地址配置' },
  { key:'cellular', label:'移动网络', icon:'▥', description:'蜂窝网络、SIM 与拨号状态' },
  { key:'offline-cache', label:'断线续存', icon:'◫', description:'可移动存储与离线补传' },
  { key:'cloud', label:'云平台连接', icon:'☁', description:'MQTT 通道和微控云激活' },
  { key:'users', label:'用户管理', icon:'♙', description:'用户、角色和权限管理' },
  { key:'maintenance', label:'系统维护', icon:'⌕', description:'诊断、审计和维护操作' },
]
const current = computed(() => navItems.find(item => item.key === active.value)!)
const workspaceMode = computed(() => active.value === 'edge' ? edgeMode.value : gatewayMode.value)
function setWorkspaceMode(value:'config'|'run') {
  if (active.value === 'edge') edgeMode.value = value
  else gatewayMode.value = value
}
const resourceTree = computed(() => {
  const resources = Array.isArray(config.value.resources) ? config.value.resources : []
  const channels = Array.isArray(config.value.channels) ? config.value.channels : []
  const devices = Array.isArray(config.value.devices) ? config.value.devices : []
  const forwardDevices = Array.isArray(config.value.forwardDevices) ? config.value.forwardDevices : []
  return resources.map((resource: any, resourceIndex: number) => ({
    ...resource,
    _resourceIndex: resourceIndex,
    channels: channels.filter((channel: any) => channel.resourceKey === resource.resourceKey).map((channel: any) => ({
      ...channel,
      _channelIndex: channels.indexOf(channel),
      devices: [
        ...devices.filter((device: any) => device.channelKey === channel.channelKey).map((device: any) => ({ ...device, _deviceIndex: devices.indexOf(device), _forwardDeviceIndex: -1 })),
        ...forwardDevices.filter((device: any) => device.channelKey === channel.channelKey).map((device: any) => ({ ...device, _deviceIndex: -1, _forwardDeviceIndex: forwardDevices.indexOf(device) })),
      ],
    })),
  }))
})
const allTreeChildrenFolded = computed(() => resourceTree.value.length > 0 && resourceTree.value.every((resource:any) => resourceFolded.value[resource.resourceKey] === true))
function toggleAllTreeChildren(){
  const folded=!allTreeChildrenFolded.value
  for(const resource of resourceTree.value){
    resourceFolded.value[resource.resourceKey]=folded
    for(const channel of resource.channels||[])channelFolded.value[channel.channelKey]=folded
  }
}
function toggleResourceChildren(resource:any){resourceFolded.value[resource.resourceKey]=!resourceFolded.value[resource.resourceKey]}
function toggleChannelChildren(channel:any){channelFolded.value[channel.channelKey]=!channelFolded.value[channel.channelKey]}
function navFromURL(){
  const key=new URLSearchParams(window.location.search).get('page') as NavKey
  return navItems.some(item=>item.key===key)?key:undefined
}
function rememberNav(key:NavKey){
  localStorage.setItem('gateway-vue-active-tab',key)
  const url=new URL(window.location.href)
  url.searchParams.set('page',key)
  window.history.replaceState(window.history.state,'',url)
}
function applyNav(key:NavKey){active.value=key;rememberNav(key)}
function select(key: NavKey) { if(active.value==='cloud'&&key!=='cloud'&&cloudDirty.value){pendingNav.value=key;cloudUnsavedOpen.value=true;return}applyNav(key) }
async function discardCloudAndLeave(){const target=pendingNav.value;cloudUnsavedOpen.value=false;await configStore.load();cloudDirty.value=false;if(target)applyNav(target)}
async function saveCloudAndLeave(){const target=pendingNav.value;if(await cloudView.value?.saveBeforeLeave()){cloudUnsavedOpen.value=false;cloudDirty.value=false;if(target)applyNav(target)}}
function toggleNav() { collapsed.value = !collapsed.value; localStorage.setItem('gateway-vue-nav-collapsed', String(collapsed.value)) }
function closeMore() { moreOpen.value = false }
async function importProject(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !window.confirm('导入工程会覆盖当前配置，确认继续？')) { input.value = ''; return }
  try {
    const form = new FormData()
    form.append('file', file)
    await api('/api/project/import', { method:'POST', body:form, timeoutMs:40000 })
    await configStore.load()
    window.alert('工程导入成功，配置已热加载')
  } catch (cause) { window.alert(cause instanceof Error ? cause.message : '工程导入失败') }
  finally { input.value = ''; closeMore() }
}
async function logout() {
  await fetch('/logout', { method:'POST' })
  window.location.assign('/login')
}
function openDevice(resource: any, channel: any, device?: any) {
  select('config')
  treeSelection.value = { resourceKey:resource.resourceKey, channelKey:channel.channelKey, deviceKey:device?.deviceKey, nonce:Date.now() }
}
function addFromTree(resource:any){
  select('config')
  const serialChannel=resource.type==='serial'?resource.channels[0]:undefined
  treeSelection.value=resource.type==='serial'
    ? {resourceKey:resource.resourceKey,channelKey:serialChannel?.channelKey,action:'add-device',nonce:Date.now()}
    : {resourceKey:resource.resourceKey,action:'add-channel',nonce:Date.now()}
  gatewayMode.value='config'
}
function protocolName(value:string){return ({'modbus-rtu':'Modbus RTU','modbus-tcp':'Modbus TCP','iec104':'IEC104','iec104-server':'IEC104 服务端','modbus-tcp-slave':'Modbus TCP 从站','iec61850':'IEC 61850 MMS','iec61850-mms-server':'IEC61850 MMS 服务端','iec61850-goose':'IEC61850 GOOSE','iec61850-goose-publisher':'IEC61850 GOOSE 发布','siemens-s7':'Siemens S7','opcua':'OPC UA'} as Record<string,string>)[value]||value||'-'}
function forwardDeviceProtocolName(value:string){return ({'iec104-server':'IEC104','modbus-tcp-slave':'Modbus TCP','iec61850-mms-server':'IEC61850 MMS','iec61850-goose-publisher':'IEC61850 GOOSE'} as Record<string,string>)[value]||protocolName(value)}
function resourceDeviceCount(resource:any){return resource.channels.reduce((total:number,item:any)=>total+item.devices.length,0)}
function deviceTreeStatus(device:any){
  if(device.enabled===false)return'device-disabled'
  const allStatuses=gatewayStatus.value?.points||[]
  let statuses=allStatuses.filter(point=>point.deviceKey===device.deviceKey)
  const forwardDevice=(config.value.forwardDevices||[]).find((item:any)=>item.deviceKey===device.deviceKey)
  if(forwardDevice){
    const sourceKeys=new Set((forwardDevice.points||[]).map((point:any)=>`${point.sourceDeviceKey}::${point.sourceMetric}`))
    statuses=allStatuses.filter(point=>sourceKeys.has(`${point.deviceKey}::${point.metric}`))
  }
  // The tree marker represents device communication, not whether every point is
  // configured correctly. A fresh successful point proves that the device is
  // online even when another point still carries a historical/configuration error.
  if(statuses.some(point=>point.updatedAt&&!point.stale))return'device-online'
  if(statuses.some(point=>point.error))return'device-error'
  if(statuses.some(point=>point.stale))return'device-stale'
  return'device-pending'
}
function closeMoreFromPage() { closeMore() }
const wifiNav=computed(()=>{const value=wifiStatus.value;if(!value)return{className:'offline',text:'读取中',title:'正在读取 WiFi 状态'};if(value.connected)return{className:'online',text:value.ssid||'已连接',title:`WiFi 已连接：${value.ssid||'-'}`};if(value.available)return{className:'',text:'未连接',title:value.message||'WiFi 未连接'};return{className:'offline',text:'无WiFi',title:value.message||'未检测到 WiFi 网卡'}})
const cellularNav=computed(()=>{const value=cellularStatus.value;if(!value)return{className:'offline',text:'读取中',title:'正在读取移动网络状态'};if(!value.enabled)return{className:'offline',text:'未启用',title:'移动网络未启用'};const signal=Number(value.sim?.signal||0);if(!value.available||!signal)return{className:'offline',text:'无信号',title:'移动网络信号：无信号'};if(signal<45)return{className:'weak',text:`${signal}%`,title:`移动网络信号：${signal}%`};return{className:'online',text:`${signal}%`,title:`移动网络信号：${signal}%`}})
const cloudNav=computed(()=>{const enabled=Object.entries(gatewayStatus.value?.mqttChannels||{}).filter(([key,value])=>(key==='activation'||key.startsWith('manual-'))&&value.enabled).map(([,value])=>value);if(!enabled.length)return{className:'offline',text:'未启用',title:'当前没有启用的云平台连接'};const connected=enabled.filter(item=>item.connected).length;if(connected===enabled.length)return{className:'online',text:'连接成功',title:`已启用 ${enabled.length} 个连接，连接成功`};if(connected)return{className:'weak',text:`${connected}/${enabled.length}`,title:`已启用 ${enabled.length} 个连接，${connected} 个已连接`};return{className:'offline',text:'未连接',title:`已启用 ${enabled.length} 个连接，均未连接`}})
async function loadNavStatus(){const [wifi,cellular]=await Promise.allSettled([api('/api/network/wifi'),api('/api/network/cellular')]);wifiStatus.value=wifi.status==='fulfilled'?wifi.value:{available:false,message:'WiFi 状态读取失败'};cellularStatus.value=cellular.status==='fulfilled'?cellular.value:{enabled:config.value.cellular?.enabled!==false,available:false}}
function indexGatewayPoints(points:PointStatus[]){gatewayPointIndex.clear();points.forEach((point,index)=>gatewayPointIndex.set(`${point.deviceKey}::${point.metric}`,index));return points}
function mergeGatewayPoints(current:PointStatus[],payload:any){const incoming=Array.isArray(payload?.points)?payload.points as PointStatus[]:[];if(payload?.replace)return indexGatewayPoints(incoming);if(!incoming.length)return current;if(gatewayPointIndex.size!==current.length)indexGatewayPoints(current);const next=current.slice();for(const point of incoming){const key=`${point.deviceKey}::${point.metric}`,index=gatewayPointIndex.get(key);if(index===undefined){gatewayPointIndex.set(key,next.length);next.push(point)}else next[index]=point}return next}
function handleRealtime(message:any){if(message.type==='snapshot'){const points=configStore.largeMode?[]:Array.isArray(message.payload?.points)?indexGatewayPoints(message.payload.points):[];gatewayStatus.value={...message.payload,points}}else if(message.type==='gateway.status'&&gatewayStatus.value)gatewayStatus.value={...gatewayStatus.value,...message.payload,points:gatewayStatus.value.points,errors:gatewayStatus.value.errors};else if(message.type==='points.diff'&&gatewayStatus.value&&!configStore.largeMode)gatewayStatus.value={...gatewayStatus.value,points:mergeGatewayPoints(gatewayStatus.value.points||[],message.payload||{})};else if(message.type==='communications.diff'&&gatewayStatus.value)gatewayStatus.value={...gatewayStatus.value,mqttEnabled:!!message.payload?.mqttEnabled,mqttConnected:!!message.payload?.mqttConnected,mqttChannels:message.payload?.mqttChannels||{}}}
onMounted(async () => { window.addEventListener('click', closeMoreFromPage); const saved = navFromURL()||(localStorage.getItem('gateway-vue-active-tab') as NavKey); if (navItems.some(item => item.key === saved)) active.value = saved;rememberNav(active.value); try { await configStore.load() } catch {};await loadNavStatus();if(!configStore.largeMode)stopRealtime=openGatewayRealtime(handleRealtime);navStatusTimer=window.setInterval(loadNavStatus,10000) })
onBeforeUnmount(() => {window.removeEventListener('click', closeMoreFromPage);stopRealtime();window.clearInterval(navStatusTimer)})
</script>

<template>
  <GlobalConfirmDialog />
  <div class="app-shell" :class="{ collapsed }">
    <aside class="sidebar main-nav">
      <div class="brand">
        <img class="brand-mark" :src="brandLogo" alt="微控网关" />
        <span class="brand-text"><span class="brand-title">微控网关</span><span class="brand-subtitle">Edge Gateway</span></span>
        <button class="nav-collapse" :title="collapsed ? '展开菜单' : '折叠菜单'" :aria-label="collapsed ? '展开菜单' : '折叠菜单'" @click="toggleNav"><svg class="nav-icon" viewBox="0 0 24 24"><path d="M15 6l-6 6 6 6"/><path d="M20 4v16"/></svg></button>
      </div>
      <nav class="main-menu">
        <template v-for="item in navItems" :key="item.key">
          <button :class="[{active:active===item.key}, {'nav-with-status':['wifi','cellular','cloud'].includes(item.key)}]" :title="item.label" @click="select(item.key)">
            <svg v-if="item.key==='status'" class="nav-icon" viewBox="0 0 24 24"><path d="M4 19h16"/><path d="M6 16l4-5 4 3 4-7"/></svg>
            <svg v-else-if="item.key==='config'" class="nav-icon" viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6"/><path d="M9 13h6"/><path d="M9 17h3"/></svg>
            <svg v-else-if="item.key==='edge'" class="nav-icon" viewBox="0 0 24 24"><circle cx="6" cy="12" r="2"/><circle cx="18" cy="6" r="2"/><circle cx="18" cy="18" r="2"/><path d="M8 11l8-4"/><path d="M8 13l8 4"/></svg>
            <svg v-else-if="item.key==='ai'" class="nav-icon" viewBox="0 0 24 24"><path d="M12 3l1.4 4.2L18 9l-4.6 1.8L12 15l-1.4-4.2L6 9l4.6-1.8z"/><path d="M18.5 14l.8 2.2 2.2.8-2.2.8-.8 2.2-.8-2.2-2.2-.8 2.2-.8z"/></svg>
            <svg v-else-if="item.key==='wifi'" class="nav-icon" viewBox="0 0 24 24"><path d="M5 10a11 11 0 0 1 14 0"/><path d="M8 14a6 6 0 0 1 8 0"/><path d="M12 18h.01"/></svg>
            <svg v-else-if="item.key==='cellular'" class="nav-icon" viewBox="0 0 24 24"><path d="M6 18h.01"/><path d="M10 18v-4"/><path d="M14 18v-8"/><path d="M18 18V6"/></svg>
            <svg v-else-if="item.key==='offline-cache'" class="nav-icon" viewBox="0 0 24 24"><path d="M4 7c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3z"/><path d="M4 7v5c0 1.7 3.6 3 8 3s8-1.3 8-3V7"/><path d="M4 12v5c0 1.7 3.6 3 8 3s8-1.3 8-3v-5"/></svg>
            <svg v-else-if="item.key==='cloud'" class="nav-icon" viewBox="0 0 24 24"><path d="M7 18a4 4 0 0 1 .7-7.9A5.5 5.5 0 0 1 18.5 12H19a3 3 0 0 1 0 6H7z"/></svg>
            <svg v-else-if="item.key==='users'" class="nav-icon" viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            <svg v-else class="nav-icon" viewBox="0 0 24 24"><path d="M14.5 6.5l3 3"/><path d="M5 19l4.5-1 8-8a2.1 2.1 0 0 0-3-3l-8 8z"/></svg>
            <span class="nav-label">{{ item.label }}</span>
            <span v-if="item.key==='wifi'" class="nav-status-pill" :class="wifiNav.className" :title="wifiNav.title"><i class="dot"/><span>{{wifiNav.text}}</span></span>
            <span v-if="item.key==='cellular'" class="nav-status-pill" :class="cellularNav.className" :title="cellularNav.title"><i class="dot"/><span>{{cellularNav.text}}</span></span>
            <span v-if="item.key==='cloud'" class="nav-status-pill" :class="cloudNav.className" :title="cloudNav.title"><i class="dot"/><span>{{cloudNav.text}}</span></span>
          </button>
          <div v-if="item.key==='config' && !collapsed" class="device-tree gateway-nav-tree">
            <div class="tree-title gateway-tree-title"><span>资源 / 通道 / 设备</span><button @click.stop="toggleAllTreeChildren">{{ allTreeChildrenFolded ? '全部展开' : '全部折叠' }}</button></div>
            <div v-for="resource in resourceTree" :key="resource.resourceKey || resource.name" class="gateway-tree-resource">
              <div class="gateway-tree-row resource-row"><button class="gateway-tree-group" :class="{active:treeSelection.resourceKey===resource.resourceKey&&!treeSelection.channelKey}" @click="select('config');treeSelection={resourceKey:resource.resourceKey,nonce:Date.now()}"><span>{{ resource.name || resource.resourceKey }}</span><small>{{ resource.type==='serial' ? `串口资源 · ${resource.serial?.port||resource.serial?.name||resource.name||'-'} · ${resourceDeviceCount(resource)} 台设备` : `网口资源 · ${resource.network?.interface||'-'} · ${resource.channels.length} 个通道` }}</small></button><button class="tree-mini" :title="resource.type==='serial'?'新增设备':'新增通道'" @click.stop="addFromTree(resource)">+</button><button class="tree-mini" :title="resourceFolded[resource.resourceKey]?'展开资源':'折叠资源'" :aria-label="resourceFolded[resource.resourceKey]?'展开资源':'折叠资源'" @click.stop="toggleResourceChildren(resource)"><span class="tree-chevron" :class="{collapsed:resourceFolded[resource.resourceKey]}"/></button></div>
              <template v-if="!resourceFolded[resource.resourceKey]&&resource.type==='serial'">
                <template v-for="channel in resource.channels" :key="channel.channelKey || channel.name">
                  <button v-for="device in channel.devices" :key="device.deviceKey" class="tree-device serial-tree-device" :class="[{active:treeSelection.deviceKey===device.deviceKey},deviceTreeStatus(device)]" @click="openDevice(resource,channel,device)"><i class="device-status-indicator" aria-hidden="true"/><span>{{ device.name || device.deviceKey }} <b>({{device.deviceKey}})</b></span><small>{{device.address||resource.serial?.port||'-'}} · 从站 {{device.slaveId||1}}</small></button>
                </template>
              </template>
              <template v-else-if="!resourceFolded[resource.resourceKey]" v-for="channel in resource.channels" :key="channel.channelKey || channel.name">
                <div class="gateway-tree-row channel-row"><button class="tree-channel" :class="{active:treeSelection.channelKey===channel.channelKey&&!treeSelection.deviceKey}" @click="openDevice(resource,channel)"><span><b>{{ channel.name || channel.channelKey }}</b><em :class="channel.role === 'forward' ? 'forward' : 'collect'">{{ channel.role === 'forward' ? '转发' : '采集' }}</em></span><small>{{protocolName(channel.role==='forward'?(channel.forwardProtocol||channel.protocol):channel.protocol)}} · {{channel.devices.length}} 台设备</small></button><button class="tree-mini" :title="channelFolded[channel.channelKey]?'展开通道':'折叠通道'" :aria-label="channelFolded[channel.channelKey]?'展开通道':'折叠通道'" @click.stop="toggleChannelChildren(channel)"><span class="tree-chevron" :class="{collapsed:channelFolded[channel.channelKey]}"/></button></div>
                <button v-if="!channelFolded[channel.channelKey]" v-for="device in channel.devices" :key="device.deviceKey" class="tree-device" :class="[{active:treeSelection.deviceKey===device.deviceKey},deviceTreeStatus(device)]" @click="openDevice(resource,channel,device)"><i class="device-status-indicator" aria-hidden="true"/><span>{{ device.name || device.deviceKey }} <b v-if="channel.role!=='forward'">({{device.deviceKey}})</b></span><small>{{channel.role==='forward'?`${forwardDeviceProtocolName(device.protocol||channel.forwardProtocol||channel.protocol)} · ${(device.points||[]).length} 个点位`:`${device.address||'-'} · 从站 ${device.slaveId||device.commonAddress||1}`}}</small></button>
              </template>
            </div>
          </div>
        </template>
      </nav>
    </aside>
    <main>
      <header class="header">
        <div class="header-left"><div class="header-title-block"><h1>网关状态</h1></div></div>
        <div class="header-actions">
          <span v-if="active==='config'||active==='edge'" class="point-mode-toggle"><button :class="{active:workspaceMode==='config'}" @click="setWorkspaceMode('config')">配置</button><button :class="{active:workspaceMode==='run'}" @click="setWorkspaceMode('run')">运行</button></span>
          <div class="header-more-wrap" @click.stop>
            <button class="header-more-button" title="打开设置菜单" aria-label="打开设置菜单" :aria-expanded="moreOpen" @click="moreOpen=!moreOpen"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.09a2 2 0 0 1 1 1.74v.5a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.09a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/></svg></button>
            <div v-if="moreOpen" class="header-more-menu" role="menu">
              <button @click="projectFile?.click()">导入工程</button>
              <a href="/api/project/export" @click="closeMore">导出工程</a>
              <button class="logout-button" @click="logoutOpen=true;closeMore()">退出登录</button>
            </div>
            <input ref="projectFile" class="hidden-file" type="file" accept=".json,application/json" @change="importProject" />
          </div>
        </div>
      </header>
      <div class="page"><StatusView v-if="active==='status'"/><SmartGatewayView v-else-if="active==='config'" v-model:mode="gatewayMode" :selection="treeSelection" @selection-change="treeSelection={...$event,nonce:Date.now()}"/><EdgeComputeView v-else-if="active==='edge'" v-model:mode="edgeMode"/><AiAssistantView v-else-if="active==='ai'"/><WiFiView v-else-if="active==='wifi'"/><CellularView v-else-if="active==='cellular'"/><StorageView v-else-if="active==='offline-cache'"/><CloudView v-else-if="active==='cloud'" ref="cloudView" @dirty="cloudDirty=$event"/><UserManagementView v-else-if="active==='users'"/><MaintenanceView v-else-if="active==='maintenance'"/></div>
    </main>
    <div v-if="logoutOpen" class="logout-modal"><div class="dialog-panel"><h3>退出登录</h3><p>确认退出当前网关管理会话？退出后需要重新登录。</p><div class="dialog-actions"><button @click="logoutOpen=false">取消</button><button class="primary" @click="logout">确认退出</button></div></div></div>
    <div v-if="cloudUnsavedOpen" class="cloud-unsaved-modal"><div class="dialog-panel"><h3>云平台配置尚未保存</h3><p>离开当前页面前请选择如何处理修改。</p><div class="dialog-actions"><button @click="cloudUnsavedOpen=false">继续编辑</button><button class="danger" @click="discardCloudAndLeave">放弃修改并离开</button><button class="primary" @click="saveCloudAndLeave">保存修改并离开</button></div></div></div>
  </div>
</template>
