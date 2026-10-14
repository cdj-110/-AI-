<script setup lang="ts">
import { Bell, Connection, Monitor, OfficeBuilding, Setting, SwitchButton, Tickets, User } from '@element-plus/icons-vue';
import { ElMessageBox } from 'element-plus';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import { isAdminPortal, portalEyebrow, portalLogo, portalName } from '../portal';
import { useUserStore } from '../stores/user';
import { userDisplayName } from '../utils/user-display-name';
import { getAuthItem, setAuthItem } from '../utils/auth-storage';

interface DeviceItem {
  id: string;
  deviceKey: string;
  name: string;
  status: string;
  lastSeenAt?: string;
}

interface StatusToast {
  id: number;
  alarmId?: string;
  title: string;
  message: string;
  status: string;
  tone?: string;
  icon?: string;
}

interface AlarmEvent {
  id: string;
  tenantId: string;
  deviceId: string;
  deviceName?: string;
  level: string;
  message: string;
  status: string;
  createdAt?: string;
  resolvedAt?: string;
  notificationRead?: boolean;
  device?: { id: string; name: string; deviceKey: string };
}

interface SystemNotification {
  id: number | string;
  title: string;
  message: string;
  createdAt: string;
  status: string;
}

interface AdminNotification {
  id: string;
  category: 'ORDER' | 'SUBSCRIPTION';
  title: string;
  message: string;
  createdAt: string;
  route: string;
  read: boolean;
}

const router = useRouter();
const route = useRoute();
const userStore = useUserStore();
// 记录每个设备的上一次状态，用于判断是否需要弹出上线/离线提醒。
const deviceStatusMap = new Map<string, string>();
const lastReminderMap = new Map<string, number>();
const statusMonitorReady = ref(false);
const soundEnabled = ref(true);
const statusToasts = ref<StatusToast[]>([]);
const notificationVisible = ref(false);
const notificationTab = ref('alarms');
const notificationLoading = ref(false);
const notificationAlarms = ref<AlarmEvent[]>([]);
const alarmNotificationTotal = ref(0);
const subscriptionNotificationTotal = ref(0);
const systemNotifications = ref<SystemNotification[]>([]);
const adminNotifications = ref<AdminNotification[]>([]);
const adminNotificationTotal = ref(0);
const reminderThrottleMs = 5000;
const alarmWindowMs = 10000;
const alarmSoundThrottleMs = 5000;
let statusMonitorTimer: ReturnType<typeof setInterval> | undefined;
let statusStreamController: AbortController | undefined;
let statusStreamReconnectTimer: ReturnType<typeof setTimeout> | undefined;
let alarmStreamController: AbortController | undefined;
let alarmStreamReconnectTimer: ReturnType<typeof setTimeout> | undefined;
let audioContext: AudioContext | undefined;
let audioUnlocked = false;
let statusToastId = 0;
let alarmWindowStart = 0;
let alarmWindowCount = 0;
let alarmAggregateToastId = 0;
let lastAlarmSoundAt = 0;
let adminNotificationTimer: ReturnType<typeof setInterval> | undefined;
const idleWarningVisible = ref(false);
const idleRemainingSeconds = ref(60);
let idleTimer: ReturnType<typeof setInterval> | undefined;
let lastSessionTouchAt = 0;
let sessionTouching = false;
const activeMenu = computed(() => {
  if (route.path.startsWith('/data-center')) return '/data-center';
  if (route.path.startsWith('/devices')) return '/devices';
  if (route.path.startsWith('/factory-gateways')) return '/factory-gateways';
  if (route.path.startsWith('/ota-packages')) return '/ota-packages';
  if (route.path.startsWith('/products')) return '/products';
  if (route.path.startsWith('/system-status')) return '/system-status';
  if (route.path.startsWith('/device-map')) return '/device-map';
  if (route.path.startsWith('/cloud-scada')) return '/cloud-scada';
  if (route.path.startsWith('/model-templates')) return '/model-templates';
  if (route.path.startsWith('/alarms')) return '/alarms';
  if (route.path.startsWith('/events')) return '/events';
  if (route.path.startsWith('/login-logs')) return '/login-logs';
  if (route.path.startsWith('/operation-logs')) return '/operation-logs';
  if (route.path.startsWith('/device-logs')) return '/device-logs';
  if (route.path.startsWith('/users')) return '/users';
  if (route.path.startsWith('/account')) return '/account';
  if (route.path.startsWith('/tenants')) return '/tenants';
  if (route.path.startsWith('/subscription-plans')) return '/subscription-plans';
  if (route.path.startsWith('/quota-addons')) return '/quota-addons';
  if (route.path.startsWith('/finance-reviews')) return '/finance-reviews';
  if (route.path.startsWith('/subscription-orders')) return '/subscription-orders';
  if (route.path.startsWith('/customer-subscriptions')) return '/customer-subscriptions';
  if (route.path.startsWith('/subscription')) return '/subscription';
  if (route.path.startsWith('/changelog-management')) return '/changelog-management';
  return route.path;
});
const systemLogMenuActive = computed(() => ['/login-logs', '/operation-logs', '/device-logs'].includes(activeMenu.value));
const overviewMenuActive = computed(() => activeMenu.value === '/dashboard');
const assetMenuActive = computed(() => ['/devices', '/factory-gateways', '/products'].includes(activeMenu.value));
const operationsMenuActive = computed(() => ['/system-status', '/events', '/ota-packages', '/changelog-management'].includes(activeMenu.value));
const tenantMenuActive = computed(() => activeMenu.value === '/tenants');
const accountMenuActive = computed(() => activeMenu.value === '/users');
const businessMenuActive = computed(() => ['/subscription-plans', '/quota-addons', '/customer-subscriptions', '/subscription-orders', '/finance-reviews'].includes(activeMenu.value));
const tenantDeviceMenuActive = computed(() => ['/devices', '/device-map', '/model-templates'].includes(activeMenu.value));
const tenantApplicationMenuActive = computed(() => activeMenu.value === '/cloud-scada');
const tenantMonitorMenuActive = computed(() => ['/alarms', '/events'].includes(activeMenu.value));
const tenantMemberMenuActive = computed(() => activeMenu.value === '/users');
const tenantServiceMenuActive = computed(() => activeMenu.value === '/subscription');
const overviewMenuExpanded = ref(overviewMenuActive.value);
const assetMenuExpanded = ref(assetMenuActive.value);
const operationsMenuExpanded = ref(operationsMenuActive.value);
const tenantMenuExpanded = ref(tenantMenuActive.value);
const accountMenuExpanded = ref(accountMenuActive.value);
const businessMenuExpanded = ref(businessMenuActive.value);
const tenantDeviceMenuExpanded = ref(tenantDeviceMenuActive.value);
const tenantApplicationMenuExpanded = ref(tenantApplicationMenuActive.value);
const tenantMonitorMenuExpanded = ref(tenantMonitorMenuActive.value);
const tenantMemberMenuExpanded = ref(tenantMemberMenuActive.value);
const tenantServiceMenuExpanded = ref(tenantServiceMenuActive.value);
const systemLogMenuExpanded = ref(systemLogMenuActive.value);
const roleLabel = computed(() => ({
  SUPER_ADMIN: '公司管理员',
  TENANT_ADMIN: '租户管理员',
  TENANT_USER: '租户用户',
}[userStore.userInfo?.role ?? ''] ?? userStore.userInfo?.role));
const hasPermission = (permission: string) => userStore.userInfo?.role !== 'TENANT_USER' || !Array.isArray(userStore.userInfo.permissions) || userStore.userInfo.permissions.includes(permission);

watch(systemLogMenuActive, (isActive) => {
  if (isActive) systemLogMenuExpanded.value = true;
});
watch(overviewMenuActive, (isActive) => {
  if (isActive) overviewMenuExpanded.value = true;
});
watch(assetMenuActive, (isActive) => {
  if (isActive) assetMenuExpanded.value = true;
});
watch(operationsMenuActive, (isActive) => {
  if (isActive) operationsMenuExpanded.value = true;
});
watch(tenantMenuActive, (isActive) => {
  if (isActive) tenantMenuExpanded.value = true;
});
watch(accountMenuActive, (isActive) => {
  if (isActive) accountMenuExpanded.value = true;
});
watch(businessMenuActive, (isActive) => { if (isActive) businessMenuExpanded.value = true; });
watch(tenantServiceMenuActive, (isActive) => { if (isActive) tenantServiceMenuExpanded.value = true; });
watch(tenantDeviceMenuActive, (isActive) => {
  if (isActive) tenantDeviceMenuExpanded.value = true;
});
watch(tenantApplicationMenuActive, (isActive) => {
  if (isActive) tenantApplicationMenuExpanded.value = true;
});
watch(tenantMonitorMenuActive, (isActive) => {
  if (isActive) tenantMonitorMenuExpanded.value = true;
});
watch(tenantMemberMenuActive, (isActive) => {
  if (isActive) tenantMemberMenuExpanded.value = true;
});

async function logout() {
  stopStatusMonitor();
  await userStore.logout();
  router.push('/login');
}

async function confirmLogout() {
  try {
    await ElMessageBox.confirm('退出后需要重新验证身份才能进入平台，确定退出登录吗？', '退出登录', {
      confirmButtonText: '确认退出',
      cancelButtonText: '取消',
      type: 'warning',
      closeOnClickModal: false,
    });
  } catch {
    return;
  }
  await logout();
}

async function handleAccountCommand(command: string) {
  if (command === 'account') {
    await router.push('/account');
    return;
  }
  if (command === 'logout') await confirmLogout();
}

function sessionActivityAt() {
  return Number(getAuthItem('sessionLastActivityAt')) || Date.now();
}

async function renewSession() {
  if (sessionTouching || Date.now() - lastSessionTouchAt < 60_000) return;
  sessionTouching = true;
  try {
    await userStore.keepAlive();
    lastSessionTouchAt = Date.now();
  } catch {
    await logout();
  } finally {
    sessionTouching = false;
  }
}

function recordUserActivity(event?: Event) {
  if (event && !event.isTrusted) return;
  const now = Date.now();
  setAuthItem('sessionLastActivityAt', String(now));
  idleWarningVisible.value = false;
  void renewSession();
}

function checkSessionIdle() {
  const idleMs = Math.max(300, userStore.idleTimeoutSeconds) * 1000;
  const absoluteAt = new Date(userStore.absoluteExpiresAt).getTime();
  const idleRemaining = sessionActivityAt() + idleMs - Date.now();
  const absoluteRemaining = Number.isFinite(absoluteAt) ? absoluteAt - Date.now() : idleRemaining;
  const remaining = Math.min(idleRemaining, absoluteRemaining);
  idleRemainingSeconds.value = Math.max(0, Math.ceil(remaining / 1000));
  if (remaining <= 0) {
    idleWarningVisible.value = false;
    void logout();
  } else if (remaining <= 60_000) {
    idleWarningVisible.value = true;
  }
}

function continueSession() {
  recordUserActivity();
  void renewSession();
}

function statusLabel(status: string) {
  return status === 'ONLINE' ? '上线' : status === 'OFFLINE' ? '离线' : status;
}

function statusType(status: string) {
  return status === 'ONLINE' ? 'success' : 'warning';
}

function alarmLevelLabel(level: string) {
  return { INFO: '提示', WARNING: '警告', CRITICAL: '严重' }[level] ?? level;
}

function alarmLevelClass(level: string) {
  return level === 'CRITICAL' ? 'critical' : level === 'WARNING' ? 'warning' : 'info';
}

function notificationTime(value?: string) {
  if (!value) return '刚刚';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString('zh-CN', { hour12: false });
}

async function loadAlarmNotifications() {
  if (isAdminPortal || !getAccessToken()) return;
  notificationLoading.value = true;
  try {
    const history = await apiRequest<{ items: AlarmEvent[]; total: number; unreadTotal: number }>({
        url: '/api/alarms', method: 'GET', params: { page: 1, pageSize: 50 }, silentError: true,
      });
    notificationAlarms.value = history.items;
    alarmNotificationTotal.value = history.unreadTotal;
  } catch {
    // 通知中心失败不应影响主页面和实时状态连接。
  } finally {
    notificationLoading.value = false;
  }
}

async function openNotificationCenter() {
  notificationVisible.value = true;
  if (isAdminPortal) {
    await loadAdminNotifications();
    await markAdminNotificationsRead();
    return;
  }
  await Promise.all([loadAlarmNotifications(), loadSubscriptionNotifications()]);
  await Promise.all([markAlarmNotificationsRead(), markSubscriptionNotificationsRead()]);
}

async function loadAdminNotifications() {
  if (!isAdminPortal || userStore.userInfo?.role !== 'SUPER_ADMIN') return;
  notificationLoading.value = true;
  try {
    const result = await apiRequest<{ items: AdminNotification[]; unreadTotal: number }>({ url: '/api/subscriptions/admin-notifications', method: 'GET', silentError: true });
    adminNotifications.value = result.items;
    adminNotificationTotal.value = result.unreadTotal;
  } catch { /* 管理通知失败不影响其他管理功能 */ }
  finally { notificationLoading.value = false; }
}

async function markAdminNotificationsRead() {
  if (!isAdminPortal || userStore.userInfo?.role !== 'SUPER_ADMIN') return;
  try {
    await apiRequest({ url: '/api/subscriptions/admin-notifications/read', method: 'PATCH', silentError: true });
    adminNotificationTotal.value = 0;
    adminNotifications.value = adminNotifications.value.map(item => ({ ...item, read: true }));
  } catch { /* ignore */ }
}

function openAdminNotification(item: AdminNotification) {
  notificationVisible.value = false;
  void router.push(item.route);
}

async function loadSubscriptionNotifications() {
  if (isAdminPortal || userStore.userInfo?.role !== 'TENANT_ADMIN') return;
  try {
    const result = await apiRequest<{ items: Array<{ id: string; title: string; message: string; type: string; createdAt: string }>; unreadTotal: number }>({ url: '/api/subscriptions/notifications', method: 'GET', silentError: true });
    const deviceItems = systemNotifications.value.filter(item => typeof item.id === 'number');
    systemNotifications.value = [...result.items.map(item => ({ ...item, status: item.type })), ...deviceItems].slice(0, 30);
    subscriptionNotificationTotal.value = result.unreadTotal;
  } catch { /* 订阅提醒失败不影响设备通知 */ }
}

async function markSubscriptionNotificationsRead() {
  if (isAdminPortal || userStore.userInfo?.role !== 'TENANT_ADMIN') return;
  try { await apiRequest({ url: '/api/subscriptions/notifications/read', method: 'PATCH', silentError: true }); subscriptionNotificationTotal.value = 0; } catch { /* ignore */ }
}

async function markAlarmNotificationsRead() {
  if (isAdminPortal || !getAccessToken()) return;
  try {
    await apiRequest({ url: '/api/alarms/notifications/read', method: 'PATCH', silentError: true });
    alarmNotificationTotal.value = 0;
    notificationAlarms.value = notificationAlarms.value.map((alarm) => ({ ...alarm, notificationRead: true }));
  } catch {
    // 已读回执失败不影响查看历史，下次打开时会再次提交。
  }
}

function openAlarmPage() {
  notificationVisible.value = false;
  void router.push('/alarms');
}

function getAudioContext() {
  const AudioCtor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioCtor) return undefined;
  audioContext ??= new AudioCtor();
  return audioContext;
}

function getAccessToken() {
  return userStore.token || getAuthItem('accessToken') || '';
}

async function unlockAudio() {
  if (audioUnlocked) return;
  const context = getAudioContext();
  if (!context) return;
  if (context.state === 'suspended') await context.resume();
  const source = context.createBufferSource();
  source.buffer = context.createBuffer(1, 1, 22050);
  source.connect(context.destination);
  source.start();
  audioUnlocked = true;
}

async function playStatusSound(status: string) {
  if (!soundEnabled.value) return;
  const context = getAudioContext();
  if (!context) return;
  if (context.state === 'suspended') await context.resume();
  const oscillator = context.createOscillator();
  const gain = context.createGain();
  const now = context.currentTime;
  oscillator.type = 'sine';
  oscillator.frequency.setValueAtTime(status === 'ONLINE' ? 880 : 440, now);
  oscillator.frequency.setValueAtTime(status === 'ONLINE' ? 1175 : 330, now + 0.12);
  gain.gain.setValueAtTime(0.0001, now);
  gain.gain.exponentialRampToValueAtTime(0.18, now + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.32);
  oscillator.connect(gain);
  gain.connect(context.destination);
  oscillator.start(now);
  oscillator.stop(now + 0.34);
}

async function playAlarmSound(level: string) {
  if (!soundEnabled.value) return;
  const nowMs = Date.now();
  if (nowMs - lastAlarmSoundAt < alarmSoundThrottleMs) return;
  lastAlarmSoundAt = nowMs;
  const context = getAudioContext();
  if (!context) return;
  if (context.state === 'suspended') await context.resume();
  const oscillator = context.createOscillator();
  const gain = context.createGain();
  const now = context.currentTime;
  oscillator.type = level === 'CRITICAL' ? 'square' : 'sine';
  oscillator.frequency.setValueAtTime(level === 'CRITICAL' ? 740 : 620, now);
  oscillator.frequency.setValueAtTime(level === 'CRITICAL' ? 520 : 860, now + 0.1);
  oscillator.frequency.setValueAtTime(level === 'CRITICAL' ? 740 : 620, now + 0.22);
  gain.gain.setValueAtTime(0.0001, now);
  gain.gain.exponentialRampToValueAtTime(0.16, now + 0.02);
  gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.38);
  oscillator.connect(gain);
  gain.connect(context.destination);
  oscillator.start(now);
  oscillator.stop(now + 0.4);
}

function addStatusReminder(device: DeviceItem) {
  // 同一设备同一状态短时间内只提醒一次，避免 MQTT 抖动导致声音/弹框刷屏。
  const reminderKey = `${device.id}-${device.status}`;
  const now = Date.now();
  if (now - (lastReminderMap.get(reminderKey) ?? 0) < reminderThrottleMs) return;
  lastReminderMap.set(reminderKey, now);
  const toast: StatusToast = {
    id: ++statusToastId,
    title: `设备${statusLabel(device.status)}`,
    message: `${device.name} (${device.deviceKey}) 已${statusLabel(device.status)}`,
    status: device.status,
    tone: statusType(device.status),
    icon: device.status === 'ONLINE' ? '✓' : '!',
  };
  statusToasts.value = [toast, ...statusToasts.value].slice(0, 3);
  systemNotifications.value = [{
    id: toast.id,
    title: toast.title,
    message: toast.message,
    createdAt: new Date().toISOString(),
    status: device.status,
  }, ...systemNotifications.value].slice(0, 30);
  window.setTimeout(() => {
    statusToasts.value = statusToasts.value.filter((item) => item.id !== toast.id);
  }, 8000);
  void playStatusSound(device.status);
}

function addAlarmToast(alarm: AlarmEvent) {
  if (alarm.status !== 'OPEN') {
    statusToasts.value = statusToasts.value.filter((toast) => toast.alarmId !== alarm.id);
    const historyIndex = notificationAlarms.value.findIndex((item) => item.id === alarm.id);
    if (historyIndex >= 0) {
      notificationAlarms.value = notificationAlarms.value.map((item) => item.id === alarm.id ? { ...item, ...alarm } : item);
    } else {
      notificationAlarms.value = [alarm, ...notificationAlarms.value].slice(0, 50);
    }
    return;
  }
  if (!notificationAlarms.value.some((item) => item.id === alarm.id)) {
    notificationAlarms.value = [{ ...alarm, createdAt: alarm.createdAt || new Date().toISOString() }, ...notificationAlarms.value].slice(0, 50);
    if (notificationVisible.value) {
      void markAlarmNotificationsRead();
    } else {
      alarmNotificationTotal.value += 1;
    }
  }
  const now = Date.now();
  if (now - alarmWindowStart > alarmWindowMs) {
    alarmWindowStart = now;
    alarmWindowCount = 0;
    alarmAggregateToastId = 0;
  }
  alarmWindowCount += 1;
  void playAlarmSound(alarm.level);
  const aggregateMessage = alarmWindowCount > 1
    ? `10 秒内新增 ${alarmWindowCount} 条告警，已合并为一条提醒，点击查看详情。`
    : (alarm.deviceName ? `${alarm.deviceName}：${alarm.message}` : alarm.message);
  const toast: StatusToast = {
    id: ++statusToastId,
    alarmId: alarm.id,
    title: alarmWindowCount > 1 ? '新增告警（已合并提醒）' : `${alarmLevelLabel(alarm.level)}告警`,
    message: aggregateMessage,
    status: 'ALARM',
    tone: 'alarm',
    icon: '!',
  };
  alarmAggregateToastId = toast.id;
  statusToasts.value = [toast, ...statusToasts.value.filter((item) => item.status !== 'ALARM')].slice(0, 3);
  window.setTimeout(() => {
    statusToasts.value = statusToasts.value.filter((item) => item.id !== toast.id);
    if (alarmAggregateToastId === toast.id) alarmAggregateToastId = 0;
  }, 10000);
}

function handleStatusChange(device: DeviceItem) {
  const previousStatus = deviceStatusMap.get(device.id);
  deviceStatusMap.set(device.id, device.status);
  if (!statusMonitorReady.value) return;
  // 统一向页面广播状态变化，设备列表和详情页都监听这个浏览器事件。
  window.dispatchEvent(new CustomEvent('device-status-change', { detail: device }));
  if (previousStatus && previousStatus !== device.status && ['ONLINE', 'OFFLINE'].includes(device.status) && ['ONLINE', 'OFFLINE'].includes(previousStatus)) {
    addStatusReminder(device);
  }
}

async function pollDeviceStatus() {
  if (!getAccessToken()) return;
  try {
    const data = await apiRequest<{ items: DeviceItem[] }>({ url: '/api/devices', method: 'GET', params: { page: 1, pageSize: 100 }, silentError: true });
    for (const device of data.items) {
      handleStatusChange(device);
    }
    statusMonitorReady.value = true;
  } catch {
    // 登录态过期时请求拦截器会统一处理，这里只避免监控循环中断。
  }
}

async function primeDeviceStatus() {
  // 先加载一份当前状态作为基线，否则首次 SSE 快照会被误判为“状态变化”。
  if (!getAccessToken()) return;
  try {
    const data = await apiRequest<{ items: DeviceItem[] }>({ url: '/api/devices', method: 'GET', params: { page: 1, pageSize: 100 }, silentError: true });
    for (const device of data.items) {
      deviceStatusMap.set(device.id, device.status);
    }
    statusMonitorReady.value = true;
  } catch {
    statusMonitorReady.value = true;
  }
}

async function connectStatusStream() {
  statusStreamController?.abort();
  const controller = new AbortController();
  statusStreamController = controller;
  const baseURL = import.meta.env.VITE_API_BASE_URL ?? '';
  const token = getAccessToken();
  if (!token) return;
  try {
    // 原生 EventSource 不方便带 Authorization，所以这里用 fetch 读取 SSE 文本流。
    const response = await fetch(`${baseURL}/api/devices/status/stream`, {
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    });
    if (!response.ok || !response.body) throw new Error('device status stream unavailable');
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
          handleStatusChange(payload.data ?? payload);
        }
      }
    }
  } catch {
    if (controller.signal.aborted) return;
  }
  statusStreamReconnectTimer = setTimeout(() => void connectStatusStream(), 3000);
}

async function connectAlarmStream() {
  alarmStreamController?.abort();
  const controller = new AbortController();
  alarmStreamController = controller;
  const baseURL = import.meta.env.VITE_API_BASE_URL ?? '';
  const token = getAccessToken();
  if (!token) return;
  try {
    const response = await fetch(`${baseURL}/api/alarms/stream`, {
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    });
    if (!response.ok || !response.body) throw new Error('alarm stream unavailable');
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
          addAlarmToast(payload.data ?? payload);
        }
      }
    }
  } catch {
    if (controller.signal.aborted) return;
  }
  alarmStreamReconnectTimer = setTimeout(() => void connectAlarmStream(), 3000);
}

async function startStatusMonitor() {
  await Promise.all([
    hasPermission('DEVICE_VIEW') ? primeDeviceStatus() : Promise.resolve(),
    hasPermission('ALARM_VIEW') ? loadAlarmNotifications() : Promise.resolve(),
  ]);
  if (hasPermission('DEVICE_VIEW')) {
    void connectStatusStream();
    statusMonitorTimer = setInterval(() => void pollDeviceStatus(), 30000);
  }
  if (hasPermission('ALARM_VIEW')) void connectAlarmStream();
}

function stopStatusMonitor() {
  if (statusMonitorTimer) clearInterval(statusMonitorTimer);
  if (statusStreamReconnectTimer) clearTimeout(statusStreamReconnectTimer);
  if (alarmStreamReconnectTimer) clearTimeout(alarmStreamReconnectTimer);
  statusStreamController?.abort();
  alarmStreamController?.abort();
  statusMonitorTimer = undefined;
  statusStreamReconnectTimer = undefined;
  statusStreamController = undefined;
  alarmStreamController = undefined;
  alarmStreamReconnectTimer = undefined;
}

onMounted(() => {
  window.addEventListener('click', unlockAudio, { once: true });
  window.addEventListener('keydown', unlockAudio, { once: true });
  for (const event of ['pointerdown', 'keydown', 'touchstart', 'wheel']) window.addEventListener(event, recordUserActivity, { passive: true });
  if (!getAuthItem('sessionLastActivityAt')) setAuthItem('sessionLastActivityAt', String(Date.now()));
  idleTimer = setInterval(checkSessionIdle, 5000);
  checkSessionIdle();
  if (isAdminPortal) {
    void loadAdminNotifications();
    adminNotificationTimer = setInterval(() => void loadAdminNotifications(), 60_000);
  } else {
    startStatusMonitor();
    void loadSubscriptionNotifications();
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('click', unlockAudio);
  window.removeEventListener('keydown', unlockAudio);
  for (const event of ['pointerdown', 'keydown', 'touchstart', 'wheel']) window.removeEventListener(event, recordUserActivity);
  if (idleTimer) clearInterval(idleTimer);
  if (adminNotificationTimer) clearInterval(adminNotificationTimer);
  stopStatusMonitor();
});
</script>

<template>
  <Teleport to="body">
    <TransitionGroup name="status-toast" tag="div" class="status-toast-stack">
      <div v-for="toast in statusToasts" :key="toast.id" class="status-toast" :class="toast.tone ?? statusType(toast.status)" @click="toast.status === 'ALARM' && router.push('/alarms')">
        <span class="status-toast-dot">{{ toast.icon ?? (toast.status === 'ONLINE' ? '✓' : '!') }}</span>
        <div>
          <strong>{{ toast.title }}</strong>
          <p>{{ toast.message }}</p>
        </div>
      </div>
    </TransitionGroup>
  </Teleport>
  <el-dialog v-model="idleWarningVisible" title="登录即将失效" width="420px" :close-on-click-modal="false" :show-close="false">
    <p>检测到长时间未操作，将在 {{ idleRemainingSeconds }} 秒后自动退出。</p>
    <template #footer><el-button @click="confirmLogout">立即退出</el-button><el-button type="primary" @click="continueSession">继续使用</el-button></template>
  </el-dialog>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <img class="portal-brand-logo" :class="{ admin: isAdminPortal }" :src="portalLogo" :alt="portalName" />
        <span>{{ portalName }}</span>
      </div>
      <nav class="menu">
        <template v-if="isAdminPortal">
          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: overviewMenuActive }" @click="overviewMenuExpanded = !overviewMenuExpanded">
              <el-icon><Monitor /></el-icon><span>总览</span>
              <span class="menu-group-arrow" :class="{ expanded: overviewMenuExpanded }">›</span>
            </button>
            <div v-show="overviewMenuExpanded" class="menu-subtree">
              <router-link to="/dashboard" class="menu-subitem" :class="{ active: activeMenu === '/dashboard' }">平台概览</router-link>
            </div>
          </div>

          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantMenuActive }" @click="tenantMenuExpanded = !tenantMenuExpanded">
              <el-icon><OfficeBuilding /></el-icon><span>租户中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantMenuExpanded }">›</span>
            </button>
            <div v-show="tenantMenuExpanded" class="menu-subtree">
              <router-link to="/tenants" class="menu-subitem" :class="{ active: activeMenu === '/tenants' }">租户管理</router-link>
            </div>
          </div>

          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: assetMenuActive }" @click="assetMenuExpanded = !assetMenuExpanded">
              <el-icon><Connection /></el-icon><span>设备与产品</span>
              <span class="menu-group-arrow" :class="{ expanded: assetMenuExpanded }">›</span>
            </button>
            <div v-show="assetMenuExpanded" class="menu-subtree">
              <router-link to="/devices" class="menu-subitem" :class="{ active: activeMenu === '/devices' }">全部设备</router-link>
              <router-link to="/factory-gateways" class="menu-subitem" :class="{ active: activeMenu === '/factory-gateways' }">出厂网关</router-link>
              <router-link to="/products" class="menu-subitem" :class="{ active: activeMenu === '/products' }">产品库</router-link>
            </div>
          </div>

          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: operationsMenuActive }" @click="operationsMenuExpanded = !operationsMenuExpanded">
              <el-icon><Tickets /></el-icon><span>运维中心</span>
              <span class="menu-group-arrow" :class="{ expanded: operationsMenuExpanded }">›</span>
            </button>
            <div v-show="operationsMenuExpanded" class="menu-subtree">
              <router-link to="/system-status" class="menu-subitem" :class="{ active: activeMenu === '/system-status' }">平台状态</router-link>
              <router-link to="/events" class="menu-subitem" :class="{ active: activeMenu === '/events' }">平台事件</router-link>
              <router-link to="/ota-packages" class="menu-subitem" :class="{ active: activeMenu === '/ota-packages' }">升级包管理</router-link>
              <router-link to="/changelog-management" class="menu-subitem" :class="{ active: activeMenu === '/changelog-management' }">更新日志管理</router-link>
            </div>
          </div>

          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: accountMenuActive }" @click="accountMenuExpanded = !accountMenuExpanded">
              <el-icon><User /></el-icon><span>权限中心</span>
              <span class="menu-group-arrow" :class="{ expanded: accountMenuExpanded }">›</span>
            </button>
            <div v-show="accountMenuExpanded" class="menu-subtree">
              <router-link to="/users" class="menu-subitem" :class="{ active: activeMenu === '/users' }">账号管理</router-link>
            </div>
          </div>

          <div class="menu-group admin-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: businessMenuActive }" @click="businessMenuExpanded = !businessMenuExpanded">
              <el-icon><Tickets /></el-icon><span>商业管理</span>
              <span class="menu-group-arrow" :class="{ expanded: businessMenuExpanded }">›</span>
            </button>
            <div v-show="businessMenuExpanded" class="menu-subtree">
              <router-link to="/subscription-plans" class="menu-subitem" :class="{ active: activeMenu === '/subscription-plans' }">套餐管理</router-link>
              <router-link to="/quota-addons" class="menu-subitem" :class="{ active: activeMenu === '/quota-addons' }">扩容包管理</router-link>
              <router-link to="/customer-subscriptions" class="menu-subitem" :class="{ active: activeMenu === '/customer-subscriptions' }">客户订阅</router-link>
              <router-link to="/subscription-orders" class="menu-subitem" :class="{ active: activeMenu === '/subscription-orders' }">订单管理</router-link>
              <router-link to="/finance-reviews" class="menu-subitem" :class="{ active: activeMenu === '/finance-reviews' }">财务审核</router-link>
            </div>
          </div>
        </template>
        <template v-else>
          <div v-if="hasPermission('DASHBOARD_VIEW')" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: overviewMenuActive }" @click="overviewMenuExpanded = !overviewMenuExpanded">
              <el-icon><Monitor /></el-icon><span>总览</span>
              <span class="menu-group-arrow" :class="{ expanded: overviewMenuExpanded }">›</span>
            </button>
            <div v-show="overviewMenuExpanded" class="menu-subtree">
              <router-link to="/dashboard" class="menu-subitem" :class="{ active: activeMenu === '/dashboard' }">数据概览</router-link>
            </div>
          </div>

          <div v-if="hasPermission('DEVICE_VIEW') || hasPermission('MAP_VIEW') || hasPermission('MODEL_VIEW')" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantDeviceMenuActive }" @click="tenantDeviceMenuExpanded = !tenantDeviceMenuExpanded">
              <el-icon><Connection /></el-icon><span>设备中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantDeviceMenuExpanded }">›</span>
            </button>
            <div v-show="tenantDeviceMenuExpanded" class="menu-subtree">
              <router-link v-if="hasPermission('DEVICE_VIEW')" to="/devices" class="menu-subitem" :class="{ active: activeMenu === '/devices' }">设备管理</router-link>
              <router-link v-if="hasPermission('MAP_VIEW')" to="/device-map" class="menu-subitem" :class="{ active: activeMenu === '/device-map' }">设备地图</router-link>
              <router-link v-if="hasPermission('MODEL_VIEW')" to="/model-templates" class="menu-subitem" :class="{ active: activeMenu === '/model-templates' }">物模型管理</router-link>
            </div>
          </div>

          <router-link v-if="hasPermission('DEVICE_VIEW')" to="/data-center" class="menu-item" :class="{ active: activeMenu === '/data-center' }"><el-icon><Tickets /></el-icon><span>数据中心</span></router-link>
          <router-link v-if="userStore.userInfo?.role === 'TENANT_ADMIN'" to="/device-linkage" class="menu-item" :class="{ active: activeMenu === '/device-linkage' }"><el-icon><Connection /></el-icon><span>设备联动</span></router-link>

          <div v-if="hasPermission('SCADA_VIEW')" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantApplicationMenuActive }" @click="tenantApplicationMenuExpanded = !tenantApplicationMenuExpanded">
              <el-icon><Monitor /></el-icon><span>应用中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantApplicationMenuExpanded }">›</span>
            </button>
            <div v-show="tenantApplicationMenuExpanded" class="menu-subtree">
              <router-link to="/cloud-scada" class="menu-subitem" :class="{ active: activeMenu === '/cloud-scada' }">云组态</router-link>
            </div>
          </div>

          <div v-if="hasPermission('ALARM_VIEW') || hasPermission('EVENT_VIEW')" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantMonitorMenuActive }" @click="tenantMonitorMenuExpanded = !tenantMonitorMenuExpanded">
              <el-icon><Bell /></el-icon><span>监控中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantMonitorMenuExpanded }">›</span>
            </button>
            <div v-show="tenantMonitorMenuExpanded" class="menu-subtree">
              <router-link v-if="hasPermission('ALARM_VIEW')" to="/alarms" class="menu-subitem" :class="{ active: activeMenu === '/alarms' }">告警管理</router-link>
              <router-link v-if="hasPermission('EVENT_VIEW')" to="/events" class="menu-subitem" :class="{ active: activeMenu === '/events' }">事件中心</router-link>
            </div>
          </div>

          <div v-if="userStore.userInfo?.role === 'TENANT_ADMIN'" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantMemberMenuActive }" @click="tenantMemberMenuExpanded = !tenantMemberMenuExpanded">
              <el-icon><User /></el-icon><span>权限中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantMemberMenuExpanded }">›</span>
            </button>
            <div v-show="tenantMemberMenuExpanded" class="menu-subtree">
              <router-link to="/users" class="menu-subitem" :class="{ active: activeMenu === '/users' }">成员管理</router-link>
            </div>
          </div>

          <div v-if="userStore.userInfo?.role === 'TENANT_ADMIN'" class="menu-group tenant-menu-group">
            <button type="button" class="menu-group-title" :class="{ active: tenantServiceMenuActive }" @click="tenantServiceMenuExpanded = !tenantServiceMenuExpanded">
              <el-icon><Tickets /></el-icon><span>服务中心</span>
              <span class="menu-group-arrow" :class="{ expanded: tenantServiceMenuExpanded }">›</span>
            </button>
            <div v-show="tenantServiceMenuExpanded" class="menu-subtree">
              <router-link to="/subscription" class="menu-subitem" :class="{ active: activeMenu === '/subscription' }">版本与升级</router-link>
            </div>
          </div>
        </template>
        <div v-if="isAdminPortal || userStore.userInfo?.role === 'TENANT_ADMIN'" class="menu-group" :class="{ 'admin-menu-group': isAdminPortal, 'tenant-menu-group': !isAdminPortal }">
          <button type="button" class="menu-group-title" :class="{ active: systemLogMenuActive }" @click="systemLogMenuExpanded = !systemLogMenuExpanded">
            <el-icon><Tickets /></el-icon>
            <span>系统日志</span>
            <span class="menu-group-arrow" :class="{ expanded: systemLogMenuExpanded }">›</span>
          </button>
          <div v-show="systemLogMenuExpanded" class="menu-subtree">
            <router-link to="/login-logs" class="menu-subitem" :class="{ active: activeMenu === '/login-logs' }">
              <span>登录日志</span>
            </router-link>
            <router-link to="/operation-logs" class="menu-subitem" :class="{ active: activeMenu === '/operation-logs' }">
              <span>操作日志</span>
            </router-link>
            <router-link to="/device-logs" class="menu-subitem" :class="{ active: activeMenu === '/device-logs' }">
              <span>设备日志</span>
            </router-link>
          </div>
        </div>
      </nav>
    </aside>
    <main class="main">
      <header class="header">
        <div>
          <p class="eyebrow">{{ portalEyebrow }}</p>
        </div>
        <div class="header-actions">
          <button v-if="isAdminPortal || hasPermission('ALARM_VIEW') || userStore.userInfo?.role === 'TENANT_ADMIN'" type="button" class="notification-trigger" aria-label="打开通知中心" @click="openNotificationCenter">
            <el-badge :value="isAdminPortal ? adminNotificationTotal : alarmNotificationTotal + subscriptionNotificationTotal" :max="99" :hidden="(isAdminPortal ? adminNotificationTotal : alarmNotificationTotal + subscriptionNotificationTotal) === 0">
              <el-icon><Bell /></el-icon>
            </el-badge>
          </button>
          <div class="profile-summary">
            <span class="avatar">
              <img v-if="userStore.userInfo?.avatarUrl" :src="userStore.userInfo.avatarUrl" alt="用户头像" />
              <span v-else>{{ userDisplayName(userStore.userInfo).slice(0, 1) }}</span>
            </span>
            <span class="profile-copy">
              <strong>{{ userDisplayName(userStore.userInfo) }}</strong>
              <small>{{ roleLabel }}</small>
            </span>
          </div>
          <el-dropdown trigger="click" placement="bottom-end" popper-class="account-dropdown" @command="handleAccountCommand">
            <button type="button" class="account-trigger" aria-label="打开账号菜单">
              <el-icon><Setting /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="account" :icon="User">账号与安全</el-dropdown-item>
                <el-dropdown-item command="logout" :icon="SwitchButton" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>
      <section class="content">
        <router-view />
      </section>
    </main>
  </div>

  <el-dialog
    v-model="notificationVisible"
    title="通知中心"
    width="min(620px, 94vw)"
    class="notification-dialog"
  >
    <div v-if="isAdminPortal" v-loading="notificationLoading" class="notification-list admin-notification-list">
      <button v-for="item in adminNotifications" :key="item.id" type="button" class="notification-item" @click="openAdminNotification(item)">
        <span class="notification-level admin" :class="item.category.toLowerCase()">{{ item.category === 'ORDER' ? '¥' : '期' }}</span>
        <span class="notification-copy">
          <strong>{{ item.title }}</strong>
          <span>{{ item.message }}</span>
          <small><b class="notification-read" :class="{ unread: !item.read }">{{ item.read ? '已读' : '未读' }}</b>{{ notificationTime(item.createdAt) }}</small>
        </span>
      </button>
      <div v-if="!notificationLoading && !adminNotifications.length" class="notification-empty">
        <el-icon><Bell /></el-icon><strong>暂无管理通知</strong><span>待审核订单和客户版本到期提醒会显示在这里</span>
      </div>
    </div>
    <el-tabs v-else v-model="notificationTab" class="notification-tabs">
      <el-tab-pane name="alarms">
        <template #label>
          <span class="notification-tab-label">告警通知 <em v-if="alarmNotificationTotal">{{ alarmNotificationTotal }}</em></span>
        </template>
        <div v-loading="notificationLoading" class="notification-list">
          <button
            v-for="alarm in notificationAlarms"
            :key="alarm.id"
            type="button"
            class="notification-item"
            @click="openAlarmPage"
          >
            <span class="notification-level" :class="alarmLevelClass(alarm.level)">!</span>
            <span class="notification-copy">
              <strong>{{ alarm.deviceName || alarm.device?.name || '设备告警' }}</strong>
              <span>{{ alarm.message }}</span>
              <small>
                <b class="notification-read" :class="{ unread: !alarm.notificationRead }">{{ alarm.notificationRead ? '已读' : '未读' }}</b>
                <b class="notification-status" :class="alarm.status.toLowerCase()">{{ alarm.status === 'OPEN' ? '告警中' : '已恢复' }}</b>
                {{ alarmLevelLabel(alarm.level) }} · {{ notificationTime(alarm.createdAt) }}
              </small>
            </span>
          </button>
          <div v-if="!notificationLoading && !notificationAlarms.length" class="notification-empty">
            <el-icon><Bell /></el-icon><strong>暂无告警通知</strong><span>告警发生和恢复记录会保留在这里</span>
          </div>
        </div>
        <button v-if="notificationAlarms.length" type="button" class="notification-more" @click="openAlarmPage">查看全部告警</button>
      </el-tab-pane>
      <el-tab-pane name="system">
        <template #label><span class="notification-tab-label">系统通知 <em v-if="subscriptionNotificationTotal">{{ subscriptionNotificationTotal }}</em></span></template>
        <div class="notification-list">
          <div v-for="item in systemNotifications" :key="item.id" class="notification-item system-item">
            <span class="notification-level system" :class="item.status === 'ONLINE' ? 'online' : 'offline'">{{ item.status === 'ONLINE' ? '✓' : '!' }}</span>
            <span class="notification-copy">
              <strong>{{ item.title }}</strong>
              <span>{{ item.message }}</span>
              <small>{{ notificationTime(item.createdAt) }}</small>
            </span>
          </div>
          <div v-if="!systemNotifications.length" class="notification-empty">
            <el-icon><Tickets /></el-icon><strong>暂无系统通知</strong><span>设备状态变化和平台通知会显示在这里</span>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-dialog>
</template>

<style scoped>
.shell { display: flex; align-items: flex-start; min-height: 100vh; }
.portal-brand-logo { display: block; box-sizing: border-box; width: 36px; height: 36px; padding: 2px; flex: 0 0 36px; border: 0; object-fit: contain; background: transparent; }
.portal-brand-logo.admin { width: 44px; padding: 1px; flex-basis: 44px; border-radius: 8px; background: #fff; }
.sidebar { position: sticky; top: 0; width: 220px; height: 100vh; padding: 22px 18px; overflow-y: auto; border-right: 1px solid #e9eef5; background: #fff; }
.menu { margin-top: 28px; }
.menu-item { display: flex; gap: 10px; align-items: center; min-height: 40px; padding: 9px 12px; border-radius: 9px; color: #697386; font-size: 14px; transition: color .16s ease, background-color .16s ease; }
.menu-item:hover { color: #2563eb; background: #f7faff; }
.menu-item.router-link-active, .menu-item.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.menu-group { margin-top: 2px; }
.admin-menu-group + .admin-menu-group { margin-top: 6px; }
.tenant-menu-group + .tenant-menu-group { margin-top: 6px; }
.menu-group-title { display: flex; width: 100%; gap: 10px; align-items: center; padding: 11px 12px; border: 0; border-radius: 9px; color: #697386; background: transparent; font: inherit; font-size: 14px; cursor: pointer; }
.menu-group-title.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.menu-group-arrow { margin-left: auto; color: #9aa4b2; font-size: 18px; line-height: 1; transition: transform 0.18s ease; }
.menu-group-arrow.expanded { transform: rotate(90deg); }
.menu-subtree { position: relative; margin: 4px 0 6px 23px; padding-left: 14px; border-left: 1px solid #e5e7eb; }
.menu-subitem { display: block; padding: 8px 10px; border-radius: 8px; color: #697386; font-size: 13px; }
.menu-subitem.router-link-active, .menu-subitem.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.main { flex: 1; min-width: 0; min-height: 100vh; }
.header { position: sticky; top: 0; z-index: 1000; display: flex; height: 72px; align-items: center; justify-content: space-between; padding: 0 30px; border-bottom: 1px solid #e9eef5; background: rgb(255 255 255 / 96%); box-shadow: 0 1px 0 rgb(15 23 42 / 3%); backdrop-filter: blur(10px); }
.eyebrow { color: #9aa4b2; font-size: 11px; letter-spacing: 1.5px; }
.header-actions { display: flex; gap: 16px; align-items: center; }
.notification-trigger { display: grid; width: 38px; height: 38px; place-items: center; border: 0; border-radius: 10px; color: #526075; background: transparent; cursor: pointer; transition: background .18s ease, color .18s ease; }
.notification-trigger:hover { color: #2563eb; background: #eff6ff; }
.notification-trigger > .el-badge { display: inline-flex; }
.notification-trigger .el-icon { font-size: 20px; }
.notification-trigger :deep(.el-badge__content) { min-width: 17px; height: 17px; padding: 0 4px; border-width: 2px; font-size: 10px; line-height: 13px; }
.profile-summary { display: flex; gap: 10px; align-items: center; }
.profile-copy { display: grid; min-width: 72px; gap: 2px; }
.profile-copy strong { overflow: hidden; max-width: 140px; color: #172033; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.profile-copy small { color: #94a3b8; font-size: 11px; }
.account-trigger { display: grid; width: 38px; height: 38px; padding: 0; place-items: center; border: 0; border-radius: 10px; color: #526075; background: transparent; cursor: pointer; transition: background .18s ease, color .18s ease; }
.account-trigger:hover, .account-trigger:focus-visible { outline: none; background: #eff6ff; }
.account-trigger:hover, .account-trigger:focus-visible { color: #2563eb; }
.account-trigger .el-icon { font-size: 19px; }
.avatar { display: grid; width: 36px; height: 36px; overflow: hidden; place-items: center; border-radius: 50%; color: #2563eb; background: #eff6ff; font-weight: 700; }
.avatar img { width: 100%; height: 100%; object-fit: cover; }
:global(.account-dropdown .el-dropdown-menu) { min-width: 210px; padding: 7px; }
:global(.account-dropdown .el-dropdown-menu__item) { min-height: 38px; border-radius: 7px; }
.content { padding: 30px; }
.status-toast-stack { position: fixed; top: 86px; right: 30px; z-index: 900; width: min(360px, calc(100vw - 280px)); pointer-events: none; }
.status-toast { display: flex; min-height: 56px; gap: 10px; align-items: center; padding: 9px 13px; overflow: hidden; border: 1px solid #e5e7eb; border-radius: 11px; background: #fff; box-shadow: 0 8px 24px rgb(15 23 42 / 10%); pointer-events: auto; }
.status-toast.success { border-left: 4px solid #22c55e; }
.status-toast.warning { border-left: 4px solid #f59e0b; }
.status-toast.alarm { border-left: 4px solid #fa8c16; cursor: pointer; }
.status-toast.danger { border-left: 4px solid #ef4444; cursor: pointer; }
.status-toast-dot { display: grid; flex: 0 0 22px; width: 22px; height: 22px; place-items: center; border-radius: 50%; color: #fff; background: #22c55e; font-size: 13px; font-weight: 800; }
.status-toast.warning .status-toast-dot { background: #f59e0b; }
.status-toast.alarm .status-toast-dot { background: #fa8c16; }
.status-toast.danger .status-toast-dot { background: #ef4444; }
.status-toast > div { min-width: 0; }
.status-toast strong { display: block; overflow: hidden; color: #172033; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.status-toast p { display: -webkit-box; margin: 2px 0 0; overflow: hidden; color: #64748b; font-size: 11px; line-height: 1.35; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.status-toast-enter-active, .status-toast-leave-active { transition: all 0.2s ease; }
.status-toast-enter-from, .status-toast-leave-to { opacity: 0; transform: translateY(-8px); }
.notification-dialog :deep(.el-dialog__body) { padding-top: 6px; }
.notification-tabs :deep(.el-tabs__header) { margin-bottom: 10px; }
.notification-tab-label { display: inline-flex; align-items: center; gap: 7px; }
.notification-tab-label em { display: inline-grid; min-width: 19px; height: 19px; padding: 0 5px; place-items: center; border-radius: 999px; color: #fff; background: #ef4444; font-size: 11px; font-style: normal; }
.notification-list { min-height: 250px; max-height: 430px; overflow-y: auto; }
.notification-item { display: flex; width: 100%; gap: 12px; align-items: flex-start; padding: 14px 8px; border: 0; border-bottom: 1px solid #edf1f6; color: inherit; background: #fff; text-align: left; }
button.notification-item { cursor: pointer; }
button.notification-item:hover { background: #f8fbff; }
.notification-level { display: grid; flex: 0 0 34px; width: 34px; height: 34px; place-items: center; border-radius: 10px; color: #fff; background: #f59e0b; font-weight: 800; }
.notification-level.critical { background: #ef4444; }
.notification-level.info { background: #3b82f6; }
.notification-level.system.online { background: #22c55e; }
.notification-level.system.offline { background: #94a3b8; }
.notification-level.admin.order { background: #f59e0b; }
.notification-level.admin.subscription { background: #6366f1; }
.admin-notification-list { min-height: 280px; }
.notification-copy { display: grid; min-width: 0; flex: 1; gap: 4px; }
.notification-copy strong { color: #172033; font-size: 14px; }
.notification-copy > span { overflow: hidden; color: #526075; font-size: 13px; line-height: 1.5; text-overflow: ellipsis; white-space: nowrap; }
.notification-copy small { color: #94a3b8; font-size: 11px; }
.notification-status { display: inline-flex; margin-right: 6px; padding: 1px 6px; border-radius: 999px; color: #dc2626; background: #fef2f2; font-size: 10px; font-weight: 600; }
.notification-status.resolved { color: #15803d; background: #f0fdf4; }
.notification-read { display: inline-flex; margin-right: 6px; padding: 1px 6px; border-radius: 999px; color: #64748b; background: #f1f5f9; font-size: 10px; font-weight: 600; }
.notification-read.unread { color: #1d4ed8; background: #eff6ff; }
.notification-empty { display: grid; min-height: 250px; place-content: center; justify-items: center; gap: 8px; color: #94a3b8; text-align: center; }
.notification-empty .el-icon { font-size: 34px; }
.notification-empty strong { color: #64748b; font-size: 14px; }
.notification-empty span { font-size: 12px; }
.notification-more { display: block; margin: 12px auto 0; border: 0; color: #2563eb; background: transparent; font-size: 13px; cursor: pointer; }
@media (max-width: 900px) { .status-toast-stack { top: 76px; right: 12px; width: min(320px, calc(100vw - 24px)); } }
@media (max-width: 700px) { .sidebar { display: none; } .header, .content { padding-left: 18px; padding-right: 18px; } }
</style>
