<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { profile } from '../../services/cloud'
import { clearSession, currentUser, isAdminPortal, serverURL } from '../../services/session'
import type { UserInfo } from '../../types'

const user=ref<UserInfo|null>(currentUser()),server=ref(serverURL())
function roleText(value?:string){return value==='SUPER_ADMIN'?'超级管理员':value==='TENANT_ADMIN'?'租户管理员':'普通用户'}
function logout(){
  uni.showModal({title:'退出登录',content:'确认退出当前物联云账号？',success(result){if(!result.confirm)return;clearSession();uni.reLaunch({url:'/pages/login/index'})}})
}
onShow(async()=>{try{user.value=await profile()}catch{}})
</script>

<template>
  <view class="page profile-page">
    <view class="profile-card card"><view class="avatar">{{(user?.nickname||user?.username||'U').slice(0,1).toUpperCase()}}</view><view><strong>{{user?.nickname||user?.username||'-'}}</strong><text>{{roleText(user?.role)}}</text></view></view>
    <view class="setting-card card">
      <view><text>登录账号</text><strong>{{user?.username||'-'}}</strong></view>
      <view><text>租户编号</text><strong>{{user?.tenantId||'平台级账号'}}</strong></view>
      <view><text>云平台地址</text><strong>{{server}}</strong></view>
      <view><text>客户端版本</text><strong>0.1.6</strong></view>
    </view>
    <button v-if="isAdminPortal()" class="admin-button" @click="uni.reLaunch({url:'/pages/admin/index'})">进入管理端</button>
    <button class="logout-button" @click="logout">退出登录</button>
  </view>
</template>

<style scoped lang="scss">
.profile-page{display:grid;gap:20rpx}.profile-card{display:flex;align-items:center;gap:22rpx}.avatar{display:grid;place-items:center;width:92rpx;height:92rpx;border-radius:25rpx;color:#fff;background:linear-gradient(145deg,#1677ff,#0ea5e9);font-size:38rpx;font-weight:800}.profile-card>view:last-child{display:grid;gap:7rpx}.profile-card strong{font-size:32rpx}.profile-card text{color:#7a8799;font-size:22rpx}.setting-card{padding-top:8rpx;padding-bottom:8rpx}.setting-card>view{display:grid;grid-template-columns:150rpx minmax(0,1fr);gap:20rpx;padding:25rpx 0;border-bottom:1rpx solid #edf1f6}.setting-card>view:last-child{border-bottom:0}.setting-card text{color:#7a8799;font-size:23rpx}.setting-card strong{overflow:hidden;color:#334155;font-size:23rpx;text-align:right;text-overflow:ellipsis;white-space:nowrap}.admin-button{height:84rpx;border-radius:18rpx;color:#fff;background:#1677ff;font-size:27rpx;font-weight:700;line-height:84rpx}.logout-button{height:84rpx;border:1rpx solid #fecaca;border-radius:18rpx;color:#b4232b;background:#fff;font-size:27rpx;line-height:84rpx}
</style>
