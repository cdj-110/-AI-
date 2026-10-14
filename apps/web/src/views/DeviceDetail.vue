<script setup lang="ts">
import { ArrowLeft, Connection, Plus, RefreshRight, Setting, VideoPause, VideoPlay } from '@element-plus/icons-vue';
import * as echarts from 'echarts';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import DeviceEventsPanel from '../components/DeviceEventsPanel.vue';
import DeviceCommandsPanel from '../components/DeviceCommandsPanel.vue';
import { useUserStore } from '../stores/user';
import { getAuthItem } from '../utils/auth-storage';
import { formatMetricNumber, formatMetricValue } from '../utils/metric-value';

interface DeviceItem {
  id: string;
  deviceKey: string;
  mqtt?: DeviceCredentials;
  name: string;
  protocol: string;
  deviceType: string;
  gateway?: RelatedDevice;
  children?: RelatedDevice[];
  groups?: Array<{ id: string; name: string }>;
  status: string;
  location?: string;
  lastSeenAt?: string;
  tenant: { name: string; deviceCloudAccessDisabled?: boolean };
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
  authMode: 'DEVICE_MQTT' | 'FACTORY_ACTIVATION' | 'GATEWAY_PROXY';
  activationHardwareId?: string;
  resetAffectsCurrentConnection: boolean;
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
  collectIntervalSeconds?: number;
  collectIntervalMilliseconds?: number;
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
  itemId?: string;
  ioa?: number;
  gooseIndex?: number;
  dlt645?: {
    version?: string;
    meterAddress?: string;
    dataIdentifier?: string;
    valueFormat?: string;
    dataLength?: number;
    responseTimeoutMilliseconds?: number;
  };
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
  channelKey?: string;
  name: string;
  protocol: string;
  address: string;
  slaveId: number;
  collectIntervalSeconds?: number;
  collectIntervalMilliseconds?: number;
  interfaceType?: string;
  interfaceName?: string;
  plcModel?: string;
  rack?: number;
  slot?: number;
  iedName?: string;
  commonAddress?: number;
  reconnectIntervalSeconds?: number;
  progId?: string;
  clsid?: string;
  appId?: number;
  destinationMac?: string;
  dlt645?: {
    version?: string;
    meterAddress?: string;
    responseTimeoutMilliseconds?: number;
    retries?: number;
    wakeupBytes?: number;
  };
  username?: string;
  password?: string;
  points: RemotePointConfig[];
}

interface RemoteResourceConfig {
  resourceKey: string;
  name: string;
  type: 'serial' | 'network' | string;
  enabled?: boolean;
  serial?: { port?: string; baudRate?: number };
  network?: { interface?: string; address?: string };
}

interface RemoteChannelConfig {
  channelKey: string;
  resourceKey?: string;
  name: string;
  type?: string;
  role?: 'collect' | 'forward' | string;
  protocol: string;
  forwardProtocol?: string;
  interfaceName?: string;
  enabled?: boolean;
  collectIntervalSeconds?: number;
  collectIntervalMilliseconds?: number;
  serial?: { port?: string };
  network?: { interface?: string; address?: string };
}

interface RemoteForwardDeviceConfig {
  deviceKey: string;
  channelKey?: string;
  name: string;
  protocol: string;
  enabled?: boolean;
  unitId?: number;
  commonAddress?: number;
  iedName?: string;
  logicalDevice?: string;
  interfaceName?: string;
  goCbRef?: string;
  dataSetRef?: string;
  appId?: number;
  destinationMac?: string;
  vlanId?: number;
  vlanPriority?: number;
  timeAllowedToLiveMilliseconds?: number;
  confRev?: number;
  points: RemoteForwardPointConfig[];
}

interface RemoteForwardPointConfig {
  name: string;
  metric: string;
  sourceDeviceKey: string;
  sourceMetric: string;
  pointType?: string;
  function?: number;
  register?: number;
  ioa?: number;
  quantity?: number;
  dataType?: string;
  byteOrder?: string;
  wordOrder?: string;
  scale?: number;
  offset?: number;
  unit?: string;
  decimals?: number;
  objectRef?: string;
  fc?: string;
}

interface RemoteConfigSnapshot {
  collectIntervalSeconds?: number;
  collectIntervalMilliseconds?: number;
  resources?: RemoteResourceConfig[];
  channels?: RemoteChannelConfig[];
  devices?: RemoteDeviceConfig[];
  forwardDevices?: RemoteForwardDeviceConfig[];
}

interface GatewayTopologyMetric {
  identifier: string;
  name?: string;
  dataType?: string;
  unit?: string;
}

interface GatewayTopologyItem {
  deviceKey: string;
  name?: string;
  protocol?: string;
  address?: string;
  slaveId?: number;
  pointCount?: number;
  metrics?: GatewayTopologyMetric[];
  status: 'NEW' | 'UPDATE' | 'UNCHANGED' | 'CONFLICT' | 'MISSING_UNREPORTED' | 'PLATFORM_DISABLED';
  conflictReason?: string;
  currentGatewayKey?: string;
}

interface GatewayTopologyDraft {
  id: string;
  gatewayKey: string;
  reportedConfigVersion?: string;
  status: 'PENDING' | 'CONFLICT' | 'CONFIRMED' | 'IGNORED';
  items: GatewayTopologyItem[];
  conflicts?: Array<{ deviceKey: string; reason: string }>;
  reportedAt: string;
  confirmedAt?: string;
  confirmedBy?: string;
  ignoredAt?: string;
  ignoredBy?: string;
}

const route = useRoute();
const router = useRouter();
const userStore = useUserStore();
const device = ref<DeviceItem>();
const telemetryItems = ref<TelemetryEvent[]>([]);
const recentTelemetryItems = ref<TelemetryEvent[]>([]);
const metricClock = ref(Date.now());
const streamStatus = ref<'CONNECTING' | 'LIVE' | 'DISCONNECTED'>('CONNECTING');
const detailTab = ref<'overview' | 'monitor' | 'relations' | 'events' | 'config' | 'logs'>('overview');
const connectionTab = ref<'remote' | 'mqtt' | 'access'>('remote');
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
const commandsPanel = ref<InstanceType<typeof DeviceCommandsPanel>>();
const commandCapabilities = ref<{ points: { identifier: string; dataType: string }[]; available: boolean; reason: string }>({ points: [], available: false, reason: '' });
const writableMetricKeys = computed(() => new Set(commandCapabilities.value.points.map(point => point.identifier)));
const writablePointTypes = computed(() => new Map(commandCapabilities.value.points.map(point => [point.identifier, point.dataType])));
const pendingCommandKeys = ref<string[]>([]);
function booleanTelemetryValue(value: unknown): boolean | undefined {
  if (typeof value === 'boolean') return value;
  if (value === 1 || (typeof value === 'string' && ['true', '1'].includes(value.toLowerCase()))) return true;
  if (value === 0 || (typeof value === 'string' && ['false', '0'].includes(value.toLowerCase()))) return false;
  return undefined;
}
const alarmRules = ref<AlarmRule[]>([]);
const modelTemplates = ref<ModelTemplateItem[]>([]);
const deviceModelTemplates = ref<DeviceModelTemplateOption[]>([]);
const credentials = ref<DeviceCredentials>();
const deviceLogs = ref<DeviceLogItem[]>([]);
const deviceLogsTotal = ref(0);
const deviceLogsLoading = ref(false);
const rotatingCredentials = ref(false);
const remoteConfigTasks = ref<RemoteConfigTask[]>([]);
const remoteConfigText = ref('{\n}');
const remoteConfigMode = ref<'basic' | 'advanced'>('basic');
const remoteDevices = ref<RemoteDeviceConfig[]>([]);
const remoteChannels = ref<RemoteChannelConfig[]>([]);
const remoteForwardDevices = ref<RemoteForwardDeviceConfig[]>([]);
const remoteSnapshot = ref<RemoteConfigSnapshot>();
const remoteSnapshotAt = ref<string>();
const remoteSnapshotLoading = ref(false);
const remoteConfigLoading = ref(false);
const topologyDraft = ref<GatewayTopologyDraft>();
const topologyDraftLoading = ref(false);
const topologyDraftApplying = ref(false);
const sendingRemoteConfig = ref(false);
const remoteAccessConnected = ref(false);
const openingRemoteAccess = ref(false);
const openingRemoteTerminal = ref(false);
const remoteConfigConfirmVisible = ref(false);
const pendingRemoteConfig = ref<Record<string, unknown>>();
const remoteConfigDiffRows = ref<RemoteConfigDiffRow[]>([]);
const mqttAuthPresentation = computed(() => {
  if (credentials.value?.authMode === 'FACTORY_ACTIVATION') {
    return {
      label: '网关激活凭据',
      type: 'success' as const,
      description: '当前 EG100 主连接使用出厂激活密钥认证，下方设备 MQTT 密码不是当前主连接密码。',
    };
  }
  if (credentials.value?.authMode === 'GATEWAY_PROXY') {
    return {
      label: '所属网关代发',
      type: 'warning' as const,
      description: '该子设备不直接登录 MQTT，由所属网关代发心跳、遥测和事件。',
    };
  }
  return {
    label: '设备 MQTT 凭据',
    type: 'primary' as const,
    description: '当前设备使用下方 Client ID、用户名和密码直接连接 MQTT。',
  };
});
const remoteProtocols = [
  { label: 'Modbus TCP', value: 'modbus-tcp', defaultIntervalMilliseconds: 1000 },
  { label: 'Modbus RTU', value: 'modbus-rtu', defaultIntervalMilliseconds: 1000 },
  { label: 'DL/T 645', value: 'dlt645', defaultIntervalMilliseconds: 5000 },
  { label: 'IEC 104', value: 'iec104', defaultIntervalMilliseconds: 1000 },
  { label: 'IEC 61850 MMS', value: 'iec61850', defaultIntervalMilliseconds: 1000 },
  { label: 'IEC 61850 GOOSE', value: 'iec61850-goose', defaultIntervalMilliseconds: 100 },
  { label: 'Siemens S7', value: 'siemens-s7', defaultIntervalMilliseconds: 1000 },
  { label: 'OPC UA', value: 'opcua', defaultIntervalMilliseconds: 1000 },
  { label: 'OPC DA', value: 'opcda', defaultIntervalMilliseconds: 1000 },
];
const remoteDataTypes = ['bool', 'int8', 'uint8', 'int16', 'uint16', 'int32', 'uint32', 'int64', 'uint64', 'float32', 'float64', 'string'];

function remoteForwardDataTypes(protocol: string) {
  if (protocol === 'modbus-tcp-slave') return ['auto', ...remoteDataTypes, 'datetime'];
  if (protocol === 'iec61850-goose-publisher') return remoteDataTypes.filter(type => !['int64', 'uint64', 'string'].includes(type));
  return remoteDataTypes;
}
const ruleForm = reactive({ identifier: '', operator: '>', threshold: 0, hysteresis: 0, level: 'WARNING' });
const importForm = reactive({ templateDeviceId: '', modelTemplateId: '', overwrite: false });
const deviceLogQuery = reactive({ page: 1, pageSize: 6, type: '' });
const draggingMetricKey = ref('');
const dragOverMetricKey = ref('');
const savingMetricOrder = ref(false);
const hiddenMetricCardKeys = ref<string[]>([]);
let streamSocket: WebSocket | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let statusTimer: ReturnType<typeof setInterval> | undefined;
let recentTelemetryTimer: ReturnType<typeof setInterval> | undefined;
let metricClockTimer: ReturnType<typeof setInterval> | undefined;
let settingsRefreshTimer: ReturnType<typeof setTimeout> | undefined;
let trendRefreshTimer: ReturnType<typeof setTimeout> | undefined;
let telemetryFlushTimer: ReturnType<typeof setTimeout> | undefined;
let remoteConfigTimer: ReturnType<typeof setInterval> | undefined;
let trendChart: echarts.ECharts | undefined;
let trendResizeObserver: ResizeObserver | undefined;
let detailDisposed = false;
const watchedRemoteConfigTasks = new Map<string, RemoteConfigTask['status']>();
const notifiedRemoteConfigTasks = new Set<string>();
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
const availableMetricCards = computed(() => [...latestMetrics.value.values()]
  .filter((snapshot) => metricEnabled(snapshot.key))
  .sort((left, right) => compareMetricKeys(left.key, right.key)));
const hiddenAvailableMetricKeys = computed(() => availableMetricCards.value
  .filter((snapshot) => hiddenMetricCardKeys.value.includes(snapshot.key))
  .map((snapshot) => snapshot.key));
const metrics = computed(() => [...latestMetrics.value.values()]
  // 隐藏卡片只是当前用户的展示偏好，不影响趋势和历史数据。
  .filter((snapshot) => metricEnabled(snapshot.key) && !hiddenMetricCardKeys.value.includes(snapshot.key))
  .map((snapshot) => ({ key: snapshot.key, value: snapshot.value, time: snapshot.time, deviceKey: snapshot.deviceKey }))
  .sort((left, right) => compareMetricKeys(left.key, right.key)));
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
    return compareMetricKeys(left, right);
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
    latest: selectedMetricLatestValue.value === undefined ? '-' : displayValue(selectedMetricLatestValue.value,selectedMetric.value),
    min: values.length ? displayValue(Math.min(...values),selectedMetric.value) : '-',
    max: values.length ? displayValue(Math.max(...values),selectedMetric.value) : '-',
    avg: values.length ? displayValue(total / values.length,selectedMetric.value) : '-',
  };
});
const topologyDraftCounts = computed(() => {
  const counts: Record<string, number> = { NEW: 0, UPDATE: 0, UNCHANGED: 0, CONFLICT: 0, MISSING_UNREPORTED: 0, PLATFORM_DISABLED: 0 };
  for (const item of topologyDraft.value?.items ?? []) counts[item.status] = (counts[item.status] ?? 0) + 1;
  return counts;
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

function hiddenMetricCardsStorageKey(deviceId: string) {
  return `weikong:hidden-metric-cards:${userStore.userInfo?.id ?? 'anonymous'}:${deviceId}`;
}

function loadHiddenMetricCards(deviceId: string) {
  try {
    const stored = JSON.parse(localStorage.getItem(hiddenMetricCardsStorageKey(deviceId)) ?? '[]');
    hiddenMetricCardKeys.value = Array.isArray(stored) ? stored.filter((key): key is string => typeof key === 'string') : [];
  } catch {
    hiddenMetricCardKeys.value = [];
  }
}

function saveHiddenMetricCards(keys: string[]) {
  hiddenMetricCardKeys.value = keys;
  try {
    localStorage.setItem(hiddenMetricCardsStorageKey(String(route.params.id)), JSON.stringify(keys));
  } catch {
    ElMessage.warning('当前浏览器无法保存卡片显示设置，刷新后会恢复');
  }
}

function hideMetricCard(key: string) {
  if (hiddenMetricCardKeys.value.includes(key)) return;
  saveHiddenMetricCards([...hiddenMetricCardKeys.value, key]);
}

function restoreMetricCard(key: string) {
  const keys = key === '__all__'
    ? hiddenMetricCardKeys.value.filter((item) => !hiddenAvailableMetricKeys.value.includes(item))
    : hiddenMetricCardKeys.value.filter((item) => item !== key);
  saveHiddenMetricCards(keys);
}

function metricSortOrder(key: string) {
  return deviceMetrics.value.find((metric) => metric.identifier === key)?.sortOrder ?? 999;
}

function metricOrderIndex(key: string) {
  const index = deviceMetrics.value.findIndex((metric) => metric.identifier === key);
  return index >= 0 ? index : Number.MAX_SAFE_INTEGER;
}

function compareMetricKeys(left: string, right: string) {
  const leftOrder = metricSortOrder(left);
  const rightOrder = metricSortOrder(right);
  if (leftOrder !== rightOrder) return leftOrder - rightOrder;
  const leftIndex = metricOrderIndex(left);
  const rightIndex = metricOrderIndex(right);
  if (leftIndex !== rightIndex) return leftIndex - rightIndex;
  return left.localeCompare(right);
}

function orderedMetricEntries(metrics: Record<string, unknown>) {
  return Object.entries(metrics)
    .filter(([key]) => key.trim() && metricEnabled(key))
    .sort(([left], [right]) => compareMetricKeys(left, right));
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

function topologyStatusLabel(status?: string) {
  return { PENDING: '待确认', CONFLICT: '有冲突', CONFIRMED: '已确认', IGNORED: '已忽略' }[status ?? ''] ?? status ?? '-';
}

function topologyStatusType(status?: string) {
  if (status === 'CONFIRMED') return 'success';
  if (status === 'CONFLICT') return 'danger';
  if (status === 'IGNORED') return 'info';
  return 'warning';
}

function topologyItemStatusLabel(status?: string) {
  return {
    NEW: '新增',
    UPDATE: '更新',
    UNCHANGED: '无变化',
    CONFLICT: '冲突',
    MISSING_UNREPORTED: '网关未上报',
    PLATFORM_DISABLED: '平台禁用',
  }[status ?? ''] ?? status ?? '-';
}

function topologyItemStatusType(status?: string) {
  if (status === 'NEW') return 'success';
  if (status === 'UPDATE') return 'warning';
  if (status === 'CONFLICT') return 'danger';
  if (status === 'PLATFORM_DISABLED') return 'danger';
  return 'info';
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
  const decimals = deviceMetrics.value.find((metric) => metric.identifier === key)?.decimals;
  return formatMetricValue(value,decimals);
}

function formatDateTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', { hour12: false });
}

function metricUpdateText(time?: string) {
  const timestamp = time ? Date.parse(time) : NaN;
  if (!Number.isFinite(timestamp)) return '更新时间未知';
  const seconds = Math.max(0, Math.floor((metricClock.value - timestamp) / 1000));
  if (seconds < 60) return '更新于刚刚';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `更新于${minutes}分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `更新于${hours}小时前`;
  const days = Math.floor(hours / 24);
  return days < 30 ? `更新于${days}天前` : `更新于${formatDateTime(time)}`;
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
    metrics: Object.fromEntries(
      Object.entries(event.metrics ?? {})
        .filter(([key]) => key.trim().length > 0)
        .sort(([left], [right]) => compareMetricKeys(left, right)),
    ),
  };
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
  await Promise.all([loadPage(id), loadSettings(id), loadCredentials(id), loadTopologyDraft(id, true)]);
  if (id === String(route.params.id)) await commandsPanel.value?.refresh(true);
}

async function loadCredentials(id = String(route.params.id)) {
  const data = await apiRequest<DeviceCredentials>({ url: `/api/devices/${id}/credentials`, method: 'GET' });
  if (id === String(route.params.id)) credentials.value = data;
}

async function rotateCredentials() {
  if (!credentials.value) return;
  const affectsCurrentConnection = credentials.value.resetAffectsCurrentConnection;
  const impact = affectsCurrentConnection
    ? '当前设备使用这组凭据认证。旧密码重置后将立即失效，现有连接可能暂时保持，但设备下次重连会失败，必须同步更新新密码。'
    : credentials.value.authMode === 'FACTORY_ACTIVATION'
      ? '当前 EG100 使用激活凭据连接平台，本次操作不会修改激活密钥；但任何使用下方设备 MQTT 凭据的客户端都必须同步更新密码。'
      : '当前子设备由所属网关代发数据，不直接使用这组凭据；但任何单独使用这组凭据的客户端都必须同步更新密码。';
  try {
    await ElMessageBox.confirm(
      `${impact}\n\n新密码只显示一次，系统不会自动把它写入设备。请确认已准备好立即保存并更新设备配置。`,
      '确认重置设备 MQTT 密码',
      {
        type: 'warning',
        confirmButtonText: '确认重置',
        cancelButtonText: '取消',
        distinguishCancelAndClose: true,
      },
    );
  } catch {
    return;
  }
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
      notifyRemoteConfigResults(tasks);
      remoteConfigTasks.value = tasks;
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
    const globalIntervalMilliseconds = Number(data.config.collectIntervalMilliseconds || 0)
      || Number(data.config.collectIntervalSeconds || 1) * 1000;
    remoteChannels.value = JSON.parse(JSON.stringify(data.config.channels ?? [])) as RemoteChannelConfig[];
    remoteForwardDevices.value = normalizeRemoteForwardDevices(data.config.forwardDevices);
    remoteDevices.value = normalizeRemoteDevices(data.config.devices, globalIntervalMilliseconds);
  } finally {
    remoteSnapshotLoading.value = false;
  }
}

async function loadTopologyDraft(id = String(route.params.id), silentError = false) {
  if (!id) return;
  topologyDraftLoading.value = true;
  try {
    const draft = await apiRequest<GatewayTopologyDraft | null>({
      url: `/api/devices/${id}/topology-draft`,
      method: 'GET',
      silentError,
    });
    if (id === String(route.params.id)) topologyDraft.value = draft ?? undefined;
  } finally {
    topologyDraftLoading.value = false;
  }
}

async function confirmTopologyDraft() {
  topologyDraftApplying.value = true;
  try {
    const result = await apiRequest<{ created: number; updated: number; unchanged: number; skipped: number; blocked: number }>({
      url: `/api/devices/${String(route.params.id)}/topology-draft/confirm`,
      method: 'POST',
    });
    ElMessage.success(`拓扑已确认，新增 ${result.created}，更新 ${result.updated}${result.blocked ? `，平台禁用 ${result.blocked}` : ''}`);
    await Promise.all([refreshDevice(), loadTopologyDraft(undefined, true)]);
  } finally {
    topologyDraftApplying.value = false;
  }
}

async function ignoreTopologyDraft() {
  topologyDraftApplying.value = true;
  try {
    topologyDraft.value = await apiRequest<GatewayTopologyDraft>({
      url: `/api/devices/${String(route.params.id)}/topology-draft/ignore`,
      method: 'POST',
    });
    ElMessage.success('拓扑草稿已忽略');
  } finally {
    topologyDraftApplying.value = false;
  }
}

function prepareRemoteConfig() {
  let configPatch: Record<string, unknown>;
  if (remoteConfigMode.value === 'basic') {
    if (!remoteSnapshot.value) {
      ElMessage.error('请先读取网关当前配置');
      return;
    }
    if (!validateRemoteDevices() || !validateRemoteForwardDevices()) return;
    configPatch = { devices: remoteDevices.value, forwardDevices: remoteForwardDevices.value };
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
    const task = await apiRequest<RemoteConfigTask>({
      url: `/api/devices/${String(route.params.id)}/remote-configs`,
      method: 'POST',
      data: { config: pendingRemoteConfig.value },
      silentError: true,
    });
    watchRemoteConfigTask(task);
    remoteConfigConfirmVisible.value = false;
    pendingRemoteConfig.value = undefined;
    ElNotification({
      title: '配置已发送',
      message: `版本 V${task.version} 已发送到 EG100，正在等待网关校验和应用结果。`,
      type: 'info',
      duration: 5000,
    });
    await loadRemoteConfigs();
  } catch (error) {
    ElNotification({
      title: '配置下发失败',
      message: remoteConfigErrorMessage(error),
      type: 'error',
      duration: 0,
    });
    await loadRemoteConfigs(undefined, true);
  } finally {
    sendingRemoteConfig.value = false;
  }
}

async function retryRemoteConfig(task: RemoteConfigTask) {
  try {
    const retriedTask = await apiRequest<RemoteConfigTask>({
      url: `/api/devices/${String(route.params.id)}/remote-configs/${task.id}/retry`,
      method: 'POST',
      silentError: true,
    });
    watchRemoteConfigTask(retriedTask);
    ElNotification({
      title: '配置已重新发送',
      message: `版本 V${retriedTask.version} 已重新发送，正在等待 EG100 返回执行结果。`,
      type: 'info',
      duration: 5000,
    });
    await loadRemoteConfigs();
  } catch (error) {
    ElNotification({ title: '重新下发失败', message: remoteConfigErrorMessage(error), type: 'error', duration: 0 });
    await loadRemoteConfigs(undefined, true);
  }
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
  const labels: Record<string, string> = { collectIntervalSeconds: '采集周期（秒）', devices: '采集设备与点位', forwardDevices: '转发设备与点位' };
  return Object.entries(configPatch).map(([field, value]) => ({
    field,
    label: labels[field] ?? field,
    before: field === 'devices'
      ? remoteDeviceSummary(remoteSnapshot.value?.devices)
      : field === 'forwardDevices'
        ? remoteForwardDeviceSummary(remoteSnapshot.value?.forwardDevices)
        : displayRemoteConfigValue(remoteSnapshot.value?.[field as keyof RemoteConfigSnapshot] ?? latestAppliedRemoteValue(field)),
    after: field === 'devices'
      ? remoteDeviceChangeSummary(remoteSnapshot.value?.devices ?? [], value as RemoteDeviceConfig[])
      : field === 'forwardDevices'
        ? remoteForwardDeviceChangeSummary(remoteSnapshot.value?.forwardDevices ?? [], value as RemoteForwardDeviceConfig[])
        : displayRemoteConfigValue(value),
  }));
}

function remoteDeviceSummary(devices?: RemoteDeviceConfig[]) {
  const items = devices ?? [];
  return `${items.length} 台设备 / ${items.reduce((total, item) => total + (item.points?.length ?? 0), 0)} 个点位`;
}

function remoteForwardDeviceSummary(devices?: RemoteForwardDeviceConfig[]) {
  const items = devices ?? [];
  return `${items.length} 台转发设备 / ${items.reduce((total, item) => total + (item.points?.length ?? 0), 0)} 个转发点位`;
}

function remoteForwardDeviceChangeSummary(before: RemoteForwardDeviceConfig[], after: RemoteForwardDeviceConfig[]) {
  const beforeDevices = new Map(before.map((item) => [item.deviceKey, item]));
  const afterDevices = new Map(after.map((item) => [item.deviceKey, item]));
  const addedDevices = after.filter((item) => !beforeDevices.has(item.deviceKey)).length;
  const removedDevices = before.filter((item) => !afterDevices.has(item.deviceKey)).length;
  let changedDevices = 0;
  let addedPoints = 0;
  let removedPoints = 0;
  let changedPoints = 0;
  for (const device of after) {
    const previous = beforeDevices.get(device.deviceKey);
    if (!previous) {
      addedPoints += device.points?.length ?? 0;
      continue;
    }
    const { points: previousPoints, ...previousDevice } = previous;
    const { points: nextPoints, ...nextDevice } = device;
    if (JSON.stringify(previousDevice) !== JSON.stringify(nextDevice)) changedDevices += 1;
    const previousBySource = new Map((previousPoints ?? []).map((point) => [`${point.sourceDeviceKey}::${point.sourceMetric}::${point.metric}`, point]));
    const nextBySource = new Map((nextPoints ?? []).map((point) => [`${point.sourceDeviceKey}::${point.sourceMetric}::${point.metric}`, point]));
    addedPoints += (nextPoints ?? []).filter((point) => !previousBySource.has(`${point.sourceDeviceKey}::${point.sourceMetric}::${point.metric}`)).length;
    removedPoints += (previousPoints ?? []).filter((point) => !nextBySource.has(`${point.sourceDeviceKey}::${point.sourceMetric}::${point.metric}`)).length;
    changedPoints += (nextPoints ?? []).filter((point) => {
      const oldPoint = previousBySource.get(`${point.sourceDeviceKey}::${point.sourceMetric}::${point.metric}`);
      return oldPoint && JSON.stringify(oldPoint) !== JSON.stringify(point);
    }).length;
  }
  for (const device of before) {
    if (!afterDevices.has(device.deviceKey)) removedPoints += device.points?.length ?? 0;
  }
  return `${remoteForwardDeviceSummary(after)}；设备 +${addedDevices} / -${removedDevices} / 改${changedDevices}；点位 +${addedPoints} / -${removedPoints} / 改${changedPoints}`;
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

function createRemoteDevice(channel?: RemoteChannelConfig): RemoteDeviceConfig {
  const index = remoteDevices.value.length + 1;
  const protocol = channel?.protocol || 'modbus-tcp';
  const channelInterval = Number(channel?.collectIntervalMilliseconds || 0)
    || Number(channel?.collectIntervalSeconds || 0) * 1000
    || remoteProtocolDefaultInterval(protocol);
  return {
    deviceKey: `device-${String(index).padStart(3, '0')}`,
    channelKey: channel?.channelKey,
    name: `设备${index}`,
    protocol,
    address: channel?.type === 'serial' ? (channel.serial?.port || '') : '',
    slaveId: 1,
    collectIntervalMilliseconds: channelInterval,
    interfaceType: channel?.type || 'network',
    interfaceName: channel?.interfaceName || '',
    points: [],
  };
}

function remoteProtocolDefaultInterval(protocol: string) {
  return remoteProtocols.find((item) => item.value === protocol)?.defaultIntervalMilliseconds ?? 1000;
}

function pointIntervalMilliseconds(point: RemotePointConfig) {
  const milliseconds = Number(point.collectIntervalMilliseconds || 0);
  if (milliseconds > 0) return milliseconds;
  const seconds = Number(point.collectIntervalSeconds || 0);
  return seconds > 0 ? seconds * 1000 : 0;
}

function normalizeRemoteDevices(devices?: RemoteDeviceConfig[], fallbackMilliseconds = 1000) {
  const source = Array.isArray(devices) ? devices : [];
  return source.map((device) => {
    const copy = JSON.parse(JSON.stringify(device)) as RemoteDeviceConfig;
    copy.points = Array.isArray(copy.points) ? copy.points : [];
    const configuredMilliseconds = Number(copy.collectIntervalMilliseconds || 0)
      || Number(copy.collectIntervalSeconds || 0) * 1000;
    const pointMilliseconds = copy.points.map(pointIntervalMilliseconds).find((value) => value > 0) ?? 0;
    copy.collectIntervalMilliseconds = configuredMilliseconds || pointMilliseconds || fallbackMilliseconds || remoteProtocolDefaultInterval(copy.protocol);
    delete copy.collectIntervalSeconds;
    ensureRemoteProtocolFields(copy);
    return copy;
  });
}

function normalizeRemoteForwardDevices(devices?: RemoteForwardDeviceConfig[]) {
  const source = Array.isArray(devices) ? devices : [];
  return source.map((device, deviceIndex) => {
    const copy = JSON.parse(JSON.stringify(device)) as RemoteForwardDeviceConfig;
    copy.enabled = copy.enabled !== false;
    copy.unitId = Number(copy.unitId ?? 1);
    copy.commonAddress = Number(copy.commonAddress ?? 1);
    copy.points = (Array.isArray(copy.points) ? copy.points : []).map((point, pointIndex) => ({
      ...point,
      name: point.name || `转发点位${pointIndex + 1}`,
      metric: point.metric || point.sourceMetric || `forward_point_${pointIndex + 1}`,
      sourceDeviceKey: point.sourceDeviceKey || '',
      sourceMetric: point.sourceMetric || '',
      function: Number(point.function ?? 3),
      register: Number(point.register ?? pointIndex),
      ioa: Number(point.ioa ?? pointIndex + 1),
      quantity: Number(point.quantity ?? 1),
      dataType: point.dataType || 'uint16',
      byteOrder: point.byteOrder || 'big',
      wordOrder: point.wordOrder || 'big',
      scale: Number(point.scale ?? 1),
      offset: Number(point.offset ?? 0),
      decimals: Number(point.decimals ?? 0),
    }));
    applyRemoteForwardDeviceDefaults(copy, deviceIndex);
    return copy;
  });
}

function createRemoteForwardDevice(channel: RemoteChannelConfig): RemoteForwardDeviceConfig {
  const index = remoteForwardDevices.value.length + 1;
  const device: RemoteForwardDeviceConfig = {
    deviceKey: `forward-device-${String(index).padStart(3, '0')}`,
    channelKey: channel.channelKey,
    name: `转发设备${index}`,
    protocol: channel.forwardProtocol || channel.protocol || 'modbus-tcp-slave',
    enabled: true,
    unitId: 1,
    commonAddress: 1,
    points: [],
  };
  applyRemoteForwardDeviceDefaults(device, index - 1);
  return device;
}

function applyRemoteForwardDeviceDefaults(device: RemoteForwardDeviceConfig, index = 0) {
  if (device.protocol === 'iec61850-mms-server') {
    device.iedName = device.iedName || 'WEIKONG';
    device.logicalDevice = device.logicalDevice || `LD${index + 1}`;
  }
  if (device.protocol === 'iec61850-goose-publisher') {
    device.interfaceName = device.interfaceName || '';
    device.appId = Number(device.appId ?? 4096);
    device.destinationMac = device.destinationMac || '01:0c:cd:01:00:01';
    device.vlanPriority = Number(device.vlanPriority ?? 4);
    device.timeAllowedToLiveMilliseconds = Number(device.timeAllowedToLiveMilliseconds ?? 2000);
    device.confRev = Number(device.confRev ?? 1);
  }
}

function createRemoteForwardPoint(device: RemoteForwardDeviceConfig): RemoteForwardPointConfig {
  const index = (device.points?.length ?? 0) + 1;
  const sourceDevice = remoteDevices.value[0];
  const sourcePoint = sourceDevice?.points?.[0];
  const metric = sourcePoint?.metric || `forward_point_${index}`;
  const point: RemoteForwardPointConfig = {
    name: sourcePoint?.name || `转发点位${index}`,
    metric,
    sourceDeviceKey: sourceDevice?.deviceKey || '',
    sourceMetric: sourcePoint?.metric || '',
    function: 3,
    register: Math.max(0, index - 1),
    ioa: index,
    quantity: 1,
    dataType: sourcePoint?.dataType || 'uint16',
    byteOrder: 'big',
    wordOrder: 'big',
    scale: 1,
    offset: 0,
    unit: sourcePoint?.unit || '',
    decimals: sourcePoint?.decimals ?? 0,
  };
  if (device.protocol === 'iec104-server') point.pointType = point.dataType === 'bool' ? 'M_SP_NA_1' : 'M_ME_NC_1';
  if (device.protocol === 'iec61850-mms-server') {
    const valueLeaf = point.dataType === 'bool' ? 'stVal' : 'mag.f';
    point.fc = point.dataType === 'bool' ? 'ST' : 'MX';
    point.objectRef = `${device.iedName || 'WEIKONG'}${device.logicalDevice || 'LD1'}/GGIO1.AnIn${index}.${valueLeaf}`;
  }
  return point;
}

function addRemoteForwardDevice(channel: RemoteChannelConfig) {
  remoteForwardDevices.value.push(createRemoteForwardDevice(channel));
}

function copyRemoteForwardDevice(index: number) {
  const copy = JSON.parse(JSON.stringify(remoteForwardDevices.value[index])) as RemoteForwardDeviceConfig;
  copy.deviceKey = `${copy.deviceKey}-copy`;
  copy.name = `${copy.name}副本`;
  remoteForwardDevices.value.splice(index + 1, 0, copy);
}

function removeRemoteForwardDevice(index: number) {
  remoteForwardDevices.value.splice(index, 1);
}

function addRemoteForwardPoint(device: RemoteForwardDeviceConfig) {
  if (!Array.isArray(device.points)) device.points = [];
  device.points.push(createRemoteForwardPoint(device));
}

function copyRemoteForwardPoint(device: RemoteForwardDeviceConfig, index: number) {
  const copy = JSON.parse(JSON.stringify(device.points[index])) as RemoteForwardPointConfig;
  copy.name = `${copy.name}副本`;
  copy.metric = `${copy.metric}_copy`;
  device.points.splice(index + 1, 0, copy);
}

function removeRemoteForwardPoint(device: RemoteForwardDeviceConfig, index: number) {
  device.points.splice(index, 1);
}

function remoteSourcePoints(deviceKey: string) {
  return remoteDevices.value.find((item) => item.deviceKey === deviceKey)?.points ?? [];
}

function syncRemoteForwardSource(point: RemoteForwardPointConfig) {
  const source = remoteSourcePoints(point.sourceDeviceKey).find((item) => item.metric === point.sourceMetric);
  if (!source) return;
  point.name = source.name || point.name || source.metric;
  point.metric = source.metric;
  point.dataType = source.dataType || point.dataType;
  point.quantity = source.quantity || point.quantity || 1;
  point.unit = source.unit || point.unit;
  point.decimals = source.decimals ?? point.decimals ?? 0;
}

function createRemotePoint(device: RemoteDeviceConfig): RemotePointConfig {
  const index = (device.points?.length ?? 0) + 1;
  const point: RemotePointConfig = { deviceKey: device.deviceKey, collectIntervalMilliseconds: device.collectIntervalMilliseconds, name: `点位${index}`, metric: `point_${index}`, protocol: device.protocol, address: device.address, slaveId: device.slaveId, function: 3, register: 0, quantity: 1, dataType: 'uint16', byteOrder: 'big', wordOrder: 'normal', scale: 1, offset: 0, decimals: 0 };
  if (device.protocol === 'dlt645') {
    point.dlt645 = { version: device.dlt645?.version ?? 'auto', meterAddress: device.dlt645?.meterAddress, dataIdentifier: '00000000', valueFormat: 'bcd', responseTimeoutMilliseconds: device.dlt645?.responseTimeoutMilliseconds };
  }
  return point;
}

function addRemoteDevice(channel?: RemoteChannelConfig) {
  remoteDevices.value.push(createRemoteDevice(channel));
}

function copyRemoteDevice(index: number) {
  const copy = JSON.parse(JSON.stringify(remoteDevices.value[index])) as RemoteDeviceConfig;
  copy.points = Array.isArray(copy.points) ? copy.points : [];
  copy.deviceKey = `${copy.deviceKey}-copy`;
  copy.name = `${copy.name}副本`;
  copy.points.forEach((point) => { point.deviceKey = copy.deviceKey; });
  remoteDevices.value.splice(index + 1, 0, copy);
}

function removeRemoteDevice(index: number) {
  remoteDevices.value.splice(index, 1);
}

const remoteHierarchyResources = computed<RemoteResourceConfig[]>(() => {
  const resources = [...(remoteSnapshot.value?.resources ?? [])];
  const known = new Set(resources.map((item) => item.resourceKey));
  for (const channel of remoteChannels.value) {
    const resourceKey = channel.resourceKey || '__unassigned';
    if (known.has(resourceKey)) continue;
    resources.push({
      resourceKey,
      name: channel.interfaceName || (resourceKey === '__unassigned' ? '未绑定资源' : resourceKey),
      type: channel.type || 'network',
      enabled: channel.enabled,
      serial: channel.type === 'serial' ? { port: channel.serial?.port } : undefined,
      network: channel.type !== 'serial' ? { interface: channel.network?.interface } : undefined,
    });
    known.add(resourceKey);
  }
  return resources;
});

function remoteChannelRole(channel: RemoteChannelConfig) {
  return channel.role === 'forward' || Boolean(channel.forwardProtocol && channel.forwardProtocol !== 'none') ? 'forward' : 'collect';
}

function remoteChannelsForResource(resource: RemoteResourceConfig, role: 'collect' | 'forward') {
  return remoteChannels.value.filter((channel) => (channel.resourceKey || '__unassigned') === resource.resourceKey && remoteChannelRole(channel) === role);
}

function remoteDevicesForChannel(channel: RemoteChannelConfig) {
  return remoteDevices.value.filter((device) => device.channelKey === channel.channelKey);
}

function remoteForwardDevicesForChannel(channel: RemoteChannelConfig) {
  return remoteForwardDevices.value.filter((device) => device.channelKey === channel.channelKey);
}

function remoteResourceMeta(resource: RemoteResourceConfig) {
  if (resource.type === 'serial') {
    const baudRate = resource.serial?.baudRate ? ` · ${resource.serial.baudRate} bps` : '';
    return `串口资源 · ${resource.serial?.port || resource.name || '-'}${baudRate}`;
  }
  return `网口资源 · ${resource.network?.interface || resource.name || '-'}`;
}

function remoteProtocolLabel(protocol?: string) {
  const collection = remoteProtocols.find((item) => item.value === protocol)?.label;
  if (collection) return collection;
  return ({
    'modbus-tcp-slave': 'Modbus TCP 从站',
    'iec104-server': 'IEC 104 服务端',
    'iec61850-mms-server': 'IEC 61850 MMS 服务端',
    'iec61850-goose-publisher': 'IEC 61850 GOOSE 发布',
  } as Record<string, string>)[protocol || ''] || protocol || '未配置协议';
}

function remoteChannelIntervalText(channel: RemoteChannelConfig) {
  const milliseconds = Number(channel.collectIntervalMilliseconds || 0)
    || Number(channel.collectIntervalSeconds || 0) * 1000;
  return milliseconds > 0 ? `${milliseconds} ms` : '按设备设置';
}

function addRemotePoint(device: RemoteDeviceConfig) {
  if (!Array.isArray(device.points)) device.points = [];
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

function ensureRemoteProtocolFields(device: RemoteDeviceConfig) {
  if (device.protocol === 'dlt645') {
    device.dlt645 = device.dlt645 ?? { version: 'auto', meterAddress: '', responseTimeoutMilliseconds: 3000, retries: 1, wakeupBytes: 4 };
  }
  if (device.protocol === 'iec104') device.commonAddress = Number(device.commonAddress ?? device.slaveId ?? 1);
  if (device.protocol === 'opcda') device.progId = device.progId ?? '';
  if (device.protocol === 'iec61850-goose') {
    device.appId = Number(device.appId ?? 4096);
    device.destinationMac = device.destinationMac || '01:0c:cd:01:00:01';
  }
}

function changeRemoteProtocol(device: RemoteDeviceConfig) {
  device.collectIntervalMilliseconds = remoteProtocolDefaultInterval(device.protocol);
  ensureRemoteProtocolFields(device);
  syncRemoteDevice(device, true);
}

function syncRemoteDevice(device: RemoteDeviceConfig, syncInterval = false) {
  if (!Array.isArray(device.points)) device.points = [];
  ensureRemoteProtocolFields(device);
  device.points.forEach((point) => {
    point.deviceKey = device.deviceKey;
    point.protocol = device.protocol;
    point.address = device.address;
    point.slaveId = device.slaveId;
    if (syncInterval) {
      point.collectIntervalMilliseconds = Math.max(50, Number(device.collectIntervalMilliseconds || remoteProtocolDefaultInterval(device.protocol)));
      delete point.collectIntervalSeconds;
    }
    if (device.protocol === 'dlt645') {
      point.dlt645 = point.dlt645 ?? {};
      point.dlt645.version = device.dlt645?.version ?? 'auto';
      point.dlt645.meterAddress = device.dlt645?.meterAddress;
      point.dlt645.responseTimeoutMilliseconds = device.dlt645?.responseTimeoutMilliseconds;
    }
  });
}

function validateRemoteDevices() {
  const deviceKeys = new Set<string>();
  for (const item of remoteDevices.value) {
    item.deviceKey = String(item.deviceKey ?? '').trim();
    item.name = String(item.name ?? '').trim();
    item.protocol = String(item.protocol ?? '').trim();
    item.address = String(item.address ?? '').trim();
    if (!item.deviceKey || !item.name || !item.protocol || !item.address) {
      ElMessage.error('设备编号、名称、协议和地址不能为空');
      return false;
    }
    const normalizedDeviceKey = item.deviceKey.toLowerCase();
    if (deviceKeys.has(normalizedDeviceKey)) {
      ElMessage.error(`设备编号重复：${item.deviceKey}`);
      return false;
    }
    deviceKeys.add(normalizedDeviceKey);
    const interval = Number(item.collectIntervalMilliseconds || 0);
    if (!Number.isFinite(interval) || interval < 50 || interval > 86400000) {
      ElMessage.error(`${item.name} 的采集周期必须在 50–86400000 ms 之间`);
      return false;
    }
    syncRemoteDevice(item, true);
    const metrics = new Set<string>();
    for (const point of item.points ?? []) {
      point.name = String(point.name ?? '').trim();
      point.metric = String(point.metric ?? '').trim();
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

function validateRemoteForwardDevices() {
  const deviceKeys = new Set(remoteDevices.value.map((item) => item.deviceKey.toLowerCase()));
  for (const item of remoteForwardDevices.value) {
    item.deviceKey = String(item.deviceKey ?? '').trim();
    item.name = String(item.name ?? '').trim();
    item.protocol = String(item.protocol ?? '').trim();
    if (!item.deviceKey || !item.name || !item.protocol) {
      ElMessage.error('转发设备编号、名称和协议不能为空');
      return false;
    }
    const normalizedDeviceKey = item.deviceKey.toLowerCase();
    if (deviceKeys.has(normalizedDeviceKey)) {
      ElMessage.error(`设备编号重复：${item.deviceKey}`);
      return false;
    }
    deviceKeys.add(normalizedDeviceKey);
    const metrics = new Set<string>();
    for (const point of item.points ?? []) {
      point.name = String(point.name ?? '').trim();
      point.metric = String(point.metric ?? '').trim();
      point.sourceDeviceKey = String(point.sourceDeviceKey ?? '').trim();
      point.sourceMetric = String(point.sourceMetric ?? '').trim();
      if (!point.name || !point.metric || !point.sourceDeviceKey || !point.sourceMetric) {
        ElMessage.error(`转发设备 ${item.name} 存在名称、标识符或来源为空的点位`);
        return false;
      }
      const metric = point.metric.toLowerCase();
      if (metrics.has(metric)) {
        ElMessage.error(`转发设备 ${item.name} 的转发标识符重复：${point.metric}`);
        return false;
      }
      metrics.add(metric);
    }
  }
  return true;
}

function remoteConfigErrorMessage(error: unknown) {
  const responseMessage = (error as { response?: { data?: { message?: string } } })?.response?.data?.message;
  if (responseMessage) return responseMessage;
  return error instanceof Error ? error.message : '远程配置请求失败，请检查平台网络和网关在线状态';
}

function watchRemoteConfigTask(task: RemoteConfigTask) {
  notifiedRemoteConfigTasks.delete(task.id);
  watchedRemoteConfigTasks.set(task.id, task.status);
  if (['APPLIED', 'FAILED', 'TIMEOUT'].includes(task.status)) notifyRemoteConfigResult(task);
}

function notifyRemoteConfigResults(tasks: RemoteConfigTask[]) {
  for (const task of tasks) {
    if (!watchedRemoteConfigTasks.has(task.id)) continue;
    const previous = watchedRemoteConfigTasks.get(task.id);
    watchedRemoteConfigTasks.set(task.id, task.status);
    if (previous !== task.status && ['APPLIED', 'FAILED', 'TIMEOUT'].includes(task.status)) {
      notifyRemoteConfigResult(task);
    }
  }
}

function notifyRemoteConfigResult(task: RemoteConfigTask) {
  if (notifiedRemoteConfigTasks.has(task.id)) return;
  notifiedRemoteConfigTasks.add(task.id);
  watchedRemoteConfigTasks.delete(task.id);
  if (task.status === 'APPLIED') {
    ElNotification({
      title: '配置下发成功',
      message: `版本 V${task.version} 已通过 EG100 校验并生效。`,
      type: 'success',
      duration: 7000,
    });
    void loadCurrentRemoteConfig(undefined, true);
    return;
  }
  ElNotification({
    title: task.status === 'TIMEOUT' ? '配置下发超时' : '配置下发失败',
    message: task.error || (task.status === 'TIMEOUT' ? 'EG100 未在规定时间内返回执行结果' : 'EG100 未能应用该配置'),
    type: 'error',
    duration: 0,
  });
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
  const host = trendChartRef.value;
  // The monitor panel is hidden on initial load. Do not initialize at 0 x 0.
  if (detailDisposed || detailTab.value !== 'monitor' || !host || !selectedMetric.value || !host.clientWidth || !host.clientHeight) return;
  if (trendChart && (trendChart.isDisposed() || trendChart.getDom() !== host)) {
    trendChart.dispose();
    trendChart = undefined;
  }
  trendChart ??= echarts.init(host);
  if (trendChart.getWidth() !== host.clientWidth || trendChart.getHeight() !== host.clientHeight) trendChart.resize();
  const points = trendPoints.value;
  trendChart.setOption({
    tooltip: { trigger: 'axis', valueFormatter: (value: unknown) => `${displayValue(value,selectedMetric.value)}${metricUnit(selectedMetric.value)}` },
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
      axisLabel: { color: '#94a3b8', formatter: (value:number) => formatMetricNumber(value,undefined,true) },
      splitLine: { lineStyle: { color: '#edf1f5' } },
    },
    series: [{
      name: metricLabel(selectedMetric.value),
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
  void renderTrendChart();
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

function realtimeWebSocketURL() {
  const configuredBase = import.meta.env.VITE_API_BASE_URL || window.location.origin;
  const url = new URL(configuredBase, window.location.origin);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.pathname = '/api/realtime';
  url.search = '';
  url.hash = '';
  return url.toString();
}

function connectStream(id = String(route.params.id)) {
  streamStatus.value = 'CONNECTING';
  const socket = new WebSocket(realtimeWebSocketURL());
  streamSocket = socket;
  socket.addEventListener('open', () => {
    socket.send(JSON.stringify({ type: 'auth', token: getAuthItem('accessToken') ?? '' }));
  });
  socket.addEventListener('message', (event) => {
    let message: { type?: string; deviceId?: string; payload?: TelemetryEvent };
    try {
      message = JSON.parse(String(event.data));
    } catch {
      return;
    }
    if (message.type === 'auth.ok') {
      socket.send(JSON.stringify({ type: 'telemetry.subscribe', deviceId: id }));
      return;
    }
    if (message.type === 'telemetry.subscribed' && message.deviceId === id) {
      streamStatus.value = 'LIVE';
      return;
    }
    if (message.type === 'telemetry.update' && message.deviceId === id && message.payload) {
      addTelemetry(message.payload);
      return;
    }
    if (message.type === 'auth.error' || message.type === 'subscription.error') socket.close();
  });
  socket.addEventListener('error', () => socket.close());
  socket.addEventListener('close', () => {
    if (streamSocket !== socket) return;
    streamSocket = undefined;
    if (id !== String(route.params.id)) return;
    streamStatus.value = 'DISCONNECTED';
    reconnectTimer = setTimeout(() => {
      if (id === String(route.params.id)) connectStream(id);
    }, 3000);
  });
}

function stopTelemetryStream() {
  const socket = streamSocket;
  streamSocket = undefined;
  socket?.close(1000, 'page leave');
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
  hiddenMetricCardKeys.value = [];
  alarmRules.value = [];
  credentials.value = undefined;
  deviceLogs.value = [];
  deviceLogsTotal.value = 0;
  remoteConfigTasks.value = [];
  remoteAccessConnected.value = false;
  topologyDraft.value = undefined;
  remoteDevices.value = [];
  remoteChannels.value = [];
  remoteForwardDevices.value = [];
  remoteSnapshot.value = undefined;
  remoteSnapshotAt.value = undefined;
  watchedRemoteConfigTasks.clear();
  notifiedRemoteConfigTasks.clear();
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
  loadHiddenMetricCards(id);
  await Promise.all([loadPage(id), loadSettings(id), loadCredentials(id), loadDeviceLogs(id, true)]);
  if (id !== String(route.params.id)) return;
  connectionTab.value = device.value?.deviceType === 'GATEWAY' ? 'remote' : 'mqtt';
  if (device.value?.deviceType === 'GATEWAY') await Promise.all([loadRemoteConfigs(id, true), loadCurrentRemoteConfig(id, true), loadTopologyDraft(id, true), loadRemoteAccessStatus(id)]);
  selectedMetric.value = numericMetricKeys.value[0] ?? '';
  await loadTrend(id);
  void connectStream(id);
}

async function loadRemoteAccessStatus(id = String(route.params.id)) {
  try {
    const result = await apiRequest<{ connected: boolean }>({ url: `/api/remote-access/devices/${id}/status`, method: 'GET', silentError: true });
    remoteAccessConnected.value = result.connected;
  } catch { remoteAccessConnected.value = false; }
}

async function openRemoteAccess() {
  if (!device.value || openingRemoteAccess.value) return;
  const target = window.open('', '_blank');
  openingRemoteAccess.value = true;
  try {
    const result = await apiRequest<{ url: string }>({ url: `/api/remote-access/devices/${device.value.id}/sessions`, method: 'POST' });
    if (target) target.location.href = result.url;
    else window.location.href = result.url;
  } catch (error) {
    target?.close();
    throw error;
  } finally { openingRemoteAccess.value = false; }
}

function openRemoteTerminal() {
  if (!device.value || openingRemoteTerminal.value) return;
  openingRemoteTerminal.value = true;
  const target = router.resolve({
    path: `/remote-terminal/${device.value.id}`,
    query: { name: device.value.name, key: device.value.deviceKey },
  });
  const opened = window.open(target.href, '_blank');
  if (opened) opened.opener = null;
  else ElMessage.warning('浏览器阻止了新标签页，请允许弹出窗口后重试');
  openingRemoteTerminal.value = false;
}

function formatMetrics(item: Record<string, unknown>) {
  return orderedMetricEntries(item)
    .map(([key, value]) => `${metricLabel(key)}: ${displayValue(value, key)}${metricUnit(key)}`)
    .join(' · ');
}

onMounted(async () => {
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  metricClockTimer = setInterval(() => { metricClock.value = Date.now(); }, 10000);
  statusTimer = setInterval(() => void refreshDevice(), 30000);
  recentTelemetryTimer = setInterval(() => void refreshRecentTelemetry(), 2000);
  remoteConfigTimer = setInterval(() => {
    if (device.value?.deviceType === 'GATEWAY') {
      void loadRemoteConfigs(undefined, true);
      void loadTopologyDraft(undefined, true);
      void loadRemoteAccessStatus();
    }
  }, 5000);
  window.addEventListener('resize', resizeChart);
  await loadDeviceDetail();
});

onBeforeUnmount(() => {
  detailDisposed = true;
  trendResizeObserver?.disconnect();
  stopTelemetryStream();
  if (statusTimer) clearInterval(statusTimer);
  if (recentTelemetryTimer) clearInterval(recentTelemetryTimer);
  if (metricClockTimer) clearInterval(metricClockTimer);
  if (settingsRefreshTimer) clearTimeout(settingsRefreshTimer);
  if (trendRefreshTimer) clearTimeout(trendRefreshTimer);
  if (telemetryFlushTimer) clearTimeout(telemetryFlushTimer);
  if (remoteConfigTimer) clearInterval(remoteConfigTimer);
  window.removeEventListener('device-status-change', handleDeviceStatusEvent);
  window.removeEventListener('resize', resizeChart);
  trendChart?.dispose();
  trendChart = undefined;
});

watch(trendChartRef, (host) => {
  trendResizeObserver?.disconnect();
  trendChart?.dispose();
  trendChart = undefined;
  if (!host || detailDisposed) return;
  trendResizeObserver = new ResizeObserver(resizeChart);
  trendResizeObserver.observe(host);
  void renderTrendChart();
}, { flush: 'post' });

watch(detailTab, (tab) => {
  if (tab === 'monitor') void renderTrendChart();
}, { flush: 'post' });

watch(numericMetricKeys, (keys) => {
  if (!keys.includes(selectedMetric.value)) selectedMetric.value = keys[0] ?? '';
});

watch([selectedMetric, selectedTrendRange], () => void loadTrend());

watch(() => route.params.id, (id, oldId) => {
  if (!id || id === oldId) return;
  detailTab.value = 'overview';
  connectionTab.value = 'remote';
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
      <button :class="{ active: detailTab === 'overview' }" type="button" @click="detailTab = 'overview'">
        设备概览
      </button>
      <button :class="{ active: detailTab === 'monitor' }" type="button" @click="detailTab = 'monitor'">
        实时数据
        <span>{{ metrics.length }}</span>
      </button>
      <button v-if="device?.deviceType === 'GATEWAY'" :class="{ active: detailTab === 'relations' }" type="button" @click="detailTab = 'relations'">
        设备关系
        <span>{{ device.children?.length ?? 0 }}</span>
      </button>
      <button :class="{ active: detailTab === 'events' }" type="button" @click="detailTab = 'events'">
        事件
      </button>
      <button :class="{ active: detailTab === 'config' }" type="button" @click="detailTab = 'config'">
        连接与配置
      </button>
      <button v-if="canManage" :class="{ active: detailTab === 'logs' }" type="button" @click="detailTab = 'logs'">
        设备日志
        <span>{{ deviceLogsTotal }}</span>
      </button>
    </nav>

    <nav v-show="detailTab === 'config'" class="connection-tabs" aria-label="连接与配置功能">
      <button v-if="device?.deviceType === 'GATEWAY'" :class="{ active: connectionTab === 'remote' }" type="button" @click="connectionTab = 'remote'">
        远程配置
        <small>配置下发与回执</small>
      </button>
      <button :class="{ active: connectionTab === 'mqtt' }" type="button" @click="connectionTab = 'mqtt'">
        MQTT 接入
        <small>认证、地址与主题</small>
      </button>
      <button v-if="device?.deviceType === 'GATEWAY'" :class="{ active: connectionTab === 'access' }" type="button" @click="connectionTab = 'access'">
        远程访问
        <small>安全维护通道</small>
      </button>
    </nav>

    <section v-show="detailTab === 'overview'" class="device-summary detail-section-first">
      <div><span>设备状态</span><el-tag :type="device?.tenant?.deviceCloudAccessDisabled ? 'info' : statusType(device?.status)">{{ device?.tenant?.deviceCloudAccessDisabled ? '租户禁用云连接' : statusLabel(device?.status) }}</el-tag></div>
      <div><span>设备类型</span><strong>{{ deviceTypeLabel(device?.deviceType) }}</strong></div>
      <div v-if="device?.deviceType === 'GATEWAY_CHILD'"><span>所属网关</span><strong>{{ device.gateway?.name || '-' }}</strong></div>
      <div class="device-group-summary">
        <span>所属分组</span>
        <span v-if="device?.groups?.length" class="device-group-tags">
          <el-tag v-for="group in device.groups" :key="group.id" size="small" effect="plain" :title="group.name">{{ group.name }}</el-tag>
        </span>
        <strong v-else>未分组</strong>
      </div>
      <div><span>安装位置</span><strong>{{ device?.location || '-' }}</strong></div>
      <div><span>最近遥测</span><strong>{{ formatDateTime(latestTelemetryTime) }}</strong></div>
    </section>

    <section v-show="detailTab === 'overview'" class="overview-navigation">
      <button type="button" @click="detailTab = 'monitor'">
        <strong>实时数据</strong><span>查看当前值、变化趋势和最近上报</span><em>{{ metrics.length }} 个指标 →</em>
      </button>
      <button type="button" @click="detailTab = 'events'">
        <strong>设备事件</strong><span>查看告警触发、确认与恢复过程</span><em>进入事件记录 →</em>
      </button>
      <button v-if="device?.deviceType === 'GATEWAY'" type="button" @click="detailTab = 'relations'">
        <strong>设备关系</strong><span>核对网关拓扑和下属设备状态</span><em>{{ device.children?.length ?? 0 }} 台子设备 →</em>
      </button>
      <button type="button" @click="detailTab = 'config'">
        <strong>连接与配置</strong><span>管理 MQTT 连接和设备运行参数</span><em>查看配置 →</em>
      </button>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'config' && connectionTab === 'remote'" class="remote-config-panel detail-section-first">
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
          <div class="remote-device-heading">
            <div>
              <strong>资源 / 通道 / 设备 / 点位</strong>
              <p>层级与 EG100 保持一致：物理资源下区分采集、转发通道，两类设备均可展开维护点位。</p>
              <p>设备周期为实际生效值，按点位、设备、通道的优先级计算。</p>
            </div>
            <div class="remote-read-actions">
              <small>最近读取：{{ formatDateTime(remoteSnapshotAt) }}</small>
              <el-button :icon="RefreshRight" :loading="remoteSnapshotLoading" @click="loadCurrentRemoteConfig()">重新读取</el-button>
            </div>
          </div>
          <div v-loading="remoteSnapshotLoading" class="remote-config-hierarchy">
            <section v-for="resource in remoteHierarchyResources" :key="resource.resourceKey" class="remote-resource-card">
              <header class="remote-resource-head">
                <span class="remote-resource-icon" :class="resource.type">{{ resource.type === 'serial' ? 'COM' : 'LAN' }}</span>
                <div><strong>{{ resource.name || resource.resourceKey }}</strong><small>{{ remoteResourceMeta(resource) }}</small></div>
                <el-tag :type="resource.enabled === false ? 'info' : 'success'" effect="plain">{{ resource.enabled === false ? '已停用' : '已启用' }}</el-tag>
              </header>
              <template v-for="role in (['collect', 'forward'] as const)" :key="`${resource.resourceKey}-${role}`">
                <section v-if="remoteChannelsForResource(resource, role).length" class="remote-role-group" :class="role">
                  <div class="remote-role-title"><strong>{{ role === 'collect' ? '采集' : '转发' }}</strong><span>{{ remoteChannelsForResource(resource, role).length }} 个通道</span></div>
                  <article v-for="channel in remoteChannelsForResource(resource, role)" :key="channel.channelKey" class="remote-channel-card">
                    <header class="remote-channel-head">
                      <div><strong>{{ channel.name || channel.channelKey }}</strong><span>{{ remoteProtocolLabel(role === 'forward' ? (channel.forwardProtocol || channel.protocol) : channel.protocol) }}</span></div>
                      <div class="remote-channel-summary">
                        <span v-if="role === 'collect'">通道周期 {{ remoteChannelIntervalText(channel) }}</span>
                        <span>{{ role === 'collect' ? remoteDevicesForChannel(channel).length : remoteForwardDevicesForChannel(channel).length }} 台设备</span>
                        <el-button size="small" type="primary" plain :icon="Plus" @click="role === 'collect' ? addRemoteDevice(channel) : addRemoteForwardDevice(channel)">{{ role === 'collect' ? '新增采集设备' : '新增转发设备' }}</el-button>
                      </div>
                    </header>
                    <el-table v-if="role === 'collect'" :data="remoteDevicesForChannel(channel)" row-key="deviceKey" empty-text="该通道暂未添加采集设备" class="remote-device-table">
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
                    <template v-if="remoteDevice.protocol === 'opcda'">
                      <label><span>ProgID</span><el-input v-model="remoteDevice.progId" placeholder="如 Matrikon.OPC.Simulation.1" /></label>
                      <label><span>CLSID（可选）</span><el-input v-model="remoteDevice.clsid" /></label>
                      <label><span>用户名</span><el-input v-model="remoteDevice.username" placeholder="留空使用当前身份" /></label>
                      <label><span>密码</span><el-input v-model="remoteDevice.password" type="password" show-password /></label>
                    </template>
                    <template v-if="remoteDevice.protocol === 'iec104'">
                      <label><span>公共地址</span><el-input-number v-model="remoteDevice.commonAddress" :min="1" :max="65535" controls-position="right" /></label>
                      <label><span>重连间隔（秒）</span><el-input-number v-model="remoteDevice.reconnectIntervalSeconds" :min="1" :max="3600" controls-position="right" /></label>
                    </template>
                    <template v-if="remoteDevice.protocol === 'dlt645' && remoteDevice.dlt645">
                      <label><span>协议版本</span><el-select v-model="remoteDevice.dlt645.version"><el-option label="自动识别" value="auto" /><el-option label="DL/T 645-2007" value="2007" /><el-option label="DL/T 645-1997" value="1997" /></el-select></label>
                      <label><span>电表地址</span><el-input v-model="remoteDevice.dlt645.meterAddress" placeholder="12 位十进制地址" /></label>
                      <label><span>响应超时（ms）</span><el-input-number v-model="remoteDevice.dlt645.responseTimeoutMilliseconds" :min="100" :max="30000" controls-position="right" /></label>
                    </template>
                    <label v-if="remoteDevice.protocol === 'iec61850'"><span>IED 名称</span><el-input v-model="remoteDevice.iedName" /></label>
                    <template v-if="remoteDevice.protocol === 'iec61850-goose'">
                      <label><span>APPID</span><el-input-number v-model="remoteDevice.appId" :min="0" :max="65535" controls-position="right" /></label>
                      <label><span>目标 MAC</span><el-input v-model="remoteDevice.destinationMac" placeholder="01:0c:cd:01:00:01" /></label>
                    </template>
                  </div>
                  <div class="remote-point-heading"><strong>{{ remoteDevice.name }} 的点位</strong><el-button size="small" type="primary" plain :icon="Plus" @click="addRemotePoint(remoteDevice)">新增点位</el-button></div>
                  <el-table :data="remoteDevice.points" size="small" empty-text="暂无点位">
                    <el-table-column label="名称" min-width="130"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
                    <el-table-column label="标识符" min-width="140"><template #default="{ row }"><el-input v-model="row.metric" /></template></el-table-column>
                    <el-table-column label="协议地址" min-width="245">
                      <template #default="{ row }">
                        <div v-if="remoteDevice.protocol === 'opcua'" class="remote-inline-fields"><el-input v-model="row.nodeId" placeholder="NodeId" /></div>
                        <div v-else-if="remoteDevice.protocol === 'opcda'" class="remote-inline-fields"><el-input v-model="row.itemId" placeholder="OPC ItemID" /></div>
                        <div v-else-if="remoteDevice.protocol === 'iec104'" class="remote-inline-fields"><el-input-number v-model="row.ioa" :min="0" :max="16777215" controls-position="right" /><span class="remote-address-unit">IOA</span></div>
                        <div v-else-if="remoteDevice.protocol === 'dlt645' && row.dlt645" class="remote-inline-fields"><el-input v-model="row.dlt645.dataIdentifier" placeholder="数据标识 DI" /><el-select v-model="row.dlt645.valueFormat"><el-option label="BCD" value="bcd" /><el-option label="有符号整数" value="int" /><el-option label="无符号整数" value="uint" /><el-option label="浮点数" value="float" /></el-select></div>
                        <div v-else-if="remoteDevice.protocol === 'iec61850'" class="remote-inline-fields"><el-input v-model="row.objectRef" placeholder="对象引用" /><el-input v-model="row.fc" placeholder="FC" /></div>
                        <div v-else-if="remoteDevice.protocol === 'iec61850-goose'" class="remote-inline-fields"><el-input v-model="row.objectRef" placeholder="数据对象引用" /><el-input-number v-model="row.gooseIndex" :min="0" controls-position="right" /></div>
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
            <el-table-column label="地址" min-width="190"><template #default="{ row }"><el-input v-model="row.address" placeholder="IP:端口 / COM口" @change="syncRemoteDevice(row)" /></template></el-table-column>
            <el-table-column label="从站/公共地址" width="145"><template #default="{ row }"><el-input-number v-model="row.slaveId" :min="0" :max="255" controls-position="right" @change="syncRemoteDevice(row)" /></template></el-table-column>
            <el-table-column label="采集周期" width="155"><template #default="{ row }"><div class="remote-device-interval"><el-input-number v-model="row.collectIntervalMilliseconds" :min="50" :max="86400000" :step="50" controls-position="right" @change="syncRemoteDevice(row, true)" /><span>ms</span></div></template></el-table-column>
            <el-table-column label="点位" width="75"><template #default="{ row }">{{ row.points?.length ?? 0 }}</template></el-table-column>
            <el-table-column label="操作" width="120" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="copyRemoteDevice(remoteDevices.indexOf(row))">复制</el-button><el-button link type="danger" @click="removeRemoteDevice(remoteDevices.indexOf(row))">删除</el-button></template></el-table-column>
                    </el-table>
                    <el-table v-else :data="remoteForwardDevicesForChannel(channel)" row-key="deviceKey" empty-text="该通道暂未添加转发设备" class="remote-device-table remote-forward-device-table">
                      <el-table-column type="expand" width="48">
                        <template #default="{ row: forwardDevice }">
                          <div class="remote-point-panel remote-forward-point-panel">
                            <div class="remote-device-extra">
                              <label><span>设备状态</span><el-switch v-model="forwardDevice.enabled" inline-prompt active-text="启用" inactive-text="停用" /></label>
                              <template v-if="forwardDevice.protocol === 'modbus-tcp-slave'">
                                <label><span>从站地址</span><el-input-number v-model="forwardDevice.unitId" :min="1" :max="247" controls-position="right" /></label>
                              </template>
                              <template v-else-if="forwardDevice.protocol === 'iec104-server'">
                                <label><span>公共地址</span><el-input-number v-model="forwardDevice.commonAddress" :min="1" :max="65535" controls-position="right" /></label>
                              </template>
                              <template v-else-if="forwardDevice.protocol === 'iec61850-mms-server'">
                                <label><span>IED 名称</span><el-input v-model="forwardDevice.iedName" /></label>
                                <label><span>逻辑设备</span><el-input v-model="forwardDevice.logicalDevice" /></label>
                              </template>
                              <template v-else-if="forwardDevice.protocol === 'iec61850-goose-publisher'">
                                <label><span>网卡</span><el-input v-model="forwardDevice.interfaceName" /></label>
                                <label><span>GoCBRef</span><el-input v-model="forwardDevice.goCbRef" /></label>
                                <label><span>DataSetRef</span><el-input v-model="forwardDevice.dataSetRef" /></label>
                                <label><span>APPID</span><el-input-number v-model="forwardDevice.appId" :min="0" :max="65535" controls-position="right" /></label>
                                <label><span>目标 MAC</span><el-input v-model="forwardDevice.destinationMac" /></label>
                                <label><span>VLAN 优先级</span><el-input-number v-model="forwardDevice.vlanPriority" :min="0" :max="7" controls-position="right" /></label>
                              </template>
                            </div>
                            <div class="remote-point-heading">
                              <div><strong>{{ forwardDevice.name }} 的转发点位</strong><small>选择采集来源，并配置对外暴露的地址与数据格式</small></div>
                              <el-button size="small" type="warning" plain :icon="Plus" @click="addRemoteForwardPoint(forwardDevice)">新增转发点位</el-button>
                            </div>
                            <el-table :data="forwardDevice.points" size="small" empty-text="暂无转发点位">
                              <el-table-column label="名称" min-width="130"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
                              <el-table-column label="源设备" min-width="170"><template #default="{ row }"><el-select v-model="row.sourceDeviceKey" filterable allow-create default-first-option @change="row.sourceMetric = ''"><el-option v-for="sourceDevice in remoteDevices" :key="sourceDevice.deviceKey" :label="`${sourceDevice.name} (${sourceDevice.deviceKey})`" :value="sourceDevice.deviceKey" /></el-select></template></el-table-column>
                              <el-table-column label="源点位" min-width="180"><template #default="{ row }"><el-select v-model="row.sourceMetric" filterable allow-create default-first-option :disabled="!row.sourceDeviceKey" @change="syncRemoteForwardSource(row)"><el-option v-for="sourcePoint in remoteSourcePoints(row.sourceDeviceKey)" :key="sourcePoint.metric" :label="`${sourcePoint.name} (${sourcePoint.metric})`" :value="sourcePoint.metric" /></el-select></template></el-table-column>
                              <el-table-column label="转发标识符" min-width="145"><template #default="{ row }"><el-input v-model="row.metric" /></template></el-table-column>
                              <el-table-column label="转发地址" min-width="235">
                                <template #default="{ row }">
                                  <div v-if="forwardDevice.protocol === 'modbus-tcp-slave'" class="remote-inline-fields"><el-select v-model="row.function"><el-option v-for="fc in [1, 2, 3, 4]" :key="fc" :label="`FC${fc}`" :value="fc" /></el-select><el-input-number v-model="row.register" :min="0" :max="65535" controls-position="right" /></div>
                                  <div v-else-if="forwardDevice.protocol === 'iec104-server'" class="remote-inline-fields"><el-input-number v-model="row.ioa" :min="1" :max="16777215" controls-position="right" /><el-select v-model="row.pointType"><el-option label="单点遥信" value="M_SP_NA_1" /><el-option label="双点遥信" value="M_DP_NA_1" /><el-option label="短浮点遥测" value="M_ME_NC_1" /><el-option label="累计量" value="M_IT_NA_1" /></el-select></div>
                                  <div v-else-if="forwardDevice.protocol === 'iec61850-mms-server'" class="remote-inline-fields"><el-input v-model="row.objectRef" placeholder="对象引用" /><el-select v-model="row.fc"><el-option label="MX" value="MX" /><el-option label="ST" value="ST" /></el-select></div>
                                  <span v-else class="remote-address-unit">按数据集顺序发布</span>
                                </template>
                              </el-table-column>
                              <el-table-column label="数据类型" min-width="125"><template #default="{ row }"><el-select v-model="row.dataType"><el-option v-for="type in remoteForwardDataTypes(forwardDevice.protocol)" :key="type" :label="type" :value="type" /></el-select></template></el-table-column>
                              <el-table-column label="数量" width="90"><template #default="{ row }"><el-input-number v-model="row.quantity" :min="1" :controls="false" /></template></el-table-column>
                              <el-table-column label="倍率" width="100"><template #default="{ row }"><el-input-number v-model="row.scale" :controls="false" /></template></el-table-column>
                              <el-table-column label="偏移" width="100"><template #default="{ row }"><el-input-number v-model="row.offset" :controls="false" /></template></el-table-column>
                              <el-table-column label="单位" width="90"><template #default="{ row }"><el-input v-model="row.unit" /></template></el-table-column>
                              <el-table-column label="操作" width="120" fixed="right"><template #default="{ $index }"><el-button link type="primary" @click="copyRemoteForwardPoint(forwardDevice, $index)">复制</el-button><el-button link type="danger" @click="removeRemoteForwardPoint(forwardDevice, $index)">删除</el-button></template></el-table-column>
                            </el-table>
                          </div>
                        </template>
                      </el-table-column>
                      <el-table-column label="设备编号" min-width="160"><template #default="{ row }"><el-input v-model="row.deviceKey" /></template></el-table-column>
                      <el-table-column label="设备名称" min-width="160"><template #default="{ row }"><el-input v-model="row.name" /></template></el-table-column>
                      <el-table-column label="协议" min-width="180"><template #default="{ row }"><el-tag type="warning" effect="plain">{{ remoteProtocolLabel(row.protocol) }}</el-tag></template></el-table-column>
                      <el-table-column label="状态" width="100"><template #default="{ row }"><el-switch v-model="row.enabled" /></template></el-table-column>
                      <el-table-column label="点位" width="80"><template #default="{ row }">{{ row.points?.length ?? 0 }}</template></el-table-column>
                      <el-table-column label="操作" width="120" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="copyRemoteForwardDevice(remoteForwardDevices.indexOf(row))">复制</el-button><el-button link type="danger" @click="removeRemoteForwardDevice(remoteForwardDevices.indexOf(row))">删除</el-button></template></el-table-column>
                    </el-table>
                  </article>
                </section>
              </template>
              <div v-if="!remoteChannelsForResource(resource, 'collect').length && !remoteChannelsForResource(resource, 'forward').length" class="remote-hierarchy-empty">该资源暂无通道</div>
            </section>
            <el-empty v-if="!remoteHierarchyResources.length" description="网关暂未上报资源和通道结构" :image-size="72" />
          </div>
        </div>
        <div v-else class="remote-json-editor">
          <el-input v-model="remoteConfigText" type="textarea" :autosize="{ minRows: 5, maxRows: 14 }" spellcheck="false" placeholder="请输入 JSON 配置补丁" />
          <p>高级模式适合批量修改采集与转发配置；数组字段会整体替换，例如 <code>devices</code>、<code>forwardDevices</code> 及各自的 <code>points</code>。</p>
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

    <section v-if="device?.deviceType === 'GATEWAY_CHILD' && device.gateway" v-show="detailTab === 'overview'" class="relation-panel">
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

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'relations'" class="topology-panel detail-section-first">
      <div class="section-heading">
        <div>
          <h2>网关上报拓扑</h2>
          <p>Go 网关上报本地子设备和点表后，管理员确认才会写入正式云端关系。</p>
        </div>
        <div class="topology-actions">
          <el-tag v-if="topologyDraft" :type="topologyStatusType(topologyDraft.status)">{{ topologyStatusLabel(topologyDraft.status) }}</el-tag>
          <el-button :icon="RefreshRight" :loading="topologyDraftLoading" @click="loadTopologyDraft()">刷新</el-button>
          <el-button v-if="canManage && topologyDraft && ['PENDING', 'CONFIRMED'].includes(topologyDraft.status)" type="primary" :disabled="topologyDraft.status === 'CONFIRMED'" :loading="topologyDraftApplying" @click="confirmTopologyDraft">确认拓扑</el-button>
          <el-button v-if="canManage && topologyDraft && ['PENDING', 'CONFLICT'].includes(topologyDraft.status)" :loading="topologyDraftApplying" @click="ignoreTopologyDraft">忽略</el-button>
        </div>
      </div>
      <div v-if="topologyDraft" class="topology-summary">
        <div><span>上报时间</span><strong>{{ formatDateTime(topologyDraft.reportedAt) }}</strong></div>
        <div><span>配置版本</span><code>{{ topologyDraft.reportedConfigVersion || '-' }}</code></div>
        <div><span>新增</span><strong>{{ topologyDraftCounts.NEW }}</strong></div>
        <div><span>更新</span><strong>{{ topologyDraftCounts.UPDATE }}</strong></div>
        <div><span>冲突</span><strong>{{ topologyDraftCounts.CONFLICT }}</strong></div>
        <div><span>平台禁用</span><strong>{{ topologyDraftCounts.PLATFORM_DISABLED }}</strong></div>
        <div><span>未上报</span><strong>{{ topologyDraftCounts.MISSING_UNREPORTED }}</strong></div>
      </div>
      <el-alert v-if="topologyDraft?.status === 'CONFLICT'" class="topology-alert" type="error" :closable="false" show-icon title="拓扑存在冲突，请先处理重复 deviceKey 或跨网关归属后再确认。" />
      <el-table v-loading="topologyDraftLoading" :data="topologyDraft?.items ?? []" empty-text="暂无网关上报拓扑" class="topology-table">
        <el-table-column label="状态" width="120">
          <template #default="{ row }"><el-tag :type="topologyItemStatusType(row.status)">{{ topologyItemStatusLabel(row.status) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="deviceKey" label="子设备编号" min-width="150" />
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="protocol" label="协议" width="120" />
        <el-table-column prop="address" label="地址" min-width="160" />
        <el-table-column prop="slaveId" label="站号" width="90" />
        <el-table-column label="点位" width="90"><template #default="{ row }">{{ row.pointCount ?? row.metrics?.length ?? 0 }}</template></el-table-column>
        <el-table-column label="指标摘要" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">{{ (row.metrics ?? []).map((metric: GatewayTopologyMetric) => metric.identifier).join(', ') || '-' }}</template>
        </el-table-column>
        <el-table-column label="冲突/说明" min-width="230" show-overflow-tooltip>
          <template #default="{ row }">{{ row.conflictReason || (row.status === 'MISSING_UNREPORTED' ? '当前云端子设备未出现在网关本次上报中' : '-') }}</template>
        </el-table-column>
      </el-table>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'relations'" class="relation-panel">
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

    <DeviceEventsPanel v-if="device && detailTab === 'events'" :key="device.id" :device="device" />

    <section v-if="credentials" v-show="detailTab === 'config' && connectionTab === 'mqtt'" class="mqtt-panel detail-section-first">
      <div class="section-heading mqtt-panel-heading">
        <div>
          <h2>MQTT 接入</h2>
          <p>查看当前认证方式、接入地址和 Topic；设备密码只会在创建或重置后显示一次。</p>
        </div>
        <el-button v-if="canManage" type="danger" plain :loading="rotatingCredentials" @click="rotateCredentials">重置设备 MQTT 密码</el-button>
      </div>
      <div class="mqtt-auth-card" :class="credentials.authMode.toLowerCase()">
        <div>
          <span>当前认证方式</span>
          <el-tag :type="mqttAuthPresentation.type" effect="light">{{ mqttAuthPresentation.label }}</el-tag>
        </div>
        <p>{{ mqttAuthPresentation.description }}</p>
        <code v-if="credentials.activationHardwareId">Hardware ID：{{ credentials.activationHardwareId }}</code>
      </div>
      <el-alert
        v-if="credentials.resetAffectsCurrentConnection"
        class="mqtt-impact-alert"
        type="warning"
        :closable="false"
        show-icon
        title="重置后旧密码立即失效；现有连接可能暂时保持，但下次重连必须使用新密码。"
      />
      <el-alert
        v-else
        class="mqtt-impact-alert"
        type="info"
        :closable="false"
        show-icon
        title="重置下方设备 MQTT 密码不会修改当前激活连接或网关代发链路。"
      />
      <div class="mqtt-grid">
        <div><span>Broker</span><code>{{ credentials.host }}:{{ credentials.port }}</code></div>
        <div><span>WebSocket</span><code>{{ credentials.host }}:{{ credentials.wsPort }}</code></div>
        <div><span>Client ID</span><code>{{ credentials.clientId }}</code></div>
        <div><span>Username</span><code>{{ credentials.username }}</code></div>
        <div class="mqtt-password-cell"><span>Password</span><code>{{ credentials.password || '已隐藏，重置后显示新密码' }}</code></div>
        <div><span>密码更新时间</span><strong>{{ formatDateTime(credentials.passwordUpdatedAt) }}</strong></div>
      </div>
      <div class="topic-list">
        <div><span>心跳主题</span><code>{{ credentials.heartbeatTopic }}</code></div>
        <div><span>遥测主题</span><code>{{ credentials.telemetryTopic }}</code></div>
      </div>
    </section>

    <section v-if="device?.deviceType === 'GATEWAY'" v-show="detailTab === 'config' && connectionTab === 'access'" class="remote-access-panel detail-section-first">
      <div class="remote-access-hero">
        <span class="remote-access-icon"><Connection /></span>
        <div>
          <h2>远程访问</h2>
          <p>通过平台安全维护通道打开 EG100 管理页面，无需在公网暴露网关 Web 端口。</p>
        </div>
        <el-tag :type="remoteAccessConnected ? 'success' : 'info'" effect="plain">{{ remoteAccessConnected ? '通道已连接' : '通道未连接' }}</el-tag>
      </div>
      <div class="remote-access-body">
        <div>
          <strong>{{ remoteAccessConnected ? '可以发起远程维护会话' : '暂时无法远程访问' }}</strong>
          <p>{{ remoteAccessConnected ? '访问链接按需生成并在新标签页打开。' : '请先确认 EG100 在线，并检查远程访问服务和网络连接。' }}</p>
        </div>
        <div v-if="canManage" class="remote-access-actions">
          <el-button :icon="Connection" :loading="openingRemoteTerminal" :disabled="!remoteAccessConnected" @click="openRemoteTerminal">打开 SSH 终端</el-button>
          <el-button type="primary" :icon="Connection" :loading="openingRemoteAccess" :disabled="!remoteAccessConnected" @click="openRemoteAccess">打开远程访问</el-button>
        </div>
      </div>
    </section>

    <section v-show="detailTab === 'monitor'" class="metrics-section detail-section-first">
      <div class="section-heading">
        <div><h2>实时指标</h2><p>设备推送新的标识符后会自动生成对应卡片</p></div>
        <div class="monitor-actions">
          <span v-if="paused && pausedQueue.length" class="queue-tip">已缓存 {{ pausedQueue.length }} 条</span>
          <el-button v-if="canManage && device?.deviceType === 'GATEWAY_CHILD'" @click="commandsPanel?.showHistory()">下发记录</el-button>
          <el-button :icon="RefreshRight" @click="refreshPage">手动同步</el-button>
          <el-button :type="paused ? 'primary' : 'default'" :icon="paused ? VideoPlay : VideoPause" @click="togglePause">{{ paused ? '继续显示' : '暂停画面' }}</el-button>
          <el-dropdown v-if="hiddenAvailableMetricKeys.length" trigger="click" placement="bottom-end" @command="restoreMetricCard">
            <el-button>已隐藏 {{ hiddenAvailableMetricKeys.length }} 张卡片</el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="key in hiddenAvailableMetricKeys" :key="key" :command="key">显示 {{ metricLabel(key) }}</el-dropdown-item>
                <el-dropdown-item v-if="hiddenAvailableMetricKeys.length > 1" divided command="__all__">全部显示</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
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
          <el-dropdown class="metric-card-menu" trigger="click" placement="right-start" @command="hideMetricCard(metric.key)" @mousedown.stop @touchstart.stop @dragstart.stop.prevent>
            <button class="metric-card-menu-trigger" type="button" title="更多操作" :aria-label="`${metricLabel(metric.key)}更多操作`" :draggable="false">⋯</button>
            <template #dropdown><el-dropdown-menu><el-dropdown-item command="hide">隐藏卡片</el-dropdown-item></el-dropdown-menu></template>
          </el-dropdown>
          <p>{{ metricLabel(metric.key) }}</p>
          <div v-if="canManage && device?.deviceType === 'GATEWAY_CHILD' && writablePointTypes.get(metric.key) === 'BOOLEAN'" class="metric-boolean-value" :draggable="false" @mousedown.stop @touchstart.stop @click.stop @dragstart.stop.prevent>
            <strong class="metric-boolean-state"><span>{{ booleanTelemetryValue(metric.value) === undefined ? '--' : String(booleanTelemetryValue(metric.value)) }}</span><small>{{ booleanTelemetryValue(metric.value) === undefined ? '状态未知' : booleanTelemetryValue(metric.value) ? '开启' : '关闭' }}</small></strong>
            <el-switch v-if="booleanTelemetryValue(metric.value) !== undefined" :model-value="booleanTelemetryValue(metric.value) === true" :disabled="!commandCapabilities.available || pendingCommandKeys.includes(metric.key)" :aria-label="`切换${metricLabel(metric.key)}`" :title="pendingCommandKeys.includes(metric.key) ? '等待网关执行回执' : commandCapabilities.reason || '切换前会再次确认，状态以设备上报为准'" @change="commandsPanel?.toggle(metric.key, $event === true)" />
          </div>
          <strong v-else>{{ displayValue(metric.value, metric.key) }}<small>{{ metricUnit(metric.key) }}</small></strong>
          <span v-if="metricLabel(metric.key) !== metric.key">{{ metric.key }}</span>
          <div class="metric-card-footer">
            <time class="metric-update-time" :datetime="metric.time" :title="formatDateTime(metric.time)">{{ metricUpdateText(metric.time) }}</time>
            <div v-if="canManage && device?.deviceType === 'GATEWAY_CHILD' && writableMetricKeys.has(metric.key) && writablePointTypes.get(metric.key) !== 'BOOLEAN'" class="metric-command-action" :draggable="false" @dragstart.stop.prevent>
              <el-button type="primary" plain size="small" :disabled="!commandCapabilities.available" :title="commandCapabilities.reason || '网关上报为可写点位'" @click.stop="commandsPanel?.open(metric.key)">下发</el-button>
            </div>
          </div>
        </article>
      </div>
      <el-empty v-else :description="hiddenAvailableMetricKeys.length ? '卡片已隐藏，可从上方恢复显示' : '等待设备推送遥测数据'" />
    </section>

    <DeviceCommandsPanel v-if="canManage && device?.deviceType === 'GATEWAY_CHILD'" ref="commandsPanel" :key="device.id" :device-id="device.id" :device-name="device.name" @capabilities="commandCapabilities = $event" @pending="pendingCommandKeys = $event" />

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
            <el-table-column label="权限" width="112"><template #default="{ row }"><el-tooltip v-if="device?.deviceType === 'GATEWAY_CHILD'" content="读写能力由网关自动上报，无需手动配置"><el-tag size="small" :type="row.accessMode === 'READ_WRITE' ? 'success' : 'info'">{{ row.accessMode === 'READ_WRITE' ? '可写' : '只读' }}</el-tag></el-tooltip><el-select v-else v-model="row.accessMode"><el-option label="只读" value="READ_ONLY" /><el-option label="读写" value="READ_WRITE" /></el-select></template></el-table-column>
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
.remote-access-button { color: #1677ff; }
.detail-tabs { display: flex; gap: 6px; margin: 18px 0; padding: 4px; border: 1px solid #dfe7f1; border-radius: 12px; background: #f7f9fc; }
.detail-tabs button { display: inline-flex; min-width: 126px; height: 38px; align-items: center; justify-content: center; gap: 7px; padding: 0 18px; border: 1px solid transparent; border-radius: 9px; color: #475569; background: transparent; cursor: pointer; transition: all 0.18s ease; }
.detail-tabs button:hover { color: #2563eb; background: #fff; }
.detail-tabs button.active { border-color: #bfdbfe; color: #1677ff; background: #fff; box-shadow: 0 4px 12px rgb(37 99 235 / 8%); }
.detail-tabs button span { display: inline-flex; min-width: 20px; height: 20px; align-items: center; justify-content: center; padding: 0 6px; border-radius: 999px; color: #64748b; background: #eaf0f7; font-size: 12px; }
.detail-tabs button.active span { color: #1677ff; background: #eaf4ff; }
.connection-tabs { display: grid; grid-template-columns: repeat(3, minmax(0, 220px)); gap: 8px; margin: -6px 0 14px; padding: 4px; border-bottom: 1px solid #e5edf7; }
.connection-tabs button { display: flex; min-width: 0; flex-direction: column; gap: 3px; align-items: flex-start; padding: 10px 14px; border: 1px solid transparent; border-radius: 9px 9px 0 0; color: #475569; text-align: left; background: transparent; cursor: pointer; transition: border-color .18s ease, color .18s ease, background .18s ease; }
.connection-tabs button:hover { color: #2563eb; background: #f8fbff; }
.connection-tabs button.active { border-color: #bfdbfe; color: #1677ff; background: #eff6ff; box-shadow: inset 0 -2px #1677ff; }
.connection-tabs button small { overflow: hidden; max-width: 100%; color: #94a3b8; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; }
.connection-tabs button.active small { color: #5f8fc8; }
.dot { display: inline-block; width: 8px; height: 8px; margin-right: 7px; border-radius: 50%; background: #94a3b8; }
.dot.live { background: #22c55e; box-shadow: 0 0 0 4px #dcfce7; }
.dot.connecting, .dot.idle { background: #f59e0b; }
.dot.disconnected { background: #ef4444; }
.device-summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: 12px; padding: 16px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.device-summary > div { display: flex; min-height: 50px; gap: 8px; align-items: center; justify-content: space-between; padding: 10px 12px; border-radius: 10px; background: #f8fafc; font-size: 13px; }
.device-summary > div > span:first-child { flex: 0 0 auto; color: #94a3b8; }
.device-summary > div > strong { overflow: hidden; color: #111827; text-overflow: ellipsis; white-space: nowrap; }
.device-group-summary { align-content: center; flex-wrap: wrap; }
.device-group-tags { display: flex; min-width: 0; flex: 1; flex-wrap: wrap; justify-content: flex-end; gap: 5px; }
.device-group-tags .el-tag { max-width: 100%; }
.device-group-tags .el-tag :deep(.el-tag__content) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.overview-navigation { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 14px; margin-top: 18px; }
.overview-navigation button { position: relative; min-height: 118px; padding: 18px; border: 1px solid #e2e8f0; border-radius: 12px; text-align: left; background: linear-gradient(145deg, #fff, #f8fbff); cursor: pointer; transition: border-color .18s ease, box-shadow .18s ease, transform .18s ease; }
.overview-navigation button:hover { border-color: #93c5fd; box-shadow: 0 10px 24px rgb(37 99 235 / 9%); transform: translateY(-2px); }
.overview-navigation strong, .overview-navigation span, .overview-navigation em { display: block; }
.overview-navigation strong { color: #17233b; font-size: 15px; }
.overview-navigation span { margin-top: 8px; color: #7a889e; font-size: 12px; line-height: 1.55; }
.overview-navigation em { margin-top: 12px; color: #1677ff; font-size: 12px; font-style: normal; font-weight: 600; }
.relation-panel { margin-top: 18px; padding: 20px; border: 1px solid #e9eef5; border-radius: 12px; background: #fff; }
.topology-panel { margin-top: 18px; padding: 20px; border: 1px solid #dbeafe; border-radius: 12px; background: #fff; box-shadow: 0 12px 30px rgb(37 99 235 / 5%); }
.topology-actions { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; justify-content: flex-end; }
.topology-summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; margin-top: 16px; }
.topology-summary div { min-width: 0; padding: 12px 14px; border-radius: 9px; background: #f8fafc; }
.topology-summary span { display: block; margin-bottom: 5px; color: #94a3b8; font-size: 12px; }
.topology-summary strong { color: #111827; font-size: 16px; }
.topology-summary code { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.topology-alert { margin-top: 14px; }
.topology-table { margin-top: 14px; border: 1px solid #e5eaf1; border-radius: 9px; }
.remote-config-panel { margin-top: 18px; padding: 20px; border: 1px solid #dbeafe; border-radius: 12px; background: #fff; box-shadow: 0 12px 30px rgb(37 99 235 / 5%); }
.remote-config-panel.detail-section-first, .topology-panel.detail-section-first { margin-top: 0; }
.remote-config-tip { margin-top: 14px; }
.remote-config-editor { margin-top: 14px; }
.remote-config-mode { margin-bottom: 14px; }
.remote-basic-form { padding: 16px; border: 1px solid #e5edf7; border-radius: 10px; background: #f8fafc; }
.remote-json-editor p { margin: 6px 0 0; color: #94a3b8; font-size: 12px; }
.remote-device-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 18px; }
.remote-device-heading strong { color: #111827; font-size: 14px; }
.remote-device-heading p { margin: 5px 0; color: #64748b; font-size: 12px; }
.remote-device-heading small { color: #94a3b8; }
.remote-read-actions { display: flex; flex: 0 0 auto; align-items: center; gap: 12px; }
.remote-read-actions small { white-space: nowrap; }
.remote-config-hierarchy { display: grid; gap: 14px; margin-top: 14px; }
.remote-resource-card { overflow: hidden; border: 1px solid #dbe5f1; border-radius: 12px; background: #fff; box-shadow: 0 5px 16px rgb(15 23 42 / 4%); }
.remote-resource-head { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 11px; align-items: center; padding: 13px 15px; border-bottom: 1px solid #e7edf5; background: linear-gradient(90deg, #f8fbff 0%, #fff 72%); }
.remote-resource-head > div { min-width: 0; }
.remote-resource-head strong, .remote-resource-head small { display: block; }
.remote-resource-head strong { overflow: hidden; color: #14213d; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.remote-resource-head small { margin-top: 3px; color: #7c8ba1; font-size: 12px; }
.remote-resource-icon { display: inline-flex; width: 38px; height: 38px; align-items: center; justify-content: center; border-radius: 10px; color: #1677ff; background: #eaf3ff; font-size: 10px; font-weight: 800; letter-spacing: .02em; }
.remote-resource-icon.serial { color: #7c3aed; background: #f1ebff; }
.remote-role-group { padding: 12px 14px 14px; }
.remote-role-group + .remote-role-group { padding-top: 0; }
.remote-role-title { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; color: #2563eb; }
.remote-role-title::before { width: 4px; height: 14px; border-radius: 999px; background: #3b82f6; content: ''; }
.remote-role-title strong { font-size: 13px; }
.remote-role-title span { color: #91a0b5; font-size: 12px; }
.remote-role-group.forward .remote-role-title { color: #b45309; }
.remote-role-group.forward .remote-role-title::before { background: #f59e0b; }
.remote-channel-card { overflow: hidden; border: 1px solid #e2e8f0; border-radius: 10px; background: #fff; }
.remote-channel-card + .remote-channel-card { margin-top: 10px; }
.remote-channel-head { display: flex; flex-wrap: wrap; gap: 10px 16px; align-items: center; justify-content: space-between; min-height: 48px; padding: 8px 11px 8px 14px; background: #fbfdff; }
.remote-channel-head > div:first-child { display: flex; min-width: 0; align-items: center; gap: 9px; }
.remote-channel-head strong { overflow: hidden; color: #1f2d45; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.remote-channel-head > div:first-child span { padding: 3px 7px; border-radius: 5px; color: #2563eb; background: #edf5ff; font-size: 11px; white-space: nowrap; }
.remote-role-group.forward .remote-channel-head > div:first-child span { color: #a16207; background: #fff7e6; }
.remote-channel-summary { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; color: #7c8ba1; font-size: 12px; }
.remote-device-table { border-top: 1px solid #edf1f6; }
.remote-device-table :deep(.el-input-number) { width: 100%; }
.remote-forward-device-table :deep(.el-table__expanded-cell) { border-left: 3px solid #f59e0b; }
.remote-forward-device-table :deep(.el-switch.is-checked .el-switch__core) { border-color: #f59e0b; background: #f59e0b; }
.remote-hierarchy-empty { padding: 16px; color: #94a3b8; text-align: center; font-size: 12px; }
.remote-device-interval { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 6px; color: #64748b; font-size: 12px; }
.remote-address-unit { align-self: center; color: #64748b; font-size: 12px; white-space: nowrap; }
.remote-point-panel { padding: 14px 16px 18px 62px; background: #f8fafc; }
.remote-device-extra { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 10px; margin-bottom: 14px; padding: 12px; border: 1px solid #e2e8f0; border-radius: 9px; background: #fff; }
.remote-device-extra label span { display: block; margin-bottom: 5px; color: #64748b; font-size: 12px; }
.remote-device-extra :deep(.el-input-number) { width: 100%; }
.remote-point-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.remote-point-heading strong { color: #334155; font-size: 13px; }
.remote-point-heading > div small { display: block; margin-top: 4px; color: #94a3b8; font-size: 11px; }
.remote-forward-point-panel { background: #fffbf3; }
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
code { color: #2563eb; font-family: Consolas, monospace; }
.mqtt-panel, .remote-access-panel { padding: 20px; border: 1px solid #dbeafe; border-radius: 12px; background: #fff; box-shadow: 0 12px 30px rgb(37 99 235 / 5%); }
.mqtt-panel-heading { gap: 16px; }
.mqtt-panel-heading > div { min-width: 0; }
.mqtt-auth-card { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 12px 18px; align-items: center; margin-top: 16px; padding: 15px 16px; border: 1px solid #cfe2ff; border-radius: 10px; background: linear-gradient(100deg, #f3f8ff, #fff 75%); }
.mqtt-auth-card > div { display: flex; align-items: center; gap: 10px; }
.mqtt-auth-card > div > span { color: #64748b; font-size: 12px; }
.mqtt-auth-card p { margin: 0; color: #42526b; font-size: 13px; line-height: 1.55; }
.mqtt-auth-card code { color: #64748b; font-size: 11px; overflow-wrap: anywhere; }
.mqtt-auth-card.gateway_proxy { border-color: #fde2b8; background: linear-gradient(100deg, #fff8eb, #fff 75%); }
.mqtt-impact-alert { margin-top: 12px; }
.mqtt-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 10px; }
.mqtt-grid { margin-top: 14px; }
.mqtt-grid div, .topic-list div { min-width: 0; padding: 12px 14px; border-radius: 9px; background: #f8fafc; }
.mqtt-grid span, .topic-list span { display: block; margin-bottom: 6px; color: #94a3b8; font-size: 12px; }
.mqtt-grid code, .topic-list code { display: block; overflow-wrap: anywhere; line-height: 1.5; }
.mqtt-grid strong { color: #111827; font-size: 13px; }
.mqtt-password-cell { border: 1px dashed #cbd5e1; background: #fbfdff !important; }
.topic-list { display: grid; gap: 12px; margin-top: 12px; }
.remote-access-hero { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 14px; align-items: center; padding: 18px; border-radius: 12px; background: linear-gradient(120deg, #eff6ff, #f8fbff 58%, #fff); }
.remote-access-icon { display: inline-flex; width: 48px; height: 48px; align-items: center; justify-content: center; border-radius: 14px; color: #1677ff; background: #fff; box-shadow: 0 8px 20px rgb(37 99 235 / 12%); }
.remote-access-icon :deep(svg) { width: 24px; height: 24px; }
.remote-access-hero h2, .remote-access-body strong { color: #17233b; }
.remote-access-hero p, .remote-access-body p { margin: 6px 0 0; color: #74839a; font-size: 13px; line-height: 1.55; }
.remote-access-body { display: flex; gap: 20px; align-items: center; justify-content: space-between; margin-top: 14px; padding: 16px 18px; border: 1px solid #e7edf5; border-radius: 10px; }
.remote-access-actions { display: flex; align-items: center; gap: 10px; flex: 0 0 auto; }
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
.metrics-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 10px; margin-top: 14px; }
.metrics-grid.sorting { opacity: 0.72; pointer-events: none; }
.metric-card { position: relative; display: flex; flex-direction: column; padding: 13px 14px; border: 1px solid #edf1f5; border-radius: 10px; background: #fafcff; transition: border-color 0.15s ease, box-shadow 0.15s ease, opacity 0.15s ease, transform 0.15s ease; }
.metric-card[draggable="true"] { cursor: grab; }
.metric-card[draggable="true"]:active { cursor: grabbing; }
.metric-card.dragging { opacity: 0.48; transform: scale(0.98); }
.metric-card.drag-over { border-color: #2563eb; box-shadow: 0 0 0 3px rgb(37 99 235 / 12%); }
.metric-card-menu { position: absolute; top: 7px; right: 7px; z-index: 2; }
.metric-card-menu-trigger { width: 26px; height: 26px; border: 0; border-radius: 8px; color: #94a3b8; background: transparent; cursor: pointer; font-size: 20px; line-height: 20px; }
.metric-card-menu-trigger:hover, .metric-card-menu-trigger:focus-visible { color: #2563eb; background: #eff6ff; }
.metric-card p { margin: 0 22px 8px 0; color: #64748b; font-size: 13px; }
.metric-card strong { display: block; overflow-wrap: anywhere; color: #111827; font-size: 23px; }
.metric-card small { margin-left: 4px; color: #94a3b8; font-size: 13px; }
.metric-card > span { display: block; margin-top: 7px; color: #a1aab8; font: 12px Consolas, monospace; }
.metric-boolean-value { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 42px; cursor: default; }
.metric-card .metric-boolean-state { display: flex; flex-direction: column; gap: 3px; min-width: 0; font-size: 20px; line-height: 1.1; white-space: nowrap; }
.metric-boolean-state small { margin: 0; font-size: 11px; font-weight: 500; line-height: 1.2; }
.metric-boolean-value :deep(.el-switch) { flex: none; }
.metric-card-footer { display: flex; align-items: flex-end; justify-content: space-between; gap: 8px; margin-top: auto; padding-top: 10px; }
.metric-update-time { color: #94a3b8; font-size: 11px; line-height: 24px; white-space: nowrap; }
.metric-command-action { display: flex; justify-content: flex-end; cursor: default; }
.metric-command-action :deep(.el-button span) { margin-top: 0; color: inherit; font: inherit; }
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
@media (max-width: 700px) { .detail-tabs { overflow-x: auto; } .detail-tabs button { min-width: 112px; } .connection-tabs { display: flex; overflow-x: auto; } .connection-tabs button { min-width: 150px; } .device-summary { grid-template-columns: 1fr; } .gateway-card { grid-template-columns: 1fr; } .trend-stats { grid-template-columns: repeat(2, 1fr); } .monitor-actions { align-items: flex-end; flex-direction: column; } .remote-config-actions, .remote-device-heading, .mqtt-panel-heading, .remote-access-body { align-items: stretch; flex-direction: column; } .remote-access-actions { align-items: stretch; flex-direction: column; } .remote-read-actions { align-items: flex-start; flex-direction: column; } .remote-channel-head { align-items: flex-start; flex-direction: column; } .remote-point-panel { padding-left: 12px; } .mqtt-auth-card, .remote-access-hero { grid-template-columns: 1fr; } }
</style>
