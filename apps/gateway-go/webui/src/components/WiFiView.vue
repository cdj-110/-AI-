<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { api } from '../api'

interface WiFiStatus { config: Record<string, any>; interface: string; available: boolean; connected: boolean; bootEnabled?: boolean; restartRequired?: boolean; ssid?: string; signal?: number; ipv4?: string; message?: string; apply?: { state?: string; message?: string }; interfaces?: string[] }
interface WiFiSaveResult { ok?: boolean; connected?: boolean; ipv4?: string; restartRequired?: boolean; message?: string }
interface WiFiNetwork { ssid: string; signal?: number; security?: string }

const status = ref<WiFiStatus | null>(null)
const networks = ref<WiFiNetwork[]>([])
const loading = ref(false)
const scanning = ref(false)
const message = ref('')
const showPassword = ref(false)
const lastEnabled = ref(false)
const stateChangeTarget = ref<boolean | null>(null)
const savedRestartOpen = ref(false)
const connectionResult = ref<WiFiSaveResult | null>(null)
const rebooting = ref(false)
const form = reactive({ enabled: false, interface: 'wlan0', ssid: '', password: '', mode: 'dhcp', ipAddress: '', prefixLength: 24, gateway: '', dns: [] as string[] })

async function load() {
  loading.value = true
  try {
    const data = await api<WiFiStatus>('/api/network/wifi')
    status.value = data
    Object.assign(form, data.config || {})
    lastEnabled.value = !!form.enabled
    if (data.config?.password) form.password = '***'
    message.value = data.message || ''
  } catch (error) { message.value = error instanceof Error ? error.message : 'WiFi 状态读取失败' }
  finally { loading.value = false }
}

async function scan() {
  if (!status.value?.available) {
    form.enabled = true
    stateChangeTarget.value = true
    return
  }
  scanning.value = true; networks.value = []
  try {
    const data = await api<{ networks: WiFiNetwork[]; message?: string }>('/api/network/wifi/scan?interface=' + encodeURIComponent(form.interface), { timeoutMs: 40000 })
    networks.value = data.networks || []; message.value = data.message || `扫描到 ${networks.value.length} 个热点`
  } catch (error) { message.value = error instanceof Error ? error.message : '扫描失败' }
  finally { scanning.value = false }
}

async function save() {
  loading.value = true
  try {
    const result = await api<WiFiSaveResult>('/api/network/wifi', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(form), timeoutMs: 45000 })
    message.value = result.message || 'WiFi 配置已保存，重启网关后生效。'
    if (status.value) status.value.restartRequired = result.restartRequired !== false
    lastEnabled.value = !!form.enabled
    if (result.restartRequired) savedRestartOpen.value = true
    else connectionResult.value = result
  } catch (error) { message.value = error instanceof Error ? error.message : '保存失败' }
  finally { loading.value = false }
}

function selectNetwork(network: WiFiNetwork) {
  form.ssid = network.ssid
  networks.value = []
  message.value = `已选择 WiFi：${network.ssid}`
}

function openStateChange() {
  stateChangeTarget.value = !!form.enabled
}

function cancelStateChange() {
  form.enabled = lastEnabled.value
  stateChangeTarget.value = null
}

async function rebootGateway() {
  await api('/api/maintenance/reboot', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' })
  message.value = '网关正在重启，请稍后刷新页面。'
  window.setTimeout(() => window.location.reload(), 15000)
}

async function applyStateAndReboot() {
  if (stateChangeTarget.value === null) return
  rebooting.value = true
  const enabled = stateChangeTarget.value
  try {
    const result = await api<WiFiSaveResult>('/api/network/wifi/boot', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled }) })
    message.value = result.message || `WiFi 已设置为${enabled?'启用':'停用'}，重启后生效。`
    lastEnabled.value = enabled
    if (status.value) { status.value.bootEnabled = enabled; status.value.restartRequired = true }
    stateChangeTarget.value = null
    await rebootGateway()
  } catch (error) { message.value = error instanceof Error ? error.message : '切换 WiFi 状态失败'; rebooting.value = false }
}

async function rebootAfterSave() {
  rebooting.value = true
  try { savedRestartOpen.value = false; await rebootGateway() }
  catch (error) { message.value = error instanceof Error ? error.message : '重启网关失败'; rebooting.value = false }
}
onMounted(load)
</script>

<template><section id="panel-wifi"><section class="card"><div class="section-title"><div><h2>WiFi</h2><p class="meta">配置 EG100 无线联网，并可扫描附近热点快速填入 SSID。</p></div><div><button @click="load">刷新</button><button class="primary" :disabled="loading" @click="save">保存并应用 WiFi</button></div></div>
  <section class="wireless-panel"><div class="section-title"><div><h3>无线配置</h3><p class="meta">{{status?.available?'扫描后点击热点名称，填入密码后保存应用。':'无线网卡当前未加载；启用并重启后才能扫描附近热点。'}}</p></div><button v-if="form.enabled" :disabled="scanning||loading" @click="scan">{{scanning?'正在扫描':'扫描'}}</button></div>
  <div class="wireless-grid"><label>状态<select v-model="form.enabled" @change="openStateChange"><option :value="true">启用</option><option :value="false">停用</option></select></label><label>网卡<input v-model.trim="form.interface" :disabled="!form.enabled"/></label><label class="wide">SSID<input v-model.trim="form.ssid" :disabled="!form.enabled"/></label><label class="wide">密码<span class="password-field"><input v-model="form.password" :type="showPassword?'text':'password'" autocomplete="new-password" :disabled="!form.enabled"/><button type="button" @click="showPassword=!showPassword">{{showPassword?'隐藏':'显示'}}</button></span></label><label>地址模式<select v-model="form.mode" :disabled="!form.enabled"><option value="dhcp">DHCP</option><option value="static">静态 IP</option></select></label><label v-if="form.mode==='static'">IPv4地址<input v-model.trim="form.ipAddress"/></label><label v-if="form.mode==='static'">前缀长度<input v-model.number="form.prefixLength" type="number" min="1" max="32"/></label><label v-if="form.mode==='static'">默认网关<input v-model.trim="form.gateway"/></label></div>
  <div v-if="connectionResult" class="wifi-apply-result" :class="connectionResult.ok?'success':'error'"><div><strong>{{connectionResult.ok?'WiFi 连接成功':'WiFi 连接失败'}}</strong><span>{{connectionResult.message}}</span></div><div v-if="connectionResult.ok" class="wifi-current-ip"><span>当前 IP</span><b>{{connectionResult.ipv4||'-'}}</b></div><button @click="connectionResult=null">关闭</button></div>
  <div class="toast muted">{{status?.connected?`已连接 ${status.ssid||''}${status.ipv4?` · IP ${status.ipv4}`:''}`:message||(status?.available?'网卡可用，当前未连接':'未检测到 WiFi 网卡')}}</div><div v-if="networks.length" class="wifi-scan-list"><button v-for="network in networks" :key="network.ssid" @click="selectNetwork(network)"><strong>{{network.ssid}}</strong><span>{{network.security||'开放网络'}}</span><span>{{network.signal?network.signal+'%':'-'}}</span></button></div>
  </section></section>
  <div v-if="stateChangeTarget!==null" class="restart-modal"><div class="dialog-panel"><h3>{{stateChangeTarget?'启用 WiFi':'停用 WiFi'}}</h3><p>WiFi 功能需要重启整台网关后才能{{stateChangeTarget?'启用':'停用'}}。是否立即重启网关？重启期间采集和页面访问会短暂中断。</p><div class="dialog-actions"><button :disabled="rebooting" @click="cancelStateChange">取消</button><button class="danger" :disabled="rebooting" @click="applyStateAndReboot">{{rebooting?'正在重启':'立即重启网关'}}</button></div></div></div>
  <div v-if="savedRestartOpen" class="restart-modal"><div class="dialog-panel"><h3>WiFi 配置已保存</h3><p>{{message}} 是否立即重启网关？重启期间采集和页面访问会短暂中断。</p><div class="dialog-actions"><button :disabled="rebooting" @click="savedRestartOpen=false">稍后重启</button><button class="danger" :disabled="rebooting" @click="rebootAfterSave">{{rebooting?'正在重启':'立即重启网关'}}</button></div></div></div>
  </section></template>
