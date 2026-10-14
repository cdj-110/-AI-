<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { ArrowRight, Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { useRoute, useRouter } from 'vue-router';
import { apiRequest } from '../api/request';
import { userDisplayName } from '../utils/user-display-name';

interface TenantItem {
  id: string;
  name: string;
  status: string;
  deviceCloudAccessDisabled: boolean;
  createdAt: string;
  _count: { users: number; devices: number };
  subscriptionPlan?: { id: string; name: string; code: string } | null;
  primaryAccounts: Pick<TenantAccount, 'id' | 'username' | 'nickname' | 'email' | 'phone' | 'status'>[];
}

interface TenantAccount {
  id: string;
  username: string;
  nickname: string | null;
  email: string | null;
  phone: string | null;
  role: string;
  status: string;
  createdAt: string;
  lastLoginAt: string | null;
}

const accountsVisible = ref(false);
const accountsTenant = ref<TenantItem | null>(null);
const accountsLoading = ref(false);
const accountsFailed = ref(false);
const accounts = ref<TenantAccount[]>([]);
const accountsTotal = ref(0);
const accountsQuery = reactive({ page: 1, pageSize: 10, keyword: '', role: '' });
let accountsController: AbortController | undefined;

async function loadAccounts() {
  accountsController?.abort();
  if (!accountsTenant.value || !accountsVisible.value) return;
  const controller = new AbortController();
  accountsController = controller;
  accountsLoading.value = true;
  accountsFailed.value = false;
  accounts.value = [];
  try {
    const result = await apiRequest<{ items: TenantAccount[]; total: number }>({
      url: `/api/tenants/${accountsTenant.value.id}/accounts`, method: 'GET',
      params: { ...accountsQuery, role: accountsQuery.role || undefined },
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    accounts.value = result.items;
    accountsTotal.value = result.total;
  } catch {
    if (!controller.signal.aborted) accountsFailed.value = true;
  } finally {
    if (!controller.signal.aborted) accountsLoading.value = false;
  }
}

function openAccounts(tenant: TenantItem) {
  accountsTenant.value = tenant;
  accountsTotal.value = 0;
  Object.assign(accountsQuery, { page: 1, pageSize: 10, keyword: '', role: '' });
  accountsVisible.value = true;
  void loadAccounts();
}

function searchAccounts() {
  accountsQuery.page = 1;
  void loadAccounts();
}

watch(accountsVisible, (visible) => { if (!visible) accountsController?.abort(); });
onBeforeUnmount(() => accountsController?.abort());

const loading = ref(false);
const route = useRoute();
const router = useRouter();
const dialogVisible = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const tenants = ref<TenantItem[]>([]);
const form = reactive({ name: '', status: 'ACTIVE', deviceCloudAccessDisabled: false });
const saving = ref(false);
watch(() => form.status, (status) => { if (status === 'ACTIVE') form.deviceCloudAccessDisabled = false; });
const rules: FormRules = { name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }] };
const selectedTenantId = ref(String(route.query.tenantId || ''));
const filteredTenants = computed(() => selectedTenantId.value
  ? tenants.value.filter((tenant) => tenant.id === selectedTenantId.value)
  : tenants.value);
const selectedTenantName = computed(() => filteredTenants.value[0]?.name || String(route.query.tenantName || ''));

async function loadTenants() {
  loading.value = true;
  try {
    tenants.value = await apiRequest<TenantItem[]>({ url: '/api/tenants', method: 'GET' });
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingId.value = '';
  Object.assign(form, { name: '', status: 'ACTIVE', deviceCloudAccessDisabled: false });
  dialogVisible.value = true;
}

function openEdit(tenant: TenantItem) {
  editingId.value = tenant.id;
  Object.assign(form, { name: tenant.name, status: tenant.status, deviceCloudAccessDisabled: tenant.deviceCloudAccessDisabled === true });
  dialogVisible.value = true;
}

async function submit() {
  if (saving.value) return;
  if (!(await formRef.value?.validate())) return;
  saving.value = true;
  try {
    if (form.status === 'DISABLED') {
      const current = tenants.value.find(tenant => tenant.id === editingId.value);
      if (current?.status !== 'DISABLED' || current?.deviceCloudAccessDisabled !== form.deviceCloudAccessDisabled) {
        try {
          await ElMessageBox.confirm(form.deviceCloudAccessDisabled
            ? `停用“${form.name}”后，全部成员将无法访问平台，同时禁止全部设备云连接、上报和远程操作。本地运行不受影响。`
            : `停用“${form.name}”后，全部成员将无法访问平台；设备云连接继续按各设备原有权限运行。`,
          '确认停用范围', { type: 'warning', confirmButtonText: '确认保存', cancelButtonText: '取消' });
        } catch { return; }
      }
    }
    const result = await apiRequest<{ disconnectResults?: Array<{ result: string }> }>({
      url: editingId.value ? `/api/tenants/${editingId.value}` : '/api/tenants',
      method: editingId.value ? 'PATCH' : 'POST', data: { ...form }, timeout: 120000,
    });
    if (result.disconnectResults?.some(item => item.result === 'unavailable')) {
      ElMessage.warning('限制已保存，但部分设备的现有连接未能主动断开，请检查连接服务后重新保存重试。');
    } else ElMessage.success(editingId.value ? '租户已更新' : '租户已创建');
    dialogVisible.value = false;
    await loadTenants();
  } finally { saving.value = false; }
}

async function remove(tenant: TenantItem) {
  await ElMessageBox.confirm(
    `删除租户“${tenant.name}”会同时删除其 ${tenant._count.users} 个账号和 ${tenant._count.devices} 台设备，确定继续吗？`,
    '删除租户',
    { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' },
  );
  await apiRequest({ url: `/api/tenants/${tenant.id}`, method: 'DELETE' });
  ElMessage.success('租户已删除');
  await loadTenants();
}

function openTenantDevices(tenant: TenantItem) {
  void router.push({ path: '/devices', query: { tenantId: tenant.id, tenantName: tenant.name } });
}

function clearTenantFilter() {
  selectedTenantId.value = '';
  void router.replace({ path: '/tenants' });
}

watch(() => route.query.tenantId, (value) => { selectedTenantId.value = String(value || ''); });

onMounted(loadTenants);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">租户管理</h1>
        <p class="page-description">维护租户主体、启用状态和资源归属，查看租户主账号与子账号信息</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增租户</el-button>
    </div>
    <section class="table-panel">
      <div v-if="selectedTenantId" class="tenant-filter-context">
        <div><span>当前定位租户</span><strong>{{ selectedTenantName || selectedTenantId }}</strong></div>
        <el-button text type="primary" @click="clearTenantFilter">查看全部租户</el-button>
      </div>
      <div class="tenant-scope-note">
        <strong>组织与账号</strong>
        <span>租户名称代表公司或组织；主账号为租户管理员。点击“查看账号”可查看该租户的全部成员。</span>
      </div>
      <el-table v-loading="loading" :data="filteredTenants" empty-text="未找到指定租户">
        <el-table-column prop="name" label="租户名称（组织）" min-width="220" />
        <el-table-column label="主账号" min-width="240">
          <template #default="{ row }">
            <div v-for="account in row.primaryAccounts" :key="account.id" class="primary-account">
              <div class="primary-account-heading">
                <span class="primary-account-name" :title="userDisplayName(account)">{{ userDisplayName(account) }}</span>
                <el-tag v-if="account.status !== 'ACTIVE'" size="small" type="info">停用</el-tag>
              </div>
              <span class="primary-account-contact" :title="account.email || account.phone || '未绑定邮箱或手机号'">{{ account.email || account.phone || '未绑定邮箱或手机号' }}</span>
            </div>
            <span v-if="!row.primaryAccounts?.length" class="primary-account-contact">暂无主账号</span>
          </template>
        </el-table-column>
        <el-table-column label="账号数" width="145">
          <template #default="{ row }">
            <el-button link type="primary" class="device-count-link" @click="openAccounts(row)">
              <strong>{{ row._count.users }}</strong><span>查看账号</span><el-icon><ArrowRight /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="设备数" width="140">
          <template #default="{ row }">
            <el-button link type="primary" class="device-count-link" @click="openTenantDevices(row)">
              <strong>{{ row._count.devices }}</strong><span>查看设备</span><el-icon><ArrowRight /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" min-width="130"><template #default="{ row }"><el-tag v-if="row.subscriptionPlan" type="primary" effect="plain">{{ row.subscriptionPlan.name }}</el-tag><span v-else>未配置</span></template></el-table-column>
        <el-table-column label="状态" min-width="175"><template #default="{ row }"><el-tooltip :content="row.deviceCloudAccessDisabled ? '全部账号访问和设备云连接均已停用' : row.status === 'DISABLED' ? '仅停用账号访问，设备云连接不受租户停用影响' : '设备云连接按各设备权限运行'"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">{{ row.status === 'ACTIVE' ? '启用' : row.deviceCloudAccessDisabled ? '停用（含设备云连接）' : '停用（仅账号访问）' }}</el-tag></el-tooltip></template></el-table-column>
        <el-table-column label="创建时间" min-width="180"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template></el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
    </section>
    <el-drawer v-model="accountsVisible" :title="`${accountsTenant?.name || '租户'} · 账号信息`" size="min(1060px, 96vw)">
      <p class="accounts-description">用户名称用于识别成员，邮箱或手机号用于登录。主账号为租户管理员，子账号为租户成员。</p>
      <el-alert v-if="accountsTenant?.status !== 'ACTIVE'" title="该租户已停用，其账号即使显示启用，也无法登录租户平台。" type="warning" :closable="false" show-icon class="accounts-status-note" />
      <div class="accounts-toolbar">
        <el-input v-model="accountsQuery.keyword" maxlength="100" clearable placeholder="搜索名称、邮箱或手机号" @keyup.enter="searchAccounts" @clear="searchAccounts">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="accountsQuery.role" aria-label="账号类型" @change="searchAccounts">
          <el-option label="全部账号" value="" />
          <el-option label="主账号（租户管理员）" value="TENANT_ADMIN" />
          <el-option label="子账号（租户成员）" value="TENANT_USER" />
        </el-select>
        <el-button @click="searchAccounts">查询</el-button>
      </div>
      <el-alert v-if="accountsFailed" title="账号信息加载失败，请重试" type="error" :closable="false" class="accounts-status-note">
        <el-button link type="primary" @click="loadAccounts">重新加载</el-button>
      </el-alert>
      <el-table v-loading="accountsLoading" :data="accounts" :empty-text="accountsFailed ? '加载失败' : '暂无符合条件的账号'">
        <el-table-column label="用户名称" min-width="160" show-overflow-tooltip><template #default="{ row }">{{ userDisplayName(row) }}</template></el-table-column>
        <el-table-column label="账号类型" min-width="160"><template #default="{ row }"><el-tag :type="row.role === 'TENANT_ADMIN' ? 'primary' : 'info'" effect="plain">{{ row.role === 'TENANT_ADMIN' ? '主账号 · 租户管理员' : '子账号 · 租户成员' }}</el-tag></template></el-table-column>
        <el-table-column label="登录邮箱" min-width="200" show-overflow-tooltip><template #default="{ row }">{{ row.email || '未绑定' }}</template></el-table-column>
        <el-table-column label="登录手机号" min-width="150"><template #default="{ row }">{{ row.phone || '未绑定' }}</template></el-table-column>
        <el-table-column label="账号状态" width="95"><template #default="{ row }"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">{{ row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="注册时间" min-width="180"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template></el-table-column>
        <el-table-column label="最近登录" min-width="180"><template #default="{ row }">{{ row.lastLoginAt ? new Date(row.lastLoginAt).toLocaleString() : '暂无登录记录' }}</template></el-table-column>
      </el-table>
      <div class="pagination">
        <el-pagination v-model:current-page="accountsQuery.page" :page-size="accountsQuery.pageSize" :total="accountsTotal" layout="total, prev, pager, next" @current-change="loadAccounts" />
      </div>
    </el-drawer>
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑租户' : '新增租户'" width="520px" :close-on-click-modal="!saving" :show-close="!saving" :close-on-press-escape="!saving">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="86px" :disabled="saving">
        <el-form-item label="租户名称" prop="name"><el-input v-model="form.name" placeholder="请输入公司或组织名称" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="ACTIVE">启用</el-radio><el-radio value="DISABLED">停用</el-radio></el-radio-group></el-form-item>
        <el-form-item v-if="form.status === 'DISABLED'" label="停用范围">
          <el-checkbox v-model="form.deviceCloudAccessDisabled">同时停用全部设备云连接</el-checkbox>
          <p class="policy-hint">不勾选：仅禁止全部成员访问平台，设备云连接保持原有权限。勾选：同时停止设备上报和远程操作，本地运行不受影响。</p>
        </el-form-item>
        <el-alert v-if="form.status === 'ACTIVE' && tenants.find(tenant => tenant.id === editingId)?.deviceCloudAccessDisabled" title="启用后解除租户级云连接限制；原本单独禁用的设备仍保持禁用。" type="info" :closable="false" />
      </el-form>
      <template #footer><el-button :disabled="saving" @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.policy-hint { margin: 4px 0 0; font-size: 12px; line-height: 1.7; color: #64748b; }
.primary-account { display: flex; align-items: center; gap: 12px; min-width: 0; padding: 3px 0; white-space: nowrap; }
.primary-account + .primary-account { margin-top: 7px; padding-top: 9px; border-top: 1px solid #edf1f6; }
.primary-account-heading { display: flex; align-items: center; gap: 8px; flex: 0 1 auto; min-width: 0; max-width: 45%; }
.primary-account-heading .el-tag { flex-shrink: 0; }
.primary-account-name { color: #253047; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.primary-account-contact { min-width: 0; color: #7b8799; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.accounts-description { margin: 0 0 18px; color: #64748b; font-size: 13px; }
.accounts-status-note { margin-bottom: 16px; }
.accounts-toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 18px; }
.accounts-toolbar .el-input { width: 280px; max-width: 100%; }
.accounts-toolbar .el-select { width: 205px; max-width: 100%; }
.tenant-scope-note { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; padding: 12px 14px; border: 1px solid #e2e8f0; border-radius: 10px; background: #f8fafc; color: #64748b; font-size: 13px; }
.tenant-scope-note strong { color: #334155; font-size: 14px; }
.tenant-filter-context { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; padding: 12px 14px; border: 1px solid #bfdbfe; border-radius: 10px; background: #f5f9ff; }
.tenant-filter-context > div { display: flex; align-items: center; gap: 10px; }
.tenant-filter-context span { color: #64748b; font-size: 13px; }
.tenant-filter-context strong { color: #185fae; font-size: 14px; }
.device-count-link { margin-left: 0; }
.device-count-link span { margin-left: 6px; color: #64748b; font-size: 12px; font-weight: 400; }
.device-count-link .el-icon { margin-left: 2px; }
@media (max-width: 720px) { .tenant-scope-note { align-items: flex-start; flex-direction: column; gap: 4px; } }
</style>
