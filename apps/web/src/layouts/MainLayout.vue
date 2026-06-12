<script setup lang="ts">
import { Bell, Connection, Location, Monitor, OfficeBuilding, SwitchButton, Tickets, User } from '@element-plus/icons-vue';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface DeviceItem {
  id: string;
  deviceKey: string;
  name: string;
  status: string;
  lastSeenAt?: string;
}

interface StatusToast {
  id: number;
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
const reminderThrottleMs = 5000;
const alarmWindowMs = 10000;
const alarmIndividualLimit = 2;
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
const activeMenu = computed(() => {
  if (route.path.startsWith('/devices')) return '/devices';
  if (route.path.startsWith('/system-status')) return '/system-status';
  if (route.path.startsWith('/device-map')) return '/device-map';
  if (route.path.startsWith('/model-templates')) return '/model-templates';
  if (route.path.startsWith('/alarms')) return '/alarms';
  if (route.path.startsWith('/login-logs')) return '/login-logs';
  if (route.path.startsWith('/operation-logs')) return '/operation-logs';
  if (route.path.startsWith('/device-logs')) return '/device-logs';
  if (route.path.startsWith('/users')) return '/users';
  if (route.path.startsWith('/tenants')) return '/tenants';
  return route.path;
});
const systemLogMenuActive = computed(() => ['/login-logs', '/operation-logs', '/device-logs'].includes(activeMenu.value));
const systemLogMenuExpanded = ref(systemLogMenuActive.value);

watch(systemLogMenuActive, (isActive) => {
  if (isActive) systemLogMenuExpanded.value = true;
});

function logout() {
  stopStatusMonitor();
  userStore.logout();
  router.push('/login');
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

function getAudioContext() {
  const AudioCtor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioCtor) return undefined;
  audioContext ??= new AudioCtor();
  return audioContext;
}

function getAccessToken() {
  return userStore.token || localStorage.getItem('accessToken') || '';
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
  window.setTimeout(() => {
    statusToasts.value = statusToasts.value.filter((item) => item.id !== toast.id);
  }, 8000);
  void playStatusSound(device.status);
}

function addAlarmToast(alarm: AlarmEvent) {
  if (alarm.status !== 'OPEN') return;
  const now = Date.now();
  if (now - alarmWindowStart > alarmWindowMs) {
    alarmWindowStart = now;
    alarmWindowCount = 0;
    alarmAggregateToastId = 0;
  }
  alarmWindowCount += 1;
  void playAlarmSound(alarm.level);
  if (alarmWindowCount <= alarmIndividualLimit) {
    const toast: StatusToast = {
      id: ++statusToastId,
      title: `${alarmLevelLabel(alarm.level)}告警`,
      message: alarm.deviceName ? `${alarm.deviceName}：${alarm.message}` : alarm.message,
      status: 'ALARM',
      tone: alarm.level === 'CRITICAL' ? 'danger' : 'alarm',
      icon: '!',
    };
    statusToasts.value = [toast, ...statusToasts.value].slice(0, 3);
    window.setTimeout(() => {
      statusToasts.value = statusToasts.value.filter((item) => item.id !== toast.id);
    }, 8000);
    return;
  }

  const aggregateMessage = `10 秒内新增 ${alarmWindowCount} 条告警，已为你合并提醒，可点击进入告警管理查看。`;
  if (alarmAggregateToastId) {
    statusToasts.value = statusToasts.value.map((toast) => toast.id === alarmAggregateToastId
      ? { ...toast, message: aggregateMessage }
      : toast);
    return;
  }
  const toast: StatusToast = {
    id: ++statusToastId,
    title: '告警较多，已合并提醒',
    message: aggregateMessage,
    status: 'ALARM',
    tone: 'alarm',
    icon: '!',
  };
  alarmAggregateToastId = toast.id;
  statusToasts.value = [toast, ...statusToasts.value].slice(0, 3);
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
  await primeDeviceStatus();
  void connectStatusStream();
  void connectAlarmStream();
  // 低频轮询只是兜底，主状态同步依赖 SSE。
  statusMonitorTimer = setInterval(() => void pollDeviceStatus(), 30000);
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
  startStatusMonitor();
});

onBeforeUnmount(() => {
  window.removeEventListener('click', unlockAudio);
  window.removeEventListener('keydown', unlockAudio);
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
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">微</span>
        <span>微控物联</span>
      </div>
      <nav class="menu">
        <router-link to="/dashboard" class="menu-item" :class="{ active: activeMenu === '/dashboard' }">
          <el-icon><Monitor /></el-icon>
          <span>数据概览</span>
        </router-link>
        <router-link v-if="userStore.userInfo?.role !== 'TENANT_USER'" to="/system-status" class="menu-item" :class="{ active: activeMenu === '/system-status' }">
          <el-icon><Monitor /></el-icon>
          <span>系统状态</span>
        </router-link>
        <router-link to="/devices" class="menu-item" :class="{ active: activeMenu === '/devices' }">
          <el-icon><Connection /></el-icon>
          <span>设备管理</span>
        </router-link>
        <router-link to="/device-map" class="menu-item" :class="{ active: activeMenu === '/device-map' }">
          <el-icon><Location /></el-icon>
          <span>设备地图</span>
        </router-link>
        <router-link to="/model-templates" class="menu-item" :class="{ active: activeMenu === '/model-templates' }">
          <el-icon><Connection /></el-icon>
          <span>物模型管理</span>
        </router-link>
        <router-link to="/alarms" class="menu-item" :class="{ active: activeMenu === '/alarms' }">
          <el-icon><Bell /></el-icon>
          <span>告警管理</span>
        </router-link>
        <div v-if="userStore.userInfo?.role !== 'TENANT_USER'" class="menu-group">
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
        <router-link v-if="userStore.userInfo?.role !== 'TENANT_USER'" to="/users" class="menu-item" :class="{ active: activeMenu === '/users' }">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </router-link>
        <router-link v-if="userStore.userInfo?.role === 'SUPER_ADMIN'" to="/tenants" class="menu-item" :class="{ active: activeMenu === '/tenants' }">
          <el-icon><OfficeBuilding /></el-icon>
          <span>租户管理</span>
        </router-link>
      </nav>
    </aside>
    <main class="main">
      <header class="header">
        <div>
          <p class="eyebrow">WEIKONG IOT CLOUD</p>
        </div>
        <div class="header-actions">
          <div class="profile">
          <div class="avatar">{{ (userStore.userInfo?.nickname || userStore.userInfo?.username || 'U').slice(0, 1) }}</div>
          <div>
            <strong>{{ userStore.userInfo?.nickname || userStore.userInfo?.username }}</strong>
            <span>{{ userStore.userInfo?.role }}</span>
          </div>
          <el-button text :icon="SwitchButton" @click="logout">退出</el-button>
          </div>
        </div>
      </header>
      <section class="content">
        <router-view />
      </section>
    </main>
  </div>
</template>

<style scoped>
.shell { display: flex; align-items: flex-start; min-height: 100vh; }
.sidebar { position: sticky; top: 0; width: 220px; height: 100vh; padding: 22px 18px; overflow-y: auto; border-right: 1px solid #e9eef5; background: #fff; }
.menu { margin-top: 38px; }
.menu-item { display: flex; gap: 10px; align-items: center; padding: 11px 12px; border-radius: 9px; color: #697386; font-size: 14px; }
.menu-item.router-link-active, .menu-item.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.menu-group { margin-top: 2px; }
.menu-group-title { display: flex; width: 100%; gap: 10px; align-items: center; padding: 11px 12px; border: 0; border-radius: 9px; color: #697386; background: transparent; font: inherit; font-size: 14px; cursor: pointer; }
.menu-group-title.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.menu-group-arrow { margin-left: auto; color: #9aa4b2; font-size: 18px; line-height: 1; transition: transform 0.18s ease; }
.menu-group-arrow.expanded { transform: rotate(90deg); }
.menu-subtree { position: relative; margin: 4px 0 6px 23px; padding-left: 14px; border-left: 1px solid #e5e7eb; }
.menu-subitem { display: block; padding: 8px 10px; border-radius: 8px; color: #697386; font-size: 13px; }
.menu-subitem.router-link-active, .menu-subitem.active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.main { flex: 1; min-width: 0; min-height: 100vh; }
.header { display: flex; height: 72px; align-items: center; justify-content: space-between; padding: 0 30px; border-bottom: 1px solid #e9eef5; background: #fff; }
.eyebrow { color: #9aa4b2; font-size: 11px; letter-spacing: 1.5px; }
.header-actions { display: flex; gap: 16px; align-items: center; }
.profile { display: flex; gap: 10px; align-items: center; font-size: 13px; }
.profile span { display: block; margin-top: 2px; color: #9aa4b2; font-size: 11px; }
.avatar { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 50%; color: #2563eb; background: #eff6ff; font-weight: 700; }
.content { padding: 30px; }
.status-toast-stack { position: fixed; top: 88px; right: 22px; z-index: 3000; display: flex; flex-direction: column; gap: 12px; width: min(360px, calc(100vw - 36px)); pointer-events: none; }
.status-toast { display: flex; gap: 12px; align-items: flex-start; padding: 15px 16px; border: 1px solid #e5e7eb; border-radius: 12px; background: #fff; box-shadow: 0 14px 35px rgb(15 23 42 / 14%); pointer-events: auto; }
.status-toast.success { border-left: 4px solid #22c55e; }
.status-toast.warning { border-left: 4px solid #f59e0b; }
.status-toast.alarm { border-left: 4px solid #fa8c16; cursor: pointer; }
.status-toast.danger { border-left: 4px solid #ef4444; cursor: pointer; }
.status-toast-dot { display: grid; flex: 0 0 22px; width: 22px; height: 22px; place-items: center; border-radius: 50%; color: #fff; background: #22c55e; font-size: 13px; font-weight: 800; }
.status-toast.warning .status-toast-dot { background: #f59e0b; }
.status-toast.alarm .status-toast-dot { background: #fa8c16; }
.status-toast.danger .status-toast-dot { background: #ef4444; }
.status-toast strong { display: block; color: #172033; font-size: 15px; }
.status-toast p { margin: 4px 0 0; color: #64748b; font-size: 13px; line-height: 1.5; }
.status-toast-enter-active, .status-toast-leave-active { transition: all 0.2s ease; }
.status-toast-enter-from, .status-toast-leave-to { opacity: 0; transform: translateY(-8px); }
@media (max-width: 700px) { .sidebar { display: none; } .header, .content { padding-left: 18px; padding-right: 18px; } }
</style>
