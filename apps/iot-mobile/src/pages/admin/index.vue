<script setup lang="ts">
import { computed, ref } from 'vue'
import { onPullDownRefresh, onShow } from '@dcloudio/uni-app'
import { createLabelPrintJob, devices, factoryGateways, gatewayNfcWriteHistory, labelPrintAgents, prepareGatewayNfcCredential, reclaimFactoryGateway, recordGatewayNfcWrite, removeFactoryGateway, resetGatewayNfcCredential, tenants } from '../../services/cloud'
import type { LabelPrintAgent } from '../../services/cloud'
import { cancelGatewayNfcWrite, nfcAvailable, nfcHapticFeedback, prepareGatewayNfcWrite, writeGatewayNfcTag } from '../../services/gateway-binding'
import { clearSession, currentUser } from '../../services/session'
import type { Device, FactoryGateway, Tenant } from '../../types'

type AdminTab='devices'|'gateways'|'tenants'|'profile'
const tab=ref<AdminTab>('devices')
const deviceItems=ref<Device[]>([])
const gatewayItems=ref<FactoryGateway[]>([])
const tenantItems=ref<Tenant[]>([])
const deviceTotal=ref(0)
const gatewayTotal=ref(0)
const deviceStats=ref({total:0,online:0,offline:0,disabled:0})
const gatewayStats=ref({total:0,bound:0,unbound:0})
const keyword=ref('')
const loading=ref(false)
const error=ref('')
const nfcWritingItem=ref<FactoryGateway|null>(null)
const nfcConfirmItem=ref<FactoryGateway|null>(null)
const nfcResult=ref<{type:'success'|'error';title:string;message:string}|null>(null)
const nfcHistoryVisible=ref(false)
const nfcHistoryLoading=ref(false)
const nfcHistoryItems=ref<Array<{id:string;username:string;action:string;targetName?:string;detail?:{sn?:string;success?:boolean;message?:string};createdAt:string}>>([])
const gatewayAction=ref<{type:'remove'|'reclaim';item:FactoryGateway}|null>(null)
const gatewayActionBusy=ref(false)
const printGatewayItem=ref<FactoryGateway|null>(null)
const printAgents=ref<LabelPrintAgent[]>([])
const printAgentsLoading=ref(false)
const printAgentsError=ref('')
const selectedPrintAgentId=ref('')
const printSize=ref('70x50')
const printCopies=ref(1)
const printSubmitting=ref(false)
const openGatewayId=ref('')
const draggingGatewayId=ref('')
const swipeOffset=ref(0)
let swipeStartX=0
let swipeStartY=0
const gatewayRevealWidth=88
const user=currentUser()

const tenantUserTotal=computed(()=>tenantItems.value.reduce((sum,item)=>sum+(item._count?.users||0),0))
const tenantDeviceTotal=computed(()=>tenantItems.value.reduce((sum,item)=>sum+(item._count?.devices||0),0))
const searchPlaceholder=computed(()=>tab.value==='devices'?'搜索设备名称或编号':tab.value==='gateways'?'搜索网关 SN 或硬件编号':'搜索租户名称')
const sectionTitle=computed(()=>tab.value==='devices'?'设备管理':tab.value==='gateways'?'出厂网关':'租户管理')
const sectionCount=computed(()=>tab.value==='devices'?deviceTotal.value:tab.value==='gateways'?gatewayTotal.value:tenantItems.value.length)
const visibleTenants=computed(()=>{
  const value=keyword.value.trim().toLowerCase()
  if(!value)return tenantItems.value
  return tenantItems.value.filter(item=>item.name.toLowerCase().includes(value)||item.id.toLowerCase().includes(value))
})

function statusText(value:string){return value==='ONLINE'?'在线':value==='BOUND'?'已绑定':value==='UNBOUND'?'未绑定':value||'未知'}
function formatTime(value?:string){if(!value)return'暂无';const date=new Date(value);return Number.isNaN(date.getTime())?'暂无':date.toLocaleString()}
function openDevice(item:Device){uni.navigateTo({url:`/pages/devices/detail?id=${encodeURIComponent(item.id)}`})}
function selectTab(value:AdminTab){
  if(tab.value===value)return
  tab.value=value
  keyword.value=''
  uni.pageScrollTo({scrollTop:0,duration:120})
}
function canWriteNfc(item:FactoryGateway){return !item.boundDevice&&item.status!=='BOUND'}
function touchPoint(event:any){return event.touches?.[0]||event.changedTouches?.[0]}
function gatewaySwipeStyle(item:FactoryGateway){
  const offset=draggingGatewayId.value===item.id?swipeOffset.value:openGatewayId.value===item.id?-gatewayRevealWidth:0
  return {transform:`translate3d(${offset}px,0,0)`,transition:draggingGatewayId.value===item.id?'none':''}
}
function startGatewaySwipe(event:any,item:FactoryGateway){
  if(!canWriteNfc(item))return
  const point=touchPoint(event)
  if(!point)return
  swipeStartX=point.clientX
  swipeStartY=point.clientY
  draggingGatewayId.value=item.id
  swipeOffset.value=openGatewayId.value===item.id?-gatewayRevealWidth:0
  if(openGatewayId.value&&openGatewayId.value!==item.id)openGatewayId.value=''
}
function moveGatewaySwipe(event:any,item:FactoryGateway){
  if(draggingGatewayId.value!==item.id)return
  const point=touchPoint(event)
  if(!point)return
  const deltaX=point.clientX-swipeStartX
  const deltaY=point.clientY-swipeStartY
  if(Math.abs(deltaY)>Math.abs(deltaX))return
  if(event.cancelable)event.preventDefault()
  const base=openGatewayId.value===item.id?-gatewayRevealWidth:0
  swipeOffset.value=Math.max(-gatewayRevealWidth,Math.min(0,base+deltaX))
}
function endGatewaySwipe(item:FactoryGateway){
  if(draggingGatewayId.value!==item.id)return
  openGatewayId.value=swipeOffset.value<-44?item.id:''
  draggingGatewayId.value=''
  swipeOffset.value=0
}
function requestNfcWrite(item:FactoryGateway){
  if(item.boundDevice||item.status==='BOUND'){uni.showToast({title:'已绑定网关不能写入绑定标签',icon:'none'});return}
  if(!nfcAvailable()){
    uni.showModal({title:'无法使用 NFC',content:'请在支持 NFC 的安卓手机上开启 NFC 后，使用物联云管理端重试。',showCancel:false})
    return
  }
  prepareGatewayNfcWrite()
  openGatewayId.value=''
  nfcConfirmItem.value=item
}
function cancelNfcConfirm(){cancelGatewayNfcWrite();nfcConfirmItem.value=null}
function confirmLegacyNfcReset(){
  return new Promise<boolean>((resolve)=>{
    uni.showModal({
      title:'旧版 NFC 凭证无法恢复',
      content:'平台无法还原升级前的 NFC Token。生成新凭证后，已经写入的旧 NFC 标签会失效；如果旧标签仍在使用，请选择取消。',
      confirmText:'重置并写入',
      cancelText:'保留旧标签',
      success(result){resolve(Boolean(result.confirm))},
      fail(){resolve(false)},
    })
  })
}
async function credentialForNfcWrite(item:FactoryGateway){
  try{return await prepareGatewayNfcCredential(item.id)}
  catch(reason){
    const message=reason instanceof Error?reason.message:''
    if(!message.includes('旧版 NFC 凭证')&&!message.includes('无法恢复'))throw reason
    if(!await confirmLegacyNfcReset())throw new Error('已取消 NFC 写入')
    return resetGatewayNfcCredential(item.id)
  }
}
async function confirmNfcWrite(){
  const item=nfcConfirmItem.value
  if(!item)return
  nfcConfirmItem.value=null
  try{
    const credential=await credentialForNfcWrite(item)
    nfcWritingItem.value=item
    await writeGatewayNfcTag(credential.payload)
    nfcWritingItem.value=null
    void recordGatewayNfcWrite(item.id,{sn:item.sn,success:true}).catch(()=>undefined)
    nfcHapticFeedback('success')
    nfcResult.value={type:'success',title:'写入成功',message:`标签已写入网关 ${item.sn}，现在可以使用物联云租户端碰一碰识别并绑定。`}
  }catch(reason){
    cancelGatewayNfcWrite()
    nfcWritingItem.value=null
    const message=reason instanceof Error?reason.message:'NFC 写入失败'
    if(message!=='已取消 NFC 写入'){
      void recordGatewayNfcWrite(item.id,{sn:item.sn,success:false,message}).catch(()=>undefined)
      nfcHapticFeedback('error');nfcResult.value={type:'error',title:'写入失败',message}
    }
  }
}
async function openNfcHistory(){
  nfcHistoryVisible.value=true
  nfcHistoryLoading.value=true
  try{nfcHistoryItems.value=(await gatewayNfcWriteHistory(50)).items||[]}
  catch(reason){uni.showToast({title:reason instanceof Error?reason.message:'写入历史读取失败',icon:'none'})}
  finally{nfcHistoryLoading.value=false}
}
function closeNfcHistory(){nfcHistoryVisible.value=false}
function requestGatewayAction(item:FactoryGateway){gatewayAction.value={type:item.boundDevice?'reclaim':'remove',item}}
async function requestCloudPrint(item:FactoryGateway){
  if(item.boundDevice||item.status==='BOUND'){uni.showToast({title:'已绑定网关不能打印绑定标签',icon:'none'});return}
  // Open first so a slow LAN request never makes the tap appear unresponsive.
  printGatewayItem.value=item
  printAgents.value=[]
  selectedPrintAgentId.value=''
  printAgentsError.value=''
  printAgentsLoading.value=true
  try{
    printAgents.value=await labelPrintAgents()
    if(!printAgents.value.length){
      printAgentsError.value='暂无打印终端，请先在 PC 管理平台配置连接 Godex G500 的 Windows 打印服务。'
      return
    }
    selectedPrintAgentId.value=printAgents.value.find(agent=>agent.online)?.id||printAgents.value[0].id
  }catch(reason){
    printAgentsError.value=reason instanceof Error?reason.message:'打印终端读取失败'
  }finally{
    printAgentsLoading.value=false
  }
}
function cancelCloudPrint(){if(!printSubmitting.value)printGatewayItem.value=null}
async function confirmCloudPrint(){
  if(!printGatewayItem.value||!selectedPrintAgentId.value||printSubmitting.value)return
  printSubmitting.value=true
  try{
    await createLabelPrintJob({agentId:selectedPrintAgentId.value,factoryGatewayId:printGatewayItem.value.id,labelSize:printSize.value,copies:printCopies.value})
    printGatewayItem.value=null
    uni.showToast({title:'打印任务已提交',icon:'success'})
  }catch(reason){uni.showToast({title:reason instanceof Error?reason.message:'打印任务提交失败',icon:'none'})}
  finally{printSubmitting.value=false}
}
function cancelGatewayAction(){if(!gatewayActionBusy.value)gatewayAction.value=null}
async function confirmGatewayAction(){
  const action=gatewayAction.value
  if(!action||gatewayActionBusy.value)return
  gatewayActionBusy.value=true
  try{
    const result=action.type==='reclaim'?await reclaimFactoryGateway(action.item.id):await removeFactoryGateway(action.item.id)
    gatewayAction.value=null
    uni.showToast({title:result.message,icon:'none',duration:2200})
    await refresh()
  }catch(reason){uni.showToast({title:reason instanceof Error?reason.message:'操作失败',icon:'none'})}
  finally{gatewayActionBusy.value=false}
}
function closeNfcResult(){nfcResult.value=null}
function cancelNfc(){cancelGatewayNfcWrite();nfcWritingItem.value=null}
async function refresh(){
  loading.value=true
  try{
    const [deviceData,gatewayData,tenantData]=await Promise.all([
      devices({page:1,pageSize:200,keyword:tab.value==='devices'?keyword.value.trim():''}),
      factoryGateways({page:1,pageSize:200,keyword:tab.value==='gateways'?keyword.value.trim():''}),
      tenants(),
    ])
    deviceItems.value=deviceData.items||[]
    deviceTotal.value=deviceData.total||0
    deviceStats.value=deviceData.stats||{
      total:deviceData.total||0,
      online:deviceItems.value.filter(item=>item.status==='ONLINE').length,
      offline:deviceItems.value.filter(item=>item.status==='OFFLINE').length,
      disabled:deviceItems.value.filter(item=>item.status==='DISABLED').length,
    }
    gatewayItems.value=gatewayData.items||[]
    gatewayTotal.value=gatewayData.total||0
    gatewayStats.value=gatewayData.stats||{
      total:gatewayData.total||0,
      bound:gatewayItems.value.filter(item=>item.boundDevice||item.boundTenant).length,
      unbound:gatewayItems.value.filter(item=>!item.boundDevice&&!item.boundTenant).length,
    }
    tenantItems.value=tenantData||[]
    error.value=''
  }catch(reason){error.value=reason instanceof Error?reason.message:'管理数据读取失败'}
  finally{loading.value=false;uni.stopPullDownRefresh()}
}
function logout(){
  uni.showModal({title:'退出登录',content:'确认退出管理端？',success(result){if(!result.confirm)return;clearSession();uni.reLaunch({url:'/pages/login/index'})}})
}
onShow(()=>void refresh())
onPullDownRefresh(()=>void refresh())
</script>

<template>
  <view class="page admin-page">
    <view v-if="tab==='devices'" class="summary-grid">
      <view class="summary-card"><text>设备总数</text><strong>{{deviceStats.total}}</strong></view>
      <view class="summary-card"><text>在线设备</text><strong class="green">{{deviceStats.online}}</strong></view>
      <view class="summary-card"><text>离线设备</text><strong>{{deviceStats.offline}}</strong></view>
    </view>
    <view v-else-if="tab==='gateways'" class="summary-grid">
      <view class="summary-card"><text>网关总数</text><strong>{{gatewayStats.total}}</strong></view>
      <view class="summary-card"><text>已绑定</text><strong class="green">{{gatewayStats.bound}}</strong></view>
      <view class="summary-card"><text>待绑定</text><strong>{{gatewayStats.unbound}}</strong></view>
    </view>
    <view v-else-if="tab==='tenants'" class="summary-grid">
      <view class="summary-card"><text>租户总数</text><strong>{{tenantItems.length}}</strong></view>
      <view class="summary-card"><text>租户账号</text><strong>{{tenantUserTotal}}</strong></view>
      <view class="summary-card"><text>名下设备</text><strong>{{tenantDeviceTotal}}</strong></view>
    </view>
    <view v-if="tab!=='profile'" class="admin-tools">
      <view class="search"><input v-model="keyword" confirm-type="search" :placeholder="searchPlaceholder" @confirm="refresh"/></view>
      <button @click="refresh">{{loading?'...':'刷新'}}</button>
    </view>
    <view v-if="tab!=='profile'" class="section-heading">
      <view class="section-title-row">
        <view><strong>{{sectionTitle}}</strong><text>{{sectionCount}} 项</text></view>
        <button v-if="tab==='gateways'" class="history-button" @click="openNfcHistory">
          <view class="history-clock"><i/></view><text>写入历史</text>
        </button>
      </view>
      <text v-if="tab==='gateways'">管理出厂登记、租户归属与 NFC 标签</text>
      <text v-else-if="tab==='devices'">查看全部租户的设备与在线状态</text>
      <text v-else>查看租户及其账号、设备数量</text>
    </view>
    <text v-if="error" class="error-banner">{{error}}</text>

    <view v-if="tab==='devices'" class="list">
      <view v-for="item in deviceItems" :key="item.id" class="row-card" @click="openDevice(item)">
        <view class="row-main"><strong>{{item.name}}</strong><code>{{item.deviceKey}}</code></view>
        <view class="row-side"><text>{{item.tenant?.name||'未标租户'}}</text><view class="pill" :class="{online:item.status==='ONLINE'}">{{statusText(item.status)}}</view></view>
      </view>
      <view v-if="!deviceItems.length&&!loading" class="empty">暂无设备</view>
    </view>

    <view v-else-if="tab==='gateways'" class="list">
      <view v-for="item in gatewayItems" :key="item.id" class="gateway-swipe-shell" :class="{opened:openGatewayId===item.id,revealing:draggingGatewayId===item.id&&swipeOffset<-10}">
        <view v-if="canWriteNfc(item)" class="nfc-swipe-action" @click.stop="requestNfcWrite(item)">
          <view class="mini-nfc"><i/><i/><strong>N</strong></view>
          <text>写入 NFC</text>
        </view>
        <view
          class="row-card gateway-card"
          :style="gatewaySwipeStyle(item)"
          @touchstart="startGatewaySwipe($event,item)"
          @touchmove="moveGatewaySwipe($event,item)"
          @touchend="endGatewaySwipe(item)"
          @touchcancel="endGatewaySwipe(item)"
        >
          <view class="row-main"><strong>{{item.sn}}</strong><code>{{item.hardwareId}}</code></view>
          <view class="binding-state" :class="{bound:!!item.boundTenant}">{{item.boundTenant?'已绑定':'待绑定'}}</view>
          <view class="gateway-details">
            <view class="detail-row tenant-row">
              <text>归属租户</text>
              <strong :class="{unbound:!item.boundTenant}">{{item.boundTenant?.name||'尚未绑定租户'}}</strong>
            </view>
            <view class="detail-row"><text>关联设备</text><strong>{{item.boundDevice?.name||'尚未生成设备'}}</strong></view>
            <view class="detail-row"><text>最近心跳</text><strong>{{formatTime(item.lastSeenAt)}}</strong></view>
          </view>
          <view class="gateway-actions" @touchstart.stop @touchmove.stop @touchend.stop>
            <button v-if="canWriteNfc(item)" class="print-action" @tap.stop="requestCloudPrint(item)">打印标签</button>
            <button :class="{danger:!item.boundDevice}" @tap.stop="requestGatewayAction(item)">{{item.boundDevice?'解除绑定并回收':'移除登记'}}</button>
          </view>
        </view>
      </view>
      <view v-if="!gatewayItems.length&&!loading" class="empty">暂无出厂网关</view>
    </view>

    <view v-else-if="tab==='tenants'" class="list">
      <view v-for="item in visibleTenants" :key="item.id" class="row-card">
        <view class="row-main"><strong>{{item.name}}</strong><code>{{item.id}}</code></view>
        <view class="tenant-count"><text>{{item._count?.users||0}} 账号</text><text>{{item._count?.devices||0}} 设备</text></view>
      </view>
      <view v-if="!visibleTenants.length&&!loading" class="empty">暂无租户</view>
    </view>
    <view v-else class="profile-page">
      <view class="admin-hero">
        <view>
          <text>物联云管理端</text>
          <strong>{{user?.nickname||user?.username||'管理员'}}</strong>
          <small>超级管理员</small>
        </view>
        <view class="admin-avatar">管</view>
      </view>
      <view class="profile-card">
        <view class="profile-title"><strong>账号信息</strong><text>当前管理端登录信息</text></view>
        <view class="profile-row"><text>账号</text><strong>{{user?.username||'--'}}</strong></view>
        <view class="profile-row"><text>身份</text><strong>超级管理员</strong></view>
        <view class="profile-row"><text>管理范围</text><strong>全部租户与设备</strong></view>
      </view>
      <view class="profile-card profile-overview">
        <view><strong>{{tenantItems.length}}</strong><text>租户</text></view>
        <view><strong>{{deviceTotal}}</strong><text>设备</text></view>
        <view><strong>{{gatewayTotal}}</strong><text>网关</text></view>
      </view>
      <button class="logout-button" @click="logout">退出当前账号</button>
    </view>
    <view class="admin-bottom-nav">
      <view :class="{active:tab==='devices'}" @click="selectTab('devices')"><strong>▦</strong><text>设备</text></view>
      <view :class="{active:tab==='gateways'}" @click="selectTab('gateways')"><strong>◇</strong><text>出厂网关</text></view>
      <view :class="{active:tab==='tenants'}" @click="selectTab('tenants')"><strong>♙</strong><text>租户</text></view>
      <view :class="{active:tab==='profile'}" @click="selectTab('profile')"><strong>◎</strong><text>个人中心</text></view>
    </view>
    <view v-if="nfcConfirmItem" class="nfc-mask confirm-mask" @click.self="cancelNfcConfirm">
      <view class="nfc-confirm-dialog">
        <view class="confirm-nfc-icon"><i/><i/><i/><strong>NFC</strong></view>
        <view class="confirm-copy">
          <strong>写入 NFC 标签</strong>
          <text>即将把网关 {{nfcConfirmItem.sn}} 的碰一碰凭证写入标签</text>
        </view>
        <view class="confirm-notice"><strong>凭证可重复使用</strong><text>重复写入会继续使用原凭证，不会让已贴设备的 NFC 标签失效；只有明确执行凭证重置时旧标签才会失效。</text></view>
        <view class="confirm-actions">
          <button @click="cancelNfcConfirm">取消</button>
          <button class="primary" @click="confirmNfcWrite">开始写入</button>
        </view>
      </view>
    </view>
    <view v-if="nfcResult" class="nfc-mask result-mask" @click.self="closeNfcResult">
      <view class="nfc-result-dialog" :class="nfcResult.type">
        <view class="result-icon"><i/><strong>{{nfcResult.type==='success'?'✓':'!'}}</strong></view>
        <strong>{{nfcResult.title}}</strong>
        <text>{{nfcResult.message}}</text>
        <button @click="closeNfcResult">知道了</button>
      </view>
    </view>
    <view v-if="nfcWritingItem" class="nfc-mask">
      <view class="nfc-dialog">
        <view class="nfc-waves"><i/><i/><i/><strong>NFC</strong></view>
        <strong>请将空白 NFC 标签贴近手机背部</strong>
        <text>正在写入 {{nfcWritingItem.sn}}，请保持标签不动，直到提示写入成功。</text>
        <button @click="cancelNfc">取消写入</button>
      </view>
    </view>
    <view v-if="nfcHistoryVisible" class="nfc-mask history-mask" @click.self="closeNfcHistory">
      <view class="nfc-history-dialog">
        <view class="history-head">
          <view class="history-head-main">
            <view class="history-head-icon"><view class="history-clock large"><i/></view></view>
            <view><strong>NFC 写入历史</strong><text>最近 50 条写卡记录</text></view>
          </view>
          <button @click="closeNfcHistory">×</button>
        </view>
        <scroll-view class="history-list" scroll-y>
          <view v-if="nfcHistoryLoading" class="history-empty">正在读取...</view>
          <template v-else>
            <view v-for="item in nfcHistoryItems" :key="item.id" class="history-item">
              <view class="history-status" :class="{success:item.detail?.success===true,legacy:item.detail?.success===undefined}">{{item.detail?.success===true?'✓':item.detail?.success===false?'!':'·'}}</view>
              <view class="history-copy"><strong>{{item.detail?.sn||item.targetName||'未知网关'}}</strong><text>{{item.detail?.success===true?'写入成功':item.detail?.success===false?(item.detail?.message||'写入失败'):'凭证已生成（旧记录，写入结果未知）'}}</text><small>{{formatTime(item.createdAt)}} · {{item.username||'管理员'}}</small></view>
            </view>
          </template>
          <view v-if="!nfcHistoryLoading&&!nfcHistoryItems.length" class="history-empty">暂无 NFC 写入记录</view>
        </scroll-view>
      </view>
    </view>
    <view v-if="gatewayAction" class="nfc-mask action-mask" @click.self="cancelGatewayAction">
      <view class="gateway-action-dialog">
        <view class="action-warning">!</view>
        <view class="action-copy">
          <strong>{{gatewayAction.type==='reclaim'?'解除绑定并回收网关':'移除出厂网关'}}</strong>
          <text v-if="gatewayAction.type==='reclaim'">将解除 {{gatewayAction.item.sn}} 与租户“{{gatewayAction.item.boundTenant?.name||'当前租户'}}”的绑定，并删除对应的云端网关设备。HardwareId、SN 和 NFC 凭证仍会保留，之后可重新绑定。</text>
          <text v-else>将从当前列表移除 {{gatewayAction.item.sn}}。HardwareId 与 SN 档案会保留，旧 NFC 凭证将失效；再次登记相同 HardwareId 时仍恢复原 SN。</text>
        </view>
        <view class="action-note">该操作会写入系统审计日志。</view>
        <view class="confirm-actions"><button :disabled="gatewayActionBusy" @click="cancelGatewayAction">取消</button><button class="danger-confirm" :disabled="gatewayActionBusy" @click="confirmGatewayAction">{{gatewayActionBusy?'处理中...':'确认操作'}}</button></view>
      </view>
    </view>
    <view v-if="printGatewayItem" class="nfc-mask action-mask" @click.self="cancelCloudPrint">
      <view class="gateway-action-dialog print-job-dialog">
        <view class="print-dialog-icon">▣</view>
        <view class="action-copy"><strong>打印网关标签</strong><text>{{printGatewayItem.sn}}</text></view>
        <view class="print-field"><text>打印终端</text><picker :disabled="printAgentsLoading||!printAgents.length" :range="printAgents" range-key="name" :value="Math.max(0,printAgents.findIndex(item=>item.id===selectedPrintAgentId))" @change="selectedPrintAgentId=printAgents[Number($event.detail.value)]?.id||''"><view>{{printAgentsLoading?'正在读取终端…':(printAgents.find(item=>item.id===selectedPrintAgentId)?.name||'暂无可用终端')}}</view></picker></view>
        <view v-if="printAgentsError" class="print-error">{{printAgentsError}}</view>
        <view class="print-field"><text>标签规格</text><picker :range="['50x30','60x40','70x50']" :value="['50x30','60x40','70x50'].indexOf(printSize)" @change="printSize=['50x30','60x40','70x50'][Number($event.detail.value)]"><view>{{printSize}} mm</view></picker></view>
        <view class="print-field"><text>打印份数</text><view class="copy-stepper"><button @click="printCopies=Math.max(1,printCopies-1)">−</button><strong>{{printCopies}}</strong><button @click="printCopies=Math.min(20,printCopies+1)">＋</button></view></view>
        <view class="confirm-actions"><button @tap="cancelCloudPrint">取消</button><button class="primary" :disabled="printSubmitting||printAgentsLoading||!selectedPrintAgentId" @tap="confirmCloudPrint">{{printSubmitting?'提交中…':(printAgentsLoading?'读取终端中…':'发送到打印机')}}</button></view>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
.admin-page{display:grid;align-content:start;gap:18rpx;padding-top:20rpx;padding-bottom:calc(150rpx + env(safe-area-inset-bottom))}.admin-hero{display:flex;align-items:center;justify-content:space-between;padding:30rpx;border-radius:26rpx;color:#fff;background:linear-gradient(135deg,#1266dc,#13a1f3);box-shadow:0 16rpx 38rpx rgba(22,119,255,.2)}.admin-hero>view:first-child{display:grid;gap:6rpx}.admin-hero text{font-size:22rpx;opacity:.82}.admin-hero strong{font-size:36rpx}.admin-hero small{font-size:20rpx;opacity:.75}.admin-avatar{display:grid;width:82rpx;height:82rpx;border:1rpx solid rgba(255,255,255,.38);border-radius:24rpx;background:rgba(255,255,255,.18);font-size:30rpx;font-weight:700;place-items:center}.summary-grid{display:grid;grid-template-columns:1fr 1fr;gap:12rpx}.summary-card{display:grid;gap:6rpx;min-height:116rpx;padding:21rpx 23rpx;border:1rpx solid #e1e8f0;border-radius:20rpx;background:#fff;box-shadow:0 7rpx 22rpx rgba(15,23,42,.035)}.summary-card text{color:#7a8799;font-size:21rpx}.summary-card strong{color:#172033;font-size:35rpx}.summary-card .green{color:#0b9448}.summary-card small{color:#8a97a9;font-size:18rpx}.admin-tools{display:grid;grid-template-columns:minmax(0,1fr) 104rpx;gap:10rpx}.search{display:flex;align-items:center;height:72rpx;padding:0 20rpx;border:1rpx solid #dbe3ee;border-radius:17rpx;background:#fff}.search input{flex:1;min-width:0;font-size:24rpx}.admin-tools button{height:72rpx;border:1rpx solid #d9e4f2;border-radius:17rpx;color:#1677ff;background:#fff;font-size:23rpx;line-height:72rpx}.section-heading{display:grid;gap:5rpx;padding:4rpx 2rpx}.section-heading>view{display:flex;align-items:center;gap:12rpx}.section-heading strong{color:#172033;font-size:28rpx}.section-heading view text{padding:4rpx 10rpx;border-radius:999rpx;color:#52708f;background:#eaf2fb;font-size:18rpx}.section-heading>text{color:#8a97a9;font-size:20rpx}.error-banner{display:block;padding:18rpx;border-radius:16rpx;color:#b4232b;background:#fff0f1;font-size:24rpx}.list{display:grid;gap:14rpx}.row-card{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:16rpx;padding:22rpx;border:1rpx solid #e1e8f0;border-radius:21rpx;background:#fff;box-shadow:0 7rpx 24rpx rgba(15,23,42,.035)}.row-main{display:grid;gap:7rpx;min-width:0}.row-main strong,.row-main code{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.row-main strong{font-size:27rpx}.row-main code{color:#8a97a9;font-size:19rpx}.row-side{display:grid;justify-items:end;gap:10rpx;max-width:210rpx}.row-side>text{overflow:hidden;max-width:210rpx;color:#64748b;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.pill,.binding-state{padding:7rpx 14rpx;border-radius:999rpx;color:#64748b;background:#f1f5f9;font-size:20rpx}.pill.online,.binding-state.bound{color:#087a3e;background:#e9f9ef}.gateway-card{padding:24rpx}.gateway-details{grid-column:1/-1;display:grid;border-top:1rpx solid #edf1f6}.detail-row{display:grid;grid-template-columns:130rpx minmax(0,1fr);align-items:center;gap:12rpx;min-height:58rpx;border-bottom:1rpx solid #f0f3f7}.detail-row>text{color:#8a97a9;font-size:20rpx}.detail-row>strong{overflow:hidden;color:#34445b;font-size:21rpx;font-weight:500;text-overflow:ellipsis;white-space:nowrap}.tenant-row>strong{color:#087a3e;font-weight:700}.tenant-row>strong.unbound{color:#8a97a9;font-weight:500}.tenant-count{grid-column:1/-1;display:flex;flex-wrap:wrap;gap:10rpx}.tenant-count text{padding:7rpx 12rpx;border-radius:999rpx;color:#52647d;background:#f4f7fb;font-size:20rpx}.empty{padding:80rpx 20rpx;color:#8a97a9;text-align:center}.profile-page{display:grid;gap:18rpx}.profile-card{display:grid;padding:24rpx;border:1rpx solid #e1e8f0;border-radius:22rpx;background:#fff}.profile-title{display:grid;gap:5rpx;padding-bottom:18rpx;border-bottom:1rpx solid #edf1f6}.profile-title strong{font-size:27rpx}.profile-title text{color:#8a97a9;font-size:20rpx}.profile-row{display:grid;grid-template-columns:150rpx minmax(0,1fr);align-items:center;min-height:72rpx;border-bottom:1rpx solid #f0f3f7}.profile-row:last-child{border-bottom:0}.profile-row text{color:#8a97a9;font-size:22rpx}.profile-row strong{overflow:hidden;color:#34445b;font-size:23rpx;text-align:right;text-overflow:ellipsis;white-space:nowrap}.profile-overview{grid-template-columns:repeat(3,1fr);padding:20rpx 8rpx}.profile-overview>view{display:grid;justify-items:center;gap:5rpx;border-right:1rpx solid #edf1f6}.profile-overview>view:last-child{border-right:0}.profile-overview strong{font-size:30rpx}.profile-overview text{color:#8a97a9;font-size:20rpx}.logout-button{height:76rpx;border:1rpx solid #fecaca;border-radius:18rpx;color:#dc2626;background:#fff;font-size:24rpx;line-height:76rpx}.admin-bottom-nav{position:fixed;z-index:40;right:0;bottom:0;left:0;display:grid;grid-template-columns:repeat(4,1fr);padding:10rpx 10rpx calc(10rpx + env(safe-area-inset-bottom));border-top:1rpx solid #e3e9f1;background:rgba(255,255,255,.96);box-shadow:0 -10rpx 30rpx rgba(15,23,42,.07);backdrop-filter:blur(18rpx)}.admin-bottom-nav>view{display:grid;justify-items:center;gap:2rpx;min-height:74rpx;color:#8a97a9}.admin-bottom-nav strong{height:38rpx;font-size:31rpx;line-height:38rpx}.admin-bottom-nav text{font-size:19rpx}.admin-bottom-nav>view.active{color:#1677ff}.admin-bottom-nav>view.active strong{transform:translateY(-1rpx)}
.nfc-button{grid-column:1/-1;width:100%;height:64rpx;border:1rpx solid #bfdbfe;border-radius:15rpx;color:#0b63ce;background:#eff6ff;font-size:23rpx;line-height:64rpx}.nfc-mask{position:fixed;z-index:99;inset:0;display:grid;place-items:center;padding:40rpx;background:rgba(15,23,42,.52);backdrop-filter:blur(8rpx)}.nfc-dialog{display:grid;justify-items:center;gap:22rpx;width:100%;max-width:620rpx;padding:42rpx 34rpx;border-radius:30rpx;background:#fff;text-align:center;box-shadow:0 32rpx 90rpx rgba(0,0,0,.24)}.nfc-dialog>strong{font-size:31rpx}.nfc-dialog>text{color:#64748b;font-size:23rpx;line-height:1.6}.nfc-dialog>button{width:100%;height:72rpx;border-radius:16rpx;color:#52647d;background:#f1f5f9;font-size:24rpx;line-height:72rpx}.nfc-waves{position:relative;display:grid;width:130rpx;height:130rpx;place-items:center}.nfc-waves i{position:absolute;border:3rpx solid #60a5fa;border-radius:50%;animation:nfc-wave 1.8s ease-out infinite}.nfc-waves i:nth-child(1){width:75rpx;height:75rpx}.nfc-waves i:nth-child(2){width:100rpx;height:100rpx;animation-delay:.35s}.nfc-waves i:nth-child(3){width:126rpx;height:126rpx;animation-delay:.7s}.nfc-waves strong{z-index:1;color:#1677ff;font-size:24rpx}@keyframes nfc-wave{0%{opacity:.95;transform:scale(.72)}70%,100%{opacity:.1;transform:scale(1)}}
.summary-grid{grid-template-columns:repeat(3,minmax(0,1fr));gap:10rpx}.summary-card{min-height:108rpx;padding:20rpx 18rpx}.summary-card text{overflow:hidden;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.summary-card strong{font-size:34rpx}
.binding-state{display:flex;align-items:center;justify-content:center;box-sizing:border-box;min-width:104rpx;min-height:54rpx;padding:0 16rpx;line-height:1;text-align:center}.gateway-swipe-shell{position:relative;overflow:hidden;border-radius:21rpx;background:#eaf4ff;touch-action:pan-y}.gateway-swipe-shell .gateway-card{position:relative;z-index:2;box-sizing:border-box;width:100%;margin:0;transition:transform .32s cubic-bezier(.22,1,.36,1);will-change:transform}.nfc-swipe-action{position:absolute;z-index:1;top:0;right:0;bottom:0;display:flex;width:176rpx;flex-direction:column;align-items:center;justify-content:center;gap:12rpx;color:#fff;background:linear-gradient(145deg,#1284ee,#0b62ce);font-size:21rpx;font-weight:700;box-shadow:inset 10rpx 0 24rpx rgba(3,65,135,.14)}.gateway-swipe-shell.opened .nfc-swipe-action{animation:action-glow 1.8s ease-in-out infinite}.mini-nfc{position:relative;display:grid;width:62rpx;height:62rpx;place-items:center}.mini-nfc i{position:absolute;border:2rpx solid rgba(255,255,255,.72);border-radius:50%}.mini-nfc i:first-child{width:42rpx;height:42rpx;animation:mini-wave 1.5s ease-out infinite}.mini-nfc i:nth-child(2){width:58rpx;height:58rpx;animation:mini-wave 1.5s .35s ease-out infinite}.mini-nfc strong{z-index:1;font-size:22rpx}.confirm-mask{z-index:110;align-items:end;padding:30rpx 24rpx calc(30rpx + env(safe-area-inset-bottom))}.nfc-confirm-dialog{display:grid;gap:24rpx;width:100%;max-width:650rpx;padding:34rpx;border-radius:32rpx;background:#fff;box-shadow:0 34rpx 90rpx rgba(15,23,42,.28);animation:confirm-in .34s cubic-bezier(.22,1,.36,1)}.confirm-nfc-icon{position:relative;display:grid;width:106rpx;height:106rpx;margin:0 auto;place-items:center}.confirm-nfc-icon i{position:absolute;border:3rpx solid #60a5fa;border-radius:50%;animation:confirm-wave 1.9s ease-out infinite}.confirm-nfc-icon i:first-child{width:58rpx;height:58rpx}.confirm-nfc-icon i:nth-child(2){width:80rpx;height:80rpx;animation-delay:.3s}.confirm-nfc-icon i:nth-child(3){width:102rpx;height:102rpx;animation-delay:.6s}.confirm-nfc-icon strong{z-index:1;color:#1677ff;font-size:22rpx}.confirm-copy{display:grid;justify-items:center;gap:10rpx;text-align:center}.confirm-copy strong{color:#172033;font-size:31rpx}.confirm-copy text{color:#64748b;font-size:22rpx;line-height:1.55}.confirm-notice{display:grid;gap:6rpx;padding:18rpx 20rpx;border:1rpx solid #dbeafe;border-radius:17rpx;background:#f2f7ff}.confirm-notice strong{color:#185fae;font-size:21rpx}.confirm-notice text{color:#52647d;font-size:20rpx;line-height:1.5}.confirm-actions{display:grid;grid-template-columns:1fr 1.3fr;gap:12rpx}.confirm-actions button{width:100%;height:72rpx;border-radius:17rpx;color:#52647d;background:#f1f5f9;font-size:23rpx;line-height:72rpx}.confirm-actions button.primary{color:#fff;background:linear-gradient(135deg,#1677ff,#0b63ce);box-shadow:0 10rpx 24rpx rgba(22,119,255,.25)}@keyframes mini-wave{0%{opacity:.95;transform:scale(.65)}80%,100%{opacity:.08;transform:scale(1)}}@keyframes action-glow{0%,100%{background-color:#1284ee}50%{filter:brightness(1.1)}}@keyframes confirm-in{from{opacity:0;transform:translateY(70rpx) scale(.96)}to{opacity:1;transform:none}}@keyframes confirm-wave{0%{opacity:.95;transform:scale(.62)}75%,100%{opacity:.08;transform:scale(1)}}
.confirm-mask{align-items:center;padding:30rpx 24rpx}.nfc-confirm-dialog{max-width:600rpx;animation:confirm-center-in .34s cubic-bezier(.22,1,.36,1)}.result-mask{z-index:120;align-items:center;padding:30rpx 24rpx}.nfc-result-dialog{display:grid;justify-items:center;gap:18rpx;width:100%;max-width:560rpx;padding:42rpx 34rpx 32rpx;border-radius:30rpx;background:#fff;text-align:center;box-shadow:0 34rpx 90rpx rgba(15,23,42,.28);animation:result-in .34s cubic-bezier(.22,1,.36,1)}.nfc-result-dialog>strong{color:#172033;font-size:31rpx}.nfc-result-dialog>text{color:#64748b;font-size:22rpx;line-height:1.6}.nfc-result-dialog>button{width:100%;height:72rpx;margin-top:8rpx;border-radius:17rpx;color:#fff;background:#1677ff;font-size:23rpx;line-height:72rpx}.result-icon{position:relative;display:grid;width:104rpx;height:104rpx;place-items:center}.result-icon i{position:absolute;inset:0;border:4rpx solid #22c55e;border-radius:50%;animation:result-ring .5s cubic-bezier(.22,1,.36,1)}.result-icon strong{z-index:1;color:#16a34a;font-size:48rpx;line-height:1}.nfc-result-dialog.error .result-icon i{border-color:#fb7185}.nfc-result-dialog.error .result-icon strong{color:#e11d48}.nfc-result-dialog.error>button{background:#e5485d}@keyframes confirm-center-in{from{opacity:0;transform:translateY(30rpx) scale(.94)}to{opacity:1;transform:none}}@keyframes result-in{from{opacity:0;transform:scale(.84)}to{opacity:1;transform:scale(1)}}@keyframes result-ring{from{opacity:0;transform:scale(.45)}to{opacity:1;transform:scale(1)}}
.gateway-swipe-shell{background:linear-gradient(90deg,#eef6ff,#dcecff)}.gateway-swipe-shell .gateway-card{transform-origin:left center}.gateway-swipe-shell.opened .gateway-card{box-shadow:14rpx 0 32rpx rgba(15,74,138,.18)}.nfc-swipe-action{top:14rpx;right:10rpx;bottom:14rpx;width:164rpx;border:1rpx solid rgba(255,255,255,.45);border-radius:22rpx;opacity:.12;transform:translate3d(34rpx,0,0) scale(.86);transition:opacity .25s ease,transform .38s cubic-bezier(.22,1,.36,1),box-shadow .3s ease;box-shadow:0 8rpx 18rpx rgba(11,98,206,.12),inset 0 1rpx 0 rgba(255,255,255,.32)}.gateway-swipe-shell.opened .nfc-swipe-action,.gateway-swipe-shell.revealing .nfc-swipe-action{opacity:1;transform:translate3d(0,0,0) scale(1);box-shadow:0 18rpx 38rpx rgba(11,98,206,.3),inset 0 1rpx 0 rgba(255,255,255,.38)}.gateway-swipe-shell.opened .mini-nfc{animation:nfc-hover 2s ease-in-out infinite}@keyframes nfc-hover{0%,100%{transform:translateY(0)}50%{transform:translateY(-5rpx)}}
.gateway-swipe-shell{perspective:1100px;isolation:isolate}.gateway-swipe-shell .gateway-card{z-index:3;transform-origin:left center;backface-visibility:hidden}.gateway-swipe-shell.opened .gateway-card{box-shadow:24rpx 5rpx 44rpx rgba(15,55,102,.24)}.nfc-swipe-action{top:50%;right:12rpx;bottom:auto;display:block;width:260rpx;height:160rpx;overflow:hidden;border:1rpx solid rgba(255,255,255,.55);border-radius:24rpx;opacity:.08;transform:translate3d(60rpx,-50%,0) scale(.76) rotateZ(5deg);background:linear-gradient(145deg,#178af0 0%,#0b63ce 52%,#074a9f 100%);box-shadow:0 10rpx 22rpx rgba(5,72,150,.15),inset 0 1rpx 0 rgba(255,255,255,.45);transition:opacity .28s ease,transform .46s cubic-bezier(.16,1,.3,1),box-shadow .4s ease}.gateway-swipe-shell.opened .nfc-swipe-action,.gateway-swipe-shell.revealing .nfc-swipe-action{opacity:1;transform:translate3d(0,-50%,0) scale(1) rotateZ(0);box-shadow:0 24rpx 48rpx rgba(5,72,150,.34),inset 0 1rpx 0 rgba(255,255,255,.5)}.gateway-swipe-shell.opened .nfc-swipe-action{animation:bank-card-pulse 2.4s ease-in-out infinite}.bank-card-shine{position:absolute;top:-80rpx;right:-30rpx;width:210rpx;height:230rpx;border-radius:50%;background:radial-gradient(circle,rgba(255,255,255,.28),rgba(255,255,255,0) 68%);pointer-events:none}.nfc-bank-chip{position:absolute;top:34rpx;left:26rpx;width:54rpx;height:42rpx;border:1rpx solid rgba(255,255,255,.72);border-radius:9rpx;background:linear-gradient(135deg,#ffe7a3,#d7ac46);box-shadow:inset 0 0 0 2rpx rgba(121,82,15,.13)}.nfc-bank-chip:before,.nfc-bank-chip:after{position:absolute;content:"";background:rgba(112,76,20,.28)}.nfc-bank-chip:before{top:13rpx;right:0;left:0;height:1rpx;box-shadow:0 13rpx rgba(112,76,20,.28)}.nfc-bank-chip:after{top:0;bottom:0;left:25rpx;width:1rpx}.nfc-bank-content{position:absolute;right:18rpx;bottom:18rpx;display:grid;justify-items:end;gap:1rpx}.nfc-bank-content>.mini-nfc{width:60rpx;height:60rpx;margin-bottom:1rpx}.nfc-bank-content>strong{font-size:25rpx;letter-spacing:2rpx}.nfc-bank-content>text{color:rgba(255,255,255,.8);font-size:18rpx;font-weight:500}.nfc-bank-content .mini-nfc strong{font-size:20rpx}@keyframes bank-card-pulse{0%,100%{filter:brightness(1);box-shadow:0 22rpx 46rpx rgba(5,72,150,.3),inset 0 1rpx 0 rgba(255,255,255,.5)}50%{filter:brightness(1.08);box-shadow:0 28rpx 56rpx rgba(5,72,150,.4),inset 0 1rpx 0 rgba(255,255,255,.55)}}
.section-title-row{justify-content:space-between}.section-title-row>view{display:flex;align-items:center;gap:12rpx}.history-button{height:54rpx;padding:0 18rpx;border:1rpx solid #d8e5f4;border-radius:14rpx;color:#166cc3;background:#fff;font-size:20rpx;line-height:54rpx}.gateway-swipe-shell{perspective:none;isolation:auto;background:#eaf4ff}.gateway-swipe-shell .gateway-card{z-index:2;transform-origin:center;backface-visibility:visible}.gateway-swipe-shell.opened .gateway-card{box-shadow:none}.nfc-swipe-action{top:0;right:0;bottom:0;display:flex;width:176rpx;height:auto;overflow:visible;border:0;border-radius:0;opacity:1;transform:none;transition:none;background:linear-gradient(145deg,#1284ee,#0b62ce);box-shadow:inset 10rpx 0 24rpx rgba(3,65,135,.14)}.gateway-swipe-shell.opened .nfc-swipe-action,.gateway-swipe-shell.revealing .nfc-swipe-action{opacity:1;transform:none;box-shadow:inset 10rpx 0 24rpx rgba(3,65,135,.14)}.gateway-swipe-shell.opened .nfc-swipe-action{animation:action-glow 1.8s ease-in-out infinite}.history-mask{z-index:130;align-items:center;padding:30rpx 24rpx}.nfc-history-dialog{display:grid;grid-template-rows:auto minmax(0,1fr);width:100%;max-width:650rpx;max-height:72vh;overflow:hidden;border-radius:28rpx;background:#fff;box-shadow:0 34rpx 90rpx rgba(15,23,42,.28);animation:confirm-center-in .3s cubic-bezier(.22,1,.36,1)}.history-head{display:flex;align-items:center;justify-content:space-between;padding:26rpx 28rpx 20rpx;border-bottom:1rpx solid #e9eef5}.history-head>view{display:grid;gap:5rpx}.history-head strong{color:#172033;font-size:28rpx}.history-head text{color:#8a97a9;font-size:19rpx}.history-head button{width:56rpx;height:56rpx;padding:0;border-radius:50%;color:#64748b;background:#f1f5f9;font-size:34rpx;line-height:54rpx}.history-list{max-height:58vh}.history-item{display:grid;grid-template-columns:54rpx minmax(0,1fr);align-items:start;gap:16rpx;padding:20rpx 26rpx;border-bottom:1rpx solid #edf1f6}.history-status{display:grid;width:48rpx;height:48rpx;border-radius:50%;color:#dc2846;background:#fff0f2;font-size:25rpx;font-weight:800;place-items:center}.history-status.success{color:#078247;background:#e9f9ef}.history-status.legacy{color:#64748b;background:#f1f5f9}.history-copy{display:grid;gap:5rpx;min-width:0}.history-copy strong{font-size:23rpx}.history-copy text{overflow:hidden;color:#52647d;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.history-copy small{color:#97a3b3;font-size:18rpx}.history-empty{padding:70rpx 24rpx;color:#8a97a9;text-align:center;font-size:22rpx}
.gateway-actions{grid-column:1/-1;display:flex;justify-content:flex-end;padding-top:5rpx}.gateway-actions button{height:56rpx;padding:0 18rpx;border:1rpx solid #d7e3f1;border-radius:14rpx;color:#52647d;background:#f8fafc;font-size:20rpx;line-height:56rpx}.gateway-actions button.danger{border-color:#fecaca;color:#d92d3f;background:#fff7f7}.action-mask{z-index:140}.gateway-action-dialog{display:grid;justify-items:center;gap:20rpx;width:100%;max-width:590rpx;padding:36rpx 30rpx 30rpx;border-radius:28rpx;background:#fff;box-shadow:0 34rpx 90rpx rgba(15,23,42,.3);animation:confirm-center-in .3s cubic-bezier(.22,1,.36,1)}.action-warning{display:grid;width:84rpx;height:84rpx;border-radius:50%;color:#d92d3f;background:#fff0f1;font-size:45rpx;font-weight:800;place-items:center}.action-copy{display:grid;justify-items:center;gap:12rpx;text-align:center}.action-copy strong{color:#172033;font-size:29rpx}.action-copy text{color:#52647d;font-size:21rpx;line-height:1.6}.action-note{width:100%;box-sizing:border-box;padding:14rpx 18rpx;border-radius:14rpx;color:#7c5d18;background:#fff8e6;font-size:19rpx;text-align:center}.confirm-actions .danger-confirm{color:#fff;background:#dc3548;box-shadow:0 10rpx 24rpx rgba(220,53,72,.22)}
.gateway-actions button.print-action{margin-right:10rpx;border-color:#bfdbfe;color:#0b63ce;background:#eff6ff}.print-job-dialog{justify-items:stretch}.print-dialog-icon{display:grid;width:84rpx;height:84rpx;margin:auto;border-radius:22rpx;color:#1677ff;background:#eaf4ff;font-size:38rpx;place-items:center}.print-field{display:grid;width:100%;grid-template-columns:150rpx minmax(0,1fr);align-items:center;gap:12rpx;padding:16rpx 0;border-bottom:1rpx solid #edf1f6}.print-field>text{color:#8a97a9;font-size:21rpx}.print-field picker>view{padding:14rpx;border-radius:13rpx;color:#34445b;background:#f6f8fb;font-size:21rpx}.copy-stepper{display:flex;align-items:center;justify-content:flex-end;gap:18rpx}.copy-stepper button{width:58rpx;height:52rpx;padding:0;border-radius:13rpx;color:#1677ff;background:#eff6ff;line-height:52rpx}.copy-stepper strong{min-width:40rpx;text-align:center}
.print-error{box-sizing:border-box;width:100%;padding:14rpx 16rpx;border:1rpx solid #fecaca;border-radius:14rpx;color:#b4232b;background:#fff1f2;font-size:20rpx;line-height:1.5}.confirm-actions button[disabled]{opacity:.55}
.section-title-row{width:100%;min-height:58rpx;align-items:center}.section-title-row>view:first-child{min-width:0}.history-button{display:flex;align-items:center;justify-content:center;gap:9rpx;flex:0 0 auto;height:52rpx;margin:0;padding:0 16rpx;border:1rpx solid #dbe7f4;border-radius:15rpx;color:#176fc5;background:#fff;box-shadow:0 5rpx 14rpx rgba(22,119,255,.06);font-size:20rpx;line-height:1}.history-button:after{border:0}.section-heading .history-button>text{padding:0;color:inherit;background:transparent;font-size:20rpx}.history-clock{position:relative;box-sizing:border-box;width:25rpx;height:25rpx;border:2rpx solid currentColor;border-radius:50%}.history-clock:before{position:absolute;z-index:2;top:50%;left:50%;width:4rpx;height:4rpx;border-radius:50%;content:"";background:currentColor;transform:translate(-50%,-50%)}.history-clock i:before,.history-clock i:after{position:absolute;top:50%;left:50%;content:"";background:currentColor;transform-origin:left center}.history-clock i:before{width:7rpx;height:2rpx;transform:translate(0,-1rpx) rotate(35deg)}.history-clock i:after{width:2rpx;height:7rpx;transform:translate(-1rpx,-6rpx)}.nfc-history-dialog{max-width:620rpx;max-height:70vh;border:1rpx solid rgba(219,227,238,.9);border-radius:30rpx}.history-head{padding:24rpx 24rpx 21rpx;background:linear-gradient(145deg,#f7fbff,#fff)}.history-head .history-head-main{display:flex;align-items:center;gap:15rpx;min-width:0}.history-head-main>view:last-child{display:grid;gap:4rpx}.history-head-icon{display:grid;width:66rpx;height:66rpx;flex:0 0 auto;border-radius:19rpx;color:#1677ff;background:#eaf4ff;place-items:center}.history-clock.large{width:31rpx;height:31rpx}.history-head strong{font-size:27rpx}.history-head text{font-size:19rpx}.history-head>button{width:50rpx;height:50rpx;flex:0 0 auto;font-size:30rpx;line-height:48rpx}.history-list{box-sizing:border-box;max-height:55vh;padding:8rpx 0 12rpx;background:#fff}.history-item{margin:0 18rpx;padding:19rpx 8rpx;border-bottom:1rpx solid #edf1f6}.history-item:last-child{border-bottom:0}.history-status{margin-top:2rpx}.history-copy text{white-space:normal;line-height:1.45}
.history-head{position:relative;padding-right:88rpx}.history-head>button{position:absolute;top:24rpx;right:24rpx;margin:0;padding:0}.history-head>button:after{border:0}
</style>
