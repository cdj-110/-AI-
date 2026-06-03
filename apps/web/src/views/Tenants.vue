<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { Plus } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';

interface TenantItem {
  id: string;
  name: string;
  status: string;
  createdAt: string;
  _count: { users: number };
}

const loading = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const tenants = ref<TenantItem[]>([]);
const form = reactive({ name: '', status: 'ACTIVE' });
const rules: FormRules = { name: [{ required: true, message: '请输入租户名称', trigger: 'blur' }] };

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
  Object.assign(form, { name: '', status: 'ACTIVE' });
  dialogVisible.value = true;
}

function openEdit(tenant: TenantItem) {
  editingId.value = tenant.id;
  Object.assign(form, { name: tenant.name, status: tenant.status });
  dialogVisible.value = true;
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  await apiRequest({ url: editingId.value ? `/api/tenants/${editingId.value}` : '/api/tenants', method: editingId.value ? 'PATCH' : 'POST', data: form });
  ElMessage.success(editingId.value ? '租户已更新' : '租户已创建');
  dialogVisible.value = false;
  await loadTenants();
}

async function remove(tenant: TenantItem) {
  await ElMessageBox.confirm(`删除租户“${tenant.name}”会同时删除其用户，确定继续吗？`, '删除确认', { type: 'warning' });
  await apiRequest({ url: `/api/tenants/${tenant.id}`, method: 'DELETE' });
  ElMessage.success('租户已删除');
  await loadTenants();
}

onMounted(loadTenants);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">租户管理</h1>
        <p class="page-description">维护平台中的组织和账号归属</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">新增租户</el-button>
    </div>
    <section class="table-panel">
      <el-table v-loading="loading" :data="tenants">
        <el-table-column prop="name" label="租户名称" min-width="220" />
        <el-table-column label="用户数" width="110"><template #default="{ row }">{{ row._count.users }}</template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'">{{ row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="创建时间" min-width="180"><template #default="{ row }">{{ new Date(row.createdAt).toLocaleString() }}</template></el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
    </section>
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑租户' : '新增租户'" width="480px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="86px">
        <el-form-item label="租户名称" prop="name"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="ACTIVE">启用</el-radio><el-radio value="DISABLED">停用</el-radio></el-radio-group></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" @click="submit">保存</el-button></template>
    </el-dialog>
  </div>
</template>
