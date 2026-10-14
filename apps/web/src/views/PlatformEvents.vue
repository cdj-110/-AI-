<script setup lang="ts">
import { RefreshRight, Search } from '@element-plus/icons-vue';
import { onMounted, reactive, ref } from 'vue';
import { apiRequest } from '../api/request';

interface PlatformEventItem {
  id: string;
  deviceKey: string;
  deviceName?: string;
  type: string;
  level: string;
  source: string;
  message: string;
  createdAt: string;
  tenant?: { id: string; name: string };
}

const now = new Date();
const oneDayAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000);
const loading = ref(false);
const events = ref<PlatformEventItem[]>([]);
const total = ref(0);
const dateRange = ref<[Date, Date]>([oneDayAgo, now]);
const query = reactive({ page: 1, pageSize: 20, keyword: '', type: '', level: '' });

const typeOptions = [
  { label: '设备上线', value: 'ONLINE' },
  { label: '设备离线', value: 'OFFLINE' },
  { label: 'MQTT 已连接', value: 'MQTT_CONNECTED' },
  { label: 'MQTT 已断开', value: 'MQTT_DISCONNECTED' },
];

async function loadEvents(resetPage = false) {
  if (resetPage) query.page = 1;
  loading.value = true;
  try {
    const data = await apiRequest<{ items: PlatformEventItem[]; total: number }>({
      url: '/api/device-logs',
      method: 'GET',
      params: {
        ...query,
        platformOnly: true,
        startAt: dateRange.value?.[0]?.toISOString(),
        endAt: dateRange.value?.[1]?.toISOString(),
      },
    });
    events.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

function typeLabel(type: string) {
  return typeOptions.find((item) => item.value === type)?.label ?? type;
}

function typeTag(type: string) {
  if (type === 'ONLINE' || type === 'MQTT_CONNECTED') return 'success';
  return type === 'OFFLINE' ? 'danger' : 'warning';
}

function formatTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false });
}

onMounted(() => void loadEvents());
</script>

<template>
  <div class="platform-events-page">
    <div class="page-actions">
      <div>
        <h1 class="page-title">平台事件</h1>
        <p class="page-description">仅展示网关上下线和连接状态等平台运维事件，不在此加载租户点位告警。</p>
      </div>
      <el-button :icon="RefreshRight" :loading="loading" @click="loadEvents(false)">刷新</el-button>
    </div>

    <section class="scope-notice">
      <div><strong>默认范围：近 24 小时</strong><span>事件按页加载，避免租户和设备数量增长后产生全量查询。</span></div>
      <span class="result-count">{{ total }} 条</span>
    </section>

    <section class="table-panel event-panel">
      <div class="table-toolbar event-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索设备、标识或事件内容" class="keyword-input" @keyup.enter="loadEvents(true)">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.type" clearable placeholder="全部事件类型" @change="loadEvents(true)">
          <el-option v-for="item in typeOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-select v-model="query.level" clearable placeholder="全部级别" @change="loadEvents(true)">
          <el-option label="提示" value="INFO" /><el-option label="警告" value="WARNING" /><el-option label="错误" value="ERROR" />
        </el-select>
        <el-date-picker v-model="dateRange" type="datetimerange" range-separator="至" start-placeholder="开始时间" end-placeholder="结束时间" @change="loadEvents(true)" />
        <el-button type="primary" @click="loadEvents(true)">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="events" empty-text="当前时间范围内暂无平台事件">
        <el-table-column label="发生时间" width="180"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="事件" width="132"><template #default="{ row }"><el-tag :type="typeTag(row.type)">{{ typeLabel(row.type) }}</el-tag></template></el-table-column>
        <el-table-column label="网关" min-width="210">
          <template #default="{ row }"><div class="device-cell"><strong>{{ row.deviceName || row.deviceKey }}</strong><span>{{ row.deviceKey }}</span></div></template>
        </el-table-column>
        <el-table-column prop="tenant.name" label="所属租户" min-width="160" />
        <el-table-column prop="source" label="来源" width="140" />
        <el-table-column prop="message" label="事件内容" min-width="280" show-overflow-tooltip />
      </el-table>
      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :page-sizes="[20, 50, 100]" :total="total" @current-change="loadEvents(false)" @size-change="loadEvents(true)" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.platform-events-page{display:flex;flex-direction:column;gap:18px}.scope-notice{display:flex;align-items:center;justify-content:space-between;padding:14px 18px;border:1px solid #dce8f8;border-radius:12px;background:#f7faff}.scope-notice div{display:flex;gap:8px;align-items:center}.scope-notice strong{color:#25466f;font-size:13px}.scope-notice span{color:#71809a;font-size:12px}.scope-notice .result-count{padding:4px 10px;border-radius:999px;color:#2563eb;background:#eaf2ff;font-weight:700}.event-panel{overflow:hidden;padding:0}.event-toolbar{display:grid;grid-template-columns:minmax(260px,1.5fr) 170px 130px minmax(330px,1.4fr) auto;gap:10px;padding:16px;border-bottom:1px solid #e8eef7;background:#fbfdff}.device-cell{display:flex;min-width:0;flex-direction:column;gap:3px}.device-cell strong,.device-cell span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.device-cell span{color:#8a98ad;font-size:12px}.pagination{padding:14px 16px}@media(max-width:1200px){.event-toolbar{grid-template-columns:2fr 1fr 1fr}.event-toolbar :deep(.el-date-editor){grid-column:1/3;width:100%}}@media(max-width:760px){.scope-notice,.scope-notice div{align-items:flex-start;flex-direction:column}.event-toolbar{display:flex;flex-direction:column}.event-toolbar>*{width:100%!important}}
</style>
