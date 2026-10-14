<script setup lang="ts">
import { onLaunch, onShow } from '@dcloudio/uni-app'
import { consumeNativeGatewayNfc, storePendingGatewayNfc } from './services/gateway-binding'
import { hasSession, isAdminBuild, isAdminPortal } from './services/session'
import { alarms } from './services/cloud'
import { requestSystemNotificationPermission, subscribeAlarmNotifications } from './services/alarm-notifications'

let stopAlarmNotifications:(()=>void)|undefined

function updateAlarmBadge(count:number){
  if(count>0)uni.setTabBarBadge({index:1,text:String(Math.min(count,99))})
  else uni.removeTabBarBadge({index:1})
}

async function startAlarmNotifications(){
  if(!hasSession()||isAdminBuild()||isAdminPortal())return
  requestSystemNotificationPermission()
  try{const data=await alarms({page:1,pageSize:1});uni.setStorageSync('iotCloudAlarmUnread',data.unreadTotal);updateAlarmBadge(data.unreadTotal)}catch{}
  stopAlarmNotifications?.()
  stopAlarmNotifications=subscribeAlarmNotifications((alarm)=>{
    if(alarm.status==='OPEN'){
      const current=Number(uni.getStorageSync('iotCloudAlarmUnread')||0)+1
      uni.setStorageSync('iotCloudAlarmUnread',current);updateAlarmBadge(current)
    }
  })
}

function openGatewayNfc(payload:string){
  if(!payload)return false
  storePendingGatewayNfc(payload)
  if(!hasSession()){uni.reLaunch({url:'/pages/login/index'});return true}
  if(isAdminBuild()||isAdminPortal()){
    uni.showToast({title:'请使用物联云租户端碰一碰绑定',icon:'none'})
    return true
  }
  uni.reLaunch({url:'/pages/gateways/bind?source=nfc'})
  return true
}

onLaunch((options) => {
  if(typeof window!=='undefined')window.__weikongOpenAlarms=()=>uni.switchTab({url:'/pages/alarms/index'})
  if(typeof window!=='undefined')window.__weikongNfcRead=(payload)=>{consumeNativeGatewayNfc();void openGatewayNfc(payload)}
  if(openGatewayNfc(consumeNativeGatewayNfc()))return
  if (!hasSession()) {
    uni.reLaunch({ url: '/pages/login/index' })
    return
  }
  void startAlarmNotifications()
  if (!options.path || options.path === 'pages/login/index') {
    if (isAdminBuild() || isAdminPortal()) {
      uni.reLaunch({ url: '/pages/admin/index' })
      return
    }
    uni.switchTab({ url: '/pages/devices/index' })
  }
})

onShow(()=>void startAlarmNotifications())
</script>

<template><slot /></template>
