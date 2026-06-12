<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { RefreshRight, Search } from '@element-plus/icons-vue';
import { apiRequest } from '../api/request';

interface OperationLogItem {
  id: string;
  username: string;
  module: string;
  action: string;
  targetType: string;
  targetId?: string;
  targetName?: string;
  ip?: string;
  userAgent?: string;
  detail?: Record<string, unknown>;
  createdAt: string;
  tenant?: { id: string; name: string };
  user?: { id: string; username: string; nickname?: string; role: string };
}

const loading = ref(false);
const logs = ref<OperationLogItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '', module: '', action: '' });

const moduleOptions = ['设备管理', '用户管理', '告警管理', '告警规则'];
const actionOptions = ['创建设备', '编辑设备', '删除设备', '修改物模型字段', '导入物模型', '保存告警规则', '修改告警规则', '删除告警规则', '创建用户', '编辑用户', '删除用户', '处理告警', '重置MQTT密码'];

async function loadLogs() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: OperationLogItem[]; total: number }>({
      url: '/api/operation-logs',
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

onMounted(loadLogs);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">操作日志</h1>
        <p class="page-description">记录关键管理操作，方便追踪是谁在什么时候修改了什么</p>
      </div>
      <el-button :icon="RefreshRight" @click="loadLogs">刷新</el-button>
    </div>

    <section class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索操作者、对象或 IP" style="max-width: 300px" @keyup.enter="searchLogs">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.module" clearable placeholder="全部模块" style="width: 140px" @change="searchLogs">
          <el-option v-for="item in moduleOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select v-model="query.action" clearable placeholder="全部动作" style="width: 160px" @change="searchLogs">
          <el-option v-for="item in actionOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-button @click="searchLogs">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="logs" empty-text="暂无操作日志">
        <el-table-column label="操作时间" width="180">
          <template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="username" label="操作者" min-width="120" />
        <el-table-column prop="tenant.name" label="所属租户" min-width="150" />
        <el-table-column prop="module" label="模块" width="110" />
        <el-table-column prop="action" label="动作" min-width="130" />
        <el-table-column label="对象" min-width="180">
          <template #default="{ row }">
            <strong>{{ row.targetName || row.targetId || '-' }}</strong>
            <small class="muted"> {{ row.targetType }}</small>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" min-width="130" />
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
.muted { color: #94a3b8; font-size: 12px; }
</style>
