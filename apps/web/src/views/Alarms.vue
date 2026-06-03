<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface AlarmItem {
  id: string;
  type: string;
  level: string;
  message: string;
  status: string;
  value?: number;
  threshold?: number;
  createdAt: string;
  resolvedAt?: string;
  device: { deviceKey: string; name: string };
  tenant: { name: string };
}

const userStore = useUserStore();
const loading = ref(false);
const alarms = ref<AlarmItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, status: '' });
const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const canManage = computed(() => userStore.userInfo?.role !== 'TENANT_USER');

async function loadAlarms() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: AlarmItem[]; total: number }>({ url: '/api/alarms', method: 'GET', params: query });
    alarms.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

async function resolveAlarm(alarm: AlarmItem) {
  await apiRequest({ url: `/api/alarms/${alarm.id}/resolve`, method: 'PATCH' });
  ElMessage.success('告警已处理');
  await loadAlarms();
}

function typeLabel(type: string) {
  return { TEMPERATURE_HIGH: '温度过高', HUMIDITY_HIGH: '湿度过高', BATTERY_LOW: '电量过低' }[type] ?? type;
}

onMounted(loadAlarms);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">告警管理</h1>
        <p class="page-description">查看设备遥测触发的异常记录与恢复状态</p>
      </div>
    </div>
    <section class="table-panel">
      <div class="table-toolbar">
        <el-select v-model="query.status" clearable placeholder="全部状态" style="width: 180px" @change="loadAlarms">
          <el-option label="未处理" value="OPEN" />
          <el-option label="已恢复" value="RESOLVED" />
        </el-select>
      </div>
      <el-table v-loading="loading" :data="alarms">
        <el-table-column label="告警类型" width="120"><template #default="{ row }">{{ typeLabel(row.type) }}</template></el-table-column>
        <el-table-column prop="device.name" label="设备名称" min-width="150" />
        <el-table-column prop="device.deviceKey" label="设备编号" min-width="140" />
        <el-table-column v-if="isSuperAdmin" prop="tenant.name" label="所属租户" min-width="150" />
        <el-table-column prop="message" label="告警内容" min-width="260" show-overflow-tooltip />
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'OPEN' ? 'danger' : 'success'">{{ row.status === 'OPEN' ? '未处理' : '已恢复' }}</el-tag></template></el-table-column>
        <el-table-column label="发生时间" width="180"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template></el-table-column>
        <el-table-column v-if="canManage" label="操作" width="90" fixed="right">
          <template #default="{ row }"><el-button v-if="row.status === 'OPEN'" link type="primary" @click="resolveAlarm(row)">处理</el-button><span v-else class="muted">-</span></template>
        </el-table-column>
      </el-table>
      <div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadAlarms" /></div>
    </section>
  </div>
</template>
