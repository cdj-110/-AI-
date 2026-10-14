<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { onHide, onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { alarms, markAlarmNotificationsRead } from '../../services/cloud'
import { requestSystemNotificationPermission, subscribeAlarmNotifications } from '../../services/alarm-notifications'
import type { Alarm } from '../../types'

const items=ref<Alarm[]>([]),loading=ref(false),error=ref(''),filter=ref<'ALL'|'OPEN'|'RESOLVED'>('ALL')
let stopStream:(()=>void)|undefined
const visibleItems=computed(()=>filter.value==='ALL'?items.value:items.value.filter(item=>item.status===filter.value))
const openCount=computed(()=>items.value.filter(item=>item.status==='OPEN').length)
function levelText(level:string){return level==='CRITICAL'?'严重':level==='WARNING'?'警告':'提示'}
function timeText(value?:string){if(!value)return'-';const date=new Date(value);return Number.isNaN(date.getTime())?'-':date.toLocaleString('zh-CN',{hour12:false})}
async function refresh(){
  loading.value=true
  try{const data=await alarms({page:1,pageSize:100});items.value=data.items;error.value='';await markAlarmNotificationsRead();uni.setStorageSync('iotCloudAlarmUnread',0);uni.removeTabBarBadge({index:1})}
  catch(reason){error.value=reason instanceof Error?reason.message:'告警读取失败'}
  finally{loading.value=false;uni.stopPullDownRefresh()}
}
function mergeAlarm(alarm:Alarm){
  const index=items.value.findIndex(item=>item.id===alarm.id)
  if(index>=0)items.value=items.value.map(item=>item.id===alarm.id?{...item,...alarm}:item)
  else items.value=[alarm,...items.value]
  void markAlarmNotificationsRead()
}
onShow(()=>{requestSystemNotificationPermission();void refresh();stopStream?.();stopStream=subscribeAlarmNotifications(mergeAlarm,{notifySystem:false})})
onHide(()=>{stopStream?.();stopStream=undefined})
onPullDownRefresh(refresh)
onBeforeUnmount(()=>stopStream?.())
</script>

<template>
  <view class="page alarm-page">
    <view class="alarm-hero">
      <view><text>告警中心</text><strong>{{openCount}}</strong><small>当前告警</small></view>
      <view class="hero-mark">!</view>
    </view>
    <view class="filters">
      <button :class="{active:filter==='ALL'}" @click="filter='ALL'">全部 {{items.length}}</button>
      <button :class="{active:filter==='OPEN'}" @click="filter='OPEN'">告警中 {{openCount}}</button>
      <button :class="{active:filter==='RESOLVED'}" @click="filter='RESOLVED'">已恢复 {{items.length-openCount}}</button>
    </view>
    <text v-if="error" class="error-banner">{{error}}</text>
    <view v-if="loading&&!items.length" class="empty">正在加载告警...</view>
    <view v-else class="alarm-list">
      <view v-for="alarm in visibleItems" :key="alarm.id" class="alarm-card" :class="[alarm.level.toLowerCase(),alarm.status.toLowerCase()]">
        <view class="alarm-head"><view class="alarm-icon">!</view><view><strong>{{alarm.deviceName||alarm.device?.name||'设备告警'}}</strong><text>{{alarm.device?.deviceKey||alarm.type}}</text></view><span>{{alarm.status==='OPEN'?'告警中':'已恢复'}}</span></view>
        <text class="alarm-message">{{alarm.message}}</text>
        <view class="alarm-foot"><text>{{levelText(alarm.level)}}</text><text>{{timeText(alarm.createdAt)}}</text></view>
      </view>
      <view v-if="!visibleItems.length" class="empty"><strong>暂无告警</strong><text>设备运行异常时会实时推送到这里</text></view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.alarm-page{padding-top:18rpx}.alarm-hero{display:flex;align-items:center;justify-content:space-between;padding:28rpx;border-radius:24rpx;color:#fff;background:linear-gradient(135deg,#ef4444,#f97316);box-shadow:0 14rpx 30rpx rgba(239,68,68,.2)}.alarm-hero>view:first-child{display:grid;grid-template-columns:auto auto;gap:5rpx 16rpx;align-items:end}.alarm-hero text{grid-column:1/-1;font-size:25rpx;opacity:.9}.alarm-hero strong{font-size:48rpx;line-height:1}.alarm-hero small{padding-bottom:5rpx;font-size:21rpx}.hero-mark{display:grid;width:70rpx;height:70rpx;place-items:center;border-radius:22rpx;background:rgba(255,255,255,.18);font-size:40rpx;font-weight:900}.filters{display:grid;grid-template-columns:repeat(3,1fr);gap:10rpx;margin:20rpx 0}.filters button{height:66rpx;padding:0;border-radius:15rpx;color:#64748b;background:#fff;font-size:22rpx;line-height:66rpx}.filters button.active{color:#e5484d;background:#fff1f2;font-weight:700}.alarm-list{display:grid;gap:14rpx}.alarm-card{display:grid;gap:18rpx;padding:22rpx;border:1rpx solid #e6ebf2;border-left:6rpx solid #f59e0b;border-radius:20rpx;background:#fff}.alarm-card.critical{border-left-color:#ef4444}.alarm-card.resolved{border-left-color:#22c55e}.alarm-head{display:flex;align-items:center;gap:14rpx}.alarm-icon{display:grid;flex:0 0 54rpx;width:54rpx;height:54rpx;place-items:center;border-radius:16rpx;color:#fff;background:#f59e0b;font-weight:900}.critical .alarm-icon{background:#ef4444}.resolved .alarm-icon{background:#22c55e}.alarm-head>view:nth-child(2){display:grid;min-width:0;gap:3rpx}.alarm-head strong,.alarm-head text{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.alarm-head strong{font-size:27rpx}.alarm-head text{color:#94a3b8;font-size:19rpx}.alarm-head span{margin-left:auto;padding:7rpx 12rpx;border-radius:999rpx;color:#dc2626;background:#fff1f2;font-size:19rpx}.resolved .alarm-head span{color:#15803d;background:#f0fdf4}.alarm-message{color:#475569;font-size:23rpx;line-height:1.55}.alarm-foot{display:flex;justify-content:space-between;padding-top:14rpx;border-top:1rpx solid #eef2f6;color:#94a3b8;font-size:19rpx}.empty{display:grid;min-height:300rpx;place-content:center;gap:12rpx;color:#94a3b8;text-align:center}.empty strong{color:#475569;font-size:28rpx}.error-banner{display:block;margin-bottom:16rpx;padding:16rpx;border-radius:14rpx;color:#b4232b;background:#fff0f1;font-size:22rpx}
</style>
