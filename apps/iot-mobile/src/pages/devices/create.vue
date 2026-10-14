<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { createDevice, gateways, templates } from '../../services/cloud'
import type { GatewayOption, ModelTemplate } from '../../types'

const form=reactive({deviceKey:'',name:'',protocol:'MQTT',deviceType:'DIRECT',gatewayId:'',location:'',modelTemplateId:''})
const gatewayList=ref<GatewayOption[]>([]),templateList=ref<ModelTemplate[]>([])
const busy=ref(false),loading=ref(true),error=ref(''),created=ref<any>()
const types=[['DIRECT','直连设备'],['GATEWAY_CHILD','网关子设备'],['GATEWAY','智能网关']]
const protocols=['MQTT','HTTP','MODBUS']
const filteredTemplates=computed(()=>templateList.value.filter(item=>item.deviceType===form.deviceType))
function typeChange(event:any){form.deviceType=types[Number(event.detail.value)]?.[0]||'DIRECT';form.gatewayId='';form.modelTemplateId=''}
function protocolChange(event:any){form.protocol=protocols[Number(event.detail.value)]||'MQTT'}
function gatewayChange(event:any){form.gatewayId=gatewayList.value[Number(event.detail.value)]?.id||''}
function templateChange(event:any){form.modelTemplateId=filteredTemplates.value[Number(event.detail.value)]?.id||''}
function selectedLabel(list:any[],value:string,key='id',label='name'){return list.find(item=>item[key]===value)?.[label]||'请选择'}
async function submit(){
  if(!form.deviceKey.trim()||!form.name.trim()){error.value='设备编号和设备名称不能为空';return}
  if(form.deviceType==='GATEWAY_CHILD'&&!form.gatewayId){error.value='网关子设备必须选择所属网关';return}
  busy.value=true;error.value=''
  try{
    created.value=await createDevice({
      deviceKey:form.deviceKey.trim(),name:form.name.trim(),protocol:form.protocol,deviceType:form.deviceType,
      gatewayId:form.deviceType==='GATEWAY_CHILD'?form.gatewayId:undefined,
      location:form.location.trim()||undefined,modelTemplateId:form.modelTemplateId||undefined
    })
    uni.showToast({title:'设备创建成功',icon:'success'})
  }catch(reason){error.value=reason instanceof Error?reason.message:'设备创建失败'}
  finally{busy.value=false}
}
function back(){uni.switchTab({url:'/pages/devices/index'})}
function copy(value:string){uni.setClipboardData({data:value})}
onMounted(async()=>{try{[gatewayList.value,templateList.value]=await Promise.all([gateways(),templates()])}catch(reason){error.value=reason instanceof Error?reason.message:'选项读取失败'}finally{loading.value=false}})
</script>

<template>
  <view class="page create-page">
    <view v-if="created" class="created-card card">
      <view class="success-mark">✓</view><strong>设备创建成功</strong><text>{{created.name}}（{{created.deviceKey}}）</text>
      <view v-if="created.mqtt" class="credential-box">
        <text>MQTT连接凭证仅在创建时完整显示，请妥善保存</text>
        <view v-for="item in [['Client ID',created.mqtt.clientId],['Username',created.mqtt.username],['Password',created.mqtt.password],['Broker',created.mqtt.broker]]" :key="item[0]"><text>{{item[0]}}</text><code>{{item[1]||'-'}}</code><button v-if="item[1]" @click="copy(item[1])">复制</button></view>
      </view>
      <button class="primary-button" @click="back">返回设备列表</button>
    </view>
    <view v-else class="form-card card">
      <view class="form-title"><strong>添加到物联云</strong><text>创建后将自动生成设备连接凭证</text></view>
      <label class="field"><text>设备编号 *</text><input v-model="form.deviceKey" placeholder="例如 WK-SENSOR-001"/></label>
      <label class="field"><text>设备名称 *</text><input v-model="form.name" placeholder="请输入便于识别的名称"/></label>
      <label class="field"><text>设备类型</text><picker :range="types" range-key="1" @change="typeChange"><view class="picker">{{selectedLabel(types,form.deviceType,'0','1')}}</view></picker></label>
      <label class="field"><text>通信协议</text><picker :range="protocols" @change="protocolChange"><view class="picker">{{form.protocol}}</view></picker></label>
      <label v-if="form.deviceType==='GATEWAY_CHILD'" class="field"><text>所属网关 *</text><picker :range="gatewayList" range-key="name" @change="gatewayChange"><view class="picker">{{selectedLabel(gatewayList,form.gatewayId)}}</view></picker></label>
      <label class="field"><text>物模型模板</text><picker :range="filteredTemplates" range-key="name" @change="templateChange"><view class="picker">{{selectedLabel(filteredTemplates,form.modelTemplateId)}}</view></picker></label>
      <label class="field"><text>安装位置</text><input v-model="form.location" placeholder="选填"/></label>
      <text v-if="error" class="form-error">{{error}}</text>
      <button class="primary-button submit-button" :loading="busy||loading" :disabled="busy||loading" @click="submit">{{loading?'正在读取配置':'创建设备'}}</button>
    </view>
  </view>
</template>

<style scoped lang="scss">
.create-page{padding-top:24rpx}.form-title{display:grid;gap:8rpx;margin-bottom:12rpx}.form-title strong{font-size:36rpx}.form-title text{color:#7a8799;font-size:23rpx}.form-error{display:block;margin-top:22rpx;padding:17rpx;border-radius:14rpx;color:#b4232b;background:#fff0f1;font-size:23rpx}.submit-button{margin-top:30rpx}.created-card{display:grid;justify-items:center;gap:16rpx;text-align:center}.success-mark{display:grid;place-items:center;width:90rpx;height:90rpx;border-radius:50%;color:#fff;background:#16a34a;font-size:48rpx}.created-card>strong{font-size:38rpx}.created-card>text{color:#64748b;font-size:24rpx}.created-card>.primary-button{width:100%;margin-top:10rpx}.credential-box{display:grid;gap:10rpx;width:100%;margin:20rpx 0;padding:20rpx;border-radius:18rpx;background:#0b1220;text-align:left}.credential-box>text{color:#fbbf24;font-size:21rpx;line-height:1.6}.credential-box>view{display:grid;grid-template-columns:120rpx minmax(0,1fr) 76rpx;gap:10rpx;align-items:center}.credential-box view>text{color:#94a3b8;font-size:20rpx}.credential-box code{overflow:hidden;color:#dbeafe;font-size:20rpx;text-overflow:ellipsis;white-space:nowrap}.credential-box button{height:48rpx;padding:0;color:#93c5fd;background:#172554;font-size:19rpx;line-height:48rpx}
</style>
