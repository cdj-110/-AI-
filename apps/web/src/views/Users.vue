<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface UserItem {
  id: string;
  tenantId: string | null;
  username: string;
  nickname?: string;
  phone?: string;
  email?: string;
  role: string;
  status: string;
}

interface TenantItem {
  id: string;
  name: string;
}

const userStore = useUserStore();
const loading = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const users = ref<UserItem[]>([]);
const tenants = ref<TenantItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '' });
const form = reactive({
  username: '',
  password: '',
  nickname: '',
  phone: '',
  email: '',
  tenantId: '',
  role: 'TENANT_USER',
  status: 'ACTIVE',
});

const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const roleOptions = computed(() => isSuperAdmin.value
  ? [
      { value: 'SUPER_ADMIN', label: '超级管理员' },
      { value: 'TENANT_ADMIN', label: '租户管理员' },
      { value: 'TENANT_USER', label: '普通用户' },
    ]
  : [{ value: 'TENANT_USER', label: '普通用户' }]);
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{
    validator: (_rule, value, callback) => {
      if (!editingId.value && !value) return callback(new Error('请输入密码'));
      if (value && value.length < 6) return callback(new Error('密码至少 6 位'));
      callback();
    },
    trigger: 'blur',
  }],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }],
};

async function loadUsers() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: UserItem[]; total: number }>({
      url: '/api/users',
      method: 'GET',
      params: query,
    });
    users.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

async function loadTenants() {
  if (!isSuperAdmin.value) return;
  tenants.value = await apiRequest<TenantItem[]>({ url: '/api/tenants', method: 'GET' });
}

function resetForm() {
  Object.assign(form, {
    username: '',
    password: '',
    nickname: '',
    phone: '',
    email: '',
    tenantId: '',
    role: 'TENANT_USER',
    status: 'ACTIVE',
  });
}

function openCreate() {
  editingId.value = '';
  resetForm();
  dialogVisible.value = true;
}

function openEdit(user: UserItem) {
  editingId.value = user.id;
  Object.assign(form, { ...user, password: '', tenantId: user.tenantId ?? '' });
  dialogVisible.value = true;
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  const data = { ...form, tenantId: form.tenantId || undefined, password: form.password || undefined };
  if (editingId.value) {
    await apiRequest({ url: `/api/users/${editingId.value}`, method: 'PATCH', data });
  } else {
    await apiRequest({ url: '/api/users', method: 'POST', data });
  }
  ElMessage.success(editingId.value ? '用户已更新' : '用户已创建');
  dialogVisible.value = false;
  await loadUsers();
}

async function remove(user: UserItem) {
  await ElMessageBox.confirm(`确定删除用户“${user.username}”吗？`, '删除确认', { type: 'warning' });
  await apiRequest({ url: `/api/users/${user.id}`, method: 'DELETE' });
  ElMessage.success('用户已删除');
  await loadUsers();
}

function roleLabel(role: string) {
  return { SUPER_ADMIN: '超级管理员', TENANT_ADMIN: '租户管理员', TENANT_USER: '普通用户' }[role] ?? role;
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadTenants()]);
});
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">用户管理</h1>
        <p class="page-description">管理平台账号、角色和启用状态</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增用户</el-button>
    </div>
    <section class="table-panel">
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索用户名或昵称" style="max-width: 300px" @keyup.enter="loadUsers">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="loadUsers">查询</el-button>
      </div>
      <el-table v-loading="loading" :data="users">
        <el-table-column prop="username" label="用户名" min-width="130" />
        <el-table-column prop="nickname" label="昵称" min-width="120" />
        <el-table-column label="角色" min-width="120">
          <template #default="{ row }"><el-tag effect="plain">{{ roleLabel(row.role) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" min-width="90">
          <template #default="{ row }"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'" effect="light">{{ row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" min-width="140" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadUsers" />
      </div>
    </section>
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑用户' : '新增用户'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="86px">
        <el-form-item label="用户名" prop="username"><el-input v-model="form.username" /></el-form-item>
        <el-form-item label="密码" prop="password"><el-input v-model="form.password" type="password" show-password :placeholder="editingId ? '留空表示不修改' : '至少 6 位'" /></el-form-item>
        <el-form-item label="昵称"><el-input v-model="form.nickname" /></el-form-item>
        <el-form-item label="手机号"><el-input v-model="form.phone" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item v-if="isSuperAdmin" label="所属租户"><el-select v-model="form.tenantId" clearable placeholder="超级管理员可留空" class="full-width"><el-option v-for="tenant in tenants" :key="tenant.id" :label="tenant.name" :value="tenant.id" /></el-select></el-form-item>
        <el-form-item label="角色" prop="role"><el-select v-model="form.role" class="full-width"><el-option v-for="option in roleOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="ACTIVE">启用</el-radio><el-radio value="DISABLED">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" @click="submit">保存</el-button></template>
    </el-dialog>
  </div>
</template>
