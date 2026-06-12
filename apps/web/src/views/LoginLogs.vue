<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { RefreshRight, Search } from '@element-plus/icons-vue';
import { apiRequest } from '../api/request';

interface LoginLogItem {
  id: string;
  username: string;
  ip: string;
  userAgent?: string;
  success: boolean;
  reason?: string;
  createdAt: string;
  tenant?: { id: string; name: string };
  user?: { id: string; username: string; nickname?: string; role: string };
}

const loading = ref(false);
const logs = ref<LoginLogItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '', success: '' });

async function loadLogs() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: LoginLogItem[]; total: number }>({
      url: '/api/login-logs',
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

function roleLabel(role?: string) {
  return { SUPER_ADMIN: '超级管理员', TENANT_ADMIN: '租户管理员', TENANT_USER: '普通用户' }[role ?? ''] ?? role ?? '-';
}

function shortUserAgent(userAgent?: string) {
  if (!userAgent) return '-';
  return userAgent.length > 90 ? `${userAgent.slice(0, 90)}...` : userAgent;
}

onMounted(loadLogs);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">登录日志</h1>
        <p class="page-description">记录平台账号登录来源，包含 IP、账号、结果和浏览器信息</p>
      </div>
      <el-button :icon="RefreshRight" @click="loadLogs">刷新</el-button>
    </div>

    <section class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索账号、昵称或 IP" style="max-width: 300px" @keyup.enter="searchLogs">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.success" clearable placeholder="全部结果" style="width: 130px" @change="searchLogs">
          <el-option label="登录成功" value="true" />
          <el-option label="登录失败" value="false" />
        </el-select>
        <el-button @click="searchLogs">查询</el-button>
      </div>

      <el-table v-loading="loading" :data="logs" empty-text="暂无登录日志">
        <el-table-column label="登录时间" width="180">
          <template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template>
        </el-table-column>
        <el-table-column prop="username" label="登录账号" min-width="130" />
        <el-table-column label="用户" min-width="150">
          <template #default="{ row }">
            <span>{{ row.user?.nickname || row.user?.username || '-' }}</span>
            <small v-if="row.user" class="muted"> {{ roleLabel(row.user.role) }}</small>
          </template>
        </el-table-column>
        <el-table-column prop="tenant.name" label="所属租户" min-width="150" />
        <el-table-column prop="ip" label="登录 IP" min-width="140" />
        <el-table-column label="结果" width="100">
          <template #default="{ row }">
            <el-tag :type="row.success ? 'success' : 'danger'">{{ row.success ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="失败原因" min-width="120">
          <template #default="{ row }">{{ row.reason || '-' }}</template>
        </el-table-column>
        <el-table-column label="浏览器信息" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">{{ shortUserAgent(row.userAgent) }}</template>
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
