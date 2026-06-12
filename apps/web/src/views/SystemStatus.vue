<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue';
import { computed, onMounted, ref } from 'vue';
import { apiRequest } from '../api/request';

interface HealthItem {
  name: string;
  state: 'UP' | 'DOWN' | 'WARN';
  latencyMs?: number;
  message: string;
}

interface SystemStatus {
  overall: 'UP' | 'DOWN' | 'WARN';
  checkedAt: string;
  services: HealthItem[];
  recentTelemetry?: { time: string; deviceKey: string; deviceName?: string } | null;
  recentDeviceLog?: { createdAt: string; deviceKey: string; deviceName?: string; type: string; message: string } | null;
}

const loading = ref(false);
const status = ref<SystemStatus>();

const healthyCount = computed(() => status.value?.services.filter((item) => item.state === 'UP').length ?? 0);
const issueCount = computed(() => status.value?.services.filter((item) => item.state !== 'UP').length ?? 0);

const signalCards = computed(() => [
  {
    label: '服务连通',
    value: `${healthyCount.value}/${status.value?.services.length ?? 0}`,
    description: issueCount.value ? `${issueCount.value} 个服务需要关注` : '核心服务连接正常',
    tone: issueCount.value ? 'warn' : 'ok',
  },
  {
    label: '最近遥测',
    value: status.value?.recentTelemetry?.deviceName || status.value?.recentTelemetry?.deviceKey || '-',
    description: formatTime(status.value?.recentTelemetry?.time),
    tone: status.value?.recentTelemetry ? 'ok' : 'muted',
  },
  {
    label: '最近设备事件',
    value: status.value?.recentDeviceLog?.deviceName || status.value?.recentDeviceLog?.deviceKey || '-',
    description: status.value?.recentDeviceLog ? `${status.value.recentDeviceLog.message} · ${formatTime(status.value.recentDeviceLog.createdAt)}` : '-',
    tone: status.value?.recentDeviceLog ? 'ok' : 'muted',
  },
]);

async function loadStatus() {
  loading.value = true;
  try {
    status.value = await apiRequest<SystemStatus>({ url: '/api/system-status', method: 'GET' });
  } finally {
    loading.value = false;
  }
}

function stateLabel(state?: string) {
  return { UP: '正常', DOWN: '异常', WARN: '警告' }[state ?? ''] ?? '-';
}

function stateType(state?: string) {
  return state === 'UP' ? 'success' : state === 'WARN' ? 'warning' : 'danger';
}

function formatTime(time?: string) {
  return time ? new Date(time).toLocaleString() : '-';
}

onMounted(loadStatus);
</script>

<template>
  <div class="status-page">
    <div class="page-actions status-header">
      <div>
        <h1 class="page-title">系统状态</h1>
        <p class="page-description">面向运维排查，关注平台服务连通性和数据链路是否健康。</p>
      </div>
      <div class="header-actions">
        <span class="checked-time">最近检查 {{ formatTime(status?.checkedAt) }}</span>
        <el-button :icon="RefreshRight" :loading="loading" type="primary" plain @click="loadStatus">刷新</el-button>
      </div>
    </div>

    <section class="status-hero" :class="status?.overall?.toLowerCase()">
      <div>
        <span class="eyebrow">整体健康</span>
        <div class="hero-title">
          <i class="status-orb" />
          <strong>{{ stateLabel(status?.overall) }}</strong>
        </div>
        <p>核心链路会检查 API、数据库、时序库和 MQTT Broker。</p>
      </div>
      <div class="hero-meta">
        <span>{{ healthyCount }} 个正常</span>
        <span v-if="issueCount">{{ issueCount }} 个异常</span>
        <span v-else>暂无异常</span>
      </div>
    </section>

    <section class="signal-grid">
      <article v-for="card in signalCards" :key="card.label" :class="card.tone">
        <span class="card-label">
          <i />
          {{ card.label }}
        </span>
        <strong>{{ card.value }}</strong>
        <p>{{ card.description }}</p>
      </article>
    </section>

    <section class="table-panel health-panel">
      <div class="section-heading">
        <div>
          <h2>服务健康</h2>
          <p>检查 API、主数据库、时序库和 MQTT Broker 的连通性。</p>
        </div>
      </div>
      <el-table v-loading="loading" :data="status?.services ?? []" empty-text="暂无状态数据">
        <el-table-column prop="name" label="服务" min-width="150" />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="stateType(row.state)">{{ stateLabel(row.state) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="延迟" width="110">
          <template #default="{ row }">{{ row.latencyMs === undefined ? '-' : `${row.latencyMs} ms` }}</template>
        </el-table-column>
        <el-table-column prop="message" label="说明" min-width="260" show-overflow-tooltip />
      </el-table>
    </section>
  </div>
</template>

<style scoped>
.status-page { display: grid; gap: 18px; }
.status-header { margin-bottom: 0; }
.header-actions { display: flex; align-items: center; gap: 12px; }
.checked-time { padding: 7px 11px; border: 1px solid #e5eaf2; border-radius: 999px; color: #64748b; background: #fff; font-size: 12px; }
.status-hero { position: relative; display: flex; align-items: center; justify-content: space-between; overflow: hidden; padding: 26px 28px; border: 1px solid #dbeafe; border-radius: 20px; background: radial-gradient(circle at top right, rgba(37, 99, 235, 0.14), transparent 32%), linear-gradient(135deg, #f8fbff, #fff); box-shadow: 0 16px 40px rgba(15, 23, 42, 0.05); }
.status-hero::after { position: absolute; right: 28px; bottom: -54px; width: 160px; height: 160px; border-radius: 999px; background: rgba(37, 99, 235, 0.06); content: ''; }
.status-hero.down { border-color: #fecaca; background: radial-gradient(circle at top right, rgba(239, 68, 68, 0.14), transparent 32%), linear-gradient(135deg, #fffafa, #fff); }
.status-hero.warn { border-color: #fed7aa; background: radial-gradient(circle at top right, rgba(245, 158, 11, 0.16), transparent 32%), linear-gradient(135deg, #fffdf7, #fff); }
.eyebrow { color: #64748b; font-size: 13px; font-weight: 600; }
.hero-title { display: flex; align-items: center; gap: 12px; margin-top: 8px; }
.hero-title strong { color: #0f172a; font-size: 34px; letter-spacing: -0.03em; }
.status-orb { display: inline-block; width: 14px; height: 14px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 8px rgba(34, 197, 94, 0.12); }
.status-hero.down .status-orb { background: #ef4444; box-shadow: 0 0 0 8px rgba(239, 68, 68, 0.12); }
.status-hero.warn .status-orb { background: #f59e0b; box-shadow: 0 0 0 8px rgba(245, 158, 11, 0.14); }
.status-hero p { margin: 10px 0 0; color: #64748b; font-size: 13px; }
.hero-meta { z-index: 1; display: flex; gap: 10px; }
.hero-meta span { padding: 8px 12px; border: 1px solid rgba(148, 163, 184, 0.24); border-radius: 999px; color: #475569; background: rgba(255, 255, 255, 0.72); font-size: 13px; backdrop-filter: blur(8px); }
.signal-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 14px; }
.signal-grid article { min-width: 0; padding: 18px; border: 1px solid #e9eef5; border-radius: 18px; background: #fff; box-shadow: 0 10px 28px rgba(15, 23, 42, 0.035); transition: transform 0.18s ease, box-shadow 0.18s ease; }
.signal-grid article:hover { transform: translateY(-2px); box-shadow: 0 16px 34px rgba(15, 23, 42, 0.07); }
.signal-grid article.ok { border-color: #dcfce7; background: linear-gradient(180deg, #f7fef9, #fff); }
.signal-grid article.warn { border-color: #fed7aa; background: linear-gradient(180deg, #fff9ed, #fff); }
.signal-grid article.muted { background: linear-gradient(180deg, #f8fafc, #fff); }
.card-label { display: inline-flex; align-items: center; gap: 8px; color: #64748b; font-size: 13px; font-weight: 600; }
.card-label i { width: 8px; height: 8px; border-radius: 50%; background: #22c55e; }
.warn .card-label i { background: #f59e0b; }
.muted .card-label i { background: #94a3b8; }
.signal-grid strong { display: block; overflow: hidden; margin-top: 12px; color: #111827; font-size: 21px; letter-spacing: -0.02em; text-overflow: ellipsis; white-space: nowrap; }
.signal-grid p { overflow: hidden; margin: 8px 0 0; color: #64748b; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.health-panel { padding: 20px; border-radius: 18px; box-shadow: 0 10px 28px rgba(15, 23, 42, 0.035); }
.section-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.section-heading h2 { margin: 0; color: #111827; font-size: 17px; }
.section-heading p { margin: 6px 0 0; color: #94a3b8; font-size: 13px; }
@media (max-width: 760px) {
  .status-header, .status-hero, .header-actions { align-items: flex-start; flex-direction: column; }
  .header-actions, .hero-meta { width: 100%; }
  .checked-time { width: 100%; }
}
</style>
