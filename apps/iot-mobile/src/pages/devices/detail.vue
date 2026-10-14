<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { onHide, onLoad, onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { commandCapabilities as loadCommandCapabilities, device as loadDevice, deviceCommands as loadDeviceCommands, metrics as loadMetrics, sendDeviceCommand, telemetry as loadTelemetry } from '../../services/cloud'
import { subscribeTelemetry, type RealtimeState, type TelemetryUpdate } from '../../services/realtime'
import type { Device, DeviceCommandCapabilities, DeviceCommandPoint, DeviceCommandTask, DeviceMetric, TelemetryItem } from '../../types'

const id=ref(''),current=ref<Device>(),metricList=ref<DeviceMetric[]>([]),events=ref<TelemetryItem[]>([])
const loading=ref(true),refreshing=ref(false),error=ref(''),updatedAt=ref<Date>(),keyword=ref(''),infoExpanded=ref(false)
const commandCapabilities=ref<DeviceCommandCapabilities>({points:[],available:false,reason:''})
const commandVisible=ref(false),commandPoint=ref<DeviceCommandPoint>(),commandInput=ref(''),commandBoolean=ref<boolean>(),commandSending=ref(false)
const commandTask=ref<DeviceCommandTask>(),commandHistory=ref<DeviceCommandTask[]>([]),commandError=ref('')
const realtimeState=ref<RealtimeState>('CONNECTING')
const layoutMode=ref<'grid'|'list'>('list')
let timer: ReturnType<typeof setInterval> | undefined
let stopRealtimeStream:(()=>void)|undefined
let requestInFlight=false
const allLatestValues=computed(()=>{
  const values=new Map<string,{value:unknown;time:string}>()
  for(const event of events.value)for(const [key,value] of Object.entries(event.metrics||{}))if(!values.has(key))values.set(key,{value,time:event.time})
  const meta=new Map(metricList.value.map(item=>[item.identifier,item]))
  return [...values.entries()].map(([key,item])=>({key,...item,meta:meta.get(key)}))
    .filter(item=>item.meta?.enabled!==false&&!item.meta?.ignored)
    .sort((a,b)=>(a.meta?.sortOrder??999)-(b.meta?.sortOrder??999))
})
const latestValues=computed(()=>{
  const query=keyword.value.trim().toLowerCase()
  if(!query)return allLatestValues.value
  return allLatestValues.value.filter(item=>
    item.key.toLowerCase().includes(query)||String(item.meta?.name||'').toLowerCase().includes(query)
  )
})
const online=computed(()=>current.value?.status==='ONLINE')
const realtimeLabel=computed(()=>{
  if(!online.value)return'设备当前离线'
  if(realtimeState.value==='LIVE')return'WebSocket 实时更新'
  if(realtimeState.value==='FALLBACK')return'HTTP 1 秒降级刷新'
  return'正在连接 WebSocket'
})
const deviceTypeText=computed(()=>current.value?.deviceType==='GATEWAY'?'智能网关':current.value?.deviceType==='GATEWAY_CHILD'?'网关子设备':'直连设备')
const writablePoints=computed(()=>new Map(commandCapabilities.value.points.map(point=>[point.identifier,point])))
const commandStatusText=computed(()=>({SENT:'等待网关回执',SUCCEEDED:'执行成功',FAILED:'执行失败',TIMEOUT:'结果未确认'}[commandTask.value?.status||'']||commandTask.value?.status||''))
function isBooleanMetric(metric?:DeviceMetric,value?:unknown){
  const type=String(metric?.dataType||'').toLowerCase()
  return ['bool','boolean','switch'].includes(type)||typeof value==='boolean'
}
function booleanValue(value:unknown){
  if(typeof value==='string')return ['1','true','on','open','yes'].includes(value.toLowerCase())
  return value===true||value===1
}
function booleanLabel(name:string,value:unknown){
  const active=booleanValue(value)
  if(/断路器|刀闸|开关|合闸/.test(name))return active?'合闸':'分闸'
  if(/允许|许可/.test(name))return active?'允许':'禁止'
  if(/告警|报警|故障/.test(name))return active?'告警':'正常'
  if(/动作/.test(name))return active?'动作':'未动作'
  return active?'开启':'关闭'
}
function setLayout(value:'grid'|'list'){
  layoutMode.value=value
  uni.setStorageSync('iot-mobile:metric-layout',value)
}
function displayValue(value:unknown,metric?:DeviceMetric){
  if(value===null||value===undefined)return'--'
  if(typeof value==='number'&&Number.isFinite(metric?.decimals))return value.toFixed(metric?.decimals)
  if(typeof value==='object')return JSON.stringify(value)
  return String(value)
}
function formatTime(value?:string){
  if(!value)return'暂无上报'
  const date=new Date(value)
  return Number.isNaN(date.getTime())?'暂无上报':date.toLocaleString()
}
function formatMetricTime(value?:string){
  if(!value)return'暂无更新'
  const date=new Date(value)
  return Number.isNaN(date.getTime())?'暂无更新':date.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'})
}
function createRequestId(){
  const random=()=>Math.floor(Math.random()*0x10000).toString(16).padStart(4,'0')
  return `${random()}${random()}-${random()}-4${random().slice(1)}-${((8+Math.floor(Math.random()*4)).toString(16))}${random().slice(1)}-${random()}${random()}${random()}`
}
async function refreshCommandCapabilities(){
  if(current.value?.deviceType!=='GATEWAY_CHILD'){
    commandCapabilities.value={points:[],available:false,reason:'当前仅支持网关采集子设备'}
    return
  }
  try{commandCapabilities.value=await loadCommandCapabilities(id.value)}
  catch{commandCapabilities.value={points:[],available:false,reason:'暂时无法查询点位下发能力'}}
}
async function refreshCommandResult(silent=false){
  try{
    const tasks=await loadDeviceCommands(id.value)
    commandHistory.value=tasks.filter(item=>item.metric===commandPoint.value?.identifier).slice(0,5)
    if(commandTask.value){
      const latest=tasks.find(item=>item.requestId===commandTask.value?.requestId)
      if(latest)commandTask.value=latest
    }
    commandError.value=''
  }catch(reason){if(!silent)commandError.value=reason instanceof Error?reason.message:'下发结果查询失败'}
}
async function openCommand(metric:string){
  const point=writablePoints.value.get(metric)
  if(!point)return
  commandPoint.value=point
  commandInput.value=''
  commandBoolean.value=undefined
  commandTask.value=undefined
  commandHistory.value=[]
  commandError.value=''
  commandVisible.value=true
  await refreshCommandResult(true)
}
function closeCommand(){if(!commandSending.value)commandVisible.value=false}
async function submitCommand(){
  const point=commandPoint.value
  if(!point||commandSending.value||commandTask.value)return
  let value:unknown
  if(point.dataType==='BOOLEAN'){
    if(typeof commandBoolean.value!=='boolean'){uni.showToast({title:'请选择 true 或 false',icon:'none'});return}
    value=commandBoolean.value
  }else if(point.dataType==='NUMBER'){
    if(!commandInput.value.trim()||!Number.isFinite(Number(commandInput.value))){uni.showToast({title:'请输入有效数值',icon:'none'});return}
    value=Number(commandInput.value)
  }else value=commandInput.value
  const confirmation=await uni.showModal({title:'确认下发',content:`设备：${current.value?.name||''}\n点位：${point.name||point.identifier}\n下发值：${String(value)}${point.unit||''}\n操作将写入实际设备，请确认。`,confirmText:'确认下发',confirmColor:'#1677ff'})
  if(!confirmation.confirm)return
  commandSending.value=true
  commandError.value=''
  try{
    commandTask.value=await sendDeviceCommand(id.value,{requestId:createRequestId(),metric:point.identifier,value})
    uni.showToast({title:'指令已提交',icon:'success'})
    for(let index=0;index<6&&commandTask.value?.status==='SENT';index++){
      await new Promise(resolve=>setTimeout(resolve,1500))
      await refreshCommandResult(true)
    }
  }catch(reason){
    commandError.value=reason instanceof Error?reason.message:'指令提交失败，请勿重复下发'
  }finally{commandSending.value=false}
}
async function refresh(initial=false,indicate=true){
  if(requestInFlight)return
  try{
    requestInFlight=true
    refreshing.value=indicate
    if(initial)loading.value=true
    if(initial||!current.value){
      const [deviceData,telemetryData,metricsData]=await Promise.all([loadDevice(id.value),loadTelemetry(id.value),loadMetrics(id.value)])
      current.value=deviceData
      events.value=telemetryData.items||[]
      metricList.value=metricsData||[]
      await refreshCommandCapabilities()
    }else{
      const telemetryData=await loadTelemetry(id.value)
      events.value=telemetryData.items||[]
    }
    updatedAt.value=new Date()
    error.value=''
  }catch(reason){error.value=reason instanceof Error?reason.message:'实时数据读取失败'}
  finally{loading.value=false;refreshing.value=false;requestInFlight=false;uni.stopPullDownRefresh()}
}
onLoad(options=>{
  id.value=String(options?.id||'')
  const saved=uni.getStorageSync('iot-mobile:metric-layout')
  if(saved==='grid'||saved==='list')layoutMode.value=saved
})
function stopRefreshTimer(){if(timer!==undefined){clearInterval(timer);timer=undefined}}
function applyTelemetry(update:TelemetryUpdate){
  const next:TelemetryItem={...update.payload}
  events.value=[next,...events.value.filter(item=>item.time!==next.time||item.deviceKey!==next.deviceKey)].slice(0,20)
  updatedAt.value=new Date()
  error.value=''
}
function startRealtime(){
  stopRealtimeStream?.()
  stopRealtimeStream=subscribeTelemetry(id.value,{
    onState:state=>{realtimeState.value=state},
    onTelemetry:applyTelemetry,
  })
}
function stopRealtime(){
  stopRealtimeStream?.()
  stopRealtimeStream=undefined
  realtimeState.value='CONNECTING'
}
function stopRealtimeUpdates(){stopRefreshTimer();stopRealtime()}
onShow(()=>{
  if(!id.value)return
  stopRealtimeUpdates()
  void refresh(true)
  startRealtime()
  timer=setInterval(()=>{if(realtimeState.value==='FALLBACK')void refresh(false,false)},1000)
})
onHide(stopRealtimeUpdates)
onPullDownRefresh(()=>void refresh(true))
onBeforeUnmount(stopRealtimeUpdates)
</script>

<template>
  <view class="page detail-page">
    <view v-if="current" class="device-summary card">
      <view class="hero-top">
        <view class="device-icon">{{current.deviceType==='GATEWAY'?'GW':'DEV'}}</view>
        <view class="hero-title">
          <text>{{deviceTypeText}}</text>
          <strong>{{current.name}}</strong>
          <code>{{current.deviceKey}}</code>
        </view>
        <view class="status-pill" :class="{online}">
          <i/>
          <text>{{online?'在线':'离线'}}</text>
        </view>
      </view>
      <view class="hero-stats">
        <view><text>通信协议</text><strong>{{current.protocol||'--'}}</strong></view>
        <view><text>实时指标</text><strong>{{allLatestValues.length}}</strong></view>
        <view><text>数据刷新</text><strong>{{updatedAt?updatedAt.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'}):'--'}}</strong></view>
      </view>
      <view class="info-toggle" @click="infoExpanded=!infoExpanded">
        <view><strong>设备信息</strong><text>协议、归属与最近上报</text></view>
        <text class="toggle-label">{{infoExpanded?'收起':'展开'}}</text>
      </view>
      <view v-if="infoExpanded" class="summary-grid">
        <view><text>设备编号</text><strong>{{current.deviceKey}}</strong></view>
        <view><text>设备类型</text><strong>{{deviceTypeText}}</strong></view>
        <view><text>通信协议</text><strong>{{current.protocol||'--'}}</strong></view>
        <view><text>最近上报</text><strong>{{formatTime(current.lastSeenAt)}}</strong></view>
        <view v-if="current.gateway"><text>所属网关</text><strong>{{current.gateway.name}}</strong></view>
        <view v-if="current.location"><text>安装位置</text><strong>{{current.location}}</strong></view>
      </view>
    </view>

    <view class="realtime-sticky">
      <view class="realtime-head">
        <view>
          <view class="realtime-title"><strong>实时数据</strong><text>{{allLatestValues.length}} 项</text></view>
          <text class="refresh-state" :class="{active:online&&!error&&realtimeState==='LIVE',fallback:realtimeState==='FALLBACK'}">{{error&&realtimeState!=='LIVE'?'连接异常':realtimeLabel}}</text>
        </view>
        <view class="head-actions">
          <view class="layout-switch">
            <view :class="{active:layoutMode==='grid'}" @click="setLayout('grid')">卡片</view>
            <view :class="{active:layoutMode==='list'}" @click="setLayout('list')">列表</view>
          </view>
          <button class="refresh-button" :class="{spinning:refreshing}" @click="refresh()">刷新</button>
        </view>
      </view>
      <view v-if="allLatestValues.length>6" class="metric-search">
        <input v-model="keyword" placeholder="搜索指标名称或标识符" confirm-type="search"/>
        <button v-if="keyword" class="clear-search" @click="keyword=''">清除</button>
      </view>
    </view>
    <view v-if="error" class="error-banner">
      <view><strong>数据连接异常</strong><text>{{error}}</text></view>
      <button @click="refresh()">重试</button>
    </view>
    <view v-if="loading" class="loading-card card">
      <i/><text>正在读取设备数据...</text>
    </view>
    <view v-else class="metric-surface">
      <view v-if="layoutMode==='list'&&latestValues.length" class="metric-table-head">
        <text>名称 / 标识符</text><text>值</text><text>单位</text><text>更新 / 操作</text>
      </view>
      <view class="metric-list" :class="layoutMode">
        <view v-for="item in latestValues" :key="item.key" class="metric-card" :class="{boolean:isBooleanMetric(item.meta,item.value)}">
          <view class="metric-main">
            <text>{{item.meta?.name||item.key}}</text>
            <code>{{item.key}}</code>
          </view>
          <view v-if="isBooleanMetric(item.meta,item.value)" class="switch-value" :class="{on:booleanValue(item.value)}">
            <i/><strong>{{booleanLabel(item.meta?.name||item.key,item.value)}}</strong>
          </view>
          <view v-else class="metric-value"><strong>{{displayValue(item.value,item.meta)}}</strong></view>
          <text class="metric-unit">{{item.meta?.unit||'—'}}</text>
          <view class="metric-foot">
            <text>{{formatMetricTime(item.time)}}</text>
            <button v-if="writablePoints.has(item.key)" class="command-button" @click.stop="openCommand(item.key)">下发</button>
          </view>
        </view>
        <view v-if="!latestValues.length" class="empty-state card">
          <text>{{keyword?'未找到相关指标':'暂无实时数据'}}</text>
          <text>{{keyword?'请尝试其他名称或标识符':'设备上报后会自动显示在这里'}}</text>
          <button v-if="keyword" @click="keyword=''">清除搜索</button>
        </view>
      </view>
      <view v-if="latestValues.length" class="data-source">数据来源：{{current?.protocol||'设备上报'}} · {{realtimeLabel}}</view>
    </view>

    <view v-if="commandVisible" class="command-mask" @click.self="closeCommand">
      <view class="command-sheet">
        <view class="command-sheet-head">
          <view><strong>点位下发</strong><text>写入实际设备，以网关执行回执为准</text></view>
          <button :disabled="commandSending" @click="closeCommand">×</button>
        </view>
        <view class="command-point-card">
          <strong>{{commandPoint?.name||commandPoint?.identifier}}</strong>
          <code>{{commandPoint?.identifier}}</code>
        </view>
        <view v-if="commandCapabilities.reason" class="command-warning">{{commandCapabilities.reason}}</view>
        <view class="command-field">
          <text>下发值{{commandPoint?.unit?`（${commandPoint.unit}）`:''}}</text>
          <view v-if="commandPoint?.dataType==='BOOLEAN'" class="boolean-options">
            <button :class="{active:commandBoolean===true}" :disabled="commandSending||!!commandTask" @click="commandBoolean=true">true</button>
            <button :class="{active:commandBoolean===false}" :disabled="commandSending||!!commandTask" @click="commandBoolean=false">false</button>
          </view>
          <input v-else v-model="commandInput" :type="commandPoint?.dataType==='NUMBER'?'digit':'text'" :disabled="commandSending||!!commandTask" :placeholder="commandPoint?.dataType==='NUMBER'?'请输入数值':'请输入下发值'"/>
        </view>
        <view v-if="commandTask" class="command-result" :class="commandTask.status.toLowerCase()">
          <strong>{{commandStatusText}}</strong><text>{{commandTask.message||'指令已提交，正在等待设备处理'}}</text>
        </view>
        <view v-if="commandError" class="command-warning error">{{commandError}}</view>
        <view class="command-actions">
          <button @click="refreshCommandResult()">查询结果</button>
          <button class="primary" :loading="commandSending" :disabled="!commandCapabilities.available||commandSending||!!commandTask" @click="submitCommand">{{commandTask?'已提交':commandSending?'正在下发':'确认下发'}}</button>
        </view>
        <view v-if="commandHistory.length" class="command-history">
          <strong>最近下发记录</strong>
          <view v-for="taskItem in commandHistory" :key="taskItem.id"><text>{{new Date(taskItem.createdAt).toLocaleString()}}</text><text>{{String(taskItem.value)}}</text><b>{{({SENT:'等待回执',SUCCEEDED:'成功',FAILED:'失败',TIMEOUT:'未确认'} as Record<string,string>)[taskItem.status]||taskItem.status}}</b></view>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.detail-page{display:grid;align-content:start;gap:20rpx;padding-top:20rpx}
.device-summary{padding:24rpx;box-shadow:none}.hero-top{display:flex;align-items:center;gap:16rpx}.device-icon{display:grid;place-items:center;flex:0 0 70rpx;width:70rpx;height:70rpx;border-radius:18rpx;color:#1677ff;background:#edf5ff;font-size:19rpx;font-weight:900}.hero-title{display:grid;gap:3rpx;min-width:0}.hero-title>text{color:#7b8ba1;font-size:19rpx}.hero-title strong,.hero-title code{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.hero-title strong{font-size:29rpx}.hero-title code{color:#8b99ab;font:19rpx Consolas,monospace}.status-pill{display:flex;align-items:center;gap:8rpx;margin-left:auto;padding:9rpx 13rpx;border-radius:999rpx;color:#64748b;background:#f1f5f9;font-size:20rpx}.status-pill i{width:11rpx;height:11rpx;border-radius:50%;background:#94a3b8}.status-pill.online{color:#087a3e;background:#ecf8f0}.status-pill.online i{background:#16a34a}
.hero-stats{display:grid;grid-template-columns:1.1fr .8fr 1fr;margin-top:22rpx;padding-top:20rpx;border-top:1rpx solid #edf1f6}.hero-stats>view{display:grid;gap:5rpx;min-width:0;padding:0 14rpx;border-right:1rpx solid #edf1f6}.hero-stats>view:first-child{padding-left:0}.hero-stats>view:last-child{padding-right:0;border:0}.hero-stats text{color:#8a97a9;font-size:18rpx}.hero-stats strong{overflow:hidden;color:#42526a;font-size:22rpx;text-overflow:ellipsis;white-space:nowrap}
.info-toggle{display:flex;align-items:center;justify-content:space-between;min-height:82rpx;margin-top:18rpx;padding-top:17rpx;border-top:1rpx solid #edf1f6}.info-toggle>view{display:grid;gap:4rpx}.info-toggle strong{font-size:24rpx}.info-toggle text{color:#8a97a9;font-size:19rpx}.toggle-icon{color:#64748b!important;font-size:32rpx!important;transition:transform .2s ease}.toggle-icon.expanded{transform:rotate(180deg)}.summary-grid{display:grid;grid-template-columns:1fr 1fr;gap:18rpx;padding:0 0 4rpx;border-top:1rpx solid #edf1f6}.summary-grid>view{display:grid;gap:6rpx;min-width:0;padding-top:18rpx}.summary-grid text{color:#8a97a9;font-size:19rpx}.summary-grid strong{overflow:hidden;color:#42526a;font-size:22rpx;text-overflow:ellipsis;white-space:nowrap}
.realtime-sticky{position:sticky;top:0;z-index:20;display:grid;gap:12rpx;margin:0 -6rpx;padding:12rpx 6rpx;background:rgba(244,247,251,.96);backdrop-filter:blur(12rpx)}
.realtime-head{display:flex;align-items:center;justify-content:space-between;gap:16rpx;padding:8rpx 4rpx 2rpx}.realtime-head>view:first-child{display:grid;gap:7rpx}.realtime-title{display:flex;align-items:center;gap:12rpx}.realtime-title strong{font-size:30rpx}.realtime-title>text{padding:4rpx 10rpx;border-radius:999rpx;color:#52647d;background:#e8eef6;font-size:18rpx;font-weight:700}.refresh-state{color:#94a3b8;font-size:19rpx}.refresh-state::before{display:inline-block;width:9rpx;height:9rpx;margin-right:9rpx;border-radius:50%;background:#94a3b8;content:""}.refresh-state.active::before{background:#16a34a}.head-actions{display:flex;align-items:center;gap:10rpx}.layout-switch{display:flex;padding:5rpx;border:1rpx solid #dce5ef;border-radius:15rpx;background:#fff}.layout-switch>view{display:flex;align-items:center;gap:6rpx;padding:9rpx 11rpx;border-radius:11rpx;color:#8291a5;font-size:18rpx}.layout-switch>view.active{color:#1677ff;background:#edf5ff;font-weight:700}.layout-switch i{display:block;width:20rpx;height:20rpx}.grid-icon{background:linear-gradient(#8291a5 0 0) 0 0/8rpx 8rpx no-repeat,linear-gradient(#8291a5 0 0) 100% 0/8rpx 8rpx no-repeat,linear-gradient(#8291a5 0 0) 0 100%/8rpx 8rpx no-repeat,linear-gradient(#8291a5 0 0) 100% 100%/8rpx 8rpx no-repeat}.layout-switch .active .grid-icon{background:linear-gradient(#1677ff 0 0) 0 0/8rpx 8rpx no-repeat,linear-gradient(#1677ff 0 0) 100% 0/8rpx 8rpx no-repeat,linear-gradient(#1677ff 0 0) 0 100%/8rpx 8rpx no-repeat,linear-gradient(#1677ff 0 0) 100% 100%/8rpx 8rpx no-repeat}.list-icon{background:linear-gradient(#8291a5 0 0) 0 1rpx/20rpx 3rpx no-repeat,linear-gradient(#8291a5 0 0) 0 8rpx/20rpx 3rpx no-repeat,linear-gradient(#8291a5 0 0) 0 15rpx/20rpx 3rpx no-repeat}.layout-switch .active .list-icon{background:linear-gradient(#1677ff 0 0) 0 1rpx/20rpx 3rpx no-repeat,linear-gradient(#1677ff 0 0) 0 8rpx/20rpx 3rpx no-repeat,linear-gradient(#1677ff 0 0) 0 15rpx/20rpx 3rpx no-repeat}.refresh-button{display:grid;place-items:center;width:58rpx;height:58rpx;padding:0;border:1rpx solid #dbe4ef;border-radius:15rpx;color:#1677ff;background:#fff;font-size:30rpx;line-height:58rpx}.refresh-button.spinning{animation:spin .7s linear infinite}
.metric-search{display:flex;align-items:center;gap:12rpx;height:76rpx;padding:0 20rpx;border:1rpx solid #dce5ef;border-radius:18rpx;background:#fff}.metric-search>text{color:#8291a5;font-size:29rpx}.metric-search input{flex:1;min-width:0;font-size:24rpx}.metric-search .clear-search{display:grid;place-items:center;width:38rpx;height:38rpx;border-radius:50%;color:#64748b;background:#edf2f7;font-size:27rpx}
.error-banner{display:flex;align-items:center;justify-content:space-between;gap:18rpx;padding:20rpx 22rpx;border:1rpx solid #fecaca;border-radius:18rpx;color:#b4232b;background:#fff5f5}.error-banner>view{display:grid;gap:5rpx;min-width:0}.error-banner strong{font-size:23rpx}.error-banner text{overflow:hidden;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.error-banner button,.empty-state button{flex:0 0 auto;height:56rpx;padding:0 20rpx;border-radius:13rpx;color:#b4232b;background:#fff;font-size:21rpx;line-height:56rpx}
.loading-card{display:flex;align-items:center;justify-content:center;gap:16rpx;min-height:180rpx;color:#7a8799;font-size:23rpx}.loading-card i{width:26rpx;height:26rpx;border:4rpx solid #dbeafe;border-top-color:#1677ff;border-radius:50%;animation:spin .8s linear infinite}
.metric-list{display:grid;align-items:start;gap:14rpx}.metric-list.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.metric-card{display:grid;align-self:start;grid-template-columns:minmax(0,1fr) auto;grid-template-areas:"main value" "foot foot";gap:16rpx 18rpx;min-width:0;padding:22rpx;border:1rpx solid #e1e8f0;border-radius:19rpx;background:#fff}.metric-main{grid-area:main;display:grid;gap:6rpx;min-width:0}.metric-main text,.metric-main code{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.metric-main text{color:#334155;font-size:24rpx;font-weight:700}.metric-main code{color:#94a3b8;font-size:18rpx}.metric-value{grid-area:value;display:flex;align-items:baseline;justify-content:flex-end;gap:6rpx;max-width:310rpx;min-width:0}.metric-value strong{overflow:hidden;color:#0b63ce;font-size:34rpx;line-height:1.2;text-overflow:ellipsis;white-space:nowrap}.metric-value text{color:#64748b;font-size:18rpx}.metric-foot{grid-area:foot;display:flex;align-items:center;gap:8rpx;padding-top:13rpx;border-top:1rpx solid #f0f3f7;color:#94a3b8;font-size:17rpx}.metric-foot i{width:8rpx;height:8rpx;border-radius:50%;background:#94a3b8}.metric-foot i.online{background:#22c55e}.metric-list.grid .metric-card{grid-template-columns:minmax(0,1fr);grid-template-areas:"main" "value" "foot";gap:13rpx;padding:20rpx}.metric-list.grid .metric-value{justify-content:flex-start;max-width:100%}.metric-list.grid .metric-value strong{font-size:32rpx}.metric-list.grid .metric-foot{overflow:hidden;white-space:nowrap}.metric-list.grid .switch-value{justify-self:start}
.metric-card.boolean{border-color:#dce9e2;background:linear-gradient(135deg,#fff,#fbfffc)}.switch-value{grid-area:value;display:flex;align-items:center;gap:10rpx;align-self:center;padding:10rpx 16rpx 10rpx 10rpx;border-radius:999rpx;color:#64748b;background:#eef2f6}.switch-value i{position:relative;width:42rpx;height:26rpx;border-radius:999rpx;background:#cbd5e1;transition:background .2s ease}.switch-value i::after{position:absolute;left:4rpx;top:4rpx;width:18rpx;height:18rpx;border-radius:50%;background:#fff;box-shadow:0 2rpx 5rpx rgba(15,23,42,.18);content:"";transition:transform .2s ease}.switch-value strong{font-size:23rpx}.switch-value.on{color:#087a3e;background:#e9f9ef}.switch-value.on i{background:#16a34a}.switch-value.on i::after{transform:translateX(16rpx)}
.empty-state{display:grid;justify-items:center;gap:11rpx;padding:66rpx 20rpx;color:#8a97a9;text-align:center}.empty-icon{display:grid;place-items:center;width:80rpx;height:80rpx;margin-bottom:4rpx;border-radius:24rpx;color:#1677ff;background:#edf5ff;font-size:42rpx}.empty-state>text:nth-child(2){color:#334155;font-size:28rpx;font-weight:700}.empty-state>text:nth-child(3){font-size:22rpx}.empty-state button{margin-top:8rpx;color:#1677ff;border:1rpx solid #dbeafe}
@keyframes spin{to{transform:rotate(360deg)}}
</style>

<style scoped lang="scss">
.detail-page{gap:16rpx;padding-top:16rpx}
.device-summary{padding:22rpx;border-radius:20rpx}
.hero-stats{grid-template-columns:1.15fr .72fr 1fr;margin-top:20rpx;padding-top:18rpx}
.hero-stats>view{gap:4rpx;padding:0 14rpx}
.hero-stats text{font-size:18rpx}.hero-stats strong{color:#314056;font-size:22rpx}
.info-toggle{min-height:76rpx;margin-top:16rpx;padding-top:16rpx}
.toggle-label{padding:7rpx 12rpx;border-radius:11rpx;color:#5f7087!important;background:#f2f6fa;font-size:18rpx!important}
.summary-grid{gap:16rpx}.summary-grid>view{padding-top:16rpx}.summary-grid strong{font-size:21rpx}
.realtime-sticky{position:sticky;top:0;z-index:20;gap:10rpx;margin:0;padding:16rpx 20rpx 12rpx;border:1rpx solid #e0e7ef;border-bottom:0;border-radius:20rpx 20rpx 0 0;background:rgba(255,255,255,.97);backdrop-filter:blur(12rpx)}
.realtime-head{gap:12rpx;padding:0}.realtime-title strong{font-size:29rpx}.realtime-title>text{color:#116fd5;background:#eaf3ff}
.head-actions{gap:7rpx}.layout-switch{padding:4rpx;border-radius:13rpx;background:#f8fafc}.layout-switch>view{padding:8rpx 10rpx;border-radius:9rpx;font-size:17rpx}.layout-switch>view.active{color:#116fd5;background:#e8f2ff}
.search-button,.refresh-button{display:block;width:auto;height:50rpx;padding:0 13rpx;border:1rpx solid #dbe4ef;border-radius:12rpx;color:#52647d;background:#fff;font-size:18rpx;line-height:50rpx}.refresh-button{color:#116fd5}.refresh-button.spinning{animation:none;opacity:.55}
.metric-search{height:64rpx;padding:0 10rpx 0 18rpx;border-radius:13rpx;background:#f8fafc}.metric-search input{font-size:22rpx}.metric-search .clear-search{display:block;width:auto;height:44rpx;padding:0 14rpx;border:0;border-radius:10rpx;color:#64748b;background:#eaf0f6;font-size:18rpx;line-height:44rpx}
.metric-surface{overflow:hidden;margin-top:-16rpx;border:1rpx solid #e0e7ef;border-top:0;border-radius:0 0 20rpx 20rpx;background:#fff}
.metric-table-head,.metric-list.list .metric-card{display:grid;grid-template-columns:minmax(0,1.55fr) .88fr .52fr .78fr;align-items:center;gap:12rpx}
.metric-table-head{padding:13rpx 20rpx;color:#53647a;background:#f8fafc;font-size:18rpx;font-weight:700}.metric-table-head text:nth-child(n+2){text-align:right}
.metric-list{display:grid;gap:0}.metric-list.grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:12rpx;padding:14rpx}
.metric-card{min-width:0;border:0;border-radius:0;background:#fff}
.metric-list.list .metric-card{min-height:94rpx;padding:14rpx 20rpx;border-top:1rpx solid #edf1f5}
.metric-main{grid-area:auto;gap:4rpx}.metric-main text{color:#26364b;font-size:21rpx}.metric-main code{color:#8a99ad;font-size:17rpx}
.metric-value{grid-area:auto;justify-content:flex-end;max-width:none}.metric-value strong{color:#0b6fd3;font-size:26rpx}.metric-unit{color:#52647d;font-size:18rpx;text-align:right}.metric-foot{grid-area:auto;display:flex;align-items:center;justify-content:flex-end;gap:10rpx;padding:0;border:0;color:#66768b;font-size:17rpx;text-align:right}.command-button{height:46rpx;margin:0;padding:0 14rpx;border:1rpx solid #b9d7ff;border-radius:10rpx;color:#126fd5;background:#edf5ff;font-size:18rpx;line-height:44rpx}
.metric-card.boolean{border-color:transparent;background:#fff}.switch-value{grid-area:auto;justify-self:end;gap:7rpx;padding:7rpx 11rpx;border-radius:10rpx;color:#52647d;background:#edf2f6}.switch-value i{width:10rpx;height:10rpx;background:#52647d}.switch-value i::after{display:none}.switch-value strong{font-size:19rpx}.switch-value.on{color:#087a3e;background:#e7f7ec}.switch-value.on i{background:#16a34a}
.metric-list.grid .metric-card{display:grid;grid-template-columns:minmax(0,1fr) auto;grid-template-areas:"main main" "value unit" "foot foot";gap:13rpx;padding:18rpx;border:1rpx solid #e4eaf1;border-radius:16rpx}.metric-list.grid .metric-main{grid-area:main}.metric-list.grid .metric-value{grid-area:value;justify-content:flex-start}.metric-list.grid .metric-unit{grid-area:unit;align-self:end}.metric-list.grid .metric-foot{grid-area:foot;padding-top:10rpx;border-top:1rpx solid #edf1f5;text-align:left}.metric-list.grid .switch-value{grid-area:value;justify-self:start}.metric-list.grid .metric-card.boolean .metric-unit{display:none}
.data-source{padding:18rpx;color:#8b99ab;border-top:1rpx solid #edf1f5;font-size:18rpx;text-align:center}
.command-mask{position:fixed;inset:0;z-index:100;display:flex;align-items:flex-end;justify-content:center;padding:24rpx;background:rgba(15,23,42,.48)}.command-sheet{display:grid;gap:20rpx;width:100%;max-width:720rpx;max-height:86vh;overflow-y:auto;padding:26rpx;border-radius:28rpx 28rpx 20rpx 20rpx;background:#fff;box-shadow:0 -16rpx 50rpx rgba(15,23,42,.18)}.command-sheet-head{display:flex;align-items:flex-start;justify-content:space-between;gap:20rpx}.command-sheet-head>view{display:grid;gap:6rpx}.command-sheet-head strong{font-size:31rpx}.command-sheet-head text{color:#7b8ba1;font-size:20rpx}.command-sheet-head button{width:54rpx;height:54rpx;margin:0;padding:0;border-radius:50%;color:#64748b;background:#f1f5f9;font-size:34rpx;line-height:52rpx}.command-point-card{display:grid;gap:7rpx;padding:20rpx;border-radius:17rpx;background:#f7f9fc}.command-point-card strong{color:#24364d;font-size:25rpx}.command-point-card code{color:#8392a6;font-size:19rpx}.command-warning{padding:17rpx 18rpx;border:1rpx solid #fed7aa;border-radius:14rpx;color:#9a5b0b;background:#fff8eb;font-size:20rpx;line-height:1.5}.command-warning.error{border-color:#fecaca;color:#b4232b;background:#fff5f5}.command-field{display:grid;gap:12rpx}.command-field>text{color:#42526a;font-size:21rpx;font-weight:700}.command-field input{height:76rpx;padding:0 20rpx;border:1rpx solid #d9e2ec;border-radius:15rpx;background:#fff;font-size:24rpx}.boolean-options{display:grid;grid-template-columns:1fr 1fr;gap:12rpx}.boolean-options button{height:72rpx;border:1rpx solid #d9e2ec;border-radius:14rpx;color:#52647d;background:#f8fafc;font-size:24rpx;line-height:70rpx}.boolean-options button.active{border-color:#1677ff;color:#126fd5;background:#eaf3ff;font-weight:800}.command-result{display:grid;gap:6rpx;padding:17rpx 18rpx;border-radius:14rpx;color:#8a5a0a;background:#fff7e6}.command-result strong{font-size:22rpx}.command-result text{font-size:19rpx}.command-result.succeeded{color:#087a3e;background:#eaf8ef}.command-result.failed{color:#b4232b;background:#fff0f0}.command-actions{display:grid;grid-template-columns:1fr 1.35fr;gap:12rpx}.command-actions button{height:72rpx;border:1rpx solid #d9e2ec;border-radius:14rpx;color:#52647d;background:#fff;font-size:22rpx;line-height:70rpx}.command-actions button.primary{border-color:#1677ff;color:#fff;background:#1677ff;font-weight:700}.command-actions button[disabled]{opacity:.5}.command-history{display:grid;gap:10rpx;padding-top:18rpx;border-top:1rpx solid #edf1f5}.command-history>strong{font-size:21rpx}.command-history>view{display:grid;grid-template-columns:1.5fr .7fr .55fr;gap:10rpx;color:#64748b;font-size:18rpx}.command-history>view text:nth-child(2),.command-history>view b{text-align:right}.command-history b{color:#42526a}
.empty-state{margin:14rpx;border:0;box-shadow:none}.empty-state>text:first-child{color:#334155;font-size:28rpx;font-weight:700}.empty-state>text:nth-child(2){font-size:22rpx}
@media (max-width:360px){.search-button{display:none}.metric-table-head,.metric-list.list .metric-card{grid-template-columns:minmax(0,1.45fr) .86fr .45fr .74fr;gap:8rpx}.metric-table-head,.metric-list.list .metric-card{padding-left:14rpx;padding-right:14rpx}.metric-main text{font-size:20rpx}.metric-value strong{font-size:24rpx}}
</style>
