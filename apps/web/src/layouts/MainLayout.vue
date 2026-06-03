<script setup lang="ts">
import { Bell, Connection, Monitor, OfficeBuilding, SwitchButton, User } from '@element-plus/icons-vue';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
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
}

const router = useRouter();
const userStore = useUserStore();
const deviceStatusMap = new Map<string, string>();
const lastReminderMap = new Map<string, number>();
const statusMonitorReady = ref(false);
const soundEnabled = ref(true);
const statusToasts = ref<StatusToast[]>([]);
const reminderThrottleMs = 5000;
let statusMonitorTimer: ReturnType<typeof setInterval> | undefined;
let statusStreamController: AbortController | undefined;
let statusStreamReconnectTimer: ReturnType<typeof setTimeout> | undefined;
let audioContext: AudioContext | undefined;
let audioUnlocked = false;
let statusToastId = 0;

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

function addStatusReminder(device: DeviceItem) {
  const reminderKey = `${device.id}-${device.status}`;
  const now = Date.now();
  if (now - (lastReminderMap.get(reminderKey) ?? 0) < reminderThrottleMs) return;
  lastReminderMap.set(reminderKey, now);
  const toast: StatusToast = {
    id: ++statusToastId,
    title: `设备${statusLabel(device.status)}`,
    message: `${device.name} (${device.deviceKey}) 已${statusLabel(device.status)}`,
    status: device.status,
  };
  statusToasts.value = [toast, ...statusToasts.value].slice(0, 3);
  window.setTimeout(() => {
    statusToasts.value = statusToasts.value.filter((item) => item.id !== toast.id);
  }, 8000);
  void playStatusSound(device.status);
}

function handleStatusChange(device: DeviceItem) {
  const previousStatus = deviceStatusMap.get(device.id);
  deviceStatusMap.set(device.id, device.status);
  if (!statusMonitorReady.value || !previousStatus || previousStatus === device.status) return;
  if (['ONLINE', 'OFFLINE'].includes(device.status) && ['ONLINE', 'OFFLINE'].includes(previousStatus)) {
    window.dispatchEvent(new CustomEvent('device-status-change', { detail: device }));
    addStatusReminder(device);
  }
}

async function pollDeviceStatus() {
  if (!getAccessToken()) return;
  try {
    const data = await apiRequest<{ items: DeviceItem[] }>({ url: '/api/devices', method: 'GET', params: { page: 1, pageSize: 100 } });
    for (const device of data.items) {
      handleStatusChange(device);
    }
    statusMonitorReady.value = true;
  } catch {
    // 登录态过期时请求拦截器会统一处理，这里只避免监控循环中断。
  }
}

async function primeDeviceStatus() {
  if (!getAccessToken()) return;
  try {
    const data = await apiRequest<{ items: DeviceItem[] }>({ url: '/api/devices', method: 'GET', params: { page: 1, pageSize: 100 } });
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
  const baseURL = import.meta.env.VITE_API_BASE_URL ?? `${window.location.protocol}//${window.location.hostname}:3100`;
  const token = getAccessToken();
  if (!token) return;
  try {
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

async function startStatusMonitor() {
  await primeDeviceStatus();
  void connectStatusStream();
  statusMonitorTimer = setInterval(() => void pollDeviceStatus(), 2000);
}

function stopStatusMonitor() {
  if (statusMonitorTimer) clearInterval(statusMonitorTimer);
  if (statusStreamReconnectTimer) clearTimeout(statusStreamReconnectTimer);
  statusStreamController?.abort();
  statusMonitorTimer = undefined;
  statusStreamReconnectTimer = undefined;
  statusStreamController = undefined;
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
      <div v-for="toast in statusToasts" :key="toast.id" class="status-toast" :class="statusType(toast.status)">
        <span class="status-toast-dot">{{ toast.status === 'ONLINE' ? '✓' : '!' }}</span>
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
        <router-link to="/dashboard" class="menu-item">
          <el-icon><Monitor /></el-icon>
          <span>数据概览</span>
        </router-link>
        <router-link to="/devices" class="menu-item">
          <el-icon><Connection /></el-icon>
          <span>设备管理</span>
        </router-link>
        <router-link to="/alarms" class="menu-item">
          <el-icon><Bell /></el-icon>
          <span>告警管理</span>
        </router-link>
        <router-link v-if="userStore.userInfo?.role !== 'TENANT_USER'" to="/users" class="menu-item">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </router-link>
        <router-link v-if="userStore.userInfo?.role === 'SUPER_ADMIN'" to="/tenants" class="menu-item">
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
.shell { display: flex; min-height: 100vh; }
.sidebar { width: 220px; padding: 22px 18px; border-right: 1px solid #e9eef5; background: #fff; }
.menu { margin-top: 38px; }
.menu-item { display: flex; gap: 10px; align-items: center; padding: 11px 12px; border-radius: 9px; color: #697386; font-size: 14px; }
.menu-item.router-link-active { color: #2563eb; background: #eff6ff; font-weight: 600; }
.main { flex: 1; min-width: 0; }
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
.status-toast-dot { display: grid; flex: 0 0 22px; width: 22px; height: 22px; place-items: center; border-radius: 50%; color: #fff; background: #22c55e; font-size: 13px; font-weight: 800; }
.status-toast.warning .status-toast-dot { background: #f59e0b; }
.status-toast strong { display: block; color: #172033; font-size: 15px; }
.status-toast p { margin: 4px 0 0; color: #64748b; font-size: 13px; line-height: 1.5; }
.status-toast-enter-active, .status-toast-leave-active { transition: all 0.2s ease; }
.status-toast-enter-from, .status-toast-leave-to { opacity: 0; transform: translateY(-8px); }
@media (max-width: 700px) { .sidebar { display: none; } .header, .content { padding-left: 18px; padding-right: 18px; } }
</style>
