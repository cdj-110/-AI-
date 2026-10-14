<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { bindGateway, verifyGatewayBind } from '../../services/cloud'
import { parseGatewayBindCode, scanGatewayBindCode, takePendingGatewayNfc } from '../../services/gateway-binding'
import type { Device, GatewayBindVerification } from '../../types'

const form=reactive({sn:'',bindCode:'',nfcToken:'',name:'',location:''})
const step=ref<0|1|2>(0)
const scanning=ref(false),verifying=ref(false),binding=ref(false),error=ref('')
const nfcStage=ref<''|'reading'|'success'>('')
const verification=ref<GatewayBindVerification|null>(null)
const boundDevice=ref<Device|null>(null)
let nfcAnimationTimer:ReturnType<typeof setTimeout>|undefined

function formatTime(value?:string){
  if(!value)return'暂无记录'
  const date=new Date(value)
  return Number.isNaN(date.getTime())?'暂无记录':date.toLocaleString()
}

async function scan(){
  if(scanning.value)return
  scanning.value=true;error.value=''
  try{
    const credential=await scanGatewayBindCode()
    form.sn=credential.sn
    form.bindCode=credential.bindCode||''
    form.nfcToken=credential.nfcToken||''
    await verify()
  }catch(reason){
    const message=reason instanceof Error?reason.message:'扫码失败'
    if(message!=='已取消扫码')error.value=message
  }finally{scanning.value=false}
}

async function verify(){
  const sn=form.sn.trim().toUpperCase(),bindCode=form.bindCode.trim(),nfcToken=form.nfcToken.trim()
  if(!sn||(!/^\d{6}$/.test(bindCode)&&!nfcToken)){error.value='请填写网关 SN 和 6 位数字绑定码';return}
  verifying.value=true;error.value=''
  try{
    verification.value=await verifyGatewayBind({sn,...(nfcToken?{nfcToken}:{bindCode})})
    form.sn=verification.value.sn
    if(!form.name.trim())form.name=verification.value.suggestedName
    step.value=1
  }catch(reason){error.value=reason instanceof Error?reason.message:'网关校验失败'}
  finally{verifying.value=false}
}

async function confirmBind(){
  if(!verification.value||binding.value)return
  const modal=await uni.showModal({
    title:'确认绑定网关',
    content:`将 ${verification.value.sn} 绑定为“${form.name.trim()||verification.value.suggestedName}”，绑定后该网关归属于当前租户。`,
    confirmText:'确认绑定',
    cancelText:'再检查一下',
  })
  if(!modal.confirm)return
  binding.value=true;error.value=''
  try{
    const result=await bindGateway({
      sn:form.sn.trim().toUpperCase(),...(form.nfcToken.trim()?{nfcToken:form.nfcToken.trim()}:{bindCode:form.bindCode.trim()}),
      name:form.name.trim()||undefined,location:form.location.trim()||undefined,
    })
    boundDevice.value=result.device
    step.value=2
    uni.showToast({title:'网关绑定成功',icon:'success'})
  }catch(reason){error.value=reason instanceof Error?reason.message:'网关绑定失败'}
  finally{binding.value=false}
}

function editCredential(){verification.value=null;step.value=0;error.value=''}
function toList(){uni.switchTab({url:'/pages/devices/index'})}
function toDetail(){if(boundDevice.value)uni.redirectTo({url:`/pages/devices/detail?id=${encodeURIComponent(boundDevice.value.id)}`})}

onLoad((options)=>{
  const nfcPayload=takePendingGatewayNfc()
  if(nfcPayload){
    nfcStage.value='reading'
    try{
      const credential=parseGatewayBindCode(nfcPayload)
      form.sn=credential.sn
      form.nfcToken=credential.nfcToken||''
      form.bindCode=''
      nfcAnimationTimer=setTimeout(()=>{
        nfcStage.value='success'
        nfcAnimationTimer=setTimeout(()=>{
          nfcStage.value=''
          void verify()
        },620)
      },720)
      return
    }catch(reason){nfcStage.value='';error.value=reason instanceof Error?reason.message:'NFC 标签内容无效'}
  }
  if(String(options?.scan||'')==='1')setTimeout(()=>void scan(),250)
})
onBeforeUnmount(()=>{if(nfcAnimationTimer)clearTimeout(nfcAnimationTimer)})
</script>

<template>
  <view class="page bind-page">
    <view v-if="nfcStage" class="nfc-feedback" :class="nfcStage">
      <view class="nfc-feedback-card">
        <view class="nfc-signal">
          <i/><i/><i/>
          <strong>{{nfcStage==='success'?'✓':'NFC'}}</strong>
        </view>
        <strong>{{nfcStage==='success'?'网关识别成功':'正在识别网关'}}</strong>
        <text>{{nfcStage==='success'?`${form.sn} 凭证读取完成`:'请保持手机靠近 NFC 标签'}}</text>
      </view>
    </view>
    <view class="progress">
      <view :class="{active:step>=0,done:step>0}"><i>1</i><text>识别网关</text></view>
      <b :class="{active:step>0}"/>
      <view :class="{active:step>=1,done:step>1}"><i>2</i><text>确认信息</text></view>
      <b :class="{active:step>1}"/>
      <view :class="{active:step>=2}"><i>3</i><text>完成绑定</text></view>
    </view>

    <view v-if="step===0" class="card bind-card">
      <view class="scan-hero">
        <view class="scan-icon"><i/><i/><i/><i/><span>⌁</span></view>
        <strong>扫描网关绑定二维码</strong>
        <text>二维码位于网关铭牌或交付单，扫码只读取 SN 和一次性绑定码。</text>
        <button class="primary-button scan-button" :loading="scanning" :disabled="scanning||verifying" @click="scan">{{scanning?'正在打开相机':'开始扫码'}}</button>
      </view>
      <view class="divider"><i/><text>或手动输入</text><i/></view>
      <label class="field"><text>网关 SN</text><input v-model="form.sn" maxlength="32" placeholder="例如 WK20269F899BC7" @input="form.sn=form.sn.toUpperCase()"/></label>
      <view v-if="form.nfcToken" class="nfc-recognized"><strong>NFC 网关已识别</strong><text>已安全读取 {{form.sn}} 的碰一碰凭证</text></view>
      <label v-else class="field"><text>6 位绑定码</text><input v-model="form.bindCode" type="number" maxlength="6" placeholder="请输入交付绑定码"/></label>
      <text v-if="error" class="form-error">{{error}}</text>
      <button class="secondary-button verify-button" :loading="verifying" :disabled="scanning||verifying" @click="verify">校验网关</button>
      <text class="privacy-note">相机仅用于识别二维码，照片不会保存或上传。</text>
    </view>

    <view v-else-if="step===1&&verification" class="card bind-card">
      <view class="verified-head">
        <view class="check">✓</view>
        <view><strong>网关校验通过</strong><text>{{verification.sn}}</text></view>
        <span :class="verification.online?'online':'offline'">{{verification.online?'在线':'当前离线'}}</span>
      </view>
      <view class="summary">
        <view><text>生产批次</text><strong>{{verification.batchNo||'未记录'}}</strong></view>
        <view><text>出厂时间</text><strong>{{formatTime(verification.producedAt)}}</strong></view>
        <view><text>最近上线</text><strong>{{formatTime(verification.lastSeenAt)}}</strong></view>
      </view>
      <label class="field"><text>网关名称</text><input v-model="form.name" maxlength="64" placeholder="请输入便于识别的名称"/></label>
      <label class="field"><text>安装位置</text><input v-model="form.location" maxlength="128" placeholder="选填"/></label>
      <text v-if="!verification.online" class="offline-tip">网关当前离线也可以绑定，上线后会自动更新状态。</text>
      <text v-if="error" class="form-error">{{error}}</text>
      <view class="actions"><button class="secondary-button" @click="editCredential">重新识别</button><button class="primary-button" :loading="binding" :disabled="binding" @click="confirmBind">确认绑定</button></view>
    </view>

    <view v-else class="card success-card">
      <view class="success-mark">✓</view>
      <strong>网关绑定成功</strong>
      <text>{{boundDevice?.name}} 已加入当前租户</text>
      <view class="bound-info"><text>设备编号</text><code>{{boundDevice?.deviceKey}}</code></view>
      <button class="primary-button" @click="toDetail">查看网关详情</button>
      <button class="secondary-button" @click="toList">返回设备列表</button>
    </view>
  </view>
</template>

<style scoped lang="scss">
.bind-page{padding-top:24rpx}.progress{display:grid;grid-template-columns:auto 1fr auto 1fr auto;align-items:start;margin:2rpx 20rpx 28rpx}.progress>view{display:grid;justify-items:center;gap:8rpx;color:#94a3b8;font-size:20rpx}.progress i{display:grid;width:46rpx;height:46rpx;place-items:center;border-radius:50%;background:#dce3ec;color:#fff;font-style:normal;font-weight:800}.progress b{height:4rpx;margin:21rpx 10rpx 0;background:#dce3ec}.progress .active i,.progress b.active{background:#1677ff}.progress .active text{color:#334155}.progress .done i{background:#16a34a}.bind-card{padding:30rpx}.scan-hero{display:grid;justify-items:center;text-align:center}.scan-hero>strong{margin-top:22rpx;font-size:34rpx}.scan-hero>text{max-width:590rpx;margin-top:10rpx;color:#7a8799;font-size:23rpx;line-height:1.65}.scan-icon{position:relative;width:132rpx;height:132rpx;border-radius:30rpx;background:#edf5ff}.scan-icon i{position:absolute;width:36rpx;height:36rpx;border-color:#1677ff;border-style:solid}.scan-icon i:nth-child(1){top:20rpx;left:20rpx;border-width:6rpx 0 0 6rpx;border-radius:8rpx 0 0}.scan-icon i:nth-child(2){top:20rpx;right:20rpx;border-width:6rpx 6rpx 0 0;border-radius:0 8rpx 0 0}.scan-icon i:nth-child(3){bottom:20rpx;left:20rpx;border-width:0 0 6rpx 6rpx;border-radius:0 0 0 8rpx}.scan-icon i:nth-child(4){right:20rpx;bottom:20rpx;border-width:0 6rpx 6rpx 0;border-radius:0 0 8rpx}.scan-icon span{position:absolute;inset:0;display:grid;place-items:center;color:#1677ff;font-size:44rpx}.scan-button{width:100%;margin-top:24rpx}.divider{display:grid;grid-template-columns:1fr auto 1fr;align-items:center;gap:16rpx;margin:28rpx 0 6rpx}.divider i{height:1rpx;background:#e2e8f0}.divider text{color:#94a3b8;font-size:21rpx}.verify-button{width:100%;margin-top:26rpx}.privacy-note{display:block;margin-top:16rpx;color:#94a3b8;font-size:20rpx;text-align:center}.form-error,.offline-tip{display:block;margin-top:20rpx;padding:16rpx 18rpx;border-radius:14rpx;font-size:22rpx;line-height:1.55}.form-error{color:#b4232b;background:#fff0f1}.offline-tip{color:#52647d;background:#f1f5f9}.verified-head{display:flex;align-items:center;gap:16rpx;padding:20rpx;border:1rpx solid #bbf7d0;border-radius:20rpx;background:#f0fdf4}.verified-head .check{display:grid;flex:0 0 64rpx;width:64rpx;height:64rpx;place-items:center;border-radius:50%;color:#fff;background:#16a34a;font-size:34rpx;font-weight:900}.verified-head>view:nth-child(2){display:grid;gap:5rpx;min-width:0}.verified-head strong{font-size:27rpx}.verified-head text{overflow:hidden;color:#15803d;font:22rpx Consolas,monospace;text-overflow:ellipsis}.verified-head span{margin-left:auto;padding:8rpx 14rpx;border-radius:999rpx;font-size:20rpx;white-space:nowrap}.verified-head span.online{color:#15803d;background:#dcfce7}.verified-head span.offline{color:#64748b;background:#e2e8f0}.summary{display:grid;gap:10rpx;margin:18rpx 0}.summary>view{display:flex;justify-content:space-between;gap:20rpx;padding:18rpx;border:1rpx solid #e5eaf1;border-radius:14rpx}.summary text{color:#94a3b8;font-size:21rpx}.summary strong{color:#334155;font-size:22rpx;text-align:right}.actions{display:grid;grid-template-columns:1fr 1.45fr;gap:14rpx;margin-top:28rpx}.actions button{width:100%}.success-card{display:grid;justify-items:center;gap:16rpx;text-align:center}.success-mark{display:grid;width:100rpx;height:100rpx;place-items:center;border-radius:50%;color:#fff;background:#16a34a;font-size:56rpx;font-weight:900}.success-card>strong{font-size:38rpx}.success-card>text{color:#64748b;font-size:24rpx}.success-card>button{width:100%}.bound-info{display:grid;grid-template-columns:auto minmax(0,1fr);gap:18rpx;width:100%;margin:18rpx 0;padding:20rpx;border-radius:16rpx;background:#f4f7fb;text-align:left}.bound-info text{color:#7a8799;font-size:22rpx}.bound-info code{overflow:hidden;text-align:right;text-overflow:ellipsis;white-space:nowrap}
.nfc-recognized{display:grid;gap:6rpx;margin-top:20rpx;padding:18rpx 20rpx;border:1rpx solid #bfdbfe;border-radius:16rpx;background:#eff6ff}.nfc-recognized strong{color:#0b63ce;font-size:25rpx}.nfc-recognized text{color:#52647d;font-size:21rpx}
.nfc-feedback{position:fixed;z-index:120;inset:0;display:grid;place-items:center;padding:44rpx;background:rgba(15,23,42,.42);backdrop-filter:blur(10rpx)}.nfc-feedback-card{display:grid;justify-items:center;gap:14rpx;width:100%;max-width:560rpx;padding:44rpx 32rpx 38rpx;border-radius:28rpx;background:#fff;text-align:center;box-shadow:0 28rpx 80rpx rgba(15,23,42,.24);animation:nfc-card-in .24s ease-out}.nfc-feedback-card>strong{font-size:31rpx}.nfc-feedback-card>text{color:#718198;font-size:22rpx}.nfc-signal{position:relative;display:grid;width:142rpx;height:142rpx;margin-bottom:8rpx;place-items:center}.nfc-signal i{position:absolute;border:3rpx solid rgba(22,119,255,.42);border-radius:50%;animation:nfc-read-wave 1.35s ease-out infinite}.nfc-signal i:nth-child(1){width:72rpx;height:72rpx}.nfc-signal i:nth-child(2){width:104rpx;height:104rpx;animation-delay:.22s}.nfc-signal i:nth-child(3){width:136rpx;height:136rpx;animation-delay:.44s}.nfc-signal strong{z-index:1;display:grid;width:68rpx;height:68rpx;place-items:center;border-radius:50%;color:#fff;background:#1677ff;font-size:21rpx;box-shadow:0 10rpx 28rpx rgba(22,119,255,.28)}.nfc-feedback.success .nfc-signal i{border-color:rgba(22,163,74,.32);animation:nfc-success-wave .55s ease-out both}.nfc-feedback.success .nfc-signal strong{background:#16a34a;font-size:36rpx;animation:nfc-success-pop .42s ease-out}.nfc-feedback.success .nfc-feedback-card>strong{color:#087a3e}@keyframes nfc-card-in{from{opacity:0;transform:translateY(18rpx) scale(.96)}to{opacity:1;transform:none}}@keyframes nfc-read-wave{0%{opacity:.9;transform:scale(.7)}75%,100%{opacity:0;transform:scale(1)}}@keyframes nfc-success-wave{from{opacity:.75;transform:scale(.74)}to{opacity:0;transform:scale(1.08)}}@keyframes nfc-success-pop{0%{transform:scale(.72)}65%{transform:scale(1.1)}100%{transform:scale(1)}}
</style>
