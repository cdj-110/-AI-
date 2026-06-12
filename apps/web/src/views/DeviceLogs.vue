<script setup lang="ts">
import { RefreshRight, Search } from '@element-plus/icons-vue';
import { onMounted, reactive, ref } from 'vue';
import { apiRequest } from '../api/request';

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
  tenant?: { id: string; name: string };
  device?: { id: string; name: string; deviceKey: string; status: string; deviceType: string };
}

const loading = ref(false);
const logs = ref<DeviceLogItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '', type: '', level: '', source: '' });

const typeOptions = [
  { label: '上线', value: 'ONLINE' },
  { label: '离线', value: 'OFFLINE' },
  { label: 'MQTT 连接', value: 'MQTT_CONNECTED' },
  { label: 'MQTT 断开', value: 'MQTT_DISCONNECTED' },
];
const levelOptions = ['INFO', 'WARNING', 'ERROR'];
const sourceOptions = [
  { label: 'MQTT Broker', value: 'MQTT_BROKER' },
  { label: '心跳', value: 'HEARTBEAT' },
  { label: '遥测', value: 'TELEMETRY' },
  { label: '离线扫描', value: 'OFFLINE_SCAN' },
  { label: '接口上报', value: 'API_REPORT' },
];

async function loadLogs() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: DeviceLogItem[]; total: number }>({
      url: '/api/device-logs',
      method: 'GET',
      params: query,
    });
    logs.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

async function searchLogs() {
  query.page = 1;
  await loadLogs();
}

function formatDetail(detail?: Record<string, unknown>) {
  if (!detail) return '-';
  return JSON.stringify(detail);
}

function typeLabel(type: string) {
  return typeOptions.find((item) => item.value === type)?.label ?? type;
}

function typeTag(type: string) {
  if (type === 'ONLINE' || type === 'MQTT_CONNECTED') return 'success';
  if (type === 'OFFLINE' || type === 'MQTT_DISCONNECTED') return 'info';
  return 'warning';
}

onMounted(loadLogs);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">设备日志</h1>
        <p class="page-description">记录设备上线、离线、MQTT 连接和离线扫描等链路事件，方便排查设备状态变化。</p>
      </div>
      <el-button :icon="RefreshRight" @click="loadLogs">刷新</el-button>
    </div>

    <section class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索设备名称、标识或日志内容" style="max-width: 300px" @keyup.enter="searchLogs">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.type" clearable placeholder="全部类型" style="width: 140px" @change="searchLogs">
          <el-option v-for="item in typeOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-select v-model="query.source" clearable placeholder="全部来源" style="width: 150px" @change="searchLogs">
          <el-option v-for="item in sourceOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-select v-model="query.level" clearable placeholder="全部级别" style="width: 130px" @change="searchLogs">
          <el-option v-for="item in levelOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-button @click="searchLogs">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="logs" empty-text="暂无设备日志">
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column label="设备" min-width="180">
          <template #default="{ row }">
            <strong>{{ row.deviceName || row.device?.name || '-' }}</strong>
            <small class="muted"> {{ row.deviceKey }}</small>
          </template>
        </el-table-column>
        <el-table-column prop="tenant.name" label="所属租户" min-width="150" />
        <el-table-column label="类型" width="120">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)">{{ typeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source" label="来源" width="130" />
        <el-table-column prop="level" label="级别" width="100" />
        <el-table-column prop="message" label="内容" min-width="220" show-overflow-tooltip />
        <el-table-column label="详情" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">{{ formatDetail(row.detail) }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadLogs" />
      </div>
    </section>
  </div>
</template>

<style scoped>
.muted { display: block; margin-top: 3px; color: #94a3b8; font-size: 12px; }
</style>
