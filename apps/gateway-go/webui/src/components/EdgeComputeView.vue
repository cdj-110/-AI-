<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { confirmAction } from '../confirm'
import { mergePointDiff, openGatewayRealtime } from '../realtime'
import { useConfigStore } from '../stores/config'
import type { PointStatus } from '../types'

const store = useConfigStore()
const mode = defineModel<'run' | 'config'>('mode', { default: 'run' })
const panel = ref<'basic' | 'logic'>('basic')
const groupKey = ref('')
const pointMetric = ref('')
const saving = ref(false)
const validating = ref(false)
const message = ref('')
const error = ref('')
const trial = ref<any>(null)
const statuses = ref<PointStatus[]>([])
const showSource = ref(false)
const sourceSearch = ref('')
const selectedSources = ref<string[]>([])
const sourceExpanded = ref<string[]>([])
let stopRealtime = () => {}

const cfg = computed(() => store.value)
const edge = computed<any>(() => {
  if (!cfg.value.edgeComputing) cfg.value.edgeComputing = { enabled: false, groups: [] }
  if (!Array.isArray(cfg.value.edgeComputing.groups)) cfg.value.edgeComputing.groups = []
  return cfg.value.edgeComputing
})
const groups = computed<any[]>(() => edge.value.groups)
const group = computed(() => groups.value.find(item => item.groupKey === groupKey.value))
const points = computed<any[]>(() => Array.isArray(group.value?.points) ? group.value.points : [])
const point = computed(() => points.value.find(item => item.metric === pointMetric.value))
const live = computed(() => statuses.value.find(item => item.deviceKey === groupKey.value && item.metric === pointMetric.value))
const dataTypes = ['bool', 'int16', 'uint16', 'int32', 'uint32', 'float32', 'float64']
const operators = [' + ', ' - ', ' * ', ' / ', ' % ', ' > ', ' >= ', ' < ', ' <= ', ' == ', ' != ', ' && ', ' || ', '!', ' ? : ', 'min(, )', 'max(, )', 'abs()', 'round()', 'floor()', 'ceil()', 'clamp(, , )']
const sourceDevices = computed(() => {
  const collected = (cfg.value.devices || []).map((item: any) => ({ ...item, kind: '采集设备' }))
  const derived = groups.value.map(item => ({ deviceKey: item.groupKey, name: item.name, points: item.points || [], kind: '边缘计算' }))
  return [...collected, ...derived].filter(item => item.deviceKey !== groupKey.value || item.points.some((p: any) => p.metric !== pointMetric.value))
})
const sourceTree = computed(() => {
  const q = sourceSearch.value.trim().toLowerCase()
  const channels = (cfg.value.channels || []).filter((channel: any) => channel.role !== 'forward')
  const collected = sourceDevices.value.filter((device: any) => device.kind === '采集设备')
  const result = channels.map((channel: any) => {
    const channelMatch = `${channel.name || ''} ${channel.channelKey || ''} ${channel.protocol || ''}`.toLowerCase().includes(q)
    const devices = collected.filter((device: any) => device.channelKey === channel.channelKey).map((device: any) => {
      const deviceMatch = `${device.name || ''} ${device.deviceKey || ''} ${device.protocol || ''}`.toLowerCase().includes(q)
      const points = (device.points || []).filter((item: any) => !q || channelMatch || deviceMatch || `${item.name || ''} ${item.metric || ''}`.toLowerCase().includes(q))
      return { device, points, key: `source-device:${device.deviceKey}` }
    }).filter((item: any) => item.points.length || !q)
    return { channel, devices, key: `source-channel:${channel.channelKey}` }
  }).filter((item: any) => item.devices.some((device: any) => device.points.length) || !q)

  const assigned = new Set(channels.map((channel: any) => channel.channelKey))
  const unassigned = collected.filter((device: any) => !assigned.has(device.channelKey)).map((device: any) => {
    const deviceMatch = `${device.name || ''} ${device.deviceKey || ''} ${device.protocol || ''}`.toLowerCase().includes(q)
    const points = (device.points || []).filter((item: any) => !q || deviceMatch || `${item.name || ''} ${item.metric || ''}`.toLowerCase().includes(q))
    return { device, points, key: `source-device:${device.deviceKey}` }
  }).filter((item: any) => item.points.length || !q)
  if (unassigned.length) result.push({ channel: { channelKey: 'unassigned', name: '未归属通道', protocol: '' }, devices: unassigned, key: 'source-channel:unassigned' })

  const derived = sourceDevices.value.filter((device: any) => device.kind === '边缘计算').map((device: any) => {
    const deviceMatch = `${device.name || ''} ${device.deviceKey || ''}`.toLowerCase().includes(q)
    const points = (device.points || []).filter((item: any) => !q || deviceMatch || `${item.name || ''} ${item.metric || ''}`.toLowerCase().includes(q))
    return { device, points, key: `source-device:${device.deviceKey}` }
  }).filter((item: any) => item.points.length || !q)
  if (derived.length) result.push({ channel: { channelKey: 'edge-derived', name: '边缘计算派生点', protocol: '虚拟设备' }, devices: derived, key: 'source-channel:edge-derived' })
  return result
})

function unique(prefix: string, values: string[]) {
  let index = 1
  let next = ''
  const used = values.map(value => value.toLowerCase())
  do next = `${prefix}-${String(index++).padStart(3, '0')}`
  while (used.includes(next.toLowerCase()))
  return next
}
function ensureSelection() {
  if (!group.value) groupKey.value = groups.value[0]?.groupKey || ''
  if (group.value && group.value.heartbeatSeconds == null) group.value.heartbeatSeconds = 0
  if (!points.value.some(item => item.metric === pointMetric.value)) pointMetric.value = points.value[0]?.metric || ''
}
function selectGroup(value: string) {
  groupKey.value = value
  pointMetric.value = groups.value.find(item => item.groupKey === value)?.points?.[0]?.metric || ''
  panel.value = 'basic'
  trial.value = null
}
function selectPoint(metric: string) {
  pointMetric.value = metric
  trial.value = null
}
function changeGroupKey(value: string) {
  if (!group.value) return
  const next = value.trim()
  const previous = group.value.groupKey
  if (!next) { error.value = '组标识不能为空'; return }
  const used = [
    ...(cfg.value.devices || []).map((item: any) => item.deviceKey),
    ...(cfg.value.forwardDevices || []).map((item: any) => item.deviceKey),
    ...groups.value.filter(item => item !== group.value).map(item => item.groupKey),
  ]
  if (used.some(item => String(item).toLowerCase() === next.toLowerCase())) { error.value = '组标识已存在，请使用唯一标识'; return }
  for (const input of groups.value.flatMap(entry => entry.points || []).flatMap((entry: any) => entry.inputs || [])) {
    if (input.sourceDeviceKey === previous) input.sourceDeviceKey = next
  }
  group.value.groupKey = next
  groupKey.value = next
  error.value = ''
}
function addGroup() {
  const keys = [
    ...(cfg.value.devices || []).map((item: any) => item.deviceKey),
    ...(cfg.value.forwardDevices || []).map((item: any) => item.deviceKey),
    ...groups.value.map(item => item.groupKey),
  ]
  const value = unique('edge-group', keys)
  groups.value.push({ groupKey: value, name: `计算组${groups.value.length + 1}`, enabled: true, heartbeatSeconds: 0, points: [] })
  selectGroup(value)
  mode.value = 'config'
}
async function removeGroup() {
  if (!group.value || !await confirmAction({
    title: '删除计算组',
    message: `确认删除计算组“${group.value.name}”？`,
    detail: '该计算组下的全部派生点也会一并删除。',
    confirmText: '确认删除',
    danger: true,
  })) return
  edge.value.groups = groups.value.filter(item => item.groupKey !== groupKey.value)
  groupKey.value = ''
  ensureSelection()
}
function allMetrics() {
  return [
    ...(cfg.value.devices || []).flatMap((item: any) => item.points || []),
    ...(cfg.value.forwardDevices || []).flatMap((item: any) => item.points || []),
    ...groups.value.flatMap(item => item.points || []),
  ].map((item: any) => item.metric)
}
function addPoint() {
  if (!group.value) { error.value = '请先新增计算组'; return }
  const metric = unique('derived', allMetrics())
  group.value.points.push({ name: `派生点${points.value.length + 1}`, metric, enabled: true, dataType: 'float64', unit: '', decimals: 2, expression: '', inputs: [] })
  pointMetric.value = metric
  panel.value = 'basic'
  trial.value = null
  mode.value = 'config'
}
function duplicatePoint() {
  if (!point.value) return
  const copy = JSON.parse(JSON.stringify(point.value))
  copy.metric = unique('derived', allMetrics())
  copy.name = `${point.value.name || point.value.metric} 副本`
  points.value.push(copy)
  pointMetric.value = copy.metric
  panel.value = 'basic'
  trial.value = null
}
async function removePoint() {
  if (!point.value || !await confirmAction({
    title: '删除派生点',
    message: `确认删除派生点“${point.value.name || point.value.metric}”？`,
    confirmText: '确认删除',
    danger: true,
  })) return
  group.value.points = points.value.filter(item => item !== point.value)
  pointMetric.value = group.value.points[0]?.metric || ''
  trial.value = null
}
function suggestAlias(metric: string) {
  let base = (metric || 'input').replace(/[^a-zA-Z0-9_]/g, '_')
  if (!/^[a-zA-Z_]/.test(base)) base = `v_${base}`
  const used = new Set((point.value?.inputs || []).map((item: any) => item.alias))
  let alias = base
  let index = 2
  while (used.has(alias)) alias = `${base}_${index++}`
  return alias
}
function confirmSources() {
  if (!point.value) return
  point.value.inputs = point.value.inputs || []
  for (const key of selectedSources.value) {
    const [deviceKey, metric] = key.split('::')
    if (point.value.inputs.some((item: any) => item.sourceDeviceKey === deviceKey && item.sourceMetric === metric)) continue
    point.value.inputs.push({ alias: suggestAlias(metric), sourceDeviceKey: deviceKey, sourceMetric: metric })
  }
  showSource.value = false
  selectedSources.value = []
}
function sourceKeys(devices: any[]) { return devices.flatMap(device => device.points.map((item: any) => `${device.device.deviceKey}::${item.metric}`)) }
function sourceGroupSelected(keys: string[]) { return keys.length > 0 && keys.every(key => selectedSources.value.includes(key)) }
function sourceGroupPartial(keys: string[]) { return keys.some(key => selectedSources.value.includes(key)) && !sourceGroupSelected(keys) }
function toggleSourceGroup(keys: string[]) {
  if (sourceGroupSelected(keys)) selectedSources.value = selectedSources.value.filter(key => !keys.includes(key))
  else selectedSources.value = [...new Set([...selectedSources.value, ...keys])]
}
function toggleSource(key: string) {
  const index = selectedSources.value.indexOf(key)
  if (index >= 0) selectedSources.value.splice(index, 1)
  else selectedSources.value.push(key)
}
function toggleSourceExpanded(key: string) {
  const index = sourceExpanded.value.indexOf(key)
  if (index >= 0) sourceExpanded.value.splice(index, 1)
  else sourceExpanded.value.push(key)
}
function expandSourceTree() { sourceExpanded.value = sourceTree.value.flatMap((channel: any) => [channel.key, ...channel.devices.map((device: any) => device.key)]) }
function collapseSourceTree() { sourceExpanded.value = [] }
function openSourceDialog() {
  sourceSearch.value = ''
  selectedSources.value = []
  sourceExpanded.value = sourceTree.value.flatMap((channel: any) => [channel.key, ...channel.devices.map((device: any) => device.key)])
  showSource.value = true
}
function removeInput(index: number) {
  point.value.inputs.splice(index, 1)
  trial.value = null
}
function sourceLabel(input: any) {
  const device = sourceDevices.value.find(item => item.deviceKey === input.sourceDeviceKey)
  const sourcePoint = device?.points?.find((item: any) => item.metric === input.sourceMetric)
  return `${device?.name || input.sourceDeviceKey} / ${sourcePoint?.name || input.sourceMetric}`
}
function inputLive(input: any) {
  return statuses.value.find(item => item.deviceKey === input.sourceDeviceKey && item.metric === input.sourceMetric)
}
function inputLiveText(input: any) {
  const status = inputLive(input)
  if (status?.error) return '-'
  return String(status?.value ?? '--')
}
function insertOperator(token: string) {
  if (mode.value === 'run' || !point.value) return
  point.value.expression = (point.value.expression || '') + token
}
async function loadStatus() {
  try { statuses.value = await api<PointStatus[]>('/api/point-status') } catch {}
}
function handleRealtime(event: any) {
  if (event.type === 'snapshot') statuses.value = Array.isArray(event.payload?.points) ? event.payload.points : statuses.value
  else if (event.type === 'points.diff') statuses.value = mergePointDiff(statuses.value, event.payload || {})
}
async function validate() {
  if (!group.value || !point.value) return
  validating.value = true
  error.value = ''
  trial.value = null
  try {
    trial.value = await api('/api/edge-compute/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: JSON.stringify(cfg.value), groupKey: group.value.groupKey, metric: point.value.metric }),
    })
    message.value = '表达式验证通过'
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '验证失败'
  } finally { validating.value = false }
}
async function save() {
  saving.value = true
  error.value = ''
  message.value = ''
  try {
    await store.save()
    message.value = '边缘计算配置已保存并热加载'
    mode.value = 'run'
    await Promise.all([store.load(), loadStatus()])
    ensureSelection()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '保存失败'
  } finally { saving.value = false }
}
async function reload() {
  await Promise.all([store.load(), loadStatus()])
  ensureSelection()
  message.value = '配置已重新加载'
}

onMounted(async () => {
  if (!store.value.gatewayKey) await store.load()
  ensureSelection()
  await loadStatus()
  stopRealtime = openGatewayRealtime(handleRealtime)
})
onBeforeUnmount(() => stopRealtime())
</script>

<template>
  <div class="view-stack edge-view edge-tab-view">
    <p v-if="message" class="notice good">{{ message }}</p>
    <p v-if="error" class="notice bad">{{ error }}</p>

    <section class="card edge-shell-card">
      <div class="section-title edge-page-head">
        <div>
          <h2>边缘计算</h2>
          <p class="meta">组合采集点位生成派生数据，并作为虚拟设备点位参与展示和上报。</p>
        </div>
        <div class="edge-page-actions">
          <label class="inline-toggle">总开关
            <select v-model="edge.enabled" :disabled="mode === 'run'">
              <option :value="true">启用</option><option :value="false">停用</option>
            </select>
          </label>
          <button @click="reload">重新加载</button>
          <button class="primary" :disabled="saving || mode === 'run'" @click="save">{{ saving ? '保存中…' : '保存配置' }}</button>
          <button :disabled="mode === 'run'" @click="addGroup">新增组</button>
        </div>
      </div>

      <div class="edge-layout edge-tab-layout">
        <aside class="edge-tree edge-compact-tree">
          <div class="column-head"><strong>计算组</strong><span>{{ groups.length }} 组</span></div>
          <template v-for="item in groups" :key="item.groupKey">
            <button class="edge-group" :class="{ active: item.groupKey === groupKey }" @click="selectGroup(item.groupKey)">
              <span>{{ item.name || item.groupKey }}</span><small>{{ item.points?.length || 0 }} 个派生点</small>
            </button>
            <div v-if="item.groupKey === groupKey" class="edge-points">
              <button v-for="derived in item.points" :key="derived.metric" :class="{ active: derived.metric === pointMetric }" @click="selectPoint(derived.metric)">
                <span>{{ derived.name || derived.metric }}</span><code>{{ derived.metric }}</code>
              </button>
              <button class="edge-add-point" :disabled="mode === 'run'" @click="addPoint">＋ 新增派生点</button>
            </div>
          </template>
          <p v-if="!groups.length" class="empty-mini">暂无计算组</p>
        </aside>

        <main class="edge-editor edge-tab-editor">
          <template v-if="group">
            <div class="edge-context-head">
              <div><h3>{{ group.name || group.groupKey }}</h3><p><code>{{ group.groupKey }}</code><span v-if="point"> · {{ point.name || point.metric }}</span></p></div>
              <button class="danger" :disabled="mode === 'run'" @click="removeGroup">删除计算组</button>
            </div>

            <nav class="edge-editor-tabs" aria-label="边缘计算配置步骤">
              <button :class="{ active: panel === 'basic' }" @click="panel = 'basic'">基本配置</button>
              <button :class="{ active: panel === 'logic' }" :disabled="!point" @click="panel = 'logic'">输入与表达式 <span>{{ point?.inputs?.length || 0 }}</span></button>
            </nav>

            <section v-if="panel === 'basic'" class="edge-tab-panel">
              <div class="edge-panel-title"><div><h4>计算组设置</h4><p>配置虚拟设备标识、启停状态和无变化心跳。</p></div></div>
              <div class="config-grid edge-basic-grid">
                <label>组名称<input v-model.trim="group.name" :disabled="mode === 'run'" /></label>
                <label>组标识 deviceKey<input :value="group.groupKey" :disabled="mode === 'run'" @change="changeGroupKey(($event.target as HTMLInputElement).value)" /></label>
                <label>组状态<select v-model="group.enabled" :disabled="mode === 'run'"><option :value="true">启用</option><option :value="false">停用</option></select></label>
                <label>无变化心跳（秒）<input v-model.number="group.heartbeatSeconds" type="number" min="0" :disabled="mode === 'run'" /></label>
              </div>

              <p v-if="!point" class="edge-panel-empty">该计算组暂无派生点，请在左侧新增。</p>
            </section>

            <section v-else-if="panel === 'logic' && point" class="edge-tab-panel edge-input-panel edge-logic-panel">
              <div class="edge-panel-title">
                <div><h4>输出点设置</h4><p>定义当前派生点的名称、标识符和输出格式。</p></div>
                <div class="button-row"><button :disabled="mode === 'run'" @click="duplicatePoint">复制点位</button><button class="danger" :disabled="mode === 'run'" @click="removePoint">删除点位</button></div>
              </div>
              <div class="config-grid edge-output-grid">
                <label>名称<input v-model.trim="point.name" :disabled="mode === 'run'" /></label>
                <label>标识符 metric<input v-model.trim="point.metric" :disabled="mode === 'run'" @change="pointMetric = point.metric" /></label>
                <label>输出类型<select v-model="point.dataType" :disabled="mode === 'run'"><option v-for="item in dataTypes" :key="item">{{ item }}</option></select></label>
                <label>状态<select v-model="point.enabled" :disabled="mode === 'run'"><option :value="true">启用</option><option :value="false">停用</option></select></label>
                <label>单位<input v-model.trim="point.unit" :disabled="mode === 'run'" /></label>
                <label>小数位<input v-model.number="point.decimals" type="number" min="0" max="9" :disabled="mode === 'run'" /></label>
              </div>
              <div class="edge-panel-title edge-output-title"><div><h4>输入点位</h4><p>选择采集点或其他派生点，并设置表达式中使用的别名。</p></div><button v-if="mode === 'config'" @click="openSourceDialog">选择输入点位</button></div>
              <div class="table-scroll edge-input-table"><table><thead><tr><th>别名</th><th>来源点位</th><th>当前值</th><th>操作</th></tr></thead><tbody>
                <tr v-if="!point.inputs?.length"><td colspan="4" class="empty">尚未添加输入点位</td></tr>
                <tr v-for="(input, index) in point.inputs" :key="`${input.sourceDeviceKey}:${input.sourceMetric}`">
                  <td><input v-model.trim="input.alias" class="table-input mono" :disabled="mode === 'run'" /></td>
                  <td><strong class="edge-source-name">{{ sourceLabel(input) }}</strong><small class="mono">{{ input.sourceDeviceKey }} / {{ input.sourceMetric }}</small></td>
                  <td><strong class="edge-input-value" :class="inputLive(input)?.error ? 'bad-text' : ''">{{ inputLiveText(input) }}</strong></td>
                  <td><button class="danger" :disabled="mode === 'run'" @click="removeInput(index)">删除</button></td>
                </tr>
              </tbody></table></div>
              <div class="edge-panel-title edge-expression-title"><div><h4>CEL 表达式</h4><p>使用上方输入别名、运算符和安全函数生成派生结果。</p></div></div>
              <div class="expression-panel">
                <label>表达式<textarea v-model="point.expression" :disabled="mode === 'run'" placeholder="例如：temperature * 1.8 + 32" /></label>
                <div v-if="mode === 'config'" class="operator-row"><button v-for="token in operators" :key="token" @click="insertOperator(token)">{{ token }}</button></div>
                <p>仅允许数值、布尔运算与安全函数，不允许循环、宏或脚本调用。</p>
              </div>
              <div class="edge-result edge-result-cards">
                <div><span>实时结果</span><strong :class="live?.error ? 'bad-text' : ''">{{ live?.error || String(live?.value ?? '--') }}</strong><small>{{ live?.updatedAt ? new Date(live.updatedAt).toLocaleString() : '尚无计算结果' }}</small></div>
                <div v-if="trial"><span>试算结果</span><strong>{{ String(trial.value) }}</strong><small>输入：{{ JSON.stringify(trial.inputs) }}</small></div>
                <button v-if="mode === 'config'" class="primary" :disabled="validating" @click="validate">{{ validating ? '验证中…' : '验证并试算' }}</button>
              </div>
            </section>
          </template>
          <div v-else class="edge-empty-state"><strong>尚未创建计算组</strong><p>点击“新增组”开始配置边缘计算。</p><button :disabled="mode === 'run'" @click="addGroup">新增计算组</button></div>
        </main>
      </div>
    </section>

    <div v-if="showSource" class="restart-modal">
      <div class="forward-point-select-dialog edge-source-tree-dialog">
        <div><h3>选择已有实时点位</h3><p class="meta">按通道、设备、点位逐级展开，可跨设备多选；添加后可在输入列表中调整表达式别名。</p></div>
        <div class="forward-point-select-toolbar"><input v-model.trim="sourceSearch" type="search" placeholder="搜索通道、设备、点位名称或标识符" /><button @click="expandSourceTree">全部展开</button><button @click="collapseSourceTree">全部折叠</button><button :disabled="!selectedSources.length" @click="selectedSources = []">清空选择</button></div>
        <div class="forward-source-tree">
          <section v-for="channelNode in sourceTree" :key="channelNode.key" class="forward-tree-channel">
            <div class="forward-tree-row channel">
              <button class="forward-tree-expand" :class="{ expanded: sourceExpanded.includes(channelNode.key) || !!sourceSearch.trim() }" @click="toggleSourceExpanded(channelNode.key)"><span /></button>
              <input type="checkbox" :checked="sourceGroupSelected(sourceKeys(channelNode.devices))" :indeterminate="sourceGroupPartial(sourceKeys(channelNode.devices))" @change="toggleSourceGroup(sourceKeys(channelNode.devices))" />
              <div><strong>{{ channelNode.channel.name || channelNode.channel.channelKey }}</strong><small>{{ channelNode.channel.protocol || '采集通道' }} · {{ channelNode.devices.length }} 台设备</small></div><em>{{ sourceKeys(channelNode.devices).length }} 个点位</em>
            </div>
            <div v-if="sourceExpanded.includes(channelNode.key) || sourceSearch.trim()" class="forward-tree-children">
              <section v-for="deviceNode in channelNode.devices" :key="deviceNode.key" class="forward-tree-device">
                <div class="forward-tree-row device">
                  <button class="forward-tree-expand" :class="{ expanded: sourceExpanded.includes(deviceNode.key) || !!sourceSearch.trim() }" @click="toggleSourceExpanded(deviceNode.key)"><span /></button>
                  <input type="checkbox" :checked="sourceGroupSelected(sourceKeys([deviceNode]))" :indeterminate="sourceGroupPartial(sourceKeys([deviceNode]))" @change="toggleSourceGroup(sourceKeys([deviceNode]))" />
                  <div><strong>{{ deviceNode.device.name || deviceNode.device.deviceKey }}</strong><small>{{ deviceNode.device.deviceKey }} · {{ deviceNode.device.protocol || deviceNode.device.kind || '-' }}</small></div><em>{{ deviceNode.points.length }} 个点位</em>
                </div>
                <div v-if="sourceExpanded.includes(deviceNode.key) || sourceSearch.trim()" class="forward-tree-points">
                  <label v-for="sourcePoint in deviceNode.points" :key="sourcePoint.metric"><input type="checkbox" :checked="selectedSources.includes(`${deviceNode.device.deviceKey}::${sourcePoint.metric}`)" @change="toggleSource(`${deviceNode.device.deviceKey}::${sourcePoint.metric}`)" /><span><strong>{{ sourcePoint.name || sourcePoint.metric }}</strong><small>{{ sourcePoint.metric }}</small></span><code>{{ sourcePoint.dataType || '-' }}</code></label>
                  <p v-if="!deviceNode.points.length" class="forward-tree-empty">该设备暂无可选点位</p>
                </div>
              </section>
            </div>
          </section>
          <p v-if="!sourceTree.length" class="forward-tree-empty">没有匹配的输入点位</p>
        </div>
        <div class="dialog-actions"><span class="muted">已选择 {{ selectedSources.length }} 个点位</span><button @click="showSource = false">取消</button><button class="primary" :disabled="!selectedSources.length" @click="confirmSources">添加所选输入</button></div>
      </div>
    </div>
  </div>
</template>
