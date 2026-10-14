<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { onHide, onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { devices as loadDeviceList } from '../../services/cloud'
import { canBindGateway, canManageDevices } from '../../services/session'
import type { Device } from '../../types'

const items=ref<Device[]>([]),total=ref(0),loading=ref(false),keyword=ref(''),error=ref('')
let timer: ReturnType<typeof setInterval> | undefined
const onlineCount=computed(()=>items.value.filter(item=>item.status==='ONLINE').length)
const offlineCount=computed(()=>items.value.filter(item=>item.status!=='ONLINE').length)

function statusText(value:string){return value==='ONLINE'?'在线':'离线'}
function typeText(value:string){return value==='GATEWAY'?'智能网关':value==='GATEWAY_CHILD'?'网关子设备':'直连设备'}
function formatTime(value?:string){if(!value)return'暂无上报';const date=new Date(value);return Number.isNaN(date.getTime())?'暂无上报':date.toLocaleString()}
async function refresh(silent=false){
  if(!silent)loading.value=true
  try{
    const data=await loadDeviceList({page:1,pageSize:100,keyword:keyword.value.trim()})
    items.value=data.items;total.value=data.total;error.value=''
  }catch(reason){if(!silent)error.value=reason instanceof Error?reason.message:'设备读取失败'}
  finally{loading.value=false;uni.stopPullDownRefresh()}
}
function open(item:Device){uni.navigateTo({url:`/pages/devices/detail?id=${encodeURIComponent(item.id)}`})}
function create(){
  if(!canBindGateway()){uni.navigateTo({url:'/pages/devices/create'});return}
  uni.showActionSheet({
    itemList:['扫码绑定网关','手动新增设备'],
    success:({tapIndex})=>uni.navigateTo({url:tapIndex===0?'/pages/gateways/bind?scan=1':'/pages/devices/create'})
  })
}
function stopRefreshTimer(){if(timer!==undefined){clearInterval(timer);timer=undefined}}
onShow(()=>{void refresh();stopRefreshTimer();timer=setInterval(()=>void refresh(true),5000)})
onHide(stopRefreshTimer)
onPullDownRefresh(()=>void refresh())
onBeforeUnmount(stopRefreshTimer)
</script>

<template>
  <view class="page device-page">
    <view class="overview">
      <view class="overview-item total-item">
        <view class="overview-label"><text>设备总数</text></view>
        <strong>{{total}}</strong>
      </view>
      <view class="overview-item online-item">
        <view class="overview-label"><i/><text>在线</text></view>
        <strong class="online-text">{{onlineCount}}</strong>
      </view>
      <view class="overview-item offline-item">
        <view class="overview-label"><i/><text>离线</text></view>
        <strong>{{offlineCount}}</strong>
      </view>
    </view>
    <view class="device-tools">
      <view class="search"><input v-model="keyword" confirm-type="search" placeholder="搜索设备名称或编号" @confirm="refresh()"/><button @click="refresh()">搜索</button></view>
      <button v-if="canManageDevices()" class="add-button" aria-label="新增设备或绑定网关" @click="create">+</button>
    </view>
    <text v-if="error" class="error-banner">{{error}}</text>
    <view v-if="loading&&!items.length" class="loading">正在加载设备...</view>
    <view v-else class="device-list">
      <view v-for="item in items" :key="item.id" class="device-card card" @click="open(item)">
        <view class="device-head"><view class="device-icon">{{item.deviceType==='GATEWAY'?'GW':'DEV'}}</view><view class="device-title"><strong>{{item.name}}</strong><text>{{item.deviceKey}}</text></view><view class="device-status"><i class="status-dot" :class="item.status==='ONLINE'?'online':'offline'"/><text>{{statusText(item.status)}}</text></view></view>
        <view class="device-meta"><text>{{typeText(item.deviceType)}}</text><text>{{item.protocol}}</text><text v-if="item.location">{{item.location}}</text></view>
        <view class="device-foot"><text>{{item.gateway?`归属 ${item.gateway.name}`:'最近上报'}}</text><text>{{item.gateway?item.gateway.deviceKey:formatTime(item.lastSeenAt)}}</text><b>›</b></view>
      </view>
      <view v-if="!items.length" class="empty-state"><text>暂无设备</text><text>{{canManageDevices()?'点击右上角新增第一台设备':'请联系管理员添加设备'}}</text></view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.device-page{padding-top:20rpx}.overview{display:grid;grid-template-columns:1.25fr 1fr 1fr;gap:12rpx}.overview>view{display:grid;gap:10rpx;padding:24rpx 20rpx;border-radius:20rpx;background:#fff}.overview text{color:#7a8799;font-size:22rpx}.overview strong{font-size:38rpx}.online-text{color:#0b9448}
.device-tools{display:grid;grid-template-columns:minmax(0,1fr) 82rpx;gap:14rpx;margin:24rpx 0}.search{display:flex;align-items:center;gap:12rpx;height:82rpx;padding:0 22rpx;border:1rpx solid #dbe3ee;border-radius:18rpx;background:#fff}.search>text{color:#7a8799;font-size:32rpx}.search input{flex:1;font-size:26rpx}.add-button{width:82rpx;height:82rpx;padding:0;border-radius:18rpx;color:#fff;background:#1677ff;font-size:42rpx;line-height:78rpx}.error-banner{display:block;margin-bottom:18rpx;padding:18rpx;border-radius:14rpx;color:#b4232b;background:#fff0f1;font-size:24rpx}.loading,.empty-state{padding:80rpx 20rpx;color:#7a8799;text-align:center}.device-list{display:grid;gap:18rpx}.device-card{display:grid;gap:20rpx}.device-head{display:flex;align-items:center;gap:18rpx}.device-icon{display:grid;place-items:center;flex:0 0 72rpx;width:72rpx;height:72rpx;border-radius:19rpx;color:#1677ff;background:#eaf3ff;font-size:21rpx;font-weight:900}.device-title{display:grid;gap:6rpx;min-width:0}.device-title strong,.device-title text{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.device-title strong{font-size:29rpx}.device-title text{color:#7a8799;font:22rpx Consolas,monospace}.device-status{display:flex;align-items:center;gap:10rpx;margin-left:auto;font-size:23rpx}.device-meta{display:flex;flex-wrap:wrap;gap:10rpx}.device-meta text{padding:7rpx 13rpx;border-radius:999rpx;color:#52647d;background:#f1f5f9;font-size:20rpx}.device-foot{display:grid;grid-template-columns:auto minmax(0,1fr) 20rpx;gap:12rpx;padding-top:18rpx;border-top:1rpx solid #edf1f6;color:#7a8799;font-size:21rpx}.device-foot text:nth-child(2){overflow:hidden;text-align:right;text-overflow:ellipsis;white-space:nowrap}.device-foot b{color:#1677ff;font-size:28rpx}.empty-state{display:grid;gap:12rpx}.empty-state text:first-child{color:#334155;font-size:30rpx;font-weight:700}
</style>

<style scoped lang="scss">
.device-page{padding-top:16rpx;background:transparent}.overview{overflow:visible;gap:12rpx;border:0;border-radius:0;background:transparent;box-shadow:none}.overview>view{gap:10rpx;padding:20rpx;border:1rpx solid #e1e7ef;border-radius:17rpx;background:#fff;box-shadow:0 5rpx 15rpx rgba(15,23,42,.035)}.overview-label{display:flex;align-items:center;gap:8rpx}.overview-label i{width:9rpx;height:9rpx;border-radius:50%;background:#ef4444}.online-item .overview-label i{background:#16a34a}.overview text{color:#718198;font-size:19rpx}.overview strong{color:#1f2937;font-size:35rpx;line-height:1}
.device-tools{grid-template-columns:minmax(0,1fr) 72rpx;gap:12rpx;margin:18rpx 0}.search{height:72rpx;padding:0 8rpx 0 18rpx;border-radius:15rpx}.search input{min-width:0;font-size:24rpx}.search button{height:54rpx;padding:0 16rpx;border-radius:11rpx;color:#1677ff;background:#edf5ff;font-size:21rpx;line-height:54rpx}.add-button{display:grid;place-items:center;width:72rpx;height:72rpx;padding:0;border-radius:15rpx;color:#fff;background:#1677ff;box-shadow:0 8rpx 20rpx rgba(22,119,255,.2);font-size:42rpx;font-weight:400;line-height:1}
.device-list{gap:12rpx}.device-card{gap:15rpx;padding:20rpx;box-shadow:none}.device-head{gap:15rpx}.device-icon{flex-basis:64rpx;width:64rpx;height:64rpx;border-radius:16rpx;font-size:19rpx}.device-title{gap:4rpx}.device-title strong{font-size:27rpx}.device-title text{font-size:20rpx}.device-status{gap:8rpx;font-size:21rpx}.device-meta{gap:8rpx}.device-meta text{padding:6rpx 11rpx;font-size:18rpx}.device-foot{gap:10rpx;padding-top:14rpx;font-size:19rpx}
</style>
