<script setup lang="ts">
import { ArrowLeft, Connection, RefreshRight, SwitchButton } from '@element-plus/icons-vue';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { ElMessage } from 'element-plus';
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { portalLogo } from '../portal';
import { apiRequest } from '../api/request';

type ConnectionState = 'IDLE' | 'CONNECTING' | 'CONNECTED' | 'CLOSED' | 'ERROR';

const route = useRoute();
const router = useRouter();
const terminalShell = ref<HTMLElement>();
const terminalHost = ref<HTMLElement>();
const state = ref<ConnectionState>('IDLE');
const errorMessage = ref('');
const form = reactive({ username: 'root', password: '' });
let terminal: Terminal | undefined;
let fitAddon: FitAddon | undefined;
let socket: WebSocket | undefined;
let resizeObserver: ResizeObserver | undefined;
let inputDisposable: { dispose: () => void } | undefined;
let fitFrame: number | undefined;
let lastFitWidth = 0;
let lastFitHeight = 0;

const deviceId = computed(() => String(route.params.id ?? ''));
const deviceName = computed(() => String(route.query.name ?? 'EG100'));
const deviceKey = computed(() => String(route.query.key ?? ''));
const connecting = computed(() => state.value === 'CONNECTING');
const connected = computed(() => state.value === 'CONNECTED');
const stateText = computed(() => ({
  IDLE: '等待连接', CONNECTING: '正在连接', CONNECTED: '已连接', CLOSED: '连接已断开', ERROR: '连接失败',
}[state.value]));

function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function decodeBase64(value: string) {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}

function send(payload: Record<string, unknown>) {
  if (socket?.readyState === WebSocket.OPEN) socket.send(JSON.stringify(payload));
}

function sendSize() {
  if (!terminal || !connected.value) return;
  send({ type: 'ssh.resize', cols: terminal.cols, rows: terminal.rows });
}

function fitTerminal() {
  if (!terminalHost.value || !fitAddon) return;
  const width = terminalHost.value.clientWidth;
  const height = terminalHost.value.clientHeight;
  if (width < 1 || height < 1 || (width === lastFitWidth && height === lastFitHeight)) return;
  lastFitWidth = width;
  lastFitHeight = height;
  try { fitAddon.fit(); } catch { /* hidden during initial layout */ }
  sendSize();
}

function scheduleTerminalFit() {
  if (fitFrame !== undefined) cancelAnimationFrame(fitFrame);
  fitFrame = requestAnimationFrame(() => {
    fitFrame = undefined;
    fitTerminal();
  });
}

async function connect() {
  if (connecting.value || connected.value) return;
  if (!form.username.trim() || !form.password) {
    ElMessage.warning('请输入 SSH 账号和密码');
    return;
  }
  state.value = 'CONNECTING';
  errorMessage.value = '';
  socket?.close();
  terminal?.clear();
  terminal?.writeln('\x1b[38;5;39m正在通过云平台建立安全 SSH 连接…\x1b[0m');
  try {
    const result = await apiRequest<{ url: string }>({
      url: `/api/remote-access/devices/${deviceId.value}/terminal-sessions`, method: 'POST',
    });
    const current = new WebSocket(result.url);
    socket = current;
    current.onmessage = (event) => {
      if (socket !== current) return;
      let message: { type?: string; dataBase64?: string; error?: string };
      try { message = JSON.parse(String(event.data)); } catch { return; }
      if (message.type === 'terminal.ready') {
        send({
          type: 'ssh.auth', username: form.username.trim(), password: form.password,
          cols: terminal?.cols ?? 120, rows: terminal?.rows ?? 32,
        });
      } else if (message.type === 'ssh.opened') {
        state.value = 'CONNECTED';
        form.password = '';
        terminal?.clear();
        terminal?.focus();
        sendSize();
      } else if (message.type === 'ssh.data' && message.dataBase64) {
        terminal?.write(decodeBase64(message.dataBase64));
      } else if (message.type === 'ssh.error') {
        state.value = 'ERROR';
        errorMessage.value = message.error || 'SSH 连接失败';
        terminal?.writeln(`\r\n\x1b[31m${errorMessage.value}\x1b[0m`);
      } else if (message.type === 'ssh.close') {
        state.value = 'CLOSED';
        terminal?.writeln('\r\n\x1b[33mSSH 会话已结束。\x1b[0m');
      }
    };
    current.onerror = () => {
      if (socket !== current || state.value === 'ERROR') return;
      state.value = 'ERROR';
      errorMessage.value = '安全通道连接失败，请确认网关在线后重试';
    };
    current.onclose = (event) => {
      if (socket !== current) return;
      socket = undefined;
      if (state.value === 'CONNECTING') {
        state.value = 'ERROR';
        errorMessage.value = event.code === 4001 ? '会话长时间未操作，已自动结束' : 'SSH 连接未能建立';
      } else if (state.value === 'CONNECTED') {
        state.value = 'CLOSED';
        terminal?.writeln('\r\n\x1b[33m连接已断开，可点击“重新连接”。\x1b[0m');
      }
    };
  } catch {
    state.value = 'ERROR';
    errorMessage.value = '无法创建 SSH 会话，请稍后重试';
  }
}

function disconnect() {
  socket?.close(1000, 'user disconnected');
  socket = undefined;
  state.value = 'CLOSED';
  terminal?.writeln('\r\n\x1b[33m已主动断开 SSH 会话。\x1b[0m');
}

function closePage() {
  window.close();
  if (!window.closed) void router.push(`/devices/${deviceId.value}`);
}

onMounted(async () => {
  await nextTick();
  terminal = new Terminal({
    cursorBlink: true,
    convertEol: true,
    fontFamily: 'Cascadia Mono, Consolas, Menlo, monospace',
    fontSize: 14,
    lineHeight: 1.2,
    scrollback: 5000,
    theme: {
      background: '#07111f', foreground: '#d8e5f5', cursor: '#58a6ff', cursorAccent: '#07111f',
      selectionBackground: '#1f6feb66', black: '#0b1626', brightBlack: '#52677f', red: '#ff6b6b',
      brightRed: '#ff8787', green: '#69db7c', brightGreen: '#8ce99a', yellow: '#ffd43b',
      brightYellow: '#ffe066', blue: '#4dabf7', brightBlue: '#74c0fc', magenta: '#da77f2',
      brightMagenta: '#e599f7', cyan: '#3bc9db', brightCyan: '#66d9e8', white: '#d8e5f5', brightWhite: '#ffffff',
    },
  });
  fitAddon = new FitAddon();
  terminal.loadAddon(fitAddon);
  terminal.open(terminalHost.value!);
  terminal.writeln('\x1b[1;38;5;39m微控云安全 SSH 终端\x1b[0m');
  terminal.writeln('请输入网关 SSH 账号和密码后连接。凭据不会保存在云平台。');
  inputDisposable = terminal.onData((data) => {
    if (connected.value) send({ type: 'ssh.input', dataBase64: encodeBase64(data) });
  });
  resizeObserver = new ResizeObserver(() => scheduleTerminalFit());
  resizeObserver.observe(terminalShell.value!);
  scheduleTerminalFit();
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  if (fitFrame !== undefined) cancelAnimationFrame(fitFrame);
  inputDisposable?.dispose();
  socket?.close(1000, 'page closed');
  terminal?.dispose();
});
</script>

<template>
  <main class="terminal-page">
    <header class="terminal-header">
      <div class="device-heading">
        <img v-if="portalLogo" class="brand-logo" :src="portalLogo" alt="微控物联网平台" />
        <span v-else class="brand-mark">微</span>
        <div>
          <h1>{{ deviceName }} · SSH 终端</h1>
          <p>{{ deviceKey || deviceId }} · 通过云平台安全维护通道连接</p>
        </div>
      </div>
      <div class="header-actions">
        <span class="connection-state" :class="state.toLowerCase()"><i />{{ stateText }}</span>
        <el-button v-if="connected" :icon="SwitchButton" @click="disconnect">断开连接</el-button>
        <el-button :icon="ArrowLeft" @click="closePage">返回设备</el-button>
      </div>
    </header>

    <section ref="terminalShell" class="terminal-shell">
      <div v-if="!connected" class="login-overlay">
        <form class="login-card" @submit.prevent="connect">
          <span class="login-icon"><Connection /></span>
          <h2>{{ state === 'ERROR' ? '连接失败' : state === 'CLOSED' ? '重新连接' : '连接网关 SSH' }}</h2>
          <p v-if="errorMessage" class="error-text">{{ errorMessage }}</p>
          <p v-else>使用网关本机 SSH 账号登录，账号与密码不会保存。</p>
          <label>SSH 账号<el-input v-model="form.username" autocomplete="username" :disabled="connecting" /></label>
          <label>SSH 密码<el-input v-model="form.password" type="password" show-password autocomplete="current-password" :disabled="connecting" /></label>
          <el-button native-type="submit" type="primary" size="large" :icon="state === 'ERROR' || state === 'CLOSED' ? RefreshRight : Connection" :loading="connecting">
            {{ state === 'ERROR' || state === 'CLOSED' ? '重新连接' : '连接终端' }}
          </el-button>
          <small>终端长时间无操作会自动断开，重新连接需再次输入密码。</small>
        </form>
      </div>
      <div ref="terminalHost" class="terminal-host" />
    </section>
  </main>
</template>

<style scoped>
:global(html), :global(body), :global(#app) { width: 100%; height: 100%; margin: 0; overflow: hidden; }
.terminal-page { display: grid; grid-template-rows: 72px minmax(0, 1fr); width: 100vw; max-width: 100vw; min-width: 0; height: 100%; overflow: hidden; background: #07111f; color: #17233b; }
.terminal-header { z-index: 3; display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 24px; padding: 0 24px; border-bottom: 1px solid #dce6f2; background: #fff; box-shadow: 0 4px 18px rgb(15 23 42 / 8%); }
.device-heading, .header-actions { display: flex; align-items: center; gap: 14px; min-width: 0; }
.brand-mark { display: grid; width: 38px; height: 38px; place-items: center; flex: 0 0 auto; border-radius: 11px; color: #fff; background: linear-gradient(145deg, #1769ff, #2f8cff); box-shadow: 0 8px 18px rgb(23 105 255 / 25%); font-size: 18px; font-weight: 800; }
.brand-logo { display: block; box-sizing: border-box; width: 38px; height: 38px; padding: 2px; flex: 0 0 38px; border: 0; object-fit: contain; background: transparent; }
h1 { overflow: hidden; margin: 0; color: #10213a; font-size: 18px; text-overflow: ellipsis; white-space: nowrap; }
.device-heading p { overflow: hidden; margin: 4px 0 0; color: #73839a; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.connection-state { display: inline-flex; align-items: center; gap: 7px; color: #73839a; font-size: 13px; white-space: nowrap; }
.connection-state i { width: 8px; height: 8px; border-radius: 50%; background: #9aa9bd; }
.connection-state.connected i { background: #20b26b; box-shadow: 0 0 0 5px rgb(32 178 107 / 12%); }
.connection-state.connecting i { background: #1677ff; animation: pulse 1.2s infinite; }
.connection-state.error i { background: #ef4444; }
.connection-state.closed i { background: #f59e0b; }
.terminal-shell { position: relative; width: 100%; min-width: 0; min-height: 0; overflow: hidden; padding: 14px 16px 16px; background: radial-gradient(circle at 50% -30%, #17385d 0, #0a1728 42%, #050b13 100%); contain: layout paint; }
.terminal-host { width: 100%; min-width: 0; height: 100%; min-height: 0; overflow: hidden; padding: 12px; border: 1px solid #20344c; border-radius: 12px; background: #07111f; box-shadow: inset 0 1px 0 rgb(255 255 255 / 4%), 0 18px 60px rgb(0 0 0 / 25%); }
.terminal-host :deep(.xterm) { height: 100%; }
.login-overlay { position: absolute; z-index: 2; inset: 14px 16px 16px; overflow: hidden; border-radius: 12px; background: rgb(3 10 19 / 72%); backdrop-filter: blur(5px); contain: strict; }
.login-card { position: absolute; top: 50%; left: 50%; display: grid; width: min(420px, calc(100% - 32px)); gap: 15px; padding: 28px; border: 1px solid rgb(121 163 207 / 28%); border-radius: 16px; background: rgb(15 29 48 / 96%); box-shadow: 0 24px 70px rgb(0 0 0 / 38%); color: #d9e5f3; transform: translate(-50%, -50%); transition: none; }
.login-icon { display: grid; width: 48px; height: 48px; place-items: center; border-radius: 14px; color: #69b1ff; background: #173453; }
.login-icon :deep(svg) { width: 24px; height: 24px; }
.login-card h2 { margin: 0; color: #fff; font-size: 20px; }
.login-card p { margin: -8px 0 0; color: #9eb1c8; font-size: 13px; line-height: 1.6; }
.login-card .error-text { color: #ff9b9b; }
.login-card label { display: grid; gap: 7px; color: #c6d4e4; font-size: 13px; }
.login-card small { color: #7f93aa; line-height: 1.6; text-align: center; }
.login-card :deep(.el-input__wrapper) { background: #0b1929; box-shadow: 0 0 0 1px #29435f inset; }
.login-card :deep(.el-input__inner) { color: #eef6ff; }
@keyframes pulse { 50% { opacity: .35; } }
@media (max-width: 720px) { .terminal-header { height: auto; padding: 10px 12px; } .terminal-page { grid-template-rows: auto minmax(0, 1fr); } .device-heading p { display: none; } .header-actions { gap: 8px; } .connection-state { display: none; } .terminal-shell { padding: 8px; } .login-overlay { inset: 8px; } }
</style>
