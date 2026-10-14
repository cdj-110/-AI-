<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue';
import { ArrowLeft, ArrowRight, Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';
import { isAdminPortal } from '../portal';
import { userDisplayName } from '../utils/user-display-name';

interface UserItem {
  id: string;
  tenantId: string | null;
  username: string;
  nickname?: string;
  phone?: string;
  email?: string;
  role: string;
  status: string;
  permissions?: string[];
  deviceScopeMode?: 'ALL' | 'CUSTOM';
  deviceIds?: string[];
  groupIds?: string[];
}

interface ScopeGroup { id: string; name: string; _count: { devices: number } }
interface ScopeDevice { id: string; name: string; deviceKey: string; deviceType: string; groupIds: string[] }

interface PermissionNode {
  id: string;
  label: string;
  description?: string;
  kind: 'menu' | 'page' | 'button';
  children?: PermissionNode[];
}

const permissionTree: PermissionNode[] = [
  { id: 'menu-dashboard', label: '数据概览', kind: 'menu', children: [
    { id: 'page-dashboard', label: '概览页面', kind: 'page', children: [
      { id: 'DASHBOARD_VIEW', label: '访问页面', description: '查看设备统计和趋势', kind: 'button' },
    ] },
  ] },
  { id: 'menu-device', label: '设备管理', kind: 'menu', children: [
    { id: 'page-device-list', label: '设备列表', kind: 'page', children: [
      { id: 'DEVICE_VIEW', label: '访问页面', description: '查看设备、点位和历史数据', kind: 'button' },
      { id: 'DEVICE_CREATE', label: '新增设备', kind: 'button' },
      { id: 'DEVICE_EDIT', label: '编辑设备', kind: 'button' },
      { id: 'DEVICE_DELETE', label: '删除设备', kind: 'button' },
      { id: 'DEVICE_GROUP_MANAGE', label: '管理设备分组', kind: 'button' },
    ] },
  ] },
  { id: 'menu-map', label: '设备地图', kind: 'menu', children: [
    { id: 'page-map', label: '地图页面', kind: 'page', children: [
      { id: 'MAP_VIEW', label: '访问页面', description: '查看设备地理分布', kind: 'button' },
    ] },
  ] },
  { id: 'menu-scada', label: '云组态', kind: 'menu', children: [
    { id: 'page-scada', label: '组态页面', kind: 'page', children: [
      { id: 'SCADA_VIEW', label: '访问页面', kind: 'button' },
    ] },
  ] },
  { id: 'menu-model', label: '物模型管理', kind: 'menu', children: [
    { id: 'page-model-list', label: '物模型页面', kind: 'page', children: [
      { id: 'MODEL_VIEW', label: '访问页面', description: '查看物模型模板', kind: 'button' },
      { id: 'MODEL_CREATE', label: '新增与复制模板', kind: 'button' },
      { id: 'MODEL_EDIT', label: '编辑模板和字段', kind: 'button' },
      { id: 'MODEL_DELETE', label: '删除模板', kind: 'button' },
    ] },
  ] },
  { id: 'menu-alarm', label: '告警管理', kind: 'menu', children: [
    { id: 'page-alarm-list', label: '告警页面', kind: 'page', children: [
      { id: 'ALARM_VIEW', label: '访问页面', description: '查看获授权设备的告警', kind: 'button' },
      { id: 'ALARM_RESOLVE', label: '处理告警', kind: 'button' },
    ] },
  ] },
  { id: 'menu-event', label: '事件中心', kind: 'menu', children: [
    { id: 'page-event-list', label: '事件页面', kind: 'page', children: [
      { id: 'EVENT_VIEW', label: '访问页面', description: '查看获授权设备的事件', kind: 'button' },
    ] },
  ] },
];
const pagePermissions = ['DASHBOARD_VIEW', 'DEVICE_VIEW', 'MAP_VIEW', 'SCADA_VIEW', 'MODEL_VIEW', 'ALARM_VIEW', 'EVENT_VIEW'];
const actionPermissions = ['DEVICE_CREATE', 'DEVICE_EDIT', 'DEVICE_DELETE', 'DEVICE_GROUP_MANAGE', 'MODEL_CREATE', 'MODEL_EDIT', 'MODEL_DELETE', 'ALARM_RESOLVE'];
const allPermissions = [...pagePermissions, ...actionPermissions];
const requiredViewPermission: Record<string, string> = {
  DEVICE_CREATE: 'DEVICE_VIEW', DEVICE_EDIT: 'DEVICE_VIEW', DEVICE_DELETE: 'DEVICE_VIEW', DEVICE_GROUP_MANAGE: 'DEVICE_VIEW',
  MODEL_CREATE: 'MODEL_VIEW', MODEL_EDIT: 'MODEL_VIEW', MODEL_DELETE: 'MODEL_VIEW', ALARM_RESOLVE: 'ALARM_VIEW',
};

const loading = ref(false);
const dialogVisible = ref(false);
const editingId = ref('');
const formRef = ref<FormInstance>();
const permissionTreeRef = ref<{ getCheckedKeys: (leafOnly?: boolean) => unknown[]; setCheckedKeys: (keys: string[]) => void }>();
const activeStep = ref(0);
let syncingPermissionTree = false;
const users = ref<UserItem[]>([]);
const scopeGroups = ref<ScopeGroup[]>([]);
const scopeDevices = ref<ScopeDevice[]>([]);
const availableCheckedIds = ref<string[]>([]);
const selectedCheckedIds = ref<string[]>([]);
const selectedDeviceKeyword = ref('');
const draggedDevice = ref<{ id: string; from: 'available' | 'selected' }>();
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '' });
const form = reactive({
  username: '',
  password: '',
  phone: '',
  email: '',
  role: isAdminPortal ? 'SUPER_ADMIN' : 'TENANT_USER',
  status: 'ACTIVE',
  permissions: [...pagePermissions],
  deviceScopeMode: 'ALL' as 'ALL' | 'CUSTOM',
  deviceIds: [] as string[],
  groupIds: [] as string[],
  deviceKeyword: '',
});

const visibleScopeDevices = computed(() => {
  const keyword = form.deviceKeyword.trim().toLocaleLowerCase();
  if (!keyword) return scopeDevices.value;
  return scopeDevices.value.filter(device => `${device.name} ${device.deviceKey}`.toLocaleLowerCase().includes(keyword));
});
const groupCoveredDeviceIds = computed(() => new Set(scopeDevices.value
  .filter(device => device.groupIds.some(groupId => form.groupIds.includes(groupId)))
  .map(device => device.id)));
const effectiveDeviceIds = computed(() => new Set([...groupCoveredDeviceIds.value, ...form.deviceIds]));
const effectiveDeviceCount = computed(() => effectiveDeviceIds.value.size);
const availableScopeDevices = computed(() => visibleScopeDevices.value.filter(device => !effectiveDeviceIds.value.has(device.id)));
const selectedScopeDevices = computed(() => {
  const keyword = selectedDeviceKeyword.value.trim().toLocaleLowerCase();
  return scopeDevices.value.filter(device => effectiveDeviceIds.value.has(device.id)
    && (!keyword || `${device.name} ${device.deviceKey}`.toLocaleLowerCase().includes(keyword)));
});
const selectedPageCount = computed(() => form.permissions.filter(permission => pagePermissions.includes(permission)).length);
const selectedActionCount = computed(() => form.permissions.filter(permission => actionPermissions.includes(permission)).length);

const roleOptions = computed(() => isAdminPortal
  ? [{ value: 'SUPER_ADMIN', label: '平台管理员' }]
  : [{ value: 'TENANT_USER', label: '租户成员' }]);
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名称', trigger: 'blur' }, { max: 50, message: '用户名称不能超过 50 个字符', trigger: 'blur' }],
  password: [{
    validator: (_rule, value, callback) => {
      if (!editingId.value && !value) return callback(new Error('请输入密码'));
      if (value && value.length < 6) return callback(new Error('密码至少 6 位'));
      callback();
    },
    trigger: 'blur',
  }],
  phone: [{
    validator: (_rule, _value, callback) => {
      if (!form.phone.trim() && !form.email.trim()) return callback(new Error('邮箱或手机号至少填写一个'));
      callback();
    },
    trigger: ['blur', 'change'],
  }],
  email: [{
    validator: (_rule, value, callback) => {
      if (!String(value || '').trim() && !form.phone.trim()) return callback(new Error('邮箱或手机号至少填写一个'));
      if (value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) return callback(new Error('请输入正确的邮箱地址'));
      callback();
    },
    trigger: ['blur', 'change'],
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

async function loadScopeOptions() {
  if (isAdminPortal) return;
  const data = await apiRequest<{ groups: ScopeGroup[]; devices: ScopeDevice[] }>({ url: '/api/users/device-scope-options', method: 'GET' });
  scopeGroups.value = data.groups;
  scopeDevices.value = data.devices;
}

function resetForm() {
  Object.assign(form, {
    username: '',
    password: '',
    phone: '',
    email: '',
    role: isAdminPortal ? 'SUPER_ADMIN' : 'TENANT_USER',
    status: 'ACTIVE',
    permissions: [...pagePermissions],
    deviceScopeMode: 'ALL',
    deviceIds: [],
    groupIds: [],
    deviceKeyword: '',
  });
  availableCheckedIds.value = [];
  selectedCheckedIds.value = [];
  selectedDeviceKeyword.value = '';
}

async function openCreate() {
  await loadScopeOptions();
  editingId.value = '';
  resetForm();
  activeStep.value = 0;
  dialogVisible.value = true;
  await nextTick();
  setPermissionTreeChecks();
}

async function openEdit(user: UserItem) {
  await loadScopeOptions();
  editingId.value = user.id;
  Object.assign(form, {
    username: userDisplayName(user),
    phone: user.phone || '',
    email: user.email || '',
    role: user.role,
    status: user.status,
    password: '',
    permissions: normalizePermissions(user.permissions ?? pagePermissions),
    deviceScopeMode: user.deviceScopeMode ?? 'ALL',
    deviceIds: [...(user.deviceIds ?? [])],
    groupIds: [...(user.groupIds ?? [])],
    deviceKeyword: '',
  });
  availableCheckedIds.value = [];
  selectedCheckedIds.value = [];
  selectedDeviceKeyword.value = '';
  activeStep.value = 0;
  dialogVisible.value = true;
  await nextTick();
  setPermissionTreeChecks();
}

function addSelectedDevices(ids = availableCheckedIds.value) {
  form.deviceIds = [...new Set([...form.deviceIds, ...ids.filter(id => !groupCoveredDeviceIds.value.has(id))])];
  availableCheckedIds.value = [];
}

function removeSelectedDevices(ids = selectedCheckedIds.value) {
  const removable = new Set(ids.filter(id => !groupCoveredDeviceIds.value.has(id)));
  form.deviceIds = form.deviceIds.filter(id => !removable.has(id));
  selectedCheckedIds.value = [];
}

function handleGroupChange() {
  nextTick(() => {
    availableCheckedIds.value = availableCheckedIds.value.filter(id => !effectiveDeviceIds.value.has(id));
    selectedCheckedIds.value = selectedCheckedIds.value.filter(id => effectiveDeviceIds.value.has(id) && !groupCoveredDeviceIds.value.has(id));
  });
}

function startDeviceDrag(id: string, from: 'available' | 'selected') {
  if (from === 'selected' && groupCoveredDeviceIds.value.has(id)) return;
  draggedDevice.value = { id, from };
}

function dropDevice(target: 'available' | 'selected') {
  const dragging = draggedDevice.value;
  draggedDevice.value = undefined;
  if (!dragging || dragging.from === target) return;
  if (target === 'selected') addSelectedDevices([dragging.id]);
  else removeSelectedDevices([dragging.id]);
}

function normalizePermissions(permissions: readonly string[]) {
  const selected = new Set(permissions.filter(permission => allPermissions.includes(permission)));
  for (const permission of [...selected]) {
    const required = requiredViewPermission[permission];
    if (required) selected.add(required);
  }
  return allPermissions.filter(permission => selected.has(permission));
}

function setPermissionTreeChecks() {
  if (!permissionTreeRef.value) return;
  syncingPermissionTree = true;
  permissionTreeRef.value.setCheckedKeys(form.permissions);
  nextTick(() => { syncingPermissionTree = false; });
}

function syncPermissions() {
  if (syncingPermissionTree || !permissionTreeRef.value) return;
  const normalized = normalizePermissions(permissionTreeRef.value.getCheckedKeys(true).map(String));
  form.permissions = normalized;
  if (normalized.some(permission => !permissionTreeRef.value?.getCheckedKeys(true).map(String).includes(permission))) {
    setPermissionTreeChecks();
  }
}

async function nextStep() {
  if (activeStep.value === 0 && !(await formRef.value?.validate())) return;
  activeStep.value = Math.min(2, activeStep.value + 1);
  if (activeStep.value === 1) await nextTick().then(setPermissionTreeChecks);
}

function searchUsers() {
  query.page = 1;
  loadUsers();
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  if (!isAdminPortal && form.deviceScopeMode === 'CUSTOM' && !form.deviceIds.length && !form.groupIds.length) {
    ElMessage.warning('指定设备范围时，请至少选择一个设备分组或设备');
    return;
  }
  const data = {
    ...form,
    username: form.username.trim(),
    phone: form.phone.trim(),
    email: form.email.trim().toLowerCase(),
    tenantId: isAdminPortal ? null : undefined,
    role: isAdminPortal ? 'SUPER_ADMIN' : 'TENANT_USER',
    password: form.password || undefined,
    permissions: isAdminPortal ? undefined : normalizePermissions(form.permissions),
    deviceScopeMode: isAdminPortal ? undefined : form.deviceScopeMode,
    deviceIds: isAdminPortal || form.deviceScopeMode === 'ALL' ? undefined : form.deviceIds,
    groupIds: isAdminPortal || form.deviceScopeMode === 'ALL' ? undefined : form.groupIds,
  };
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
  await ElMessageBox.confirm(`确定删除用户“${userDisplayName(user)}”吗？`, '删除确认', { type: 'warning' });
  await apiRequest({ url: `/api/users/${user.id}`, method: 'DELETE' });
  ElMessage.success('用户已删除');
  await loadUsers();
}

function roleLabel(role: string) {
  return { SUPER_ADMIN: '平台管理员', TENANT_ADMIN: '租户管理员', TENANT_USER: '租户成员' }[role] ?? role;
}

function permissionSummary(user: UserItem) {
  const permissions = normalizePermissions(user.permissions ?? []);
  const pages = permissions.filter(permission => pagePermissions.includes(permission)).length;
  const actions = permissions.filter(permission => actionPermissions.includes(permission)).length;
  if (!pages && !actions) return '无业务功能权限';
  return `${pages} 个页面${actions ? ` · ${actions} 个操作` : ' · 只读'}`;
}

function deviceScopeSummary(user: UserItem) {
  if (user.deviceScopeMode !== 'CUSTOM') return '全部设备';
  const parts = [];
  if (user.groupIds?.length) parts.push(`${user.groupIds.length} 个分组`);
  if (user.deviceIds?.length) parts.push(`${user.deviceIds.length} 台单独设备`);
  return parts.join(' + ') || '未分配设备';
}

function canEditAccount(user: UserItem) {
  return isAdminPortal || user.role === 'TENANT_USER';
}

onMounted(loadUsers);
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">{{ isAdminPortal ? '平台账号管理' : '成员管理' }}</h1>
        <p class="page-description">{{ isAdminPortal ? '维护公司管理平台用户，使用邮箱或手机号登录' : '维护当前租户成员，使用邮箱或手机号登录' }}</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreate">{{ isAdminPortal ? '新增平台账号' : '新增成员' }}</el-button>
    </div>
    <section class="table-panel">
      <div class="account-scope-note">
        <strong>{{ isAdminPortal ? '管理平台账号' : '租户成员账号' }}</strong>
        <span>{{ isAdminPortal ? '只能登录公司管理平台，不属于任何租户。' : '只在当前租户内生效，不能登录公司管理平台。' }}</span>
      </div>
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索名称、邮箱或手机号" style="max-width: 300px" @keyup.enter="searchUsers">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="searchUsers">查询</el-button>
      </div>
      <el-table v-loading="loading" :data="users">
        <el-table-column label="用户名称" min-width="160" show-overflow-tooltip><template #default="{ row }">{{ userDisplayName(row) }}</template></el-table-column>
        <el-table-column label="角色" min-width="120">
          <template #default="{ row }"><el-tag effect="plain">{{ roleLabel(row.role) }}</el-tag></template>
        </el-table-column>
        <el-table-column label="状态" min-width="90">
          <template #default="{ row }"><el-tag :type="row.status === 'ACTIVE' ? 'success' : 'info'" effect="light">{{ row.status === 'ACTIVE' ? '启用' : '停用' }}</el-tag></template>
        </el-table-column>
        <el-table-column v-if="!isAdminPortal" label="功能权限" min-width="210" show-overflow-tooltip>
          <template #default="{ row }">{{ row.role === 'TENANT_ADMIN' ? '全部功能' : permissionSummary(row) }}</template>
        </el-table-column>
        <el-table-column v-if="!isAdminPortal" label="设备范围" min-width="165">
          <template #default="{ row }"><el-tag :type="row.deviceScopeMode === 'CUSTOM' ? 'warning' : 'success'" effect="plain">{{ row.role === 'TENANT_ADMIN' ? '全部设备' : deviceScopeSummary(row) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="phone" label="登录手机号" min-width="140" />
        <el-table-column prop="email" label="登录邮箱" min-width="180" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <template v-if="canEditAccount(row)">
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" @click="remove(row)">删除</el-button>
            </template>
            <span v-else class="protected-account">租户负责人</span>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination">
        <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadUsers" />
      </div>
    </section>
    <el-dialog v-model="dialogVisible" :title="editingId ? (isAdminPortal ? '编辑平台账号' : '编辑成员') : (isAdminPortal ? '新增平台账号' : '新增成员')" :width="isAdminPortal ? '520px' : 'min(1080px, 96vw)'" class="member-dialog">
      <el-steps v-if="!isAdminPortal" :active="activeStep" finish-status="success" align-center class="member-steps">
        <el-step title="基本信息" description="账号与登录凭证" />
        <el-step title="权限配置" description="菜单、页面、按钮" />
        <el-step title="设备范围" description="分组或单台设备" />
      </el-steps>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="104px">
        <div v-show="isAdminPortal || activeStep === 0" class="wizard-panel basic-panel">
          <div v-if="!isAdminPortal" class="panel-heading"><strong>账号信息</strong><span>邮箱或手机号至少填写一个，作为成员登录凭证。</span></div>
          <el-form-item label="用户名称" prop="username"><el-input v-model.trim="form.username" maxlength="50" show-word-limit placeholder="成员在平台内显示的名称" /></el-form-item>
          <el-form-item label="密码" prop="password"><el-input v-model="form.password" type="password" show-password :placeholder="editingId ? '留空表示不修改' : '至少 6 位'" /></el-form-item>
          <el-form-item label="登录手机号" prop="phone"><el-input v-model="form.phone" placeholder="与登录邮箱至少填写一个" /></el-form-item>
          <el-form-item label="登录邮箱" prop="email"><el-input v-model="form.email" placeholder="与登录手机号至少填写一个" /></el-form-item>
          <el-form-item label="角色" prop="role"><el-select v-model="form.role" class="full-width"><el-option v-for="option in roleOptions" :key="option.value" :label="option.label" :value="option.value" /></el-select></el-form-item>
          <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio value="ACTIVE">启用</el-radio><el-radio value="DISABLED">停用</el-radio></el-radio-group></el-form-item>
        </div>

        <div v-if="!isAdminPortal" v-show="activeStep === 1" class="wizard-panel">
          <div class="panel-heading permission-heading">
            <div><strong>功能权限</strong><span>按“菜单 → 页面 → 按钮”逐级勾选；勾选上级可一次授权整组功能。</span></div>
            <div class="selection-summary"><b>{{ selectedPageCount }}</b> 个页面 · <b>{{ selectedActionCount }}</b> 个操作</div>
          </div>
          <div class="permission-tree-shell">
            <div class="permission-legend"><span>菜单</span><i></i><span>页面</span><i></i><span>按钮</span></div>
            <el-tree
              ref="permissionTreeRef"
              :data="permissionTree"
              :props="{ children: 'children', label: 'label' }"
              node-key="id"
              show-checkbox
              check-on-click-node
              :expand-on-click-node="false"
              :default-expanded-keys="['menu-device']"
              class="permission-tree"
              @check="syncPermissions"
            >
              <template #default="{ data }">
                <div class="permission-node" :class="`is-${data.kind}`">
                  <span class="node-kind">{{ data.kind === 'menu' ? '菜单' : data.kind === 'page' ? '页面' : '按钮' }}</span>
                  <strong>{{ data.label }}</strong>
                  <small v-if="data.description">{{ data.description }}</small>
                </div>
              </template>
            </el-tree>
          </div>
          <el-alert type="info" :closable="false" show-icon title="按钮权限会自动包含所属页面的访问权限，避免出现有操作权但无法进入页面的情况。" />
        </div>

        <div v-if="!isAdminPortal" v-show="activeStep === 2" class="wizard-panel">
          <div class="panel-heading permission-heading">
            <div><strong>设备范围</strong><span>分组授权会自动包含该组以后新增的设备；单独设备与分组按并集计算。</span></div>
            <div v-if="form.deviceScopeMode === 'CUSTOM'" class="selection-summary">覆盖 <b>{{ effectiveDeviceCount }}</b> 台设备</div>
          </div>
          <el-radio-group v-model="form.deviceScopeMode" class="scope-mode">
            <el-radio-button value="ALL">全部设备</el-radio-button>
            <el-radio-button value="CUSTOM">指定范围</el-radio-button>
          </el-radio-group>
          <div v-if="form.deviceScopeMode === 'CUSTOM'" class="scope-assignment">
            <div class="group-scope-bar">
              <div class="scope-column-title"><strong>按设备分组授权</strong><span>已选 {{ form.groupIds.length }} 个</span></div>
              <el-checkbox-group v-model="form.groupIds" class="group-options" @change="handleGroupChange">
                <el-checkbox v-for="group in scopeGroups" :key="group.id" :value="group.id" class="group-option">
                  <span>{{ group.name }}</span><small>{{ group._count.devices }} 台</small>
                </el-checkbox>
                <span v-if="!scopeGroups.length" class="empty-inline">暂无设备分组</span>
              </el-checkbox-group>
            </div>

            <div class="device-transfer">
              <section class="transfer-panel" @dragover.prevent @drop.prevent="dropDevice('available')">
                <div class="transfer-heading"><div><strong>未添加设备</strong><span>拖到右侧或勾选后添加</span></div><b>{{ availableScopeDevices.length }}</b></div>
                <el-input v-model="form.deviceKeyword" clearable placeholder="搜索未添加设备" size="small" :prefix-icon="Search" />
                <el-checkbox-group v-model="availableCheckedIds" class="transfer-list">
                  <div v-for="device in availableScopeDevices" :key="device.id" class="transfer-item" draggable="true" @dragstart="startDeviceDrag(device.id, 'available')" @dragend="draggedDevice = undefined">
                    <el-checkbox :value="device.id" class="device-checkbox" @click.stop>
                      <span class="device-copy"><strong>{{ device.name || device.deviceKey || '未命名设备' }}</strong><small>{{ device.deviceKey }} · {{ device.deviceType }}</small></span>
                    </el-checkbox>
                    <em>拖动</em>
                  </div>
                  <el-empty v-if="!availableScopeDevices.length" description="暂无未添加设备" :image-size="54" />
                </el-checkbox-group>
              </section>

              <div class="transfer-actions">
                <el-button type="primary" circle :icon="ArrowRight" :disabled="!availableCheckedIds.length" aria-label="添加选中设备" @click="addSelectedDevices()" />
                <el-button circle :icon="ArrowLeft" :disabled="!selectedCheckedIds.length" aria-label="移除选中设备" @click="removeSelectedDevices()" />
                <small>支持拖拽</small>
              </div>

              <section class="transfer-panel selected-panel" @dragover.prevent @drop.prevent="dropDevice('selected')">
                <div class="transfer-heading"><div><strong>已添加设备</strong><span>包含分组授权与单独添加</span></div><b>{{ effectiveDeviceCount }}</b></div>
                <el-input v-model="selectedDeviceKeyword" clearable placeholder="搜索已添加设备" size="small" :prefix-icon="Search" />
                <el-checkbox-group v-model="selectedCheckedIds" class="transfer-list">
                  <div v-for="device in selectedScopeDevices" :key="device.id" class="transfer-item" :class="{ 'is-group-covered': groupCoveredDeviceIds.has(device.id) }" :draggable="!groupCoveredDeviceIds.has(device.id)" @dragstart="startDeviceDrag(device.id, 'selected')" @dragend="draggedDevice = undefined">
                    <el-checkbox :value="device.id" :disabled="groupCoveredDeviceIds.has(device.id)" class="device-checkbox" @click.stop>
                      <span class="device-copy"><strong>{{ device.name || device.deviceKey || '未命名设备' }}</strong><small>{{ device.deviceKey }} · {{ device.deviceType }}</small></span>
                    </el-checkbox>
                    <el-tag v-if="groupCoveredDeviceIds.has(device.id)" size="small" effect="plain" class="group-source-tag">{{ form.deviceIds.includes(device.id) ? '分组 + 单独' : '分组包含' }}</el-tag>
                    <em v-else>拖动</em>
                  </div>
                  <el-empty v-if="!selectedScopeDevices.length" description="暂未添加设备" :image-size="54" />
                </el-checkbox-group>
              </section>
            </div>
          </div>
          <el-alert v-else type="success" :closable="false" show-icon title="该成员可访问当前租户下的全部设备，以及后续新增设备。" />
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button v-if="!isAdminPortal && activeStep > 0" @click="activeStep--">上一步</el-button>
        <el-button v-if="!isAdminPortal && activeStep < 2" type="primary" @click="nextStep">下一步</el-button>
        <el-button v-else type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.account-scope-note { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; padding: 12px 14px; border: 1px solid #dbeafe; border-radius: 10px; background: #f7fbff; color: #64748b; font-size: 13px; }
.account-scope-note strong { color: #1d4ed8; font-size: 14px; }
.protected-account { color: #94a3b8; font-size: 13px; }
.member-steps { margin: 2px 20px 24px; }
.member-steps :deep(.el-step__head.is-success),
.member-steps :deep(.el-step__head.is-process) { color: #1677ff; border-color: #1677ff; }
.member-steps :deep(.el-step__title.is-success),
.member-steps :deep(.el-step__title.is-process) { color: #1d4ed8; }
.member-steps :deep(.el-step__description.is-success),
.member-steps :deep(.el-step__description.is-process) { color: #6f87a8; }
.member-steps :deep(.el-step__line) { background-color: #d8e4f3; }
.member-steps :deep(.el-step__line-inner) { border-color: #79adf7; }
.wizard-panel { min-height: 500px; }
.basic-panel { padding: 8px 34px 0 12px; }
.panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 14px; }
.panel-heading > strong, .panel-heading > div > strong { display: block; color: #24364f; font-size: 15px; }
.panel-heading > span, .panel-heading > div > span { display: block; margin-top: 4px; color: #8593a7; font-size: 12px; line-height: 1.5; }
.selection-summary { flex: 0 0 auto; margin-top: 2px; padding: 6px 10px; border-radius: 14px; background: #eff6ff; color: #55708f; font-size: 12px; }
.selection-summary b { color: #1677ff; font-size: 14px; }
.permission-tree-shell { height: 330px; margin-bottom: 12px; overflow-y: auto; border: 1px solid #e3eaf3; border-radius: 10px; background: #fbfcfe; }
.permission-legend { position: sticky; z-index: 2; top: 0; display: flex; align-items: center; gap: 8px; padding: 9px 14px; border-bottom: 1px solid #e8edf4; background: rgba(248, 250, 252, .96); color: #8391a5; font-size: 11px; }
.permission-legend i { width: 16px; height: 1px; background: #cbd5e1; }
.permission-tree { padding: 7px 10px 10px; background: transparent; }
.permission-tree :deep(.el-tree-node__content) { min-height: 36px; height: auto; margin: 1px 0; border-radius: 7px; }
.permission-tree :deep(.el-tree-node__content:hover) { background: #edf5ff; }
.permission-node { display: flex; min-width: 0; flex: 1; align-items: center; gap: 8px; padding: 5px 8px 5px 2px; }
.permission-node strong { color: #3a4b62; font-size: 13px; }
.permission-node small { overflow: hidden; color: #94a3b8; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.permission-node.is-menu strong { color: #172b4d; font-size: 14px; }
.node-kind { flex: 0 0 auto; padding: 2px 6px; border-radius: 4px; background: #eef2f7; color: #64748b; font-size: 10px; line-height: 16px; }
.is-menu .node-kind { background: #eaf3ff; color: #1677ff; }
.is-page .node-kind { background: #eef8f2; color: #16a34a; }
.scope-mode { margin-bottom: 12px; }
.scope-assignment { display: grid; gap: 12px; }
.group-scope-bar { padding: 11px 13px; border: 1px solid #dfe7f2; border-radius: 9px; background: #f8fafc; }
.scope-column-title { display: flex; align-items: center; justify-content: space-between; margin-bottom: 9px; color: #334155; font-size: 13px; }
.scope-column-title span { color: #1677ff; font-size: 12px; }
.group-options { display: flex; flex-wrap: wrap; gap: 6px 10px; max-height: 82px; overflow-y: auto; }
.group-option { height: 30px; margin: 0; padding: 4px 9px; border: 1px solid #e2e8f0; border-radius: 6px; background: #fff; }
.group-option.is-checked { border-color: #93c5fd; background: #eff6ff; }
.group-option :deep(.el-checkbox__label) { display: flex; align-items: center; gap: 8px; padding-left: 6px; }
.group-option small { color: #94a3b8; font-size: 11px; }
.empty-inline { color: #94a3b8; font-size: 12px; line-height: 30px; }
.device-transfer { display: grid; grid-template-columns: minmax(0, 1fr) 58px minmax(0, 1fr); gap: 10px; align-items: stretch; }
.transfer-panel { min-width: 0; padding: 12px; border: 1px solid #dfe7f2; border-radius: 10px; background: #fbfcfe; transition: border-color .18s, background .18s; }
.selected-panel { background: #f7faff; }
.transfer-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 9px; }
.transfer-heading > div { display: grid; gap: 2px; }
.transfer-heading strong { color: #263950; font-size: 14px; }
.transfer-heading span { color: #94a3b8; font-size: 11px; }
.transfer-heading > b { min-width: 28px; padding: 3px 8px; border-radius: 12px; background: #eaf3ff; color: #1677ff; font-size: 12px; text-align: center; }
.transfer-list { display: grid; align-content: start; gap: 4px; height: 310px; margin-top: 9px; overflow-y: auto; }
.transfer-item { display: flex; min-width: 0; align-items: center; gap: 8px; padding: 8px 9px; border: 1px solid transparent; border-radius: 7px; background: #fff; cursor: grab; user-select: none; }
.transfer-item:hover { border-color: #bfdbfe; background: #f0f7ff; }
.transfer-item:active { cursor: grabbing; }
.transfer-item.is-group-covered { border-color: #e0ebfa; background: #f3f7fd; cursor: default; }
.device-checkbox { min-width: 0; flex: 1; height: auto; margin-right: 0; }
.device-checkbox :deep(.el-checkbox__label) { display: block; min-width: 0; flex: 1; padding-left: 8px; }
.device-checkbox.is-disabled :deep(.el-checkbox__label) { color: inherit; }
.device-copy { display: grid; min-width: 0; gap: 2px; }
.transfer-item strong { overflow: hidden; color: #334155; font-size: 12px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.transfer-item small { overflow: hidden; color: #94a3b8; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.transfer-item em { flex: 0 0 auto; color: #a3afbf; font-size: 10px; font-style: normal; }
.group-source-tag { --el-tag-bg-color: #edf5ff; --el-tag-border-color: #b7d5fb; --el-tag-text-color: #2878d4; flex: 0 0 auto; }
.transfer-actions { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; }
.transfer-actions .el-button + .el-button { margin-left: 0; }
.transfer-actions small { color: #9aa7b8; font-size: 10px; }
@media (max-width: 820px) {
  .account-scope-note { align-items: flex-start; flex-direction: column; gap: 4px; }
  .member-steps { margin-right: 0; margin-left: 0; }
  .basic-panel { padding-right: 0; padding-left: 0; }
  .device-transfer { grid-template-columns: 1fr; }
  .transfer-actions { flex-direction: row; padding: 2px 0; }
  .transfer-actions small { display: none; }
  .transfer-list { height: 220px; }
}
</style>
