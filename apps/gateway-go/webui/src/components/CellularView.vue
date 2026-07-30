<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import { useConfigStore } from '../stores/config'

const store = useConfigStore()
const status = ref<Record<string, any>>({})
const loading = ref(false)
const message = ref('')
const showLog = ref(false)
const form = reactive({ enabled: true, interface: 'usbeth0' })

async function load() {
  loading.value = true
  try {
    if (!store.value.gatewayKey) await store.load()
    Object.assign(form, store.value.cellular || {})
    status.value = await api('/api/network/cellular')
    message.value = status.value.message || ''
  } catch (error) { message.value = error instanceof Error ? error.message : '移动网络状态读取失败' }
  finally { loading.value = false }
}
async function save() { loading.value = true; try { status.value = await api('/api/network/cellular',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(form),timeoutMs:40000}); await store.load(); message.value='配置已保存并应用。' } catch(error){ message.value=error instanceof Error?error.message:'保存失败' } finally{loading.value=false} }
async function redial() { loading.value=true; try { status.value=await api('/api/network/cellular',{method:'POST',timeoutMs:60000}); message.value='重拨已完成。' } catch(error){message.value=error instanceof Error?error.message:'重拨失败'} finally{loading.value=false} }
onMounted(load)
</script>
<template><section id="panel-cellular"><section class="card"><div class="section-title"><div><h2>移动网络</h2><p class="meta">读取 4G 模块、蜂窝接口和拨号状态。</p></div><div class="point-actions"><button class="primary" :disabled="loading" @click="save">保存并应用</button><button :disabled="!form.enabled||loading" @click="redial">立即重拨</button><button @click="load">刷新</button></div></div>
  <section class="wireless-panel"><div class="wireless-grid cellular-config"><label>移动网络<select v-model="form.enabled"><option :value="true">启用</option><option :value="false">停用</option></select></label><label>蜂窝接口<input v-model.trim="form.interface" :disabled="!form.enabled"/></label></div><div class="wireless-status"><div class="detail-grid"><article><span>当前 IP</span><strong>{{status.currentIp||'-'}}</strong></article><article><span>信号强度</span><strong>{{status.sim?.signalText||(status.sim?.signal?status.sim.signal+'%':'-')}}</strong></article><article><span>网络服务商</span><strong>{{status.provider||status.sim?.operator||'-'}}</strong></article><article><span>SIM 状态</span><strong>{{status.sim?.available?(status.sim.pinStatus||'已识别'):'未识别'}}</strong></article><article><span>IMEI</span><strong>{{status.sim?.imei||'-'}}</strong></article><article><span>ICCID</span><strong>{{status.cardNumber||status.sim?.iccid||'-'}}</strong></article></div><div class="cellular-status-foot"><span class="muted">{{message}}</span><button @click="showLog=true">查看详情</button></div></div></section>
  <div v-if="showLog" class="cellular-log-modal"><div class="dialog-panel"><div class="dialog-head"><div><h3>拨号日志</h3><p>移动网络重拨和 DHCP 获取过程。</p></div><button @click="showLog=false">关闭</button></div><pre>{{status.dialLog||'暂无拨号日志'}}</pre><div class="dialog-actions"><button @click="load">刷新</button><button class="primary" @click="showLog=false">确定</button></div></div></div>
</section></section></template>
