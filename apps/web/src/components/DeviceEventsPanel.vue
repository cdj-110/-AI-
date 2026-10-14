<script setup lang="ts">
import { RefreshRight, Right } from '@element-plus/icons-vue';
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { apiRequest } from '../api/request';

interface DeviceContext {
  id: string;
  name: string;
  deviceKey: string;
  deviceType: string;
}

interface EventItem {
  id: string;
  state: string;
  level: string;
  ruleName?: string;
  limitName?: string;
  pointName?: string;
  metric?: string;
  valueText?: string;
  unit?: string;
  message: string;
  sourceUpdatedAt: string;
  gateway: { id: string; name: string; deviceKey: string };
  device?: { id: string; name: string; deviceKey: string };
  deviceKey?: string;
}

const props = defineProps<{ device: DeviceContext }>();
const router = useRouter();
const loading = ref(false);
const items = ref<EventItem[]>([]);
const total = ref(0);
const stateCounts = ref<Record<string, number>>({});
let refreshTimer: ReturnType<typeof setInterval> | undefined;

const isGateway = computed(() => props.device.deviceType === 'GATEWAY');
const filterQuery = computed(() => isGateway.value
  ? { gatewayId: props.device.id }
  : { deviceId: props.device.id });
const activeCount = computed(() => stateCounts.value.ACTIVE ?? 0);
const acknowledgedCount = computed(() => stateCounts.value.ACKNOWLEDGED ?? 0);
const recoveredCount = computed(() => (stateCounts.value.RESOLVED ?? 0) + (stateCounts.value.MANUAL_RESOLVED ?? 0));

async function loadEvents(silent = false) {
  if (!silent) loading.value = true;
  try {
    const result = await apiRequest<{ items: EventItem[]; total: number; stateCounts: Record<string, number> }>({
      url: '/api/events',
      method: 'GET',
      params: { page: 1, pageSize: 10, ...filterQuery.value },
      silentError: silent,
    });
    items.value = result.items;
    total.value = result.total;
    stateCounts.value = result.stateCounts;
  } finally {
    if (!silent) loading.value = false;
  }
}

function eventCenterQuery(eventId?: string) {
  return {
    ...filterQuery.value,
    sourceName: props.device.name,
    ...(eventId ? { eventId } : {}),
  };
}

function openEventCenter(eventId?: string) {
  void router.push({ path: '/events', query: eventCenterQuery(eventId) });
}

function stateLabel(state: string) {
  return { ACTIVE: '告警触发', ACKNOWLEDGED: '已确认', RESOLVED: '自动恢复', MANUAL_RESOLVED: '手动恢复' }[state] ?? state;
}

function stateType(state: string) {
  return state === 'ACTIVE' ? 'danger' : state === 'ACKNOWLEDGED' ? 'warning' : 'success';
}

function levelLabel(level: string) {
  return { INFO: '提示', WARNING: '警告', CRITICAL: '严重' }[level] ?? level;
}

function formatTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN', { hour12: false });
}

function displayValue(row: EventItem) {
  if (!row.valueText) return '-';
  return `${row.valueText}${row.unit ? ` ${row.unit}` : ''}`;
}

onMounted(() => {
  void loadEvents();
  refreshTimer = setInterval(() => {
    if (document.visibilityState === 'visible') void loadEvents(true);
  }, 5000);
});

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<template>
  <section class="device-events-panel detail-section-first">
    <div class="event-heading">
      <div>
        <h2>设备事件</h2>
        <p>{{ isGateway ? '显示本网关及其下属设备的事件，用于定位整条采集链路。' : '仅显示当前设备产生的事件，便于结合实时数据排查。' }}</p>
      </div>
      <div class="heading-actions">
        <el-button :icon="RefreshRight" :loading="loading" @click="loadEvents()">刷新</el-button>
        <el-button type="primary" :icon="Right" @click="openEventCenter()">进入事件中心</el-button>
      </div>
    </div>

    <div class="event-summary">
      <article><span>全部事件</span><strong>{{ total }}</strong></article>
      <article class="active"><span>当前触发</span><strong>{{ activeCount }}</strong></article>
      <article class="acknowledged"><span>已确认</span><strong>{{ acknowledgedCount }}</strong></article>
      <article class="recovered"><span>已恢复</span><strong>{{ recoveredCount }}</strong></article>
    </div>

    <div class="event-table">
      <el-table v-loading="loading" :data="items" row-key="id" table-layout="fixed" empty-text="当前设备暂无事件" @row-click="(row: EventItem) => openEventCenter(row.id)">
        <el-table-column label="等级" width="82">
          <template #default="{ row }"><span class="level" :class="row.level.toLowerCase()">{{ levelLabel(row.level) }}</span></template>
        </el-table-column>
        <el-table-column label="动作" width="112">
          <template #default="{ row }"><el-tag :type="stateType(row.state)" effect="light">{{ stateLabel(row.state) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="来源设备" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.device?.name || row.deviceKey || '网关本机' }}</template>
        </el-table-column>
        <el-table-column label="规则 / 点位" min-width="210" show-overflow-tooltip>
          <template #default="{ row }">{{ row.ruleName || '未命名规则' }} · {{ row.pointName || row.metric || '-' }}</template>
        </el-table-column>
        <el-table-column label="值" width="112"><template #default="{ row }"><strong class="value">{{ displayValue(row) }}</strong></template></el-table-column>
        <el-table-column prop="message" label="事件说明" min-width="280" show-overflow-tooltip />
        <el-table-column label="发生时间" width="176"><template #default="{ row }">{{ formatTime(row.sourceUpdatedAt) }}</template></el-table-column>
        <el-table-column label="" width="42"><template #default><el-icon class="open-icon"><Right /></el-icon></template></el-table-column>
      </el-table>
    </div>
    <p class="event-footnote">点击任意事件，将进入事件中心并自动保留“{{ device.name }}”筛选条件。</p>
  </section>
</template>

<style scoped>
.device-events-panel{overflow:hidden;border:1px solid #dfe7f2;border-radius:14px;background:#fff;box-shadow:0 8px 24px rgb(30 64 175 / 4%)}
.event-heading{display:flex;align-items:center;justify-content:space-between;gap:20px;padding:18px 20px;border-bottom:1px solid #e8eef6;background:linear-gradient(135deg,#fbfdff,#f5f9ff)}
.event-heading h2{margin:0;color:#17233b;font-size:18px}.event-heading p{margin:6px 0 0;color:#71809a;font-size:13px}.heading-actions{display:flex;gap:10px;flex-shrink:0}
.event-summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;padding:16px 20px}.event-summary article{position:relative;padding:14px 16px;border:1px solid #e5ebf4;border-radius:11px;background:#fafcff}.event-summary article:before{position:absolute;inset:0 auto 0 0;width:3px;border-radius:11px 0 0 11px;background:#7690b6;content:""}.event-summary article.active:before{background:#ef4444}.event-summary article.acknowledged:before{background:#f59e0b}.event-summary article.recovered:before{background:#10b981}.event-summary span{display:block;color:#78869b;font-size:12px}.event-summary strong{display:block;margin-top:6px;color:#15223a;font-size:24px;line-height:1}
.event-table{padding:0 20px}.event-table :deep(.el-table__row){cursor:pointer}.event-table :deep(.el-table__row:hover) .open-icon{color:#1677ff;transform:translateX(2px)}.level{font-weight:700}.level.info{color:#2563eb}.level.warning{color:#d97706}.level.critical{color:#dc2626}.value{font-variant-numeric:tabular-nums}.open-icon{color:#9aa8bc;transition:.16s}.event-footnote{margin:0;padding:13px 20px 16px;color:#7b899e;font-size:12px}
@media(max-width:900px){.event-summary{grid-template-columns:repeat(2,1fr)}.event-heading{align-items:flex-start;flex-direction:column}.heading-actions{width:100%}.heading-actions .el-button{flex:1}}
</style>
