<script setup lang="ts">
import { ref } from 'vue'
import { login } from '../../services/cloud'
import { hasPendingGatewayNfc } from '../../services/gateway-binding'
import { defaultPortal, isAdminBuild, saveServer, saveSession, serverURL } from '../../services/session'

const username=ref(''),password=ref(''),server=ref(serverURL())
const portal=ref<'tenant'|'admin'>(defaultPortal())
const showServer=ref(false),busy=ref(false),error=ref('')

async function submit(){
  if(!username.value.trim()||!password.value){error.value='请输入邮箱或手机号和密码';return}
  if(!/^https?:\/\//i.test(server.value.trim())){error.value='服务器地址需要以 http:// 或 https:// 开头';showServer.value=true;return}
  busy.value=true;error.value=''
  try{
    saveServer(server.value)
    const result=await login(username.value.trim(),password.value,portal.value)
    saveSession(result.accessToken,result.user,portal.value)
    uni.reLaunch({url:portal.value==='admin'?'/pages/admin/index':hasPendingGatewayNfc()?'/pages/gateways/bind?source=nfc':'/pages/devices/index'})
  }catch(reason){error.value=reason instanceof Error?reason.message:'登录失败'}
  finally{busy.value=false}
}
</script>

<template>
  <view class="login-page">
    <view class="login-brand"><view class="brand-mark">WK</view><view><text>物联云</text><text>WEIKONG IoT Cloud</text></view></view>
    <view class="login-card">
      <view class="login-title"><text>欢迎登录</text><text>{{isAdminBuild()?'公司管理平台账号':'使用物联云平台同一账号'}}</text></view>
      <view v-if="false" class="portal-switch">
        <view :class="{active:portal==='tenant'}" @click="portal='tenant'">租户端</view>
        <view :class="{active:portal==='admin'}" @click="portal='admin'">管理端</view>
      </view>
      <label class="field"><text>邮箱或手机号</text><input v-model="username" placeholder="请输入邮箱或手机号" autocomplete="username"/></label>
      <label class="field"><text>密码</text><input v-model="password" password placeholder="请输入密码" autocomplete="current-password" @confirm="submit"/></label>
      <button class="server-toggle" @click="showServer=!showServer">{{showServer?'收起服务器设置':'服务器设置'}}</button>
      <label v-if="showServer" class="field server-field"><text>物联云地址</text><input v-model="server" placeholder="例如 https://iot.example.com"/></label>
      <text v-if="error" class="login-error">{{error}}</text>
      <button class="primary-button login-button" :loading="busy" :disabled="busy" @click="submit">{{busy?'正在登录':'登录'}}</button>
      <text class="login-hint">登录凭证、租户和权限与 Web 管理平台保持一致</text>
    </view>
  </view>
</template>

<style scoped lang="scss">
.login-page{min-height:100vh;padding:calc(96rpx + env(safe-area-inset-top)) 42rpx 48rpx;background:radial-gradient(circle at 85% 8%,#ddecff 0,transparent 34%),linear-gradient(160deg,#f8fbff,#edf4fc)}
.login-brand{display:flex;align-items:center;gap:20rpx;margin:0 10rpx 58rpx}.login-brand>view:last-child{display:grid;gap:5rpx}.login-brand text:first-child{color:#12213a;font-size:38rpx;font-weight:800}.login-brand text:last-child{color:#7a8799;font-size:20rpx;letter-spacing:2rpx}.brand-mark{display:grid;place-items:center;width:80rpx;height:80rpx;border-radius:22rpx;color:#fff;background:linear-gradient(145deg,#1677ff,#0ea5e9);box-shadow:0 16rpx 34rpx rgba(22,119,255,.22);font-size:24rpx;font-weight:900}
.login-card{padding:42rpx 34rpx;border:1rpx solid rgba(255,255,255,.9);border-radius:32rpx;background:rgba(255,255,255,.94);box-shadow:0 28rpx 80rpx rgba(31,52,82,.12)}.login-title{display:grid;gap:10rpx;margin-bottom:24rpx}.login-title text:first-child{font-size:42rpx;font-weight:800}.login-title text:last-child{color:#7a8799;font-size:25rpx}.portal-switch{display:grid;grid-template-columns:1fr 1fr;gap:8rpx;padding:8rpx;border-radius:18rpx;background:#eef4fb}.portal-switch>view{height:62rpx;border-radius:14rpx;color:#64748b;font-size:25rpx;font-weight:700;line-height:62rpx;text-align:center}.portal-switch>view.active{color:#0b63ce;background:#fff;box-shadow:0 8rpx 20rpx rgba(22,119,255,.12)}.server-toggle{width:auto;height:58rpx;margin:20rpx 0 0;padding:0;color:#3971b7;background:transparent;font-size:23rpx;line-height:58rpx}.server-field{margin-top:4rpx}.login-error{display:block;margin-top:22rpx;padding:16rpx 20rpx;border-radius:12rpx;color:#b4232b;background:#fff0f1;font-size:24rpx}.login-button{margin-top:30rpx}.login-hint{display:block;margin-top:24rpx;color:#8a97a9;font-size:21rpx;text-align:center}
</style>
