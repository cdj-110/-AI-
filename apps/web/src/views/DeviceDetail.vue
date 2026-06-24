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
  mqtt?: DeviceCredentials;
  name: string;
  protocol: string;
  deviceType: string;
  gateway?: RelatedDevice;
  children?: RelatedDevice[];
  status: string;
  location?: string;
  lastSeenAt?: string;
  tenant: { name: string };
}

interface RelatedDevice {
  id: string;
  name: string;
  deviceKey: string;
  status: string;
  location?: string;
  lastSeenAt?: string;
}

interface TelemetryEvent {
  deviceKey: string;
  time: string;
  metrics: Record<string, unknown>;
}

interface MetricSnapshot {
  key: string;
  value: unknown;
  time: string;
  deviceKey: string;
}

interface TrendPoint {
  time: string;
  value: number;
}

interface DeviceMetric {
  id: string;
  identifier: string;
  name: string;
  dataType: string;
  unit?: string;
  decimals: number;
  accessMode: string;
  ignored: boolean;
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

interface DeviceModelTemplateOption {
  id: string;
  name: string;
  deviceType: string;
  _count: { metrics: number };
}

interface DeviceCredentials {
  host: string;
  port: number;
  wsPort: number;
  clientId: string;
  username: string;
  password?: string;
  passwordUpdatedAt: string;
  heartbeatTopic: string;
  telemetryTopic: string;
}

interface DeviceLogItem {
  id: string;
  deviceKey: string;
  deviceName?: string;
  type: string;
  level: string;
  source: string;
  message: string;
  detail?: Record<string, unknown>;
  createdAt: string;
}

interface RemoteConfigTask {
  id: string;
  version: number;
  status: 'PENDING' | 'SENT' | 'APPLIED' | 'FAILED' | 'TIMEOUT';
  config: Record<string, unknown>;
  createdBy: string;
  error?: string;
  sentAt?: string;
  completedAt?: string;
  createdAt: string;
}

interface RemoteConfigDiffRow {
  field: string;
  label: string;
  before: string;
  after: string;
}

interface RemotePointConfig {
  deviceKey: string;
  name: string;
  metric: string;
  protocol: string;
  address: string;
  slaveId: number;
  area?: string;
  dbNumber?: number;
  objectRef?: string;
  fc?: string;
  nodeId?: string;
  function: number;
  register: number;
  quantity: number;
  dataType: string;
  byteOrder: string;
  wordOrder: string;
  bitIndex?: number | null;
  scale: number;
  offset: number;
  unit?: string;
  decimals?: number;
  password?: string;
}

interface RemoteDeviceConfig {
  deviceKey: string;
  name: string;
  protocol: string;
  address: string;
  slaveId: number;
  interfaceType?: string;
  interfaceName?: string;
  plcModel?: string;
  rack?: number;
  slot?: number;
  iedName?: string;
  username?: string;
  password?: string;
  points: RemotePointConfig[];
}

interface RemoteConfigSnapshot {
  collectIntervalSeconds?: number;
  devices?: RemoteDeviceConfig[];
}

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const device = ref<DeviceItem>();
const telemetryItems = ref<TelemetryEvent[]>([]);
const recentTelemetryItems = ref<TelemetryEvent[]>([]);
const streamStatus = ref<'CONNECTING' | 'LIVE' | 'DISCONNECTED'>('CONNECTING');
const detailTab = ref<'monitor' | 'config' | 'logs'>('monitor');
const trendChartRef = ref<HTMLDivElement>();
const selectedMetric = ref('');
const selectedTrendRange = ref('1m');
const trendPoints = ref<TrendPoint[]>([]);
const trendLoading = ref(false);
const paused = ref(false);
const pausedQueue = ref<TelemetryEvent[]>([]);
const settingsVisible = ref(false);
const settingsTab = ref('metrics');
const settingsLoading = ref(false);
const ruleDialogVisible = ref(false);
const ruleEditingId = ref('');
const deviceMetrics = ref<DeviceMetric[]>([]);
const alarmRules = ref<AlarmRule[]>([]);
const modelTemplates = ref<ModelTemplateItem[]>([]);
const deviceModelTemplates = ref<DeviceModelTemplateOption[]>([]);
const credentials = ref<DeviceCredentials>();
const deviceLogs = ref<DeviceLogItem[]>([]);
const deviceLogsTotal = ref(0);
const deviceLogsLoading = ref(false);
const rotatingCredentials = ref(false);
const remoteConfigTasks = ref<RemoteConfigTask[]>([]);
const remoteConfigText = ref('{\n  "collectIntervalSeconds": 1\n}');
const remoteConfigMode = ref<'basic' | 'advanced'>('basic');
const remoteConfigForm = reactive({ collectIntervalSeconds: 1 });
const remoteDevices = ref<RemoteDeviceConfig[]>([]);
const remoteSnapshot = ref<RemoteConfigSnapshot>();
const remoteSnapshotAt = ref<string>();
const remoteSnapshotLoading = ref(false);
const remoteConfigLoading = ref(false);
const sendingRemoteConfig = ref(false);
const remoteConfigInitialized = ref(false);
const remoteConfigConfirmVisible = ref(false);
const pendingRemoteConfig = ref<Record<string, unknown>>();
const remoteConfigDiffRows = ref<RemoteConfigDiffRow[]>([]);
const remoteProtocols = ['modbus-tcp', 'modbus-rtu', 'siemens-s7', 'opcua', 'iec104', 'iec61850'];
const remoteDataTypes = ['bool', 'uint16', 'int16', 'uint32', 'int32', 'float32'];
const ruleForm = reactive({ identifier: '', operator: '>', threshold: 0, hysteresis: 0, level: 'WARNING' });
const importForm = reactive({ templateDeviceId: '', modelTemplateId: '', overwrite: false });
const deviceLogQuery = reactive({ page: 1, pageSize: 6, type: '' });
const draggingMetricKey = ref('');
const dragOverMetricKey = ref('');
const savingMetricOrder = ref(false);
let streamController: AbortController | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let statusTimer: ReturnType<typeof setInterval> | undefined;
let recentTelemetryTimer: ReturnType<typeof setInterval> | undefined;
let settingsRefreshTimer: ReturnType<typeof setTimeout> | undefined;
let trendRefreshTimer: ReturnType<typeof setTimeout> | undefined;
let telemetryFlushTimer: ReturnType<typeof setTimeout> | undefined;
let remoteConfigTimer: ReturnType<typeof setInterval> | undefined;
let trendChart: echarts.ECharts | undefined;
const pendingTelemetryEvents: TelemetryEvent[] = [];
const RECENT_TELEMETRY_LIMIT = 20;
const trendRanges = [
  { label: '1分钟', value: '1m' },
  { label: '15分钟', value: '15m' },
  { label: '30分钟', value: '30m' },
  { label: '1小时', value: '1h' },
  { label: '3小时', value: '3h' },
  { label: '6小时', value: '6h' },
  { label: '1天', value: '1d' },
  { label: '1周', value: '1w' },
];

const latest = computed(() => telemetryItems.value[0]);
const latestMetrics = computed(() => {
  const snapshots = new Map<string, MetricSnapshot>();
  for (const item of telemetryItems.value) {
    for (const [key, value] of Object.entries(item.metrics)) {
      if (!key.trim() || snapshots.has(key)) continue;
      snapshots.set(key, { key, value, time: item.time, deviceKey: item.deviceKey });
    }
  }
  return snapshots;
});
const canManage = computed(() => userStore.userInfo?.role !== 'TENANT_USER');
const importableTemplates = computed(() => modelTemplates.value.filter((template) => template.id !== String(route.params.id)));
const importableModelTemplates = computed(() => deviceModelTemplates.value.filter((template) => template.deviceType === device.value?.deviceType));
const visibleTelemetryItems = computed(() => recentTelemetryItems.value.filter((item) => Object.keys(item.metrics).some((key) => key.trim() && metricEnabled(key))));
const metrics = computed(() => [...latestMetrics.value.values()]
  // 卡片只展示未忽略字段，顺序跟随设备配置中的排序。
  .filter((snapshot) => metricEnabled(snapshot.key))
  .map((snapshot) => ({ key: snapshot.key, value: snapshot.value, time: snapshot.time, deviceKey: snapshot.deviceKey }))
  .sort((left, right) => metricSortOrder(left.key) - metricSortOrder(right.key)));
const sortedDeviceMetrics = computed(() => {
  // 配置列表优先展示当前正在上报的字段，方便维护实时点位。
  const latestKeys = new Set(latestMetrics.value.keys());
  return [...deviceMetrics.value].sort((left, right) => {
    const leftLive = latestKeys.has(left.identifier);
    const rightLive = latestKeys.has(right.identifier);
    const leftVisibleLive = leftLive && !left.ignored;
    const rightVisibleLive = rightLive && !right.ignored;
    if (leftVisibleLive !== rightVisibleLive) return leftVisibleLive ? -1 : 1;
    if (leftLive !== rightLive) return leftLive ? -1 : 1;
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder;
    return left.identifier.localeCompare(right.identifier);
  });
});
const ruleMetricOptions = computed(() => deviceMetrics.value.filter((metric) => !metric.ignored));
const numericMetricKeys = computed(() => {
  const keys = new Set<string>();
  for (const item of telemetryItems.value) {
    for (const [key, value] of Object.entries(item.metrics)) {
      if (key.trim() && typeof value === 'number' && Number.isFinite(value) && metricEnabled(key)) keys.add(key);
    }
  }
  return [...keys].sort((left, right) => {
    const leftOrder = metricSortOrder(left);
    const rightOrder = metricSortOrder(right);
    if (leftOrder !== rightOrder) return leftOrder - rightOrder;
    return left.localeCompare(right);
  });
});
const selectedMetricValues = computed(() => trendPoints.value.map((point) => point.value).filter((value) => Number.isFinite(value)));
const selectedMetricLatestValue = computed(() => {
  const value = latestMetrics.value.get(selectedMetric.value)?.value;
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
});
const selectedMetricStats = computed(() => {
  const values = selectedMetricValues.value;
  if (!values.length && selectedMetricLatestValue.value === undefined) return { latest: '-', min: '-', max: '-', avg: '-' };
  const total = values.reduce((sum, value) => sum + value, 0);
  return {
    latest: selectedMetricLatestValue.value === undefined ? '-' : formatNumber(selectedMetricLatestValue.value),
    min: values.length ? formatNumber(Math.min(...values)) : '-',
    max: values.length ? formatNumber(Math.max(...values)) : '-',
    avg: values.length ? formatNumber(total / values.length) : '-',
  };
});
const latestTelemetryTime = computed(() => latest.value?.time ?? device.value?.lastSeenAt);
const telemetryState = computed(() => {
  // 区分“浏览器实时通道状态”和“设备是否真的在线/上报”，避免文案误导。
  if (streamStatus.value === 'CONNECTING') return { dot: 'connecting', label: '正在连接实时通道' };
  if (streamStatus.value === 'DISCONNECTED') return { dot: 'disconnected', label: '实时通道已断开' };
  if (device.value?.status !== 'ONLINE') return { dot: 'idle', label: '实时通道已连接，设备未在线' };
  if (!latest.value) return { dot: 'idle', label: '实时通道已连接，等待设备上报' };
  return { dot: 'live', label: '设备正在实时上报' };
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
  const metric = deviceMetrics.value.find((item) => item.identifier === key);
  return metric ? !metric.ignored : true;
}

function metricSortOrder(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.sortOrder ?? 999;
}

function deviceTypeLabel(type?: string) {
  return { GATEWAY: '网关', GATEWAY_CHILD: '网关子设备', DIRECT: '直连设备' }[type ?? ''] ?? type ?? '-';
}

function statusType(status?: string) {
  return status === 'ONLINE' ? 'success' : 'info';
}

function statusLabel(status?: string) {
  return { ONLINE: '在线', OFFLINE: '离线', DISABLED: '停用' }[status ?? ''] ?? status ?? '-';
}

function childHeartbeatTopic(childKey: string) {
  return `weikong/gateways/${device.value?.deviceKey ?? '<网关编号>'}/children/${childKey}/heartbeat`;
}

function childTelemetryTopic(childKey: string) {
  return `weikong/gateways/${device.value?.deviceKey ?? '<网关编号>'}/children/${childKey}/telemetry`;
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

function formatDateTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', { hour12: false });
}

function addTelemetry(event: TelemetryEvent) {
  event = sanitizeTelemetryEvent(event);
  if (!Object.keys(event.metrics).length) return;
  // 暂停画面时不丢数据，先放进队列，恢复后再合并显示。
  if (paused.value) {
    pausedQueue.value = [event, ...pausedQueue.value].slice(0, RECENT_TELEMETRY_LIMIT);
    return;
  }
  pendingTelemetryEvents.push(event);
  if (telemetryFlushTimer) return;
  // 高频采集时先做极短缓冲，避免每条 SSE 都触发表格和卡片整块重绘。
  telemetryFlushTimer = setTimeout(flushTelemetryEvents, 180);
}

function flushTelemetryEvents() {
  telemetryFlushTimer = undefined;
  const events = pendingTelemetryEvents.splice(0);
  if (!events.length) return;
  const newestFirst = [...events].reverse();
  mergeTelemetryItems(newestFirst);
  prependRecentTelemetryItems(newestFirst);
  if (events.some((item) => Object.keys(item.metrics).some((key) => !deviceMetrics.value.some((metric) => metric.identifier === key)))) {
    if (settingsRefreshTimer) clearTimeout(settingsRefreshTimer);
    settingsRefreshTimer = setTimeout(() => void loadSettings(undefined, true), 600);
  }
  if (device.value) {
    device.value.status = 'ONLINE';
  }
  if (events.some((item) => item.metrics[selectedMetric.value] !== undefined)) scheduleTrendRefresh();
}

function sanitizeTelemetryEvent(event: TelemetryEvent): TelemetryEvent {
  return {
    ...event,
    metrics: Object.fromEntries(Object.entries(event.metrics ?? {}).filter(([key]) => key.trim().length > 0)),
  };
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '');
}

async function refreshDevice() {
  const id = String(route.params.id);
  const data = await apiRequest<DeviceItem>({ url: `/api/devices/${id}`, method: 'GET', silentError: true });
  if (id !== String(route.params.id)) return;
  device.value = data;
  if (data.mqtt) credentials.value = data.mqtt;
}

function handleDeviceStatusEvent(event: Event) {
  const detail = (event as CustomEvent<Partial<DeviceItem>>).detail;
  if (!device.value || !detail?.id || !detail.status) return;
  if (detail.id === device.value.id) {
    device.value.status = detail.status;
    if (detail.lastSeenAt) device.value.lastSeenAt = detail.lastSeenAt;
  }
  if (detail.id === device.value.gateway?.id) {
    device.value.gateway.status = detail.status;
    if (detail.lastSeenAt) device.value.gateway.lastSeenAt = detail.lastSeenAt;
  }
  const child = device.value.children?.find((item) => item.id === detail.id);
  if (child) {
    child.status = detail.status;
    if (detail.lastSeenAt) child.lastSeenAt = detail.lastSeenAt;
  }
}

async function refreshPage() {
  const id = String(route.params.id);
  await Promise.all([loadPage(id), loadSettings(id), loadCredentials(id)]);
}

async function loadCredentials(id = String(route.params.id)) {
  const data = await apiRequest<DeviceCredentials>({ url: `/api/devices/${id}/credentials`, method: 'GET' });
  if (id === String(route.params.id)) credentials.value = data;
}

async function rotateCredentials() {
  rotatingCredentials.value = true;
  try {
    credentials.value = await apiRequest<DeviceCredentials>({
      url: `/api/devices/${String(route.params.id)}/credentials/rotate`,
      method: 'POST',
    });
    ElMessage.success('新 MQTT 密码已生成，请及时更新真实设备配置');
  } finally {
    rotatingCredentials.value = false;
  }
}

async function loadRemoteConfigs(id = String(route.params.id), silentError = false) {
  remoteConfigLoading.value = true;
  try {
    const tasks = await apiRequest<RemoteConfigTask[]>({
      url: `/api/devices/${id}/remote-configs`,
      method: 'GET',
      silentError,
    });
    if (id === String(route.params.id)) {
      remoteConfigTasks.value = tasks;
      if (!remoteConfigInitialized.value) {
        const interval = latestAppliedRemoteValue('collectIntervalSeconds', tasks);
        if (typeof interval === 'number' && Number.isFinite(interval)) remoteConfigForm.collectIntervalSeconds = interval;
        remoteConfigInitialized.value = true;
      }
    }
  } finally {
    remoteConfigLoading.value = false;
  }
}

async function loadCurrentRemoteConfig(id = String(route.params.id), silentError = false) {
  remoteSnapshotLoading.value = true;
  try {
    const data = await apiRequest<{ config: RemoteConfigSnapshot; reportedAt?: string; stale?: boolean }>({
      url: `/api/devices/${id}/remote-configs/current`,
      method: 'GET',
      silentError,
    });
    if (id !== String(route.params.id)) return;
    remoteSnapshot.value = data.config;
    remoteSnapshotAt.value = data.reportedAt;
    remoteConfigForm.collectIntervalSeconds = Number(data.config.collectIntervalSeconds || 1);
    remoteDevices.value = JSON.parse(JSON.stringify(data.config.devices ?? [])) as RemoteDeviceConfig[];
  } finally {
    remoteSnapshotLoading.value = false;
  }
}

function prepareRemoteConfig() {
  let configPatch: Record<string, unknown>;
  if (remoteConfigMode.value === 'basic') {
    if (!remoteSnapshot.value) {
      ElMessage.error('请先读取网关当前配置');
      return;
    }
    if (!validateRemoteDevices()) return;
    configPatch = {
      collectIntervalSeconds: remoteConfigForm.collectIntervalSeconds,
      devices: remoteDevices.value,
    };
  } else {
    try {
      const parsed = JSON.parse(remoteConfigText.value) as unknown;
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error();
      configPatch = parsed as Record<string, unknown>;
    } catch {
      ElMessage.error('请输入有效的 JSON 对象');
      return;
    }
  }
  pendingRemoteConfig.value = configPatch;
  remoteConfigDiffRows.value = buildRemoteConfigDiff(configPatch);
  remoteConfigConfirmVisible.value = true;
}

async function sendRemoteConfig() {
  if (!pendingRemoteConfig.value) return;
  sendingRemoteConfig.value = true;
  try {
    await apiRequest<RemoteConfigTask>({
      url: `/api/devices/${String(route.params.id)}/remote-configs`,
      method: 'POST',
      data: { config: pendingRemoteConfig.value },
    });
    remoteConfigConfirmVisible.value = false;
    pendingRemoteConfig.value = undefined;
    ElMessage.success('远程配置已下发，正在等待网关回执');
    await loadRemoteConfigs();
  } finally {
    sendingRemoteConfig.value = false;
  }
}

async function retryRemoteConfig(task: RemoteConfigTask) {
  await apiRequest<RemoteConfigTask>({
    url: `/api/devices/${String(route.params.id)}/remote-configs/${task.id}/retry`,
    method: 'POST',
  });
  ElMessage.success(`版本 V${task.version} 已重新下发`);
  await loadRemoteConfigs();
}

function useRemoteConfig(task: RemoteConfigTask) {
  remoteConfigText.value = JSON.stringify(task.config, null, 2);
  remoteConfigMode.value = 'advanced';
}

function latestAppliedRemoteValue(field: string, tasks = remoteConfigTasks.value) {
  return tasks.find((task) => task.status === 'APPLIED' && task.config[field] !== undefined)?.config[field];
}

function displayRemoteConfigValue(value: unknown) {
  if (value === undefined) return '平台暂无记录';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

function buildRemoteConfigDiff(configPatch: Record<string, unknown>): RemoteConfigDiffRow[] {
  const labels: Record<string, string> = { collectIntervalSeconds: '采集周期（秒）', devices: '设备与点位' };
  return Object.entries(configPatch).map(([field, value]) => ({
    field,
    label: labels[field] ?? field,
    before: field === 'devices' ? remoteDeviceSummary(remoteSnapshot.value?.devices) : displayRemoteConfigValue(remoteSnapshot.value?.[field as keyof RemoteConfigSnapshot] ?? latestAppliedRemoteValue(field)),
    after: field === 'devices' ? remoteDeviceChangeSummary(remoteSnapshot.value?.devices ?? [], value as RemoteDeviceConfig[]) : displayRemoteConfigValue(value),
  }));
}

function remoteDeviceSummary(devices?: RemoteDeviceConfig[]) {
  const items = devices ?? [];
  return `${items.length} 台设备 / ${items.reduce((total, item) => total + (item.points?.length ?? 0), 0)} 个点位`;
}

function remoteDeviceChangeSummary(before: RemoteDeviceConfig[], after: RemoteDeviceConfig[]) {
  const beforeDevices = new Map(before.map((item) => [item.deviceKey, item]));
  const afterDevices = new Map(after.map((item) => [item.deviceKey, item]));
  const addedDevices = after.filter((item) => !beforeDevices.has(item.deviceKey)).length;
  const removedDevices = before.filter((item) => !afterDevices.has(item.deviceKey)).length;
  const changedDevices = after.filter((item) => {
    const previous = beforeDevices.get(item.deviceKey);
    if (!previous) return false;
    const { points: previousPoints, ...previousDevice } = previous;
    const { points: nextPoints, ...nextDevice } = item;
    return JSON.stringify(previousDevice) !== JSON.stringify(nextDevice);
  }).length;
  let addedPoints = 0;
  let removedPoints = 0;
  let changedPoints = 0;
  for (const device of after) {
    const previous = beforeDevices.get(device.deviceKey);
    if (!previous) {
      addedPoints += device.points?.length ?? 0;
      continue;
    }
    const previousPoints = new Map((previous.points ?? []).map((point) => [point.metric, point]));
    const nextPoints = new Map((device.points ?? []).map((point) => [point.metric, point]));
    addedPoints += (device.points ?? []).filter((point) => !previousPoints.has(point.metric)).length;
    removedPoints += (previous.points ?? []).filter((point) => !nextPoints.has(point.metric)).length;
    changedPoints += (device.points ?? []).filter((point) => {
      const oldPoint = previousPoints.get(point.metric);
      return oldPoint && JSON.stringify(oldPoint) !== JSON.stringify(point);
    }).length;
  }
  for (const device of before) {
    if (!afterDevices.has(device.deviceKey)) removedPoints += device.points?.length ?? 0;
  }
  return `${remoteDeviceSummary(after)}；设备 +${addedDevices} / -${removedDevices} / 改${changedDevices}；点位 +${addedPoints} / -${removedPoints} / 改${changedPoints}`;
}

function createRemoteDevice(): RemoteDeviceConfig {
  const index = remoteDevices.value.length + 1;
  return { deviceKey: `device-${String(index).padStart(3, '0')}`, name: `设备${index}`, protocol: 'modbus-tcp', address: '', slaveId: 1, interfaceType: 'network', interfaceName: '', points: [] };
}

function createRemotePoint(device: RemoteDeviceConfig): RemotePointConfig {
  const index = device.points.length + 1;
  return { deviceKey: device.deviceKey, name: `点位${index}`, metric: `point_${index}`, protocol: device.protocol, address: device.address, slaveId: device.slaveId, function: 3, register: 0, quantity: 1, dataType: 'uint16', byteOrder: 'big', wordOrder: 'normal', scale: 1, offset: 0, decimals: 0 };
}

function addRemoteDevice() {
  remoteDevices.value.push(createRemoteDevice());
}

function copyRemoteDevice(index: number) {
  const copy = JSON.parse(JSON.stringify(remoteDevices.value[index])) as RemoteDeviceConfig;
  copy.deviceKey = `${copy.deviceKey}-copy`;
  copy.name = `${copy.name}副本`;
  copy.points.forEach((point) => { point.deviceKey = copy.deviceKey; });
  remoteDevices.value.splice(index + 1, 0, copy);
}

function removeRemoteDevice(index: number) {
  remoteDevices.value.splice(index, 1);
}

function addRemotePoint(device: RemoteDeviceConfig) {
  device.points.push(createRemotePoint(device));
}

function copyRemotePoint(device: RemoteDeviceConfig, index: number) {
  const copy = JSON.parse(JSON.stringify(device.points[index])) as RemotePointConfig;
  copy.name = `${copy.name}副本`;
  copy.metric = `${copy.metric}_copy`;
  device.points.splice(index + 1, 0, copy);
}

function removeRemotePoint(device: RemoteDeviceConfig, index: number) {
  device.points.splice(index, 1);
}

function syncRemoteDevice(device: RemoteDeviceConfig) {
  device.points.forEach((point) => {
    point.deviceKey = device.deviceKey;
    point.protocol = device.protocol;
    point.address = device.address;
    point.slaveId = device.slaveId;
  });
}

function validateRemoteDevices() {
  const deviceKeys = new Set<string>();
  for (const item of remoteDevices.value) {
    item.deviceKey = item.deviceKey.trim();
    item.name = item.name.trim();
    if (!item.deviceKey || !item.name || !item.protocol || !item.address) {
      ElMessage.error('设备编号、名称、协议和地址不能为空');
      return false;
    }
    if (deviceKeys.has(item.deviceKey)) {
      ElMessage.error(`设备编号重复：${item.deviceKey}`);
      return false;
    }
    deviceKeys.add(item.deviceKey);
    syncRemoteDevice(item);
    const metrics = new Set<string>();
    for (const point of item.points) {
      point.name = point.name.trim();
      point.metric = point.metric.trim();
      if (!point.name || !point.metric) {
        ElMessage.error(`设备 ${item.name} 存在名称或标识符为空的点位`);
        return false;
      }
      if (metrics.has(point.metric)) {
        ElMessage.error(`设备 ${item.name} 的点位标识符重复：${point.metric}`);
        return false;
      }
      metrics.add(point.metric);
    }
  }
  return true;
}

function remoteConfigStatusLabel(status: RemoteConfigTask['status']) {
  return { PENDING: '待发送', SENT: '等待回执', APPLIED: '已生效', FAILED: '失败', TIMEOUT: '超时' }[status];
}

function remoteConfigStatusType(status: RemoteConfigTask['status']) {
  if (status === 'APPLIED') return 'success';
  if (status === 'FAILED') return 'danger';
  if (status === 'TIMEOUT') return 'warning';
  return 'info';
}

async function loadSettings(id = String(route.params.id), silentError = false) {
  const [metricsData, alarmRulesData] = await Promise.all([
    apiRequest<DeviceMetric[]>({ url: `/api/devices/${id}/metrics`, method: 'GET', silentError }),
    apiRequest<AlarmRule[]>({ url: `/api/devices/${id}/alarm-rules`, method: 'GET', silentError }),
  ]);
  if (id !== String(route.params.id)) return;
  deviceMetrics.value = metricsData;
  alarmRules.value = alarmRulesData;
}

async function openSettings(tab = 'metrics') {
  const id = String(route.params.id);
  settingsTab.value = tab;
  settingsVisible.value = true;
  settingsLoading.value = true;
  try {
    const [deviceTemplates, templateOptions] = await Promise.all([
      apiRequest<ModelTemplateItem[]>({ url: '/api/devices/model-templates', method: 'GET' }),
      apiRequest<DeviceModelTemplateOption[]>({ url: '/api/model-templates/options', method: 'GET' }),
      loadSettings(id),
    ]);
    if (id !== String(route.params.id)) return;
    modelTemplates.value = deviceTemplates;
    deviceModelTemplates.value = templateOptions;
    if (importForm.templateDeviceId === id) importForm.templateDeviceId = '';
  } finally {
    settingsLoading.value = false;
  }
}

async function saveMetric(metric: DeviceMetric) {
  await apiRequest({ url: `/api/devices/${String(route.params.id)}/metrics/${metric.id}`, method: 'PATCH', data: metric });
  ElMessage.success('指标配置已保存');
}

async function toggleMetricIgnored(metric: DeviceMetric) {
  // 忽略是软隐藏，不删除历史遥测；恢复后字段仍可继续展示。
  const nextIgnored = !metric.ignored;
  await apiRequest({
    url: `/api/devices/${String(route.params.id)}/metrics/${metric.id}`,
    method: 'PATCH',
    data: { ignored: nextIgnored },
  });
  metric.ignored = nextIgnored;
  ElMessage.success(nextIgnored ? '字段已忽略' : '字段已恢复');
  await renderTrendChart();
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
  // 拖拽卡片本质上是更新设备物模型字段 sortOrder。
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
  if (!importForm.modelTemplateId && !importForm.templateDeviceId) {
    ElMessage.warning('请选择物模型模板或已有设备');
    return;
  }
  const result = await apiRequest<{ created: number; updated: number; skipped: number }>({
    url: `/api/devices/${String(route.params.id)}/metrics/import`,
    method: 'POST',
    data: {
      modelTemplateId: importForm.modelTemplateId || undefined,
      templateDeviceId: importForm.modelTemplateId ? undefined : importForm.templateDeviceId || undefined,
      overwrite: importForm.overwrite,
    },
  });
  ElMessage.success(`物模型已导入：新增 ${result.created} 个，更新 ${result.updated} 个，跳过 ${result.skipped} 个`);
  await loadSettings();
}

function openRuleDialog(rule?: AlarmRule) {
  ruleEditingId.value = rule?.id ?? '';
  if (rule) {
    Object.assign(ruleForm, {
      identifier: rule.identifier,
      operator: rule.operator,
      threshold: rule.threshold,
      hysteresis: rule.hysteresis,
      level: rule.level,
    });
    ruleDialogVisible.value = true;
    return;
  }
  Object.assign(ruleForm, { identifier: ruleMetricOptions.value[0]?.identifier ?? '', operator: '>', threshold: 0, hysteresis: 0, level: 'WARNING' });
  ruleDialogVisible.value = true;
}

async function saveRule() {
  const method = ruleEditingId.value ? 'PATCH' : 'POST';
  const url = ruleEditingId.value
    ? `/api/devices/${String(route.params.id)}/alarm-rules/${ruleEditingId.value}`
    : `/api/devices/${String(route.params.id)}/alarm-rules`;
  await apiRequest({ url, method, data: ruleForm });
  ElMessage.success(ruleEditingId.value ? '告警规则已更新' : '告警规则已保存');
  ruleDialogVisible.value = false;
  ruleEditingId.value = '';
  await loadSettings();
}

async function toggleRuleEnabled(rule: AlarmRule) {
  const enabled = !rule.enabled;
  await apiRequest({
    url: `/api/devices/${String(route.params.id)}/alarm-rules/${rule.id}`,
    method: 'PATCH',
    data: { enabled },
  });
  rule.enabled = enabled;
  ElMessage.success(enabled ? '告警规则已启用' : '告警规则已停用');
}

function alarmLevelLabel(level: string) {
  return { INFO: '提示', WARNING: '警告', CRITICAL: '严重' }[level] ?? level;
}

function alarmLevelType(level: string) {
  return { INFO: 'info', WARNING: 'warning', CRITICAL: 'danger' }[level] ?? 'info';
}

function deviceLogTypeLabel(type: string) {
  return { ONLINE: '上线', OFFLINE: '离线', MQTT_CONNECTED: 'MQTT连接', MQTT_DISCONNECTED: 'MQTT断开' }[type] ?? type;
}

function deviceLogTypeTag(type: string) {
  if (type === 'ONLINE' || type === 'MQTT_CONNECTED') return 'success';
  if (type === 'OFFLINE' || type === 'MQTT_DISCONNECTED') return 'info';
  return 'warning';
}

function formatDeviceLogDetail(detail?: Record<string, unknown>) {
  return detail ? JSON.stringify(detail) : '-';
}

async function removeRule(rule: AlarmRule) {
  await apiRequest({ url: `/api/devices/${String(route.params.id)}/alarm-rules/${rule.id}`, method: 'DELETE' });
  ElMessage.success('告警规则已删除');
  await loadSettings();
}

function togglePause() {
  paused.value = !paused.value;
  if (!paused.value && pausedQueue.value.length) {
    mergeTelemetryItems(pausedQueue.value);
    prependRecentTelemetryItems(pausedQueue.value);
    if (device.value) {
      device.value.status = 'ONLINE';
    }
    pausedQueue.value = [];
  }
}

async function renderTrendChart() {
  await nextTick();
  if (!trendChartRef.value || !selectedMetric.value) return;
  trendChart ??= echarts.init(trendChartRef.value);
  const points = trendPoints.value;
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    grid: { left: 46, right: 18, top: 24, bottom: 28 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: points.map((item) => formatTrendTime(item.time)),
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
      data: points.map((item) => item.value),
      lineStyle: { color: '#2563eb', width: 3 },
      areaStyle: { color: 'rgba(37, 99, 235, 0.08)' },
    }],
  });
}

async function loadTrend(id = String(route.params.id), silentError = false) {
  // 趋势数据走后端聚合接口，避免前端一次性拉大量历史遥测。
  if (!selectedMetric.value) {
    trendPoints.value = [];
    trendChart?.clear();
    return;
  }
  const showLoading = !silentError || !trendPoints.value.length;
  if (showLoading) trendLoading.value = true;
  try {
    const data = await apiRequest<{ items: TrendPoint[] }>({
      url: `/api/devices/${id}/telemetry/trend`,
      method: 'GET',
      params: { metric: selectedMetric.value, range: selectedTrendRange.value },
      silentError,
    });
    if (id !== String(route.params.id)) return;
    trendPoints.value = data.items;
    await renderTrendChart();
  } finally {
    if (showLoading) trendLoading.value = false;
  }
}

function scheduleTrendRefresh() {
  if (trendRefreshTimer) clearTimeout(trendRefreshTimer);
  trendRefreshTimer = setTimeout(() => void loadTrend(undefined, true), selectedTrendRange.value === '1m' ? 800 : 1500);
}

function formatTrendTime(time: string) {
  const date = new Date(time);
  return ['1d', '1w'].includes(selectedTrendRange.value)
    ? `${date.getMonth() + 1}-${date.getDate()} ${date.toLocaleTimeString()}`
    : date.toLocaleTimeString();
}

function resizeChart() {
  trendChart?.resize();
}

function normalizeTelemetryItems(deviceKey: string, items: Array<{ time: string; deviceKey?: string; metrics: Record<string, unknown> }>) {
  return items
    .map((item) => sanitizeTelemetryEvent({ deviceKey, ...item }))
    .filter((item) => Object.keys(item.metrics).length > 0);
}

function telemetryRowKey(row?: TelemetryEvent) {
  if (!row) return '';
  return `${row.deviceKey}-${row.time}-${JSON.stringify(row.metrics)}`;
}

function telemetryTimeValue(item: TelemetryEvent) {
  return new Date(item.time).getTime() || 0;
}

function sameTelemetryRows(left: TelemetryEvent[], right: TelemetryEvent[]) {
  return left.map(telemetryRowKey).join('|') === right.map(telemetryRowKey).join('|');
}

function stableTelemetryRows(items: TelemetryEvent[], oldItems: TelemetryEvent[]) {
  const oldByKey = new Map(oldItems.map((item) => [telemetryRowKey(item), item]));
  const uniqueByKey = new Map<string, TelemetryEvent>();
  for (const item of items) {
    const key = telemetryRowKey(item);
    if (!uniqueByKey.has(key)) uniqueByKey.set(key, oldByKey.get(key) ?? item);
  }
  return [...uniqueByKey.values()]
    .sort((left, right) => telemetryTimeValue(right) - telemetryTimeValue(left))
    .slice(0, RECENT_TELEMETRY_LIMIT);
}

function mergeTelemetryItems(items: TelemetryEvent[]) {
  const nextRows = stableTelemetryRows([...items, ...telemetryItems.value], telemetryItems.value);
  if (sameTelemetryRows(telemetryItems.value, nextRows)) return;
  telemetryItems.value = nextRows;
}

function mergeRecentTelemetryItems(items: TelemetryEvent[]) {
  const nextRows = stableTelemetryRows(items, recentTelemetryItems.value);
  if (sameTelemetryRows(recentTelemetryItems.value, nextRows)) return;
  recentTelemetryItems.value = nextRows;
}

function prependRecentTelemetryItems(items: TelemetryEvent[]) {
  mergeRecentTelemetryItems([...items, ...recentTelemetryItems.value]);
}

async function loadPage(id = String(route.params.id)) {
  const telemetryPromise = apiRequest<{ deviceKey: string; items: Array<{ time: string; metrics: Record<string, unknown> }> }>({
    url: `/api/devices/${id}/telemetry`,
    method: 'GET',
  });
  const deviceData = await apiRequest<DeviceItem>({ url: `/api/devices/${id}`, method: 'GET' });
  if (id !== String(route.params.id)) return;
  device.value = deviceData;
  if (deviceData.mqtt) credentials.value = deviceData.mqtt;
  const telemetryData = await telemetryPromise;
  if (id !== String(route.params.id)) return;
  telemetryItems.value = normalizeTelemetryItems(telemetryData.deviceKey, telemetryData.items);
  mergeTelemetryItems([]);
  mergeRecentTelemetryItems(telemetryItems.value);
}

async function refreshRecentTelemetry(id = String(route.params.id), silentError = true) {
  const data = await apiRequest<{ deviceKey: string; items: Array<{ time: string; deviceKey?: string; metrics: Record<string, unknown> }> }>({
    url: `/api/devices/${id}/telemetry`,
    method: 'GET',
    silentError,
  });
  if (id !== String(route.params.id)) return;
  const items = normalizeTelemetryItems(data.deviceKey, data.items);
  mergeTelemetryItems(items);
  mergeRecentTelemetryItems(items);
}

async function loadDeviceLogs(id = String(route.params.id), silentError = false) {
  deviceLogsLoading.value = true;
  try {
    const data = await apiRequest<{ items: DeviceLogItem[]; total: number }>({
      url: '/api/device-logs',
      method: 'GET',
      params: { ...deviceLogQuery, deviceId: id },
      silentError,
    });
    if (id !== String(route.params.id)) return;
    deviceLogs.value = data.items;
    deviceLogsTotal.value = data.total;
  } finally {
    deviceLogsLoading.value = false;
  }
}

async function connectStream(id = String(route.params.id)) {
  const controller = new AbortController();
  streamController = controller;
  streamStatus.value = 'CONNECTING';
  const baseURL = import.meta.env.VITE_API_BASE_URL ?? '';
  try {
    // 设备详情遥测也用 fetch 读取 SSE，方便携带 JWT。
    const response = await fetch(`${baseURL}/api/devices/${id}/telemetry/stream`, {
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
  if (id !== String(route.params.id)) return;
  streamStatus.value = 'DISCONNECTED';
  reconnectTimer = setTimeout(() => {
    if (id === String(route.params.id)) void connectStream(id);
  }, 3000);
}

function stopTelemetryStream() {
  streamController?.abort();
  streamController = undefined;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  reconnectTimer = undefined;
}

function resetPageState() {
  stopTelemetryStream();
  if (telemetryFlushTimer) clearTimeout(telemetryFlushTimer);
  telemetryFlushTimer = undefined;
  pendingTelemetryEvents.splice(0);
  device.value = undefined;
  telemetryItems.value = [];
  recentTelemetryItems.value = [];
  pausedQueue.value = [];
  deviceMetrics.value = [];
  alarmRules.value = [];
  credentials.value = undefined;
  deviceLogs.value = [];
  deviceLogsTotal.value = 0;
  remoteConfigTasks.value = [];
  remoteDevices.value = [];
  remoteSnapshot.value = undefined;
  remoteSnapshotAt.value = undefined;
  remoteConfigInitialized.value = false;
  pendingRemoteConfig.value = undefined;
  remoteConfigConfirmVisible.value = false;
  deviceLogQuery.page = 1;
  deviceLogQuery.type = '';
  selectedMetric.value = '';
  trendPoints.value = [];
  streamStatus.value = 'CONNECTING';
}

async function loadDeviceDetail(id = String(route.params.id)) {
  resetPageState();
  await Promise.all([loadPage(id), loadSettings(id), loadCredentials(id), loadDeviceLogs(id, true)]);
  if (id !== String(route.params.id)) return;
  if (device.value?.deviceType === 'GATEWAY') await Promise.all([loadRemoteConfigs(id, true), loadCurrentRemoteConfig(id, true)]);
  selectedMetric.value = numericMetricKeys.value[0] ?? '';
  await loadTrend(id);
  void connectStream(id);
}

function formatMetrics(item: Record<string, unknown>) {
  return Object.entries(item)
    .filter(([key]) => key.trim() && metricEnabled(key))
    .sort(([left], [right]) => metricSortOrder(left) - metricSortOrder(right))
    .map(([key, value]) => `${metricLabel(key)}: ${displayValue(value, key)}${metricUnit(key)}`)
    .join(' · ');
}

onMounted(async () => {
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  statusTimer = setInterval(() => void refreshDevice(), 30000);
  recentTelemetryTimer = setInterval(() => void refreshRecentTelemetry(), 2000);
  remoteConfigTimer = setInterval(() => {
    if (device.value?.deviceType === 'GATEWAY') void loadRemoteConfigs(undefined, true);
  }, 5000);
  window.addEventListener('resize', resizeChart);
  await loadDeviceDetail();
});

onBeforeUnmount(() => {
  stopTelemetryStream();
  if (statusTimer) clearInterval(statusTimer);
  if (recentTelemetryTimer) clearInterval(recentTelemetryTimer);
  if (settingsRefreshTimer) clearTimeout(settingsRefreshTimer);
  if (trendRefreshTimer) clearTimeout(trendRefreshTimer);
  if (telemetryFlushTimer) clearTimeout(telemetryFlushTimer);
  if (remoteConfigTimer) clearInterval(remoteConfigTimer);
  window.removeEventListener('device-status-change', handleDeviceStatusEvent);
  window.removeEventListener('resize', resizeChart);
  trendChart?.dispose();
});

watch(numericMetricKeys, (keys) => {
  if (!keys.includes(selectedMetric.value)) selectedMetric.value = keys[0] ?? '';
});

watch([selectedMetric, selectedTrendRange], () => void loadTrend());

watch(() => route.params.id, (id, oldId) => {
  if (!id || id === oldId) return;
  void loadDeviceDetail(String(id));
});
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
        <span :class="['dot', telemetryState.dot]" />
        {{ telemetryState.label }}
        <el-button v-if="canManage" text :icon="Setting" class="settings-button" @click="openSettings('metrics')">物模型配置</el-button>
        <el-button v-if="canManage" text class="settings-button secondary" @click="openSettings('rules')">告警规则</el-button>
      </div>
    </div>

    <nav class="detail-tabs" aria-label="设备详情分区">
      <button :class="{ active: detailTab === 'monitor' }" type="button" @click="detailTab = 'monitor'">
        实时监控
        <span>{{ metrics.length }}</span>
      </button>
      <button :class="{ active: detailTab === 'config' }" type="button" @click="detailTab = 'config'">
        设备配置
      </button>
      <button v-if="canManage" :class="{ active: detailTab === 'logs' }" type="button" @click="detailTab = 'logs'">
        设备日志
        <span>{{ deviceLogsTotal }}</span>
      </button>
    </nav>

    <section v-show="detailTab === 'config'" class="device-summary detail-section-first">
      <div><span>设备状态</span><el-tag :type="statusType(device?.status)">{{ statusLabel(device?.status) }}</el-tag></div>
      <div><span>设备类型</span><strong>{{ deviceTypeLabel(device?.deviceType) }}</strong></div>
      <div v-if="device?.deviceType === 'GATEWAY_CHILD'"><span>所属网关</span><strong>{{ device.gateway?.name || '-' }}</strong></div>
      <div><span>安装位置</span><strong>{{ device?.location || '-' }}</strong></div>
      <div><span>最近遥测</span><strong>{{ formatDateTime(latestTelemetryTime) }}</strong></div>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'config'" class="remote-config-panel">
      <div class="section-heading">
        <div><h2>远程配置</h2><p>通过 MQTT 向在线网关下发局部 JSON 配置，并跟踪网关执行回执。</p></div>
        <el-button :icon="RefreshRight" :loading="remoteConfigLoading" @click="loadRemoteConfigs()">刷新状态</el-button>
      </div>
      <el-alert class="remote-config-tip" type="warning" :closable="false" show-icon>
        <template #title>网关编号、激活信息、MQTT、Web 监听、网口和离线缓存属于本地保护项，远程下发时会忽略。</template>
      </el-alert>
      <div v-if="canManage" class="remote-config-editor">
        <el-segmented v-model="remoteConfigMode" :options="[{ label: '常用配置', value: 'basic' }, { label: '高级 JSON', value: 'advanced' }]" class="remote-config-mode" />
        <div v-if="remoteConfigMode === 'basic'" class="remote-basic-form">
          <div class="remote-field-card">
            <div>
              <strong>采集周期</strong>
              <p>网关每隔多少秒执行一次全部点位采集，推荐 1–10 秒。</p>
            </div>
            <div class="remote-interval-input">
              <el-input-number v-model="remoteConfigForm.collectIntervalSeconds" :min="1" :max="3600" :step="1" step-strictly controls-position="right" />
              <span>秒</span>
            </div>
          </div>
          <div class="remote-device-heading">
            <div>
              <strong>设备与点位</strong>
              <p>基于网关实时回传配置编辑。展开设备行可维护该设备的采集点位。</p>
              <small>读取于 {{ formatDateTime(remoteSnapshotAt) }}</small>
            </div>
            <div>
              <el-button :icon="RefreshRight" :loading="remoteSnapshotLoading" @click="loadCurrentRemoteConfig()">重新读取</el-button>
              <el-button type="primary" :icon="Plus" @click="addRemoteDevice">新增设备</el-button>
            </div>
          </div>
          <el-table v-loading="remoteSnapshotLoading" :data="remoteDevices" row-key="deviceKey" empty-text="网关当前没有采集设备" class="remote-device-table">
            <el-table-column type="expand" width="48">
              <template #default="{ row: remoteDevice }">
                <div class="remote-point-panel">
                  <div class="remote-device-extra">
                    <label><span>所属接口</span><el-input v-model="remoteDevice.interfaceName" placeholder="如 网口1 / Serial1" /></label>
                    <template v-if="remoteDevice.protocol === 'siemens-s7'">
                      <label><span>PLC 型号</span><el-select v-model="remoteDevice.plcModel"><el-option label="S7-200 SMART" value="s7-200-smart" /><el-option label="S7-1200/1500" value="s7-1200" /></el-select></label>
                      <label><span>Rack</span><el-input-number v-model="remoteDevice.rack" :min="0" controls-position="right" /></label>
                      <label><span>Slot</span><el-input-number v-model="remoteDevice.slot" :min="0" controls-position="right" /></label>
                    </template>
                    <template v-if="remoteDevice.protocol === 'opcua'">
                      <label><span>用户名</span><el-input v-model="remoteDevice.username" placeholder="留空为匿名" /></label>
                      <label><span>密码</span><el-input v-model="remoteDevice.password" type="password" show-password /></label>
                    </template>
                    <label v-if="remoteDevice.protocol === 'iec61850'"><span>IED 名称</span><el-input v-model="remoteDevice.iedName" /></label>
                  </div>
                  <div class="remote-point-heading"><strong>{{ remoteDevice.name }} 的点位</strong><el-button size="small" type="primary" plain :icon="Plus" @click="addRemotePoint(remoteDevice)">新增点位</el-button></div>
                  <el-table :data="remoteDevice.points" size="small" empty-text="暂无点位">
                    <el-table-column label="名称" min-width="130"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
                    <el-table-column label="标识符" min-width="140"><template #default="{ row }"><el-input v-model="row.metric" /></template></el-table-column>
                    <el-table-column label="协议地址" min-width="245">
                      <template #default="{ row }">
                        <div v-if="remoteDevice.protocol === 'opcua'" class="remote-inline-fields"><el-input v-model="row.nodeId" placeholder="NodeId" /></div>
                        <div v-else-if="remoteDevice.protocol === 'iec61850'" class="remote-inline-fields"><el-input v-model="row.objectRef" placeholder="对象引用" /><el-input v-model="row.fc" placeholder="FC" /></div>
                        <div v-else-if="remoteDevice.protocol === 'siemens-s7'" class="remote-inline-fields"><el-select v-model="row.area"><el-option v-for="area in ['V', 'DB', 'M', 'I', 'Q']" :key="area" :label="area" :value="area" /></el-select><el-input-number v-model="row.register" :min="0" controls-position="right" /></div>
                        <div v-else class="remote-inline-fields"><el-select v-model="row.function"><el-option v-for="fc in [1, 2, 3, 4]" :key="fc" :label="`FC${fc}`" :value="fc" /></el-select><el-input-number v-model="row.register" :min="0" controls-position="right" /></div>
                      </template>
                    </el-table-column>
                    <el-table-column label="数据类型" min-width="130"><template #default="{ row }"><el-select v-model="row.dataType"><el-option v-for="type in remoteDataTypes" :key="type" :label="type" :value="type" /></el-select></template></el-table-column>
                    <el-table-column label="数量" width="105"><template #default="{ row }"><el-input-number v-model="row.quantity" :min="1" :controls="false" /></template></el-table-column>
                    <el-table-column label="倍率" width="115"><template #default="{ row }"><el-input-number v-model="row.scale" :controls="false" /></template></el-table-column>
                    <el-table-column label="偏移" width="115"><template #default="{ row }"><el-input-number v-model="row.offset" :controls="false" /></template></el-table-column>
                    <el-table-column label="单位" width="100"><template #default="{ row }"><el-input v-model="row.unit" /></template></el-table-column>
                    <el-table-column label="操作" width="120" fixed="right"><template #default="{ $index }"><el-button link type="primary" @click="copyRemotePoint(remoteDevice, $index)">复制</el-button><el-button link type="danger" @click="removeRemotePoint(remoteDevice, $index)">删除</el-button></template></el-table-column>
                  </el-table>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="设备编号" min-width="145"><template #default="{ row }"><el-input v-model="row.deviceKey" @change="syncRemoteDevice(row)" /></template></el-table-column>
            <el-table-column label="设备名称" min-width="145"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
            <el-table-column label="协议" min-width="145"><template #default="{ row }"><el-select v-model="row.protocol" @change="syncRemoteDevice(row)"><el-option v-for="protocol in remoteProtocols" :key="protocol" :label="protocol" :value="protocol" /></el-select></template></el-table-column>
            <el-table-column label="地址" min-width="190"><template #default="{ row }"><el-input v-model="row.address" placeholder="IP:端口 / COM口" @change="syncRemoteDevice(row)" /></template></el-table-column>
            <el-table-column label="从站/公共地址" width="145"><template #default="{ row }"><el-input-number v-model="row.slaveId" :min="0" :max="255" controls-position="right" @change="syncRemoteDevice(row)" /></template></el-table-column>
            <el-table-column label="点位" width="75"><template #default="{ row }">{{ row.points?.length ?? 0 }}</template></el-table-column>
            <el-table-column label="操作" width="120" fixed="right"><template #default="{ $index }"><el-button link type="primary" @click="copyRemoteDevice($index)">复制</el-button><el-button link type="danger" @click="removeRemoteDevice($index)">删除</el-button></template></el-table-column>
          </el-table>
        </div>
        <div v-else class="remote-json-editor">
          <el-input v-model="remoteConfigText" type="textarea" :autosize="{ minRows: 5, maxRows: 14 }" spellcheck="false" placeholder="请输入 JSON 配置补丁" />
          <p>高级模式适合批量修改设备和点位；数组字段会整体替换，例如 <code>devices</code>、<code>points</code>。</p>
        </div>
        <div class="remote-config-actions">
          <span>点击后会先展示本次配置差异，不会直接下发。</span>
          <el-button type="primary" :disabled="remoteConfigMode === 'basic' && !remoteSnapshot" @click="prepareRemoteConfig">预览并下发</el-button>
        </div>
      </div>
      <el-table v-loading="remoteConfigLoading" :data="remoteConfigTasks" empty-text="暂无远程配置任务" class="remote-config-table">
        <el-table-column label="版本" width="80"><template #default="{ row }">V{{ row.version }}</template></el-table-column>
        <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag :type="remoteConfigStatusType(row.status)">{{ remoteConfigStatusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="createdBy" label="操作人" width="130" />
        <el-table-column label="下发时间" min-width="180"><template #default="{ row }">{{ formatDateTime(row.sentAt || row.createdAt) }}</template></el-table-column>
        <el-table-column label="完成时间" min-width="180"><template #default="{ row }">{{ formatDateTime(row.completedAt) }}</template></el-table-column>
        <el-table-column prop="error" label="结果" min-width="220" show-overflow-tooltip />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="useRemoteConfig(row)">复用</el-button>
            <el-button v-if="canManage && ['FAILED', 'TIMEOUT'].includes(row.status)" link type="primary" @click="retryRemoteConfig(row)">重试</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="remoteConfigConfirmVisible" title="确认远程配置" width="min(680px, 94vw)" :close-on-click-modal="!sendingRemoteConfig">
      <el-alert type="info" :closable="false" show-icon title="确认后配置将通过 MQTT 下发，网关校验通过后立即热加载。" />
      <el-table :data="remoteConfigDiffRows" class="remote-diff-table">
        <el-table-column prop="label" label="配置项" min-width="150" />
        <el-table-column prop="before" label="当前已知值" min-width="180" show-overflow-tooltip />
        <el-table-column prop="after" label="下发值" min-width="180" show-overflow-tooltip />
      </el-table>
      <template #footer>
        <el-button :disabled="sendingRemoteConfig" @click="remoteConfigConfirmVisible = false">取消</el-button>
        <el-button type="primary" :loading="sendingRemoteConfig" @click="sendRemoteConfig">确认下发</el-button>
      </template>
    </el-dialog>

    <section v-if="canManage" v-show="detailTab === 'logs'" class="table-panel device-log-panel detail-section-first">
      <div class="section-heading">
        <div><h2>设备日志</h2><p>当前设备的上线、离线和 MQTT 连接事件，排查链路时优先看这里</p></div>
        <div class="device-log-actions">
          <el-select v-model="deviceLogQuery.type" clearable placeholder="全部类型" size="small" style="width: 130px" @change="deviceLogQuery.page = 1; loadDeviceLogs()">
            <el-option label="上线" value="ONLINE" />
            <el-option label="离线" value="OFFLINE" />
            <el-option label="MQTT连接" value="MQTT_CONNECTED" />
            <el-option label="MQTT断开" value="MQTT_DISCONNECTED" />
          </el-select>
          <el-button size="small" :icon="RefreshRight" @click="loadDeviceLogs()">刷新</el-button>
        </div>
      </div>
      <el-table v-loading="deviceLogsLoading" :data="deviceLogs" empty-text="暂无设备日志">
        <el-table-column label="时间" width="180"><template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="类型" width="120"><template #default="{ row }"><el-tag :type="deviceLogTypeTag(row.type)">{{ deviceLogTypeLabel(row.type) }}</el-tag></template></el-table-column>
        <el-table-column prop="source" label="来源" width="130" />
        <el-table-column prop="message" label="内容" min-width="220" show-overflow-tooltip />
        <el-table-column label="详情" min-width="260" show-overflow-tooltip><template #default="{ row }">{{ formatDeviceLogDetail(row.detail) }}</template></el-table-column>
      </el-table>
      <div class="pagination compact-pagination">
        <el-pagination v-model:current-page="deviceLogQuery.page" v-model:page-size="deviceLogQuery.pageSize" layout="total, prev, pager, next" :total="deviceLogsTotal" @current-change="loadDeviceLogs" />
      </div>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY_CHILD' && device.gateway" v-show="detailTab === 'config'" class="relation-panel">
      <div class="section-heading">
        <div><h2>所属网关</h2><p>子设备绑定的上级网关状态会随设备上下线实时刷新</p></div>
      </div>
      <div class="gateway-card" @click="router.push(`/devices/${device.gateway.id}`)">
        <div>
          <strong>{{ device.gateway.name }}</strong>
          <code>{{ device.gateway.deviceKey }}</code>
        </div>
        <el-tag :type="statusType(device.gateway.status)">{{ statusLabel(device.gateway.status) }}</el-tag>
        <span>{{ formatDateTime(device.gateway.lastSeenAt) }}</span>
      </div>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'config'" class="relation-panel">
      <div class="section-heading">
        <div><h2>子设备</h2><p>网关使用自己的 MQTT 凭证代发下列子设备心跳和遥测</p></div>
      </div>
      <el-table :data="device.children ?? []" empty-text="暂无子设备">
        <el-table-column label="设备名称" min-width="150">
          <template #default="{ row }"><router-link :to="`/devices/${row.id}`">{{ row.name }}</router-link></template>
        </el-table-column>
        <el-table-column prop="deviceKey" label="设备编号" min-width="150" />
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="location" label="位置" min-width="140" />
        <el-table-column label="最后上报" min-width="180"><template #default="{ row }">{{ formatDateTime(row.lastSeenAt) }}</template></el-table-column>
        <el-table-column label="心跳 Topic" min-width="300"><template #default="{ row }"><code class="topic-code">{{ childHeartbeatTopic(row.deviceKey) }}</code></template></el-table-column>
        <el-table-column label="遥测 Topic" min-width="300"><template #default="{ row }"><code class="topic-code">{{ childTelemetryTopic(row.deviceKey) }}</code></template></el-table-column>
      </el-table>
    </section>

    <el-alert v-show="detailTab === 'config'" class="topic-tip" type="info" :closable="false" show-icon>
      <template #title>
        当前设备遥测主题：<code>weikong/devices/{{ device?.deviceKey }}/telemetry</code>
      </template>
    </el-alert>

    <el-collapse v-if="credentials" v-show="detailTab === 'config'" class="mqtt-collapse">
      <el-collapse-item name="mqtt">
        <template #title>
          <div class="mqtt-summary">
            <strong>MQTT 连接信息</strong>
            <span>{{ credentials.host }}:{{ credentials.port }}</span>
            <code>{{ credentials.clientId }}</code>
          </div>
        </template>
        <div class="mqtt-detail">
          <div class="mqtt-detail-head">
            <p>真实设备使用这组凭证连接平台，密码只会在创建设备或重置时显示一次。</p>
            <el-button v-if="canManage" type="primary" plain :loading="rotatingCredentials" @click.stop="rotateCredentials">重置密码</el-button>
          </div>
          <div class="mqtt-grid">
            <div><span>Broker</span><code>{{ credentials.host }}:{{ credentials.port }}</code></div>
            <div><span>WebSocket</span><code>{{ credentials.host }}:{{ credentials.wsPort }}</code></div>
            <div><span>Client ID</span><code>{{ credentials.clientId }}</code></div>
            <div><span>Username</span><code>{{ credentials.username }}</code></div>
            <div><span>Password</span><code>{{ credentials.password || '已隐藏，重置后显示新密码' }}</code></div>
            <div><span>更新时间</span><strong>{{ formatDateTime(credentials.passwordUpdatedAt) }}</strong></div>
          </div>
          <div class="topic-list">
            <div><span>心跳主题</span><code>{{ credentials.heartbeatTopic }}</code></div>
            <div><span>遥测主题</span><code>{{ credentials.telemetryTopic }}</code></div>
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>

    <section v-show="detailTab === 'monitor'" class="metrics-section detail-section-first">
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

    <section v-show="detailTab === 'monitor'" class="chart-panel">
      <div class="section-heading trend-heading">
        <div class="trend-title"><h2>实时趋势</h2><p>按时间范围聚合显示数值变化</p></div>
        <div class="trend-controls">
          <el-select v-model="selectedMetric" placeholder="选择指标" class="trend-metric-select">
            <el-option v-for="key in numericMetricKeys" :key="key" :label="metricLabel(key)" :value="key" />
          </el-select>
        </div>
      </div>
      <el-segmented v-model="selectedTrendRange" :options="trendRanges" size="small" class="trend-range-tabs" />
      <div v-if="selectedMetric" class="trend-stats">
        <div><span>最新值</span><strong>{{ selectedMetricStats.latest }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>最小值</span><strong>{{ selectedMetricStats.min }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>最大值</span><strong>{{ selectedMetricStats.max }}{{ metricUnit(selectedMetric) }}</strong></div>
        <div><span>平均值</span><strong>{{ selectedMetricStats.avg }}{{ metricUnit(selectedMetric) }}</strong></div>
      </div>
      <div v-if="numericMetricKeys.length" v-loading="trendLoading" ref="trendChartRef" class="trend-chart" />
      <el-empty v-else description="暂无可绘制的数值指标" />
    </section>

    <section v-show="detailTab === 'monitor'" class="table-panel history">
      <div class="section-heading"><div><h2>最近上报</h2><p>固定展示最新 20 条；第 21 条进入时，最早一条会自动移出</p></div></div>
      <el-table :data="visibleTelemetryItems" :row-key="telemetryRowKey" empty-text="暂无可展示的遥测数据">
        <el-table-column label="上报时间" width="190"><template #default="{ row }">{{ formatDateTime(row.time) }}</template></el-table-column>
        <el-table-column label="设备" width="150"><template #default="{ row }">{{ row.deviceKey }}</template></el-table-column>
        <el-table-column label="指标"><template #default="{ row }">{{ formatMetrics(row.metrics) }}</template></el-table-column>
      </el-table>
    </section>

    <el-drawer v-model="settingsVisible" title="设备配置" size="min(1120px, 96vw)" class="settings-drawer">
      <el-tabs v-model="settingsTab" v-loading="settingsLoading" class="settings-tabs">
        <el-tab-pane name="metrics">
          <template #label>物模型字段 <span class="tab-count">{{ deviceMetrics.length }}</span></template>
          <div class="drawer-heading sticky-heading"><div><h3>物模型</h3><p>设备首次上报字段后自动登记。配置会直接影响实时卡片、趋势图和历史记录展示。</p></div></div>
          <div v-if="canManage" class="model-import">
            <el-select v-model="importForm.modelTemplateId" clearable placeholder="选择物模型模板" class="template-select" @change="importForm.templateDeviceId = ''">
              <el-option v-for="template in importableModelTemplates" :key="template.id" :label="`${template.name} · ${template._count.metrics} 个字段`" :value="template.id" />
            </el-select>
            <el-select v-model="importForm.templateDeviceId" clearable placeholder="选择已有设备物模型" class="template-select" @change="importForm.modelTemplateId = ''">
              <el-option v-for="template in importableTemplates" :key="template.id" :label="`${template.name} (${template.deviceKey}) · ${template._count.metrics} 个字段`" :value="template.id" />
            </el-select>
            <el-switch v-model="importForm.overwrite" active-text="覆盖同名字段" />
            <el-button type="primary" plain @click="importMetrics">导入物模型</el-button>
          </div>
          <el-table :data="sortedDeviceMetrics" size="small" row-key="id" height="calc(100vh - 310px)">
            <el-table-column prop="identifier" label="标识符" min-width="105" show-overflow-tooltip />
            <el-table-column label="状态" width="82"><template #default="{ row }"><el-tag v-if="row.ignored" type="info">已忽略</el-tag><el-tag v-else-if="latest?.metrics?.[row.identifier] !== undefined" type="success">上报中</el-tag><span v-else>-</span></template></el-table-column>
            <el-table-column label="显示名称" min-width="120"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
            <el-table-column label="单位" width="72"><template #default="{ row }"><el-input v-model="row.unit" /></template></el-table-column>
            <el-table-column prop="dataType" label="类型" width="74" />
            <el-table-column label="权限" width="92"><template #default="{ row }"><el-select v-model="row.accessMode"><el-option label="只读" value="READ_ONLY" /><el-option label="读写" value="READ_WRITE" /></el-select></template></el-table-column>
            <el-table-column label="排序" width="88"><template #default="{ row }"><el-input-number v-model="row.sortOrder" class="metric-number" :min="0" controls-position="right" /></template></el-table-column>
            <el-table-column label="小数位" width="88"><template #default="{ row }"><el-input-number v-model="row.decimals" class="metric-number" :min="0" :max="6" controls-position="right" /></template></el-table-column>
            <el-table-column label="操作" width="108"><template #default="{ row }"><el-button link type="primary" @click="saveMetric(row)">保存</el-button><el-button link :type="row.ignored ? 'success' : 'warning'" @click="toggleMetricIgnored(row)">{{ row.ignored ? '恢复' : '忽略' }}</el-button></template></el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane name="rules">
          <template #label>告警规则 <span class="tab-count">{{ alarmRules.length }}</span></template>
          <div class="drawer-heading sticky-heading">
            <div><h3>告警规则</h3><p>数值满足条件时产生告警，恢复正常后自动关闭。</p></div>
            <el-button type="primary" :icon="Plus" @click="openRuleDialog()">新增规则</el-button>
          </div>
          <el-table :data="alarmRules" size="small" height="calc(100vh - 235px)" empty-text="暂无告警规则">
            <el-table-column label="指标" min-width="110"><template #default="{ row }">{{ metricLabel(row.identifier) }}</template></el-table-column>
            <el-table-column prop="operator" label="条件" width="70" />
            <el-table-column prop="threshold" label="阈值" width="90" />
            <el-table-column prop="hysteresis" label="恢复缓冲" width="100" />
            <el-table-column label="级别" width="90"><template #default="{ row }"><el-tag :type="alarmLevelType(row.level)">{{ alarmLevelLabel(row.level) }}</el-tag></template></el-table-column>
            <el-table-column label="启用" width="82"><template #default="{ row }"><el-switch :model-value="row.enabled" @change="toggleRuleEnabled(row)" /></template></el-table-column>
            <el-table-column label="操作" width="118"><template #default="{ row }"><el-button link type="primary" @click="openRuleDialog(row)">编辑</el-button><el-button link type="danger" @click="removeRule(row)">删除</el-button></template></el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <el-dialog v-model="ruleDialogVisible" :title="ruleEditingId ? '编辑告警规则' : '新增告警规则'" width="460px">
      <el-form :model="ruleForm" label-width="76px">
        <el-form-item label="指标"><el-select v-model="ruleForm.identifier" class="full-width"><el-option v-for="metric in ruleMetricOptions" :key="metric.id" :label="`${metric.name} (${metric.identifier})`" :value="metric.identifier" /></el-select></el-form-item>
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
.settings-button.secondary { margin-left: 4px; }
.detail-tabs { display: flex; gap: 6px; margin: 18px 0; padding: 4px; border: 1px solid #dfe7f1; border-radius: 12px; background: #f7f9fc; }
.detail-tabs button { display: inline-flex; min-width: 126px; height: 38px; align-items: center; justify-content: center; gap: 7px; padding: 0 18px; border: 1px solid transparent; border-radius: 9px; color: #475569; background: transparent; cursor: pointer; transition: all 0.18s ease; }
.detail-tabs button:hover { color: #2563eb; background: #fff; }
.detail-tabs button.active { border-color: #bfdbfe; color: #1677ff; background: #fff; box-shadow: 0 4px 12px rgb(37 99 235 / 8%); }
.detail-tabs button span { display: inline-flex; min-width: 20px; height: 20px; align-items: center; justify-content: center; padding: 0 6px; border-radius: 999px; color: #64748b; background: #eaf0f7; font-size: 12px; }
.detail-tabs button.active span { color: #1677ff; background: #eaf4ff; }
.dot { display: inline-block; width: 8px; height: 8px; margin-right: 7px; border-radius: 50%; background: #94a3b8; }
.dot.live { background: #22c55e; box-shadow: 0 0 0 4px #dcfce7; }
.dot.connecting, .dot.idle { background: #f59e0b; }
.dot.disconnected { background: #ef4444; }
.device-summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 12px; padding: 16px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.device-summary div { display: flex; min-height: 50px; gap: 8px; align-items: center; justify-content: space-between; padding: 10px 12px; border-radius: 10px; background: #f8fafc; font-size: 13px; }
.device-summary span { color: #94a3b8; }
.device-summary strong { overflow: hidden; color: #111827; text-overflow: ellipsis; white-space: nowrap; }
.relation-panel { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.remote-config-panel { margin-top: 18px; padding: 20px; border: 1px solid #dbeafe; border-radius: 12px; background: #fff; box-shadow: 0 12px 30px rgb(37 99 235 / 5%); }
.remote-config-tip { margin-top: 14px; }
.remote-config-editor { margin-top: 14px; }
.remote-config-mode { margin-bottom: 14px; }
.remote-basic-form { padding: 16px; border: 1px solid #e5edf7; border-radius: 10px; background: #f8fafc; }
.remote-field-card { display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.remote-field-card strong { color: #111827; font-size: 14px; }
.remote-field-card p, .remote-json-editor p { margin: 6px 0 0; color: #94a3b8; font-size: 12px; }
.remote-interval-input { display: flex; flex: 0 0 auto; align-items: center; gap: 9px; color: #64748b; font-size: 13px; }
.remote-interval-input :deep(.el-input-number) { width: 150px; }
.remote-device-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 18px; margin-top: 20px; padding-top: 18px; border-top: 1px solid #e2e8f0; }
.remote-device-heading strong { color: #111827; font-size: 14px; }
.remote-device-heading p { margin: 5px 0; color: #64748b; font-size: 12px; }
.remote-device-heading small { color: #94a3b8; }
.remote-device-table { margin-top: 12px; border: 1px solid #e5eaf1; border-radius: 9px; }
.remote-device-table :deep(.el-input-number) { width: 100%; }
.remote-point-panel { padding: 14px 16px 18px 62px; background: #f8fafc; }
.remote-device-extra { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 10px; margin-bottom: 14px; padding: 12px; border: 1px solid #e2e8f0; border-radius: 9px; background: #fff; }
.remote-device-extra label span { display: block; margin-bottom: 5px; color: #64748b; font-size: 12px; }
.remote-device-extra :deep(.el-input-number) { width: 100%; }
.remote-point-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.remote-point-heading strong { color: #334155; font-size: 13px; }
.remote-inline-fields { display: flex; gap: 6px; }
.remote-inline-fields :deep(.el-select) { min-width: 82px; }
.remote-inline-fields :deep(.el-input-number) { width: 135px; }
.remote-json-editor { padding: 14px; border: 1px solid #e5edf7; border-radius: 10px; background: #f8fafc; }
.remote-config-editor :deep(textarea) { font-family: Consolas, monospace; line-height: 1.55; }
.remote-config-actions { display: flex; justify-content: space-between; align-items: center; gap: 16px; margin-top: 10px; }
.remote-config-actions span { color: #94a3b8; font-size: 12px; }
.remote-config-table { margin-top: 18px; }
.remote-diff-table { margin-top: 14px; }
.relation-panel .section-heading { margin-bottom: 16px; }
.gateway-card { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 16px; align-items: center; padding: 14px 16px; border: 1px solid #edf1f5; border-radius: 10px; background: #fafcff; cursor: pointer; transition: border-color 0.15s ease, box-shadow 0.15s ease; }
.gateway-card:hover { border-color: #2563eb; box-shadow: 0 0 0 3px rgb(37 99 235 / 10%); }
.gateway-card strong { display: block; color: #111827; font-size: 14px; }
.gateway-card code { display: block; margin-top: 5px; font-size: 12px; }
.gateway-card span { color: #94a3b8; font-size: 13px; }
.topic-code { display: block; overflow-wrap: anywhere; line-height: 1.5; }
.topic-tip { margin-top: 18px; }
code { color: #2563eb; font-family: Consolas, monospace; }
.mqtt-collapse { margin-top: 14px; overflow: hidden; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.mqtt-collapse :deep(.el-collapse-item__header) { height: 48px; padding: 0 16px; border-bottom: 0; }
.mqtt-collapse :deep(.el-collapse-item__wrap) { border-bottom: 0; }
.mqtt-collapse :deep(.el-collapse-item__content) { padding: 0; }
.mqtt-summary { display: flex; min-width: 0; align-items: center; gap: 14px; color: #64748b; }
.mqtt-summary strong { flex: 0 0 auto; color: #111827; font-size: 14px; }
.mqtt-summary span { flex: 0 0 auto; font-size: 13px; }
.mqtt-summary code { max-width: 360px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.mqtt-detail { padding: 0 16px 16px; }
.mqtt-detail-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; padding-top: 2px; }
.mqtt-detail-head p { margin: 0; color: #94a3b8; font-size: 13px; }
.mqtt-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
.mqtt-grid div, .topic-list div { min-width: 0; padding: 12px 14px; border-radius: 9px; background: #f8fafc; }
.mqtt-grid span, .topic-list span { display: block; margin-bottom: 6px; color: #94a3b8; font-size: 12px; }
.mqtt-grid code, .topic-list code { display: block; overflow-wrap: anywhere; line-height: 1.5; }
.mqtt-grid strong { color: #111827; font-size: 13px; }
.topic-list { display: grid; gap: 12px; margin-top: 12px; }
.metrics-section { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.metrics-section.detail-section-first, .device-log-panel.detail-section-first { margin-top: 0; }
.chart-panel { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.trend-chart { height: 280px; margin-top: 12px; }
.trend-heading { gap: 16px; align-items: flex-start; }
.trend-title { min-width: 180px; }
.trend-controls { display: flex; flex-wrap: wrap; gap: 10px; justify-content: flex-end; align-items: center; }
.trend-metric-select { width: 180px; }
.trend-range-tabs { margin-top: 14px; max-width: 100%; padding: 4px; border-radius: 10px; background: #f8fafc; }
.trend-range-tabs :deep(.el-segmented__group) { flex-wrap: wrap; gap: 4px; }
.trend-range-tabs :deep(.el-segmented__item) { border-radius: 8px; }
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
.device-log-panel { margin-top: 18px; }
.priority-log-panel { border-color: #dbeafe; box-shadow: 0 12px 30px rgb(37 99 235 / 6%); }
.device-log-panel .section-heading { margin-bottom: 16px; }
.device-log-actions { display: flex; gap: 10px; align-items: center; }
.compact-pagination { margin-top: 14px; }
.settings-tabs { margin-top: -8px; }
.settings-tabs :deep(.el-tabs__header) { margin-bottom: 18px; }
.tab-count { display: inline-flex; min-width: 18px; height: 18px; align-items: center; justify-content: center; margin-left: 6px; padding: 0 6px; border-radius: 999px; color: #64748b; background: #f1f5f9; font-size: 12px; line-height: 18px; }
.drawer-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px; }
.sticky-heading { position: sticky; top: 0; z-index: 2; padding-bottom: 12px; background: #fff; }
.drawer-heading h3 { margin: 0; color: #111827; font-size: 16px; }
.drawer-heading p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
.model-import { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin: 0 0 14px; padding: 12px; border: 1px solid #edf1f5; border-radius: 10px; background: #f8fafc; }
.template-select { min-width: 310px; flex: 1; }
.metric-number { width: 100%; }
@media (max-width: 700px) { .detail-tabs { overflow-x: auto; } .detail-tabs button { min-width: 112px; } .device-summary { grid-template-columns: 1fr; } .gateway-card { grid-template-columns: 1fr; } .trend-stats { grid-template-columns: repeat(2, 1fr); } .monitor-actions { align-items: flex-end; flex-direction: column; } .remote-field-card, .remote-config-actions, .remote-device-heading { align-items: stretch; flex-direction: column; } .remote-point-panel { padding-left: 12px; } }
</style>
