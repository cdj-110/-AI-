<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';

interface EventItem {
  id: string;
  sourceEventId: string;
  deviceKey?: string | null;
  eventType: string;
  state: string;
  level: string;
  ruleName?: string;
  limitName?: string;
  pointName?: string;
  metric?: string;
  valueText?: string;
  unit?: string;
  threshold?: number;
  quality?: string;
  message: string;
  occurredAt: string;
  sourceUpdatedAt: string;
  acknowledgedAt?: string;
  recoveredAt?: string;
  gateway: { id: string; name: string; deviceKey: string };
  device?: { id: string; name: string; deviceKey: string };
  tenant?: { id: string; name: string };
}

interface GatewayOption { id: string; name: string; deviceKey: string }

const route = useRoute();
const router = useRouter();
const loading = ref(false);
const items = ref<EventItem[]>([]);
const total = ref(0);
const stateCounts = ref<Record<string, number>>({});
const gateways = ref<GatewayOption[]>([]);
const detail = ref<EventItem>();
const detailVisible = ref(false);
const dateRange = ref<[Date, Date]>();
const query = reactive({
  page: 1,
  pageSize: 20,
  keyword: '',
  gatewayId: typeof route.query.gatewayId === 'string' ? route.query.gatewayId : '',
  deviceId: typeof route.query.deviceId === 'string' ? route.query.deviceId : '',
  level: '',
  state: '',
});
const sourceFilterName = ref(typeof route.query.sourceName === 'string' ? route.query.sourceName : '');
const pendingDetailEventId = ref(typeof route.query.eventId === 'string' ? route.query.eventId : '');
let refreshTimer: ReturnType<typeof setInterval> | undefined;

const triggerCount = computed(() => stateCounts.value.ACTIVE ?? 0);
const acknowledgedCount = computed(() => stateCounts.value.ACKNOWLEDGED ?? 0);
const recoveredCount = computed(() => (stateCounts.value.RESOLVED ?? 0) + (stateCounts.value.MANUAL_RESOLVED ?? 0));

async function loadEvents(resetPage = false, silent = false) {
  if (resetPage) query.page = 1;
  if (!silent) loading.value = true;
  try {
    const result = await apiRequest<{
      items: EventItem[];
      total: number;
      stateCounts: Record<string, number>;
      gateways: GatewayOption[];
    }>({
      url: '/api/events',
      method: 'GET',
      params: {
        ...query,
        startAt: dateRange.value?.[0]?.toISOString(),
        endAt: dateRange.value?.[1]?.toISOString(),
      },
      silentError: silent,
    });
    items.value = result.items;
    total.value = result.total;
    stateCounts.value = result.stateCounts;
    gateways.value = result.gateways;
    if (pendingDetailEventId.value) {
      const requested = result.items.find((item) => item.id === pendingDetailEventId.value);
      if (requested) showDetail(requested);
      pendingDetailEventId.value = '';
    }
  } finally {
    if (!silent) loading.value = false;
  }
}

function clearDeviceFilter() {
  query.gatewayId = '';
  query.deviceId = '';
  sourceFilterName.value = '';
  void router.replace({ path: '/events' });
  void loadEvents(true);
}

function handleGatewayChange() {
  query.deviceId = '';
  sourceFilterName.value = '';
  void router.replace({ path: '/events', query: query.gatewayId ? { gatewayId: query.gatewayId } : {} });
  void loadEvents(true);
}

function showDetail(row: EventItem) {
  detail.value = row;
  detailVisible.value = true;
}

function stateLabel(state: string) {
  return { ACTIVE: '告警触发', ACKNOWLEDGED: '已确认知晓', RESOLVED: '自动恢复', MANUAL_RESOLVED: '手动恢复' }[state] ?? state;
}

function stateType(state: string) {
  return state === 'ACTIVE' ? 'danger' : state === 'ACKNOWLEDGED' ? 'warning' : 'success';
}

function levelLabel(level: string) {
  return { INFO: '提示', WARNING: '警告', CRITICAL: '严重' }[level] ?? level;
}

function levelClass(level: string) {
  return level.toLowerCase();
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
    if (document.visibilityState === 'visible' && !detailVisible.value) void loadEvents(false, true);
  }, 5000);
});

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<template>
  <div class="events-page">
    <div class="page-actions events-heading">
      <div>
        <p class="event-eyebrow">GATEWAY EVENT CENTER</p>
        <h1 class="page-title">事件中心</h1>
        <p class="page-description">集中查看网关告警的触发、确认与恢复过程，事件由网关可靠上报并自动去重。</p>
      </div>
      <el-button :loading="loading" @click="loadEvents(false)">刷新事件</el-button>
    </div>

    <section class="event-summary">
      <article class="summary-card total"><span>筛选结果</span><strong>{{ total }}</strong><small>条事件记录</small></article>
      <article class="summary-card active"><span>告警触发</span><strong>{{ triggerCount }}</strong><small>异常条件成立</small></article>
      <article class="summary-card acknowledged"><span>确认知晓</span><strong>{{ acknowledgedCount }}</strong><small>人员已确认</small></article>
      <article class="summary-card recovered"><span>恢复事件</span><strong>{{ recoveredCount }}</strong><small>自动或手动恢复</small></article>
    </section>

    <section class="table-panel event-panel">
      <div v-if="query.deviceId || (query.gatewayId && sourceFilterName)" class="source-filter-tip">
        <div><strong>已按设备筛选</strong><span>{{ sourceFilterName || '当前设备' }} 的相关事件</span></div>
        <el-button link type="primary" @click="clearDeviceFilter">查看全部设备事件</el-button>
      </div>
      <div class="event-filter-bar">
        <el-input v-model="query.keyword" clearable placeholder="搜索规则、点位、设备或事件内容" class="keyword-input" @keyup.enter="loadEvents(true)" />
        <el-select v-model="query.gatewayId" clearable placeholder="全部网关" @change="handleGatewayChange">
          <el-option v-for="gateway in gateways" :key="gateway.id" :label="`${gateway.name}（${gateway.deviceKey}）`" :value="gateway.id" />
        </el-select>
        <el-select v-model="query.level" clearable placeholder="全部等级" @change="loadEvents(true)">
          <el-option label="提示" value="INFO" /><el-option label="警告" value="WARNING" /><el-option label="严重" value="CRITICAL" />
        </el-select>
        <el-select v-model="query.state" clearable placeholder="全部动作" @change="loadEvents(true)">
          <el-option label="告警触发" value="ACTIVE" /><el-option label="确认知晓" value="ACKNOWLEDGED" />
          <el-option label="自动恢复" value="RESOLVED" /><el-option label="手动恢复" value="MANUAL_RESOLVED" />
        </el-select>
        <el-date-picker v-model="dateRange" type="datetimerange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" @change="loadEvents(true)" />
        <el-button type="primary" @click="loadEvents(true)">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="items" row-key="id" table-layout="fixed" @row-click="showDetail">
        <el-table-column label="等级" width="88">
          <template #default="{ row }"><span class="level-pill" :class="levelClass(row.level)"><i />{{ levelLabel(row.level) }}</span></template>
        </el-table-column>
        <el-table-column label="事件动作" width="124">
          <template #default="{ row }"><el-tag :type="stateType(row.state)" effect="light">{{ stateLabel(row.state) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="网关 / 设备" min-width="210">
          <template #default="{ row }"><div class="two-line"><strong>{{ row.device?.name || row.deviceKey || '网关本机' }}</strong><span>{{ row.gateway.name }} · {{ row.gateway.deviceKey }}</span></div></template>
        </el-table-column>
        <el-table-column label="规则 / 点位" min-width="190">
          <template #default="{ row }"><div class="two-line"><strong>{{ row.ruleName || '未命名规则' }}</strong><span>{{ row.pointName || row.metric || '-' }}<template v-if="row.limitName"> · {{ row.limitName }}</template></span></div></template>
        </el-table-column>
        <el-table-column label="当前值" width="118"><template #default="{ row }"><strong class="event-value">{{ displayValue(row) }}</strong></template></el-table-column>
        <el-table-column prop="message" label="事件内容" min-width="300" show-overflow-tooltip />
        <el-table-column label="发生时间" width="176"><template #default="{ row }">{{ formatTime(row.sourceUpdatedAt) }}</template></el-table-column>
      </el-table>
      <div v-if="!loading && !items.length" class="event-empty"><span>✓</span><strong>暂无匹配事件</strong><small>网关产生告警后，触发、确认和恢复过程会显示在这里。</small></div>
      <div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :page-sizes="[20, 50, 100]" :total="total" @current-change="loadEvents(false)" @size-change="loadEvents(true)" /></div>
    </section>

    <el-drawer v-model="detailVisible" title="事件详情" size="520px">
      <div v-if="detail" class="event-detail">
        <div class="detail-hero" :class="levelClass(detail.level)"><span>{{ levelLabel(detail.level) }}</span><strong>{{ stateLabel(detail.state) }}</strong><small>{{ formatTime(detail.sourceUpdatedAt) }}</small></div>
        <dl>
          <div><dt>事件编号</dt><dd>{{ detail.sourceEventId }}</dd></div>
          <div><dt>网关</dt><dd>{{ detail.gateway.name }}（{{ detail.gateway.deviceKey }}）</dd></div>
          <div><dt>来源设备</dt><dd>{{ detail.device?.name || detail.deviceKey || '网关本机' }}</dd></div>
          <div><dt>告警规则</dt><dd>{{ detail.ruleName || '-' }}</dd></div>
          <div><dt>点位</dt><dd>{{ detail.pointName || detail.metric || '-' }}</dd></div>
          <div><dt>值 / 阈值</dt><dd>{{ displayValue(detail) }} / {{ detail.threshold ?? '-' }}</dd></div>
          <div><dt>质量</dt><dd>{{ detail.quality || '-' }}</dd></div>
          <div class="wide"><dt>事件说明</dt><dd>{{ detail.message }}</dd></div>
          <div><dt>首次触发</dt><dd>{{ formatTime(detail.occurredAt) }}</dd></div>
          <div><dt>上报时间</dt><dd>{{ formatTime(detail.sourceUpdatedAt) }}</dd></div>
        </dl>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.events-page{display:flex;flex-direction:column;gap:18px}.events-heading{align-items:flex-end}.event-eyebrow{margin:0 0 6px;color:#3b82f6;font-size:11px;font-weight:800;letter-spacing:.16em}.event-summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.summary-card{position:relative;overflow:hidden;padding:18px 20px;border:1px solid #e3eaf5;border-radius:14px;background:#fff;box-shadow:0 8px 24px rgba(31,55,92,.05)}.summary-card:before{position:absolute;inset:0 auto 0 0;width:4px;content:"";background:#8da2c2}.summary-card span,.summary-card small{display:block;color:#71809a;font-size:12px}.summary-card strong{display:block;margin:7px 0 4px;color:#13213a;font-size:28px;line-height:1}.summary-card.active:before{background:#ef4444}.summary-card.acknowledged:before{background:#f59e0b}.summary-card.recovered:before{background:#10b981}.event-panel{overflow:hidden;padding:0}.source-filter-tip{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:12px 16px;border-bottom:1px solid #d9e8ff;background:#f2f7ff}.source-filter-tip div{display:flex;align-items:center;gap:10px}.source-filter-tip strong{color:#1d4ed8;font-size:13px}.source-filter-tip span{color:#62738d;font-size:12px}.event-filter-bar{display:grid;grid-template-columns:minmax(250px,1.6fr) minmax(180px,1fr) 130px 150px minmax(330px,1.3fr) auto;gap:10px;padding:16px;border-bottom:1px solid #e8eef7;background:linear-gradient(180deg,#fbfdff,#f7faff)}.event-panel :deep(.el-table__row){cursor:pointer}.two-line{display:flex;min-width:0;flex-direction:column;gap:4px}.two-line strong,.two-line span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.two-line strong{color:#1a2942}.two-line span{color:#8290a7;font-size:12px}.level-pill{display:inline-flex;align-items:center;gap:6px;font-size:12px;font-weight:700}.level-pill i{width:7px;height:7px;border-radius:50%;background:#3b82f6}.level-pill.warning i{background:#f59e0b}.level-pill.critical i{background:#ef4444}.event-value{font-variant-numeric:tabular-nums}.event-empty{display:flex;min-height:250px;align-items:center;justify-content:center;flex-direction:column;color:#8a98ad}.event-empty span{display:grid;width:46px;height:46px;place-items:center;border-radius:50%;background:#ecfdf5;color:#10b981;font-size:22px}.event-empty strong{margin-top:12px;color:#3a4961}.event-empty small{margin-top:5px}.pagination{padding:14px 16px}.detail-hero{display:grid;grid-template-columns:auto 1fr;gap:6px 12px;padding:18px;border-radius:12px;background:#f7f9fd;border-left:4px solid #3b82f6}.detail-hero.warning{border-left-color:#f59e0b}.detail-hero.critical{border-left-color:#ef4444}.detail-hero span{font-size:12px}.detail-hero strong{font-size:18px}.detail-hero small{grid-column:1/-1;color:#75839a}.event-detail dl{display:grid;grid-template-columns:1fr 1fr;gap:1px;margin-top:18px;background:#e8edf5}.event-detail dl div{min-width:0;padding:13px;background:#fff}.event-detail .wide{grid-column:1/-1}.event-detail dt{margin-bottom:5px;color:#7a889d;font-size:12px}.event-detail dd{overflow-wrap:anywhere;margin:0;color:#1c2b43}.event-detail .critical{background:#fff5f5}.event-detail .warning{background:#fffaf0}@media(max-width:1280px){.event-filter-bar{grid-template-columns:2fr 1.2fr 1fr 1fr}.event-filter-bar :deep(.el-date-editor){grid-column:1/3;width:100%}.event-summary{grid-template-columns:repeat(2,1fr)}}@media(max-width:760px){.event-summary{grid-template-columns:1fr 1fr}.source-filter-tip,.source-filter-tip div{align-items:flex-start;flex-direction:column}.event-filter-bar{display:flex;flex-direction:column}.event-filter-bar>*{width:100%!important}.event-detail dl{grid-template-columns:1fr}}
</style>
