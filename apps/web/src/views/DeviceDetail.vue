<script setup lang="ts">
import { ArrowLeft, Connection, Plus, RefreshRight, Setting, VideoPause, VideoPlay } from '@element-plus/icons-vue';
import * as echarts from 'echarts';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface DeviceItem {
  id: string;
  deviceKey: string;
  name: string;
  protocol: string;
  status: string;
  location?: string;
  lastSeenAt?: string;
  tenant: { name: string };
}

interface TelemetryEvent {
  deviceKey: string;
  time: string;
  metrics: Record<string, unknown>;
}

interface DeviceMetric {
  id: string;
  identifier: string;
  name: string;
  dataType: string;
  unit?: string;
  decimals: number;
  accessMode: string;
  enabled: boolean;
  sortOrder: number;
}

interface AlarmRule {
  id: string;
  identifier: string;
  operator: string;
  threshold: number;
  hysteresis: number;
  level: string;
  enabled: boolean;
}

interface ModelTemplateItem {
  id: string;
  deviceKey: string;
  name: string;
  _count: { metrics: number };
}

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const device = ref<DeviceItem>();
const telemetryItems = ref<TelemetryEvent[]>([]);
const streamStatus = ref<'CONNECTING' | 'LIVE' | 'DISCONNECTED'>('CONNECTING');
const trendChartRef = ref<HTMLDivElement>();
const selectedMetric = ref('');
const paused = ref(false);
const pausedQueue = ref<TelemetryEvent[]>([]);
const settingsVisible = ref(false);
const ruleDialogVisible = ref(false);
const deviceMetrics = ref<DeviceMetric[]>([]);
const alarmRules = ref<AlarmRule[]>([]);
const modelTemplates = ref<ModelTemplateItem[]>([]);
const ruleForm = reactive({ identifier: '', operator: '>', threshold: 0, hysteresis: 0, level: 'WARNING' });
const importForm = reactive({ templateDeviceId: '', overwrite: false });
const draggingMetricKey = ref('');
const dragOverMetricKey = ref('');
const savingMetricOrder = ref(false);
let streamController: AbortController | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let statusTimer: ReturnType<typeof setInterval> | undefined;
let settingsRefreshTimer: ReturnType<typeof setTimeout> | undefined;
let trendChart: echarts.ECharts | undefined;

const latest = computed(() => telemetryItems.value[0]);
const canManage = computed(() => userStore.userInfo?.role !== 'TENANT_USER');
const importableTemplates = computed(() => modelTemplates.value.filter((template) => template.id !== String(route.params.id)));
const visibleTelemetryItems = computed(() => telemetryItems.value.filter((item) => Object.keys(item.metrics).some((key) => metricEnabled(key))));
const metrics = computed(() => Object.entries(latest.value?.metrics ?? {})
  .filter(([key]) => metricEnabled(key))
  .map(([key, value]) => ({ key, value }))
  .sort((left, right) => metricSortOrder(left.key) - metricSortOrder(right.key)));
const numericMetricKeys = computed(() => {
  const keys = new Set<string>();
  for (const item of telemetryItems.value) {
    for (const [key, value] of Object.entries(item.metrics)) {
      if (typeof value === 'number' && Number.isFinite(value) && metricEnabled(key)) keys.add(key);
    }
  }
  return [...keys];
});
const selectedMetricValues = computed(() => telemetryItems.value
  .map((item) => item.metrics[selectedMetric.value])
  .filter((value): value is number => typeof value === 'number' && Number.isFinite(value)));
const selectedMetricStats = computed(() => {
  const values = selectedMetricValues.value;
  if (!values.length) return { latest: '-', min: '-', max: '-', avg: '-' };
  const total = values.reduce((sum, value) => sum + value, 0);
  return {
    latest: formatNumber(values[0]),
    min: formatNumber(Math.min(...values)),
    max: formatNumber(Math.max(...values)),
    avg: formatNumber(total / values.length),
  };
});

function metricLabel(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.name
    ?? { temperature: '温度', humidity: '湿度', battery: '电量', pressure: '压力', voltage: '电压', current: '电流', power: '功率', fenbei: '分贝' }[key]
    ?? key;
}

function metricUnit(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.unit
    ?? { temperature: '°C', humidity: '%', battery: '%', pressure: 'kPa', voltage: 'V', current: 'A', power: 'W', fenbei: 'dB' }[key]
    ?? '';
}

function metricEnabled(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.enabled ?? true;
}

function metricSortOrder(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.sortOrder ?? 999;
}

function metricConfig(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key);
}

function displayValue(value: unknown, key?: string) {
  if (typeof value === 'number' && key) {
    const decimals = deviceMetrics.value.find((metric) => metric.identifier === key)?.decimals;
    return decimals === undefined ? String(value) : value.toFixed(decimals);
  }
  return typeof value === 'object' ? JSON.stringify(value) : String(value);
}

function addTelemetry(event: TelemetryEvent) {
  if (paused.value) {
    pausedQueue.value = [event, ...pausedQueue.value].slice(0, 20);
    return;
  }
  telemetryItems.value = [event, ...telemetryItems.value].slice(0, 20);
  if (Object.keys(event.metrics).some((key) => !deviceMetrics.value.some((metric) => metric.identifier === key))) {
    if (settingsRefreshTimer) clearTimeout(settingsRefreshTimer);
    settingsRefreshTimer = setTimeout(() => void loadSettings(), 600);
  }
  if (device.value) {
    device.value.status = 'ONLINE';
    device.value.lastSeenAt = event.time;
  }
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '');
}

async function refreshDevice() {
  device.value = await apiRequest<DeviceItem>({ url: `/api/devices/${String(route.params.id)}`, method: 'GET' });
}

function handleDeviceStatusEvent(event: Event) {
  const detail = (event as CustomEvent<Partial<DeviceItem>>).detail;
  if (!device.value || detail?.id !== device.value.id || !detail.status) return;
  device.value.status = detail.status;
  if (detail.lastSeenAt) device.value.lastSeenAt = detail.lastSeenAt;
}

async function refreshPage() {
  await Promise.all([loadPage(), loadSettings()]);
}

async function loadSettings() {
  const id = String(route.params.id);
  [deviceMetrics.value, alarmRules.value] = await Promise.all([
    apiRequest<DeviceMetric[]>({ url: `/api/devices/${id}/metrics`, method: 'GET' }),
    apiRequest<AlarmRule[]>({ url: `/api/devices/${id}/alarm-rules`, method: 'GET' }),
  ]);
}

async function openSettings() {
  const id = String(route.params.id);
  [modelTemplates.value] = await Promise.all([
    apiRequest<ModelTemplateItem[]>({ url: '/api/devices/model-templates', method: 'GET' }),
    loadSettings(),
  ]);
  if (importForm.templateDeviceId === id) importForm.templateDeviceId = '';
  settingsVisible.value = true;
}

async function saveMetric(metric: DeviceMetric) {
  await apiRequest({ url: `/api/devices/${String(route.params.id)}/metrics/${metric.id}`, method: 'PATCH', data: metric });
  ElMessage.success('指标配置已保存');
}

function startMetricDrag(event: DragEvent, key: string) {
  if (!canManage.value) {
    event.preventDefault();
    return;
  }
  draggingMetricKey.value = key;
  event.dataTransfer?.setData('text/plain', key);
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
}

function overMetricCard(event: DragEvent, key: string) {
  if (!canManage.value || !draggingMetricKey.value || draggingMetricKey.value === key) return;
  event.preventDefault();
  dragOverMetricKey.value = key;
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
}

function endMetricDrag() {
  draggingMetricKey.value = '';
  dragOverMetricKey.value = '';
}

async function dropMetricCard(event: DragEvent, targetKey: string) {
  event.preventDefault();
  if (!canManage.value || savingMetricOrder.value) return;
  const sourceKey = draggingMetricKey.value || event.dataTransfer?.getData('text/plain') || '';
  endMetricDrag();
  if (!sourceKey || sourceKey === targetKey) return;
  await reorderMetricCards(sourceKey, targetKey);
}

async function reorderMetricCards(sourceKey: string, targetKey: string) {
  const orderedKeys = metrics.value.map((metric) => metric.key);
  const sourceIndex = orderedKeys.indexOf(sourceKey);
  const targetIndex = orderedKeys.indexOf(targetKey);
  if (sourceIndex < 0 || targetIndex < 0) return;
  orderedKeys.splice(sourceIndex, 1);
  orderedKeys.splice(targetIndex, 0, sourceKey);

  const changedMetrics: DeviceMetric[] = [];
  orderedKeys.forEach((key, index) => {
    const metric = metricConfig(key);
    if (!metric) return;
    const nextSortOrder = (index + 1) * 10;
    if (metric.sortOrder === nextSortOrder) return;
    metric.sortOrder = nextSortOrder;
    changedMetrics.push(metric);
  });
  if (!changedMetrics.length) return;

  savingMetricOrder.value = true;
  try {
    await Promise.all(changedMetrics.map((metric) => apiRequest({
      url: `/api/devices/${String(route.params.id)}/metrics/${metric.id}`,
      method: 'PATCH',
      data: { sortOrder: metric.sortOrder },
    })));
    ElMessage.success('指标排序已保存');
  } catch (error) {
    await loadSettings();
    throw error;
  } finally {
    savingMetricOrder.value = false;
  }
}

async function importMetrics() {
  if (!importForm.templateDeviceId) {
    ElMessage.warning('请选择要复用的模板设备');
    return;
  }
  const result = await apiRequest<{ created: number; updated: number; skipped: number }>({
    url: `/api/devices/${String(route.params.id)}/metrics/import`,
    method: 'POST',
    data: importForm,
  });
  ElMessage.success(`物模型已导入：新增 ${result.created} 个，更新 ${result.updated} 个，跳过 ${result.skipped} 个`);
  await loadSettings();
}

function openRuleDialog() {
  Object.assign(ruleForm, { identifier: deviceMetrics.value[0]?.identifier ?? '', operator: '>', threshold: 0, hysteresis: 0, level: 'WARNING' });
  ruleDialogVisible.value = true;
}

async function saveRule() {
  await apiRequest({ url: `/api/devices/${String(route.params.id)}/alarm-rules`, method: 'POST', data: ruleForm });
  ElMessage.success('告警规则已保存');
  ruleDialogVisible.value = false;
  await loadSettings();
}

async function removeRule(rule: AlarmRule) {
  await apiRequest({ url: `/api/devices/${String(route.params.id)}/alarm-rules/${rule.id}`, method: 'DELETE' });
  ElMessage.success('告警规则已删除');
  await loadSettings();
}

function togglePause() {
  paused.value = !paused.value;
  if (!paused.value && pausedQueue.value.length) {
    telemetryItems.value = [...pausedQueue.value, ...telemetryItems.value].slice(0, 20);
    if (device.value) {
      device.value.status = 'ONLINE';
      device.value.lastSeenAt = pausedQueue.value[0].time;
    }
    pausedQueue.value = [];
  }
}

async function renderTrendChart() {
  await nextTick();
  if (!trendChartRef.value || !selectedMetric.value) return;
  trendChart ??= echarts.init(trendChartRef.value);
  const points = [...telemetryItems.value]
    .reverse()
    .filter((item) => typeof item.metrics[selectedMetric.value] === 'number');
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 46, right: 18, top: 24, bottom: 28 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: points.map((item) => new Date(item.time).toLocaleTimeString()),
      axisLabel: { color: '#94a3b8' },
    },
    yAxis: {
      type: 'value',
      name: metricUnit(selectedMetric.value),
      axisLabel: { color: '#94a3b8' },
      splitLine: { lineStyle: { color: '#edf1f5' } },
    },
    series: [{
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: points.map((item) => item.metrics[selectedMetric.value]),
      lineStyle: { color: '#2563eb', width: 3 },
      areaStyle: { color: 'rgba(37, 99, 235, 0.08)' },
    }],
  });
}

function resizeChart() {
  trendChart?.resize();
}

async function loadPage() {
  const id = String(route.params.id);
  const [deviceData, telemetryData] = await Promise.all([
    apiRequest<DeviceItem>({ url: `/api/devices/${id}`, method: 'GET' }),
    apiRequest<{ deviceKey: string; items: Array<{ time: string; metrics: Record<string, unknown> }> }>({
      url: `/api/devices/${id}/telemetry`,
      method: 'GET',
    }),
  ]);
  device.value = deviceData;
  telemetryItems.value = telemetryData.items.map((item) => ({ deviceKey: telemetryData.deviceKey, ...item }));
}

async function connectStream() {
  const controller = new AbortController();
  streamController = controller;
  streamStatus.value = 'CONNECTING';
  const baseURL = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:3100';
  try {
    const response = await fetch(`${baseURL}/api/devices/${String(route.params.id)}/telemetry/stream`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('accessToken') ?? ''}` },
      signal: controller.signal,
    });
    if (!response.ok || !response.body) throw new Error('telemetry stream unavailable');
    streamStatus.value = 'LIVE';
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const messages = buffer.split('\n\n');
      buffer = messages.pop() ?? '';
      for (const message of messages) {
        const data = message.split('\n').find((line) => line.startsWith('data:'));
        if (data) {
          const payload = JSON.parse(data.slice(5).trim());
          addTelemetry(payload.data ?? payload);
        }
      }
    }
  } catch (error) {
    if (controller.signal.aborted) return;
  }
  streamStatus.value = 'DISCONNECTED';
  reconnectTimer = setTimeout(() => void connectStream(), 3000);
}

function formatMetrics(item: Record<string, unknown>) {
  return Object.entries(item)
    .filter(([key]) => metricEnabled(key))
    .sort(([left], [right]) => metricSortOrder(left) - metricSortOrder(right))
    .map(([key, value]) => `${metricLabel(key)}: ${displayValue(value, key)}${metricUnit(key)}`)
    .join(' · ');
}

onMounted(async () => {
  await Promise.all([loadPage(), loadSettings()]);
  selectedMetric.value = numericMetricKeys.value[0] ?? '';
  await renderTrendChart();
  void connectStream();
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  statusTimer = setInterval(() => void refreshDevice(), 10000);
  window.addEventListener('resize', resizeChart);
});

onBeforeUnmount(() => {
  streamController?.abort();
  if (reconnectTimer) clearTimeout(reconnectTimer);
  if (statusTimer) clearInterval(statusTimer);
  if (settingsRefreshTimer) clearTimeout(settingsRefreshTimer);
  window.removeEventListener('device-status-change', handleDeviceStatusEvent);
  window.removeEventListener('resize', resizeChart);
  trendChart?.dispose();
});

watch(numericMetricKeys, (keys) => {
  if (!keys.includes(selectedMetric.value)) selectedMetric.value = keys[0] ?? '';
});

watch([telemetryItems, selectedMetric], () => void renderTrendChart(), { deep: true });
</script>

<template>
  <div>
    <el-button text :icon="ArrowLeft" class="back" @click="router.push('/devices')">返回设备列表</el-button>
    <div class="page-actions">
      <div>
        <h1 class="page-title">{{ device?.name }}</h1>
        <p class="page-description">{{ device?.deviceKey }} · {{ device?.protocol }} · {{ device?.tenant.name }}</p>
      </div>
      <div class="live-state">
        <span :class="['dot', streamStatus.toLowerCase()]" />
        {{ streamStatus === 'LIVE' ? '实时数据已连接' : streamStatus === 'CONNECTING' ? '正在连接实时数据' : '实时连接已断开' }}
        <el-button v-if="canManage" text :icon="Setting" class="settings-button" @click="openSettings">设备配置</el-button>
      </div>
    </div>

    <section class="device-summary">
      <div><span>设备状态</span><el-tag :type="device?.status === 'ONLINE' ? 'success' : 'warning'">{{ device?.status === 'ONLINE' ? '在线' : '离线' }}</el-tag></div>
      <div><span>安装位置</span><strong>{{ device?.location || '-' }}</strong></div>
      <div><span>最后上报</span><strong>{{ device?.lastSeenAt ? new Date(device.lastSeenAt).toLocaleString() : '-' }}</strong></div>
    </section>

    <el-alert class="topic-tip" type="info" :closable="false" show-icon>
      <template #title>
        当前设备遥测主题：<code>weikong/devices/{{ device?.deviceKey }}/telemetry</code>
      </template>
    </el-alert>

    <section class="metrics-section">
      <div class="section-heading">
        <div><h2>实时指标</h2><p>设备推送新的标识符后会自动生成对应卡片</p></div>
        <div class="monitor-actions">
          <span v-if="paused && pausedQueue.length" class="queue-tip">已缓存 {{ pausedQueue.length }} 条</span>
          <el-button :icon="RefreshRight" @click="refreshPage">手动同步</el-button>
          <el-button :type="paused ? 'primary' : 'default'" :icon="paused ? VideoPlay : VideoPause" @click="togglePause">{{ paused ? '继续显示' : '暂停画面' }}</el-button>
        </div>
      </div>
      <div v-if="metrics.length" class="metrics-grid" :class="{ sorting: savingMetricOrder }">
        <article
          v-for="metric in metrics"
          :key="metric.key"
          class="metric-card"
          :class="{ dragging: draggingMetricKey === metric.key, 'drag-over': dragOverMetricKey === metric.key }"
          :draggable="canManage"
          @dragstart="startMetricDrag($event, metric.key)"
          @dragover="overMetricCard($event, metric.key)"
          @dragleave="dragOverMetricKey = ''"
          @drop="dropMetricCard($event, metric.key)"
          @dragend="endMetricDrag"
        >
          <button v-if="canManage" class="metric-drag-handle" type="button" title="拖动排序">⋮⋮</button>
          <p>{{ metricLabel(metric.key) }}</p>
          <strong>{{ displayValue(metric.value, metric.key) }}<small>{{ metricUnit(metric.key) }}</small></strong>
          <span>{{ metric.key }}</span>
        </article>
      </div>
      <el-empty v-else description="等待设备推送遥测数据" />
    </section>

    <section class="chart-panel">
      <div class="section-heading">
        <div><h2>实时趋势</h2><p>根据最近 20 条上报绘制数值变化</p></div>
        <el-select v-model="selectedMetric" placeholder="选择指标" style="width: 150px">
          <el-option v-for="key in numericMetricKeys" :key="key" :label="metricLabel(key)" :value="key" />
        </el-select>
      </div>
      <div v-if="selectedMetric" class="trend-stats">
        <div><span>最新值</span><strong>{{ selectedMetricStats.latest }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>最小值</span><strong>{{ selectedMetricStats.min }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>最大值</span><strong>{{ selectedMetricStats.max }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>平均值</span><strong>{{ selectedMetricStats.avg }}{{ metricUnit(selectedMetric) }}</strong></div>
      </div>
      <div v-if="numericMetricKeys.length" ref="trendChartRef" class="trend-chart" />
      <el-empty v-else description="暂无可绘制的数值指标" />
    </section>

    <section class="table-panel history">
      <div class="section-heading"><div><h2>最近上报</h2><p>最多保留最近 20 条展示记录</p></div></div>
      <el-table :data="visibleTelemetryItems" empty-text="暂无可展示的遥测数据">
        <el-table-column label="上报时间" width="190"><template #default="{ row }">{{ new Date(row.time).toLocaleString() }}</template></el-table-column>
        <el-table-column label="指标"><template #default="{ row }">{{ formatMetrics(row.metrics) }}</template></el-table-column>
      </el-table>
    </section>

    <el-drawer v-model="settingsVisible" title="设备配置" size="min(1080px, 96vw)">
      <div class="drawer-heading"><div><h3>物模型</h3><p>设备首次上报字段后自动登记。配置会直接影响实时卡片、趋势图和历史记录展示。</p></div></div>
      <div v-if="canManage" class="model-import">
        <el-select v-model="importForm.templateDeviceId" clearable placeholder="选择已有设备物模型" class="template-select">
          <el-option v-for="template in importableTemplates" :key="template.id" :label="`${template.name} (${template.deviceKey}) · ${template._count.metrics} 个字段`" :value="template.id" />
        </el-select>
        <el-switch v-model="importForm.overwrite" active-text="覆盖同名字段" />
        <el-button type="primary" plain @click="importMetrics">导入物模型</el-button>
      </div>
      <el-table :data="deviceMetrics" size="small">
        <el-table-column prop="identifier" label="标识符" min-width="105" show-overflow-tooltip />
        <el-table-column label="显示名称" min-width="120"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
        <el-table-column label="单位" width="72"><template #default="{ row }"><el-input v-model="row.unit" /></template></el-table-column>
        <el-table-column prop="dataType" label="类型" width="74" />
        <el-table-column label="权限" width="92"><template #default="{ row }"><el-select v-model="row.accessMode"><el-option label="只读" value="READ_ONLY" /><el-option label="读写" value="READ_WRITE" /></el-select></template></el-table-column>
        <el-table-column label="排序" width="88"><template #default="{ row }"><el-input-number v-model="row.sortOrder" class="metric-number" :min="0" controls-position="right" /></template></el-table-column>
        <el-table-column label="小数位" width="88"><template #default="{ row }"><el-input-number v-model="row.decimals" class="metric-number" :min="0" :max="6" controls-position="right" /></template></el-table-column>
        <el-table-column label="展示" width="58"><template #default="{ row }"><el-switch v-model="row.enabled" /></template></el-table-column>
        <el-table-column label="操作" width="56"><template #default="{ row }"><el-button link type="primary" @click="saveMetric(row)">保存</el-button></template></el-table-column>
      </el-table>

      <div class="drawer-heading rules-heading">
        <div><h3>告警规则</h3><p>数值满足条件时产生告警，恢复正常后自动关闭。</p></div>
        <el-button type="primary" :icon="Plus" @click="openRuleDialog">新增规则</el-button>
      </div>
      <el-table :data="alarmRules" size="small">
        <el-table-column label="指标" min-width="110"><template #default="{ row }">{{ metricLabel(row.identifier) }}</template></el-table-column>
        <el-table-column prop="operator" label="条件" width="70" />
        <el-table-column prop="threshold" label="阈值" width="90" />
        <el-table-column prop="hysteresis" label="恢复缓冲" width="100" />
        <el-table-column prop="level" label="级别" width="100" />
        <el-table-column label="操作" width="70"><template #default="{ row }"><el-button link type="danger" @click="removeRule(row)">删除</el-button></template></el-table-column>
      </el-table>
    </el-drawer>

    <el-dialog v-model="ruleDialogVisible" title="新增告警规则" width="460px">
      <el-form :model="ruleForm" label-width="76px">
        <el-form-item label="指标"><el-select v-model="ruleForm.identifier" class="full-width"><el-option v-for="metric in deviceMetrics" :key="metric.id" :label="`${metric.name} (${metric.identifier})`" :value="metric.identifier" /></el-select></el-form-item>
        <el-form-item label="条件"><el-select v-model="ruleForm.operator" class="full-width"><el-option v-for="operator in ['>', '>=', '<', '<=']" :key="operator" :label="operator" :value="operator" /></el-select></el-form-item>
        <el-form-item label="阈值"><el-input-number v-model="ruleForm.threshold" class="full-width" /></el-form-item>
        <el-form-item label="恢复缓冲"><el-input-number v-model="ruleForm.hysteresis" :min="0" class="full-width" /></el-form-item>
        <el-form-item label="级别"><el-select v-model="ruleForm.level" class="full-width"><el-option label="提示" value="INFO" /><el-option label="警告" value="WARNING" /><el-option label="严重" value="CRITICAL" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="ruleDialogVisible = false">取消</el-button><el-button type="primary" @click="saveRule">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.back { margin: -8px 0 14px -12px; }
.live-state { color: #64748b; font-size: 13px; }
.settings-button { margin-left: 12px; }
.dot { display: inline-block; width: 8px; height: 8px; margin-right: 7px; border-radius: 50%; background: #94a3b8; }
.dot.live { background: #22c55e; box-shadow: 0 0 0 4px #dcfce7; }
.dot.disconnected { background: #ef4444; }
.device-summary { display: flex; gap: 36px; padding: 18px 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.device-summary div { display: flex; gap: 10px; align-items: center; font-size: 13px; }
.device-summary span { color: #94a3b8; }
.topic-tip { margin-top: 18px; }
code { color: #2563eb; font-family: Consolas, monospace; }
.metrics-section { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.chart-panel { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.trend-chart { height: 280px; margin-top: 12px; }
.monitor-actions { display: flex; gap: 10px; align-items: center; }
.queue-tip { color: #f59e0b; font-size: 12px; }
.trend-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-top: 18px; }
.trend-stats div { padding: 12px 14px; border-radius: 9px; background: #f8fafc; }
.trend-stats span { display: block; color: #94a3b8; font-size: 12px; }
.trend-stats strong { display: block; margin-top: 5px; color: #111827; font-size: 18px; }
.section-heading { display: flex; align-items: center; justify-content: space-between; color: #2563eb; }
h2 { margin: 0; color: #111827; font-size: 16px; }
.section-heading p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
.metrics-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(170px, 1fr)); gap: 14px; margin-top: 18px; }
.metrics-grid.sorting { opacity: 0.72; pointer-events: none; }
.metric-card { position: relative; padding: 18px; border: 1px solid #edf1f5; border-radius: 10px; background: #fafcff; transition: border-color 0.15s ease, box-shadow 0.15s ease, opacity 0.15s ease, transform 0.15s ease; }
.metric-card[draggable="true"] { cursor: grab; }
.metric-card[draggable="true"]:active { cursor: grabbing; }
.metric-card.dragging { opacity: 0.48; transform: scale(0.98); }
.metric-card.drag-over { border-color: #2563eb; box-shadow: 0 0 0 3px rgb(37 99 235 / 12%); }
.metric-drag-handle { position: absolute; top: 10px; right: 10px; width: 26px; height: 26px; border: 0; border-radius: 8px; color: #94a3b8; background: transparent; cursor: grab; font-size: 15px; letter-spacing: -4px; }
.metric-drag-handle:hover { color: #2563eb; background: #eff6ff; }
.metric-card p { margin: 0 0 12px; color: #64748b; font-size: 13px; }
.metric-card strong { display: block; overflow-wrap: anywhere; color: #111827; font-size: 25px; }
.metric-card small { margin-left: 4px; color: #94a3b8; font-size: 13px; }
.metric-card span { display: block; margin-top: 10px; color: #a1aab8; font: 12px Consolas, monospace; }
.history { margin-top: 18px; }
.history .section-heading { margin-bottom: 16px; }
.drawer-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.drawer-heading h3 { margin: 0; color: #111827; font-size: 16px; }
.drawer-heading p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
.model-import { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin: 0 0 14px; padding: 12px; border: 1px solid #edf1f5; border-radius: 10px; background: #f8fafc; }
.template-select { min-width: 310px; flex: 1; }
.metric-number { width: 100%; }
.rules-heading { margin-top: 28px; }
@media (max-width: 700px) { .device-summary { flex-direction: column; gap: 12px; } .trend-stats { grid-template-columns: repeat(2, 1fr); } .monitor-actions { align-items: flex-end; flex-direction: column; } }
</style>
