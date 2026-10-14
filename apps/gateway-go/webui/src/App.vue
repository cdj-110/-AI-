<script setup lang="ts">
import AlarmNotificationPanel from './components/AlarmNotificationPanel.vue'
import {pageLeaveGuardKey,type RegisterPageLeaveGuard} from './composables/usePageDraftGuard.ts'
import { computed, nextTick, onBeforeUnmount, onMounted, provide, ref } from 'vue'
import { groupResourceTree } from './channelGroups.ts'
import { IconSparkles, IconX } from '@tabler/icons-vue'
import StatusWorkspace from './components/StatusWorkspace.vue'
import type {TopologySelection} from './topology'
import CollectionIncidentView from './components/CollectionIncidentView.vue'
import MaintenanceView from './components/MaintenanceView.vue'
import WiFiView from './components/WiFiView.vue'
import CellularView from './components/CellularView.vue'
import StorageView from './components/StorageView.vue'
import CloudView from './components/CloudView.vue'
import UserManagementView from './components/UserManagementView.vue'
import SmartGatewayView from './components/SmartGatewayView.vue'
import EdgeComputeView from './components/EdgeComputeView.vue'
import AlarmView from './components/AlarmView.vue'
import AiAssistantView from './components/AiAssistantView.vue'
import GlobalConfirmDialog from './components/GlobalConfirmDialog.vue'
import { useConfigStore } from './stores/config'
import { api } from './api'
import { openGatewayRealtime } from './realtime'
import type { GatewayStatus, PointStatus } from './types'
import { updatePersistentAlarmNotifications, detectNewActiveAlarms, emptyAlarmSnapshot, type AlarmNotification, type AlarmSnapshot } from './alarmNotifications.ts'
import {alarmPresentationKey,useAlarmPresentation} from './alarmPresentation.ts'
import {alarmPanelWindowKey,useAlarmPanelWindow} from './composables/useAlarmPanelWindow.ts'
import brandLogo from '../../internal/web/frontend/brand-logo.png'
import ActivationView from './components/ActivationView.vue'
import type { ActivationStatus } from './components/ActivationView.vue'

type NavKey = 'status'|'topology'|'config'|'collection-incidents'|'edge'|'alarms'|'wifi'|'cellular'|'offline-cache'|'cloud'|'users'|'maintenance'
type CurrentSession = {
  username:string
  displayName?:string
  roleKey:string
  permissions:string[]
}
const active = ref<NavKey>('status')
const currentSession = ref<CurrentSession | null>(null)
const sessionReady = ref(false)
const activationStatus = ref<ActivationStatus | null>(null)
const activationStatusError = ref('')
const authorizationReady = ref(false)
const needsActivation = computed(()=>authorizationReady.value && (!activationStatus.value?.license.valid || activationStatus.value?.license.restricted))
async function refreshAuthorization(){
  activationStatusError.value=''
  try{
    const status=await api<ActivationStatus>('/api/license/status')
    if(typeof status?.license?.valid!=='boolean'||typeof status?.license?.restricted!=='boolean')throw new Error('授权状态无效，请重新检查')
    activationStatus.value=status;authorizationReady.value=true
  }
  catch(cause){activationStatusError.value=cause instanceof Error?cause.message:'授权状态读取失败';authorizationReady.value=false}
}
async function retryAuthorization(){await refreshAuthorization();if(authorizationReady.value&&!needsActivation.value)window.location.reload()}
const aiOpen = ref(false)
const aiMounted = ref(false)
const aiAssistant=ref<InstanceType<typeof AiAssistantView>>()
const gatewayMode = ref<'config'|'run'>('run')
const edgeMode = ref<'config'|'run'>('run')
const treeSelection = ref<{resourceKey?:string;channelKey?:string;deviceKey?:string;action?:'add-channel'|'add-device';role?:'collect'|'forward';nonce:number}>({nonce:0})
const collapsed = ref(localStorage.getItem('gateway-vue-nav-collapsed') === 'true')
const mobileNavOpen = ref(false)
const resourceFolded = ref<Record<string,boolean>>({})
const roleFolded = ref<Record<string,boolean>>({})
const channelFolded = ref<Record<string,boolean>>({})
const moreOpen = ref(false)
const logoutOpen = ref(false)
const idleWarningOpen = ref(false)
const idleRemainingSeconds = ref(60)
const projectFile = ref<HTMLInputElement>()
const configStore = useConfigStore()
const config = computed(() => configStore.value)
const gatewayStatus = ref<GatewayStatus | null>(null)
const alarmRuleTarget = ref<{deviceKey:string;metric:string;name?:string;nonce:number}>()
const alarmCenterNonce = ref(0)
const globalAlarmSnapshot = ref<AlarmSnapshot>(emptyAlarmSnapshot())
const alarmToasts = ref<AlarmNotification[]>([])
const alarmPanelOpen = ref(false)
provide('gatewayRealtimeStatus', gatewayStatus)
const gatewayPointIndex = new Map<string,number>()
const wifiStatus = ref<any>(null)
const cellularStatus = ref<any>(null)
let stopRealtime = () => {}
let navStatusTimer = 0
let alarmStatusTimer = 0
let alarmStatusLoading = false
let sessionIdleTimer = 0
let lastSessionTouchAt = 0
let sessionTouching = false
let alarmsInitialized = false
const knownAlarmEventIds = new Set<string>()
const alarmWindow=useAlarmPanelWindow(()=>currentSession.value?.username||'')
provide(alarmPanelWindowKey,alarmWindow)
const alarmPresentation=useAlarmPresentation(()=>currentSession.value?.username||'',()=>alarmWindow.ownsAudio.value,alarmWindow.setAudioReady,()=>alarmWindow.audioAvailable.value)
provide(alarmPresentationKey,alarmPresentation)
const navItems: Array<{key:NavKey; label:string; icon:string; description:string}> = [
  { key:'status', label:'运行状态', icon:'⌁', description:'网关运行概览与设备拓扑' },
  { key:'config', label:'智能网关', icon:'▣', description:'资源、通道、设备和点位配置' },
  { key:'collection-incidents', label:'采集异常', icon:'!', description:'采集失败、恢复过程与原因诊断' },
  { key:'edge', label:'边缘计算', icon:'⌘', description:'计算组、表达式和输出点位' },
  { key:'alarms', label:'告警管理', icon:'!', description:'告警规则、当前告警和历史告警' },
  { key:'wifi', label:'WiFi', icon:'⌁', description:'无线网络连接与地址配置' },
  { key:'cellular', label:'移动网络', icon:'▥', description:'蜂窝网络、SIM 与拨号状态' },
  { key:'offline-cache', label:'历史数据', icon:'◫', description:'指定点位、保存策略与容量管理' },
  { key:'cloud', label:'云平台连接', icon:'☁', description:'MQTT、微控云与断线续存' },
  { key:'users', label:'用户管理', icon:'♙', description:'用户、角色和权限管理' },
  { key:'maintenance', label:'系统维护', icon:'⌕', description:'诊断、审计和维护操作' },
]
const permissionImplications:Record<string,string[]> = {
  'status.view':['alarms.view','collection_incidents.view'],
  'config.view':['alarms.view','edge.view'],
  'config.manage':['config.view','collection.run','collection_incidents.view','collection_incidents.manage','points.write','packet.view','edge.view','edge.manage','alarms.view','alarms.manage','history.view','history.manage','offline.manage'],
  'collection_incidents.manage':['collection_incidents.view'],
  'edge.view':['config.view'],
  'edge.manage':['edge.view','config.view'],
  'alarms.manage':['alarms.view'],
  'history.manage':['history.view'],
  'cloud.manage':['offline.manage'],
  'maintenance.run':['maintenance.view','maintenance.diagnose','maintenance.time','maintenance.remote','maintenance.restart','maintenance.factory_reset','maintenance.audit'],
}
const navPermissions:Record<NavKey,string[]> = {
  status:['status.view'],
  topology:['config.view'],
  config:['config.view'],
  'collection-incidents':['collection_incidents.view'],
  edge:['edge.view'],
  alarms:['alarms.view'],
  wifi:['network.manage'],
  cellular:['network.manage'],
  'offline-cache':['history.view'],
  cloud:['cloud.manage','offline.manage'],
  users:['users.manage'],
  maintenance:['maintenance.view','maintenance.diagnose','maintenance.time','maintenance.remote','maintenance.restart','maintenance.factory_reset','maintenance.audit','maintenance.run','cloud.manage'],
}
function permissionGranted(permission:string){
  const granted=currentSession.value?.permissions||[]
  return granted.some(item=>item==='*'||item===permission||permissionImplications[item]?.includes(permission))
}
function canAccessNav(key:NavKey){return navPermissions[key]?.some(permissionGranted)??false}
const visibleNavItems=computed(()=>sessionReady.value?navItems.filter(item=>canAccessNav(item.key)||(item.key==='status'&&canAccessNav('topology'))):[])
const activeMenu=computed(()=>active.value==='topology'?'status':active.value)
const current = computed(() => navItems.find(item => item.key === activeMenu.value) || visibleNavItems.value[0] || navItems[0])
function menuTarget(key:NavKey):NavKey{return key==='status'?(active.value==='topology'&&canAccessNav('topology')?'topology':canAccessNav('status')?'status':'topology'):key}
function firstAccessibleNav():NavKey{return menuTarget(visibleNavItems.value[0]?.key||'status')}
const workspaceMode = computed(() => active.value === 'edge' ? edgeMode.value : gatewayMode.value)
const canConfigureWorkspace=computed(()=>active.value==='config'?permissionGranted('config.manage'):active.value==='edge'?permissionGranted('edge.manage'):false)
function setWorkspaceMode(value:'config'|'run') {
  if(value==='config'&&!canConfigureWorkspace.value)return
  if (active.value === 'edge') edgeMode.value = value
  else gatewayMode.value = value
}
function openAIAssistant(){aiMounted.value=true;aiOpen.value=true}
async function closeAIAssistant(){if(!aiAssistant.value||await aiAssistant.value.requestLeave())aiOpen.value=false}
function handleAIEscape(event:KeyboardEvent){if(event.key==='Escape'&&aiOpen.value&&!document.querySelector('.global-confirm-modal'))void closeAIAssistant()}
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
const groupedResources = computed(() => groupResourceTree(resourceTree.value))
const allTreeChildrenFolded = computed(() => groupedResources.value.length > 0 && groupedResources.value.every(resource => resourceFolded.value[resource.resourceKey] === true))
function toggleAllTreeChildren(){
  const folded=!allTreeChildrenFolded.value
  for(const resource of groupedResources.value){
    resourceFolded.value[resource.resourceKey]=folded
    for(const group of resource.groups)roleFolded.value[group.treeKey]=folded
    for(const channel of resource.channels||[])channelFolded.value[channel.channelKey]=folded
  }
}
function toggleResourceChildren(resource:any){resourceFolded.value[resource.resourceKey]=!resourceFolded.value[resource.resourceKey]}
function toggleChannelChildren(channel:any){channelFolded.value[channel.channelKey]=!channelFolded.value[channel.channelKey]}
function navFromURL(){
  const key=new URLSearchParams(window.location.search).get('page') as NavKey
  return Object.hasOwn(navPermissions,key)?key:undefined
}
function rememberNav(key:NavKey){
  localStorage.setItem('gateway-vue-active-tab',key)
  const url=new URL(window.location.href)
  url.searchParams.set('page',key)
  window.history.replaceState(window.history.state,'',url)
}
const pageLeaveGuards=new Set<()=>Promise<boolean>>()
const registerPageLeaveGuard:RegisterPageLeaveGuard=guard=>{pageLeaveGuards.add(guard);return()=>{pageLeaveGuards.delete(guard)}}
provide(pageLeaveGuardKey,registerPageLeaveGuard)
let checkingPageLeave=false
async function applyNav(key:NavKey){
  if(key!==active.value){
    if(checkingPageLeave)return false
    checkingPageLeave=true
    try{for(const guard of [...pageLeaveGuards].reverse()){if(!await guard())return false}}finally{checkingPageLeave=false}
  }
  if(sessionReady.value&&!canAccessNav(key))key=firstAccessibleNav()
  active.value=key;mobileNavOpen.value=false;rememberNav(key)
  void nextTick(()=>document.querySelector<HTMLElement>('.page')?.scrollTo({top:0,left:0}))
  return true
}
async function select(key: NavKey) { const changed=await applyNav(key);if(changed&&key==='alarms'){alarmRuleTarget.value=undefined;alarmCenterNonce.value++}return changed }
function toggleNav() {
  if(window.matchMedia('(max-width: 900px)').matches){mobileNavOpen.value=false;return}
  collapsed.value = !collapsed.value
  localStorage.setItem('gateway-vue-nav-collapsed', String(collapsed.value))
}
function closeMore() { moreOpen.value = false }
async function importProject(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) { input.value = ''; return }
  if(checkingPageLeave){input.value='';return}
  checkingPageLeave=true
  try{for(const guard of [...pageLeaveGuards].reverse()){if(!await guard()){input.value='';return}}}
  finally{checkingPageLeave=false}
  if(!window.confirm('导入工程会覆盖当前配置，确认继续？')){input.value='';return}
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
  if(checkingPageLeave)return
  checkingPageLeave=true
  try{for(const guard of [...pageLeaveGuards].reverse()){if(!await guard())return false}}
  finally{checkingPageLeave=false}
  await fetch('/logout', { method:'POST' })
  window.location.assign('/login')
}
function gatewayActivityAt(){return Number(localStorage.getItem('gateway-session-last-activity'))||Date.now()}
async function renewGatewaySession(){
  if(!currentSession.value||sessionTouching||Date.now()-lastSessionTouchAt<60000)return
  sessionTouching=true
  try{
    const lease=await api<{idleTimeoutSeconds:number;absoluteExpiresAt:string}>('/api/session/activity',{method:'POST'})
    localStorage.setItem('gateway-session-idle-seconds',String(lease.idleTimeoutSeconds||3600))
    localStorage.setItem('gateway-session-absolute-expires',lease.absoluteExpiresAt||'')
    lastSessionTouchAt=Date.now()
  }catch{await expireGatewaySession()}
  finally{sessionTouching=false}
}
function recordGatewayActivity(event?:Event){
  if((event&&!event.isTrusted)||!currentSession.value)return
  localStorage.setItem('gateway-session-last-activity',String(Date.now()))
  idleWarningOpen.value=false
  void renewGatewaySession()
}
function checkGatewaySessionIdle(){
  if(!currentSession.value)return
  const idleMs=Math.max(300,Number(localStorage.getItem('gateway-session-idle-seconds'))||3600)*1000
  const absoluteAt=new Date(localStorage.getItem('gateway-session-absolute-expires')||'').getTime()
  const idleRemaining=gatewayActivityAt()+idleMs-Date.now()
  const remaining=Math.min(idleRemaining,Number.isFinite(absoluteAt)?absoluteAt-Date.now():idleRemaining)
  idleRemainingSeconds.value=Math.max(0,Math.ceil(remaining/1000))
  if(remaining<=0){idleWarningOpen.value=false;void expireGatewaySession()}
  else if(remaining<=60000)idleWarningOpen.value=true
}
async function expireGatewaySession(){
  currentSession.value=null
  for(const key of ['gateway-session-last-activity','gateway-session-idle-seconds','gateway-session-absolute-expires'])localStorage.removeItem(key)
  try{await fetch('/logout',{method:'POST'})}catch{}
  window.location.assign('/login?reason=idle')
}
function continueGatewaySession(){recordGatewayActivity();void renewGatewaySession()}
async function openDevice(resource: any, channel: any, device?: any) {
  if(active.value==='config'&&(treeSelection.value.resourceKey!==resource.resourceKey||treeSelection.value.channelKey!==channel.channelKey||treeSelection.value.deviceKey!==device?.deviceKey)){
    if(checkingPageLeave)return
    checkingPageLeave=true
    try{for(const guard of [...pageLeaveGuards].reverse()){if(!await guard())return}}finally{checkingPageLeave=false}
  }
  if(!await select('config'))return
  treeSelection.value = { resourceKey:resource.resourceKey, channelKey:channel.channelKey, deviceKey:device?.deviceKey, nonce:Date.now() }
  mobileNavOpen.value = false
}
async function openTopologyDevice(selection:TopologySelection){
  if(!canAccessNav('config')||!await select('config'))return
  gatewayMode.value='run'
  treeSelection.value={...selection,nonce:Date.now()}
}
async function addFromTree(resource:any,role:'collect'|'forward'='collect'){
  if(active.value==='config'){
    if(checkingPageLeave)return
    checkingPageLeave=true
    try{for(const guard of [...pageLeaveGuards].reverse()){if(!await guard())return}}finally{checkingPageLeave=false}
  }
  if(!await select('config'))return
  const serialChannel=resource.type==='serial'?resource.channels[0]:undefined
  treeSelection.value=resource.type==='serial'
    ? {resourceKey:resource.resourceKey,channelKey:serialChannel?.channelKey,action:'add-device',nonce:Date.now()}
    : {resourceKey:resource.resourceKey,action:'add-channel',role,nonce:Date.now()}
  gatewayMode.value='config'
}
function protocolName(value:string){return ({'modbus-rtu':'Modbus RTU','dlt645':'DL/T 645','modbus-tcp':'Modbus TCP','iec104':'IEC104','iec104-server':'IEC104 服务端','modbus-tcp-slave':'Modbus TCP 从站','iec61850':'IEC 61850 MMS','iec61850-mms-server':'IEC61850 MMS 服务端','iec61850-goose':'IEC61850 GOOSE','iec61850-goose-publisher':'IEC61850 GOOSE 发布','siemens-s7':'Siemens S7','opcua':'OPC UA','opcda':'OPC DA'} as Record<string,string>)[value]||value||'-'}
function deviceConnectionLabel(device:any){return device.protocol==='dlt645'?`${device.address||'-'} · 表 ${device.dlt645?.meterAddress||'-'}`:`${device.address||'-'} · 从站 ${device.slaveId||device.commonAddress||1}`}
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
function openAlarmRuleFromPoint(event:Event){const detail=(event as CustomEvent).detail||{};alarmRuleTarget.value={deviceKey:String(detail.deviceKey||''),metric:String(detail.metric||''),name:detail.name?String(detail.name):undefined,nonce:Date.now()};applyNav('alarms')}
function acknowledgePanelEvents(eventIds:string[]){
 const ids=new Set(eventIds)
 globalAlarmSnapshot.value.active=globalAlarmSnapshot.value.active.map(e=>ids.has(e.eventId)?{...e,acknowledged:true}:e)
 alarmToasts.value=updatePersistentAlarmNotifications(alarmToasts.value,globalAlarmSnapshot.value.active)
 alarmPresentation.update(globalAlarmSnapshot.value.active,[])
 window.dispatchEvent(new Event('gateway-alarms-changed'))
}
function closeAlarmPanel(){alarmPanelOpen.value=false}
function openAlarmPanel(){
  if(!permissionGranted('alarms.view'))return
  if(alarmWindow.mode.value==='tab'){alarmWindow.open();return}
  // Explicitly reopening the panel can show still-active reminders; polling cannot.
  alarmToasts.value=updatePersistentAlarmNotifications(alarmToasts.value,globalAlarmSnapshot.value.active)
  alarmPanelOpen.value=true
}
function removeAlarmSoundUnlockListeners(){window.removeEventListener('pointerdown',unlockAlarmSound);window.removeEventListener('keydown',unlockAlarmSound)}
async function unlockAlarmSound(){await alarmPresentation.unlock()}
function openAlarmCenter(){if(!permissionGranted('alarms.view'))return;closeAlarmPanel();alarmRuleTarget.value=undefined;alarmCenterNonce.value++;applyNav('alarms')}
async function loadGlobalAlarms(){
  if(alarmStatusLoading)return
  alarmStatusLoading=true
  try{
    const next=await api<AlarmSnapshot>('/api/alarms?activeOnly=1')
    const detected=detectNewActiveAlarms(knownAlarmEventIds,next,alarmsInitialized)
    knownAlarmEventIds.clear();detected.eventIds.forEach(id=>knownAlarmEventIds.add(id))
    globalAlarmSnapshot.value=next
    const freshEvents=detected.events.slice().reverse()
    alarmToasts.value=updatePersistentAlarmNotifications(alarmToasts.value,next.active)
    // Closing the panel suppresses repeat polls, not future genuinely new alarms.
    if(!alarmPanelOpen.value&&freshEvents.some(e=>e.popupEnabled!==false&&!e.acknowledged)){
      if(alarmWindow.mode.value==='page')alarmPanelOpen.value=true
      else alarmWindow.open(true)
    }
    alarmPresentation.update(next.active,freshEvents)
    alarmsInitialized=true
  }catch{/* 告警接口短暂不可用时保留上一帧，避免界面闪烁 */}
  finally{alarmStatusLoading=false}
}
const wifiNav=computed(()=>{const value=wifiStatus.value;if(!value)return{className:'offline',text:'读取中',title:'正在读取 WiFi 状态'};if(value.connected)return{className:'online',text:value.ssid||'已连接',title:`WiFi 已连接：${value.ssid||'-'}`};if(value.available)return{className:'',text:'未连接',title:value.message||'WiFi 未连接'};return{className:'offline',text:'无WiFi',title:value.message||'未检测到 WiFi 网卡'}})
const cellularNav=computed(()=>{const value=cellularStatus.value;if(!value)return{className:'offline',text:'读取中',title:'正在读取移动网络状态'};if(!value.enabled)return{className:'offline',text:'未启用',title:'移动网络未启用'};const signal=Number(value.sim?.signal||0);if(!value.available||!signal)return{className:'offline',text:'无信号',title:'移动网络信号：无信号'};if(signal<45)return{className:'weak',text:`${signal}%`,title:`移动网络信号：${signal}%`};return{className:'online',text:`${signal}%`,title:`移动网络信号：${signal}%`}})
const cloudNav=computed(()=>{const enabled=Object.entries(gatewayStatus.value?.mqttChannels||{}).filter(([key,value])=>(key==='activation'||key.startsWith('manual-'))&&value.enabled).map(([,value])=>value);if(!enabled.length)return{className:'offline',text:'未启用',title:'当前没有启用的云平台连接'};const connected=enabled.filter(item=>item.connected).length;if(connected===enabled.length)return{className:'online',text:'连接成功',title:`已启用 ${enabled.length} 个连接，连接成功`};if(connected)return{className:'weak',text:`${connected}/${enabled.length}`,title:`已启用 ${enabled.length} 个连接，${connected} 个已连接`};return{className:'offline',text:'未连接',title:`已启用 ${enabled.length} 个连接，均未连接`}})
async function loadNavStatus(){const [wifi,cellular]=await Promise.allSettled([api('/api/network/wifi'),api('/api/network/cellular')]);wifiStatus.value=wifi.status==='fulfilled'?wifi.value:{available:false,message:'WiFi 状态读取失败'};cellularStatus.value=cellular.status==='fulfilled'?cellular.value:{enabled:config.value.cellular?.enabled!==false,available:false}}
function indexGatewayPoints(points:PointStatus[]){gatewayPointIndex.clear();points.forEach((point,index)=>gatewayPointIndex.set(`${point.deviceKey}::${point.metric}`,index));return points}
function mergeGatewayPoints(current:PointStatus[],payload:any){const incoming=Array.isArray(payload?.points)?payload.points as PointStatus[]:[];if(payload?.replace)return indexGatewayPoints(incoming);if(!incoming.length)return current;if(gatewayPointIndex.size!==current.length)indexGatewayPoints(current);const next=current.slice();for(const point of incoming){const key=`${point.deviceKey}::${point.metric}`,index=gatewayPointIndex.get(key);if(index===undefined){gatewayPointIndex.set(key,next.length);next.push(point)}else next[index]=point}return next}
function handleRealtime(message:any){if(message.type==='snapshot'){const points=configStore.largeMode?[]:Array.isArray(message.payload?.points)?indexGatewayPoints(message.payload.points):[];gatewayStatus.value={...message.payload,points}}else if(message.type==='gateway.status'&&gatewayStatus.value)gatewayStatus.value={...gatewayStatus.value,...message.payload,points:gatewayStatus.value.points,errors:gatewayStatus.value.errors};else if(message.type==='points.diff'&&gatewayStatus.value&&!configStore.largeMode)gatewayStatus.value={...gatewayStatus.value,points:mergeGatewayPoints(gatewayStatus.value.points||[],message.payload||{})};else if(message.type==='communications.diff'&&gatewayStatus.value)gatewayStatus.value={...gatewayStatus.value,mqttEnabled:!!message.payload?.mqttEnabled,mqttConnected:!!message.payload?.mqttConnected,mqttChannels:message.payload?.mqttChannels||{}}}
onMounted(async () => {
  window.addEventListener('gateway-alarms-changed',loadGlobalAlarms);window.addEventListener('click', closeMoreFromPage);window.addEventListener('gateway-open-alarm-rule',openAlarmRuleFromPoint);window.addEventListener('pointerdown',unlockAlarmSound,{passive:true});window.addEventListener('keydown',unlockAlarmSound);window.addEventListener('keydown',handleAIEscape)
  for(const event of ['pointerdown','keydown','touchstart','wheel'])window.addEventListener(event,recordGatewayActivity,{passive:true})
  sessionIdleTimer=window.setInterval(checkGatewaySessionIdle,5000)
  await refreshAuthorization()
  if(authorizationReady.value&&needsActivation.value&&activationStatus.value?.authenticated===false){sessionReady.value=true;return}
  if(!authorizationReady.value)return
  try{currentSession.value=await api<CurrentSession>('/api/session');recordGatewayActivity();await renewGatewaySession()}catch{currentSession.value=null}
  sessionReady.value=true
  if(needsActivation.value)return
  const saved=navFromURL()||(localStorage.getItem('gateway-vue-active-tab') as NavKey)
  active.value=saved&&canAccessNav(saved)?saved:firstAccessibleNav()
  rememberNav(active.value)
  if(permissionGranted('config.view'))try{await configStore.load()}catch{}
  const initialLoads:Promise<unknown>[]=[]
  if(permissionGranted('network.manage'))initialLoads.push(loadNavStatus())
  if(permissionGranted('alarms.view'))initialLoads.push(loadGlobalAlarms())
  await Promise.all(initialLoads)
  if(!configStore.largeMode)stopRealtime=openGatewayRealtime(handleRealtime)
  if(permissionGranted('network.manage'))navStatusTimer=window.setInterval(loadNavStatus,30000)
  if(permissionGranted('alarms.view'))alarmStatusTimer=window.setInterval(loadGlobalAlarms,2000)
})
onBeforeUnmount(() => {window.removeEventListener('gateway-alarms-changed',loadGlobalAlarms);window.removeEventListener('click', closeMoreFromPage);window.removeEventListener('gateway-open-alarm-rule',openAlarmRuleFromPoint);window.removeEventListener('keydown',handleAIEscape);for(const event of ['pointerdown','keydown','touchstart','wheel'])window.removeEventListener(event,recordGatewayActivity);removeAlarmSoundUnlockListeners();stopRealtime();window.clearInterval(navStatusTimer);window.clearInterval(alarmStatusTimer);window.clearInterval(sessionIdleTimer)})
</script>

<template>
  <GlobalConfirmDialog />
  <main v-if="!authorizationReady || (!needsActivation && !sessionReady)" class="gateway-entry-page" :aria-busy="!activationStatusError">
    <header class="gateway-entry-header"><img :src="brandLogo" alt="微控网关"/><span>微控网关<small>Edge Gateway</small></span></header>
    <section class="gateway-entry-message">
      <template v-if="activationStatusError"><h1>暂时无法进入网关</h1><p role="alert">{{activationStatusError}}</p><button class="primary" @click="retryAuthorization">重新检查</button></template>
      <template v-else><span class="gateway-entry-spinner" aria-hidden="true"/><p role="status">正在加载网关…</p></template>
    </section>
  </main>
  <ActivationView v-else-if="needsActivation" :status="activationStatus" :error="activationStatusError" :authenticated="!!currentSession" :can-import="permissionGranted('cloud.manage')" @refresh="retryAuthorization" />
  <div v-else class="app-shell" :class="{ collapsed, 'mobile-nav-open':mobileNavOpen }">
    <button v-if="mobileNavOpen" class="mobile-nav-backdrop" aria-label="关闭导航" @click="mobileNavOpen=false"/>
    <aside class="sidebar main-nav">
      <div class="brand">
        <img class="brand-mark" :src="brandLogo" alt="微控网关" />
        <span class="brand-text"><span class="brand-title">微控网关</span><span class="brand-subtitle">Edge Gateway</span></span>
        <button class="nav-collapse" :title="collapsed ? '展开菜单' : '折叠菜单'" :aria-label="collapsed ? '展开菜单' : '折叠菜单'" @click="toggleNav"><svg class="nav-icon" viewBox="0 0 24 24"><path d="M15 6l-6 6 6 6"/><path d="M20 4v16"/></svg></button>
      </div>
      <nav class="main-menu">
        <template v-for="item in visibleNavItems" :key="item.key">
          <button :class="[{active:activeMenu===item.key}, {'nav-with-status':['wifi','cellular','cloud'].includes(item.key)}]" :title="item.label" @click="select(menuTarget(item.key))">
            <svg v-if="item.key==='status'" class="nav-icon" viewBox="0 0 24 24"><path d="M4 19h16"/><path d="M6 16l4-5 4 3 4-7"/></svg>
            <svg v-else-if="item.key==='config'" class="nav-icon" viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6"/><path d="M9 13h6"/><path d="M9 17h3"/></svg>
            <svg v-else-if="item.key==='edge'" class="nav-icon" viewBox="0 0 24 24"><circle cx="6" cy="12" r="2"/><circle cx="18" cy="6" r="2"/><circle cx="18" cy="18" r="2"/><path d="M8 11l8-4"/><path d="M8 13l8 4"/></svg>
            <svg v-else-if="item.key==='collection-incidents'" class="nav-icon" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v6"/><path d="M12 17h.01"/></svg>
            <svg v-else-if="item.key==='alarms'" class="nav-icon" viewBox="0 0 24 24"><path d="M12 3l9 16H3z"/><path d="M12 9v4"/><path d="M12 16h.01"/></svg>
            <svg v-else-if="item.key==='wifi'" class="nav-icon" viewBox="0 0 24 24"><path d="M5 10a11 11 0 0 1 14 0"/><path d="M8 14a6 6 0 0 1 8 0"/><path d="M12 18h.01"/></svg>
            <svg v-else-if="item.key==='cellular'" class="nav-icon" viewBox="0 0 24 24"><path d="M6 18h.01"/><path d="M10 18v-4"/><path d="M14 18v-8"/><path d="M18 18V6"/></svg>
            <svg v-else-if="item.key==='offline-cache'" class="nav-icon" viewBox="0 0 24 24"><path d="M4 7c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3z"/><path d="M4 7v5c0 1.7 3.6 3 8 3s8-1.3 8-3V7"/><path d="M4 12v5c0 1.7 3.6 3 8 3s8-1.3 8-3v-5"/></svg>
            <svg v-else-if="item.key==='cloud'" class="nav-icon" viewBox="0 0 24 24"><path d="M7 18a4 4 0 0 1 .7-7.9A5.5 5.5 0 0 1 18.5 12H19a3 3 0 0 1 0 6H7z"/></svg>
            <svg v-else-if="item.key==='users'" class="nav-icon" viewBox="0 0 24 24"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            <svg v-else class="nav-icon" viewBox="0 0 24 24"><path d="M14.5 6.5l3 3"/><path d="M5 19l4.5-1 8-8a2.1 2.1 0 0 0-3-3l-8 8z"/></svg>
            <span class="nav-label">{{ item.label }}</span>
            <span v-if="item.key==='alarms'&&globalAlarmSnapshot.activeCount" class="nav-alarm-count" :class="{critical:globalAlarmSnapshot.criticalCount>0}">{{globalAlarmSnapshot.activeCount>99?'99+':globalAlarmSnapshot.activeCount}}</span>
            <span v-if="item.key==='wifi'" class="nav-status-pill" :class="wifiNav.className" :title="wifiNav.title"><i class="dot"/><span>{{wifiNav.text}}</span></span>
            <span v-if="item.key==='cellular'" class="nav-status-pill" :class="cellularNav.className" :title="cellularNav.title"><i class="dot"/><span>{{cellularNav.text}}</span></span>
            <span v-if="item.key==='cloud'" class="nav-status-pill" :class="cloudNav.className" :title="cloudNav.title"><i class="dot"/><span>{{cloudNav.text}}</span></span>
          </button>
          <div v-if="item.key==='config' && !collapsed" class="device-tree gateway-nav-tree">
            <div class="tree-title gateway-tree-title"><span>资源 / 通道 / 设备</span><button @click.stop="toggleAllTreeChildren">{{ allTreeChildrenFolded ? '全部展开' : '全部折叠' }}</button></div>
            <div v-for="resource in groupedResources" :key="resource.resourceKey" class="gateway-tree-resource" :data-resource="resource.resourceKey">
              <div class="gateway-tree-row resource-row" :class="{'network-resource-row':resource.type!=='serial'}"><button class="gateway-tree-group" :class="{active:treeSelection.resourceKey===resource.resourceKey&&!treeSelection.channelKey}" @click="select('config');treeSelection={resourceKey:resource.resourceKey,nonce:Date.now()}"><span>{{ resource.name || resource.resourceKey }}</span><small>{{ resource.type==='serial' ? `串口资源 · ${resource.serial?.port||resource.serial?.name||resource.name||'-'} · ${resourceDeviceCount(resource)} 台设备` : `网口资源 · ${resource.network?.interface||'-'} · ${resource.channels.length} 个通道` }}</small></button><button v-if="resource.type==='serial'" class="tree-mini" title="新增设备" @click.stop="addFromTree(resource)">+</button><button class="tree-mini" :title="resourceFolded[resource.resourceKey]?'展开资源':'折叠资源'" :aria-label="resourceFolded[resource.resourceKey]?'展开资源':'折叠资源'" @click.stop="toggleResourceChildren(resource)"><span class="tree-chevron" :class="{collapsed:resourceFolded[resource.resourceKey]}"/></button></div>
              <template v-if="!resourceFolded[resource.resourceKey]&&resource.type==='serial'">
                <template v-for="channel in resource.channels" :key="channel.channelKey || channel.name">
                  <button v-for="device in channel.devices" :key="device.deviceKey" class="tree-device serial-tree-device" :class="[{active:treeSelection.deviceKey===device.deviceKey},deviceTreeStatus(device)]" @click="openDevice(resource,channel,device)"><i class="device-status-indicator" aria-hidden="true"/><span>{{ device.name || device.deviceKey }} <b>({{device.deviceKey}})</b></span><small>{{device.address||resource.serial?.port||'-'}} · 从站 {{device.slaveId||1}}</small></button>
                </template>
              </template>
              <template v-else-if="!resourceFolded[resource.resourceKey]">
                <section v-for="group in resource.groups" :key="group.treeKey" class="gateway-tree-role-section" :data-role="group.key">
                  <div class="gateway-tree-role-row">
                    <button class="gateway-tree-role-create" :disabled="!permissionGranted('config.manage')" :title="`新增${group.label}通道`" @click="addFromTree(resource,group.key)"><svg class="gateway-tree-role-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 16v4h16v-4"/><path v-if="group.key==='collect'" d="M12 3v12m-4-4 4 4 4-4"/><path v-else d="M12 15V3m-4 4 4-4 4 4"/></svg><span class="gateway-tree-role-label">{{group.label}}</span><small class="gateway-tree-role-count">{{group.channels.length}}</small><b class="gateway-tree-role-plus" aria-hidden="true">+</b></button>
                    <button class="tree-mini" :aria-label="`${roleFolded[group.treeKey]?'展开':'折叠'}${group.label}`" @click="roleFolded[group.treeKey]=!roleFolded[group.treeKey]"><span class="tree-chevron" :class="{collapsed:roleFolded[group.treeKey]}"/></button>
                  </div>
                  <template v-if="!roleFolded[group.treeKey]">
                    <template v-for="channel in group.channels" :key="channel.channelKey">
                <div class="gateway-tree-row channel-row"><button class="tree-channel" :class="{active:treeSelection.channelKey===channel.channelKey&&!treeSelection.deviceKey}" @click="openDevice(resource,channel)"><span><b>{{ channel.name || channel.channelKey }}</b></span><small>{{protocolName(channel.role==='forward'?(channel.forwardProtocol||channel.protocol):channel.protocol)}} · {{channel.devices.length}} 台设备</small></button><button class="tree-mini" :title="channelFolded[channel.channelKey]?'展开通道':'折叠通道'" :aria-label="channelFolded[channel.channelKey]?'展开通道':'折叠通道'" @click.stop="toggleChannelChildren(channel)"><span class="tree-chevron" :class="{collapsed:channelFolded[channel.channelKey]}"/></button></div>
                <button v-if="!channelFolded[channel.channelKey]" v-for="device in channel.devices" :key="device.deviceKey" class="tree-device" :class="[{active:treeSelection.deviceKey===device.deviceKey},deviceTreeStatus(device)]" @click="openDevice(resource,channel,device)"><i class="device-status-indicator" aria-hidden="true"/><span>{{ device.name || device.deviceKey }} <b v-if="channel.role!=='forward'">({{device.deviceKey}})</b></span><small>{{channel.role==='forward'?`${forwardDeviceProtocolName(device.protocol||channel.forwardProtocol||channel.protocol)} · ${(device.points||[]).length} 个点位`:deviceConnectionLabel(device)}}</small></button>
                    </template>
                    <p v-if="!group.channels.length" class="gateway-tree-role-empty">暂无{{group.label}}通道</p>
                  </template>
                </section>
              </template>
            </div>
          </div>
        </template>
      </nav>
    </aside>
    <main>
      <header class="header">
        <div class="header-left">
          <button class="mobile-menu-trigger" aria-label="打开导航" @click="mobileNavOpen=true"><svg class="nav-icon" viewBox="0 0 24 24"><path d="M4 7h16"/><path d="M4 12h16"/><path d="M4 17h16"/></svg></button>
          <div class="header-title-block"><h1>{{current.label}}</h1></div>
        </div>
        <div class="header-actions">
          <button v-if="permissionGranted('alarms.view')" class="header-alarm-button" :class="{active:globalAlarmSnapshot.activeCount>0,critical:globalAlarmSnapshot.criticalCount>0}" :title="`打开告警面板：当前 ${globalAlarmSnapshot.activeCount} 条告警，${globalAlarmSnapshot.unackedCount} 条未确认`" aria-label="打开告警面板" :aria-expanded="alarmPanelOpen" @click="openAlarmPanel">
            <svg viewBox="0 0 24 24"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 21h4"/></svg>
            <span>告警</span><b v-if="globalAlarmSnapshot.activeCount">{{globalAlarmSnapshot.activeCount>99?'99+':globalAlarmSnapshot.activeCount}}</b>
          </button>
          <span v-if="(active==='config'||active==='edge')&&canConfigureWorkspace" class="point-mode-toggle"><button :class="{active:workspaceMode==='config'}" @click="setWorkspaceMode('config')">配置</button><button :class="{active:workspaceMode==='run'}" @click="setWorkspaceMode('run')">运行</button></span>
          <div class="header-more-wrap" @click.stop>
            <button class="header-more-button" title="打开设置菜单" aria-label="打开设置菜单" :aria-expanded="moreOpen" @click="moreOpen=!moreOpen"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.09a2 2 0 0 1 1 1.74v.5a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.38a2 2 0 0 0-.73-2.73l-.15-.09a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/></svg></button>
            <div v-if="moreOpen" class="header-more-menu" role="menu">
              <a href="/change-password?next=/" @click="closeMore">修改密码</a>
              <button v-if="permissionGranted('config.manage')" @click="projectFile?.click()">导入工程</button>
              <a v-if="permissionGranted('config.view')" href="/api/project/export" @click="closeMore">导出工程</a>
              <button class="logout-button" @click="logoutOpen=true;closeMore()">退出登录</button>
            </div>
            <input ref="projectFile" class="hidden-file" type="file" accept=".json,application/json" @change="importProject" />
          </div>
        </div>
      </header>
      <div v-if="alarmWindow.notice.value" class="alarm-tab-notice" role="status">{{alarmWindow.notice.value}} <button type="button" @click="alarmWindow.open()">打开告警标签页</button></div>
      <div class="page"><StatusWorkspace v-if="sessionReady&&canAccessNav(active)&&(active==='status'||active==='topology')" :active="active" :can-status="canAccessNav('status')" :can-topology="canAccessNav('topology')" :can-config="canAccessNav('config')" :can-incidents="canAccessNav('collection-incidents')" :status="gatewayStatus" @navigate="select" @open-device="openTopologyDevice"/><SmartGatewayView v-else-if="sessionReady&&canAccessNav('config')&&active==='config'" v-model:mode="gatewayMode" :selection="treeSelection" @selection-change="treeSelection={...$event,nonce:Date.now()}"/><CollectionIncidentView v-else-if="sessionReady&&canAccessNav('collection-incidents')&&active==='collection-incidents'" :can-manage="permissionGranted('collection_incidents.manage')"/><EdgeComputeView v-else-if="sessionReady&&canAccessNav('edge')&&active==='edge'" v-model:mode="edgeMode"/><AlarmView v-else-if="sessionReady&&canAccessNav('alarms')&&active==='alarms'" :target="alarmRuleTarget"  :can-manage="permissionGranted('alarms.manage')" :center-nonce="alarmCenterNonce"/><WiFiView v-else-if="sessionReady&&canAccessNav('wifi')&&active==='wifi'"/><CellularView v-else-if="sessionReady&&canAccessNav('cellular')&&active==='cellular'"/><StorageView :can-manage="permissionGranted('history.manage')" v-else-if="sessionReady&&canAccessNav('offline-cache')&&active==='offline-cache'"/><CloudView v-else-if="sessionReady&&canAccessNav('cloud')&&active==='cloud'"/><UserManagementView v-else-if="sessionReady&&canAccessNav('users')&&active==='users'"/><MaintenanceView v-else-if="sessionReady&&canAccessNav('maintenance')&&active==='maintenance'"/><section v-else-if="sessionReady" class="panel"><h2>暂无可用功能</h2><p class="meta">当前账号尚未分配任何页面访问权限，请联系管理员调整角色权限。</p></section></div>
    </main>
    <button v-if="permissionGranted('ai.use')||permissionGranted('ai.manage')" class="ai-floating-button" title="打开 AI 助手" aria-label="打开 AI 助手" :aria-expanded="aiOpen" @click="openAIAssistant">
      <IconSparkles aria-hidden="true"/>
    </button>
    <div v-if="aiMounted&&(permissionGranted('ai.use')||permissionGranted('ai.manage'))" v-show="aiOpen" class="ai-assistant-modal" @click.self="closeAIAssistant">
      <section class="ai-assistant-window" role="dialog" aria-modal="true" aria-label="AI 助手">
        <button class="ai-assistant-close" title="关闭 AI 助手" aria-label="关闭 AI 助手" @click="closeAIAssistant"><IconX aria-hidden="true"/></button>
        <AiAssistantView ref="aiAssistant" />
      </section>
    </div>
    <AlarmNotificationPanel v-if="permissionGranted('alarms.view')&&alarmPanelOpen" :notifications="alarmToasts" :can-acknowledge="permissionGranted('alarms.manage')" @acknowledged="acknowledgePanelEvents" :layout-scope="currentSession?.username||''" @close="closeAlarmPanel" @open="openAlarmCenter"/>
    <nav class="mobile-bottom-nav" aria-label="手机快捷导航">
      <button v-if="canAccessNav('status')" :class="{active:active==='status'}" @click="select('status')"><svg class="nav-icon" viewBox="0 0 24 24"><path d="M4 19h16"/><path d="M6 16l4-5 4 3 4-7"/></svg><span>状态</span></button>
      <button v-if="canAccessNav('config')" :class="{active:active==='config'}" @click="select('config')"><svg class="nav-icon" viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6"/><path d="M9 13h6"/><path d="M9 17h3"/></svg><span>网关</span></button>
      <button v-if="canAccessNav('edge')" :class="{active:active==='edge'}" @click="select('edge')"><svg class="nav-icon" viewBox="0 0 24 24"><circle cx="6" cy="12" r="2"/><circle cx="18" cy="6" r="2"/><circle cx="18" cy="18" r="2"/><path d="M8 11l8-4"/><path d="M8 13l8 4"/></svg><span>边缘</span></button>
      <button :class="{active:mobileNavOpen}" @click="mobileNavOpen=true"><svg class="nav-icon" viewBox="0 0 24 24"><circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/></svg><span>更多</span></button>
    </nav>
    <div v-if="logoutOpen" class="logout-modal"><div class="dialog-panel"><h3>退出登录</h3><p>确认退出当前网关管理会话？退出后需要重新登录。</p><div class="dialog-actions"><button @click="logoutOpen=false">取消</button><button class="primary" @click="logout">确认退出</button></div></div></div>
    <div v-if="idleWarningOpen" class="logout-modal"><div class="dialog-panel"><h3>登录即将失效</h3><p>检测到长时间未操作，将在 {{idleRemainingSeconds}} 秒后自动退出。</p><div class="dialog-actions"><button @click="expireGatewaySession">立即退出</button><button class="primary" @click="continueGatewaySession">继续使用</button></div></div></div>
  </div>
</template>

<style scoped>
.gateway-entry-page{min-height:100dvh;background:#f5f8fc;color:#14243e}.gateway-entry-header{height:56px;box-sizing:border-box;display:flex;align-items:center;gap:9px;padding:0 24px;background:#fff;border-bottom:1px solid #e8edf4}.gateway-entry-header img{width:34px;height:34px;object-fit:contain}.gateway-entry-header span{font-size:14px;line-height:18px;font-weight:800}.gateway-entry-header small{display:block;margin-top:2px;font-size:10px;line-height:14px;color:#94a3b8;font-weight:700;letter-spacing:.04em}.gateway-entry-message{min-height:calc(100dvh - 56px);box-sizing:border-box;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:16px;padding:24px;text-align:center}.gateway-entry-message h1{margin:0;font-size:20px}.gateway-entry-message p{margin:0;color:#6b7d98;font-size:14px;line-height:1.7;overflow-wrap:anywhere}.gateway-entry-message button{padding:9px 16px;border-radius:8px}.gateway-entry-spinner{width:28px;height:28px;border:3px solid #dde8f6;border-top-color:#2478f5;border-radius:50%;animation:gateway-entry-spin .8s linear infinite}@keyframes gateway-entry-spin{to{transform:rotate(360deg)}}@media(prefers-reduced-motion:reduce){.gateway-entry-spinner{animation:none}}@media(max-width:600px){.gateway-entry-header{padding:0 14px}}
.alarm-tab-notice{margin:10px 16px;padding:10px 14px;border:1px solid #ecd6a9;border-radius:8px;background:#fff8e9;color:#8a631a;font-size:12px;line-height:1.6}.alarm-tab-notice button{margin-left:8px;border:1px solid #d5b87b;border-radius:5px;background:#fff;padding:5px 9px;color:inherit;cursor:pointer}
</style>
