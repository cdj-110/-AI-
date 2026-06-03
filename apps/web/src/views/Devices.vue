<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';
import { useUserStore } from '../stores/user';

interface TenantItem {
  id: string;
  name: string;
}

interface DeviceItem {
  id: string;
  tenantId: string;
  tenant: TenantItem;
  deviceKey: string;
  name: string;
  protocol: string;
  status: string;
  location?: string;
  lastSeenAt?: string;
}

interface ModelTemplateItem {
  id: string;
  deviceKey: string;
  name: string;
  _count: { metrics: number };
}

const userStore = useUserStore();
const loading = ref(false);
const dialogVisible = ref(false);
const telemetryVisible = ref(false);
const telemetryLoading = ref(false);
const telemetryDevice = ref<DeviceItem | null>(null);
const telemetryItems = ref<Array<{ time: string; metrics: Record<string, unknown> }>>([]);
const editingId = ref('');
const formRef = ref<FormInstance>();
const devices = ref<DeviceItem[]>([]);
const modelTemplates = ref<ModelTemplateItem[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '' });
const form = reactive({
  deviceKey: '',
  name: '',
  protocol: 'MQTT',
  location: '',
  templateDeviceId: '',
});
let refreshTimer: ReturnType<typeof setInterval> | undefined;

const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const canManage = computed(() => userStore.userInfo?.role !== 'TENANT_USER');
const rules: FormRules = {
  deviceKey: [{ required: true, message: '请输入设备编号', trigger: 'blur' }],
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
};

async function loadDevices() {
  loading.value = true;
  try {
    const data = await apiRequest<{ items: DeviceItem[]; total: number }>({ url: '/api/devices', method: 'GET', params: query });
    devices.value = data.items;
    total.value = data.total;
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  Object.assign(form, { deviceKey: '', name: '', protocol: 'MQTT', location: '', templateDeviceId: '' });
}

async function openCreate() {
  editingId.value = '';
  resetForm();
  modelTemplates.value = await apiRequest<ModelTemplateItem[]>({ url: '/api/devices/model-templates', method: 'GET' });
  dialogVisible.value = true;
}

function openEdit(device: DeviceItem) {
  editingId.value = device.id;
  Object.assign(form, {
    deviceKey: device.deviceKey,
    name: device.name,
    protocol: device.protocol,
    location: device.location ?? '',
    templateDeviceId: '',
  });
  dialogVisible.value = true;
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  const data = { ...form, location: form.location || undefined, templateDeviceId: !editingId.value && form.templateDeviceId ? form.templateDeviceId : undefined };
  await apiRequest({ url: editingId.value ? `/api/devices/${editingId.value}` : '/api/devices', method: editingId.value ? 'PATCH' : 'POST', data });
  ElMessage.success(editingId.value ? '设备已更新' : '设备已创建');
  dialogVisible.value = false;
  await loadDevices();
}

async function remove(device: DeviceItem) {
  await ElMessageBox.confirm(`确定删除设备“${device.name}”吗？`, '删除确认', { type: 'warning' });
  await apiRequest({ url: `/api/devices/${device.id}`, method: 'DELETE' });
  ElMessage.success('设备已删除');
  await loadDevices();
}

async function openTelemetry(device: DeviceItem) {
  telemetryDevice.value = device;
  telemetryVisible.value = true;
  telemetryLoading.value = true;
  try {
    const data = await apiRequest<{ items: Array<{ time: string; metrics: Record<string, unknown> }> }>({
      url: `/api/devices/${device.id}/telemetry`,
      method: 'GET',
    });
    telemetryItems.value = data.items;
  } finally {
    telemetryLoading.value = false;
  }
}

function formatMetrics(metrics: Record<string, unknown>) {
  return Object.entries(metrics).map(([key, value]) => `${key}: ${String(value)}`).join(' · ');
}

function statusType(status: string) {
  return status === 'ONLINE' ? 'success' : status === 'DISABLED' ? 'info' : 'warning';
}

function statusLabel(status: string) {
  return { ONLINE: '在线', OFFLINE: '离线', DISABLED: '停用' }[status] ?? status;
}

function handleDeviceStatusEvent(event: Event) {
  const detail = (event as CustomEvent<Partial<DeviceItem>>).detail;
  if (!detail?.id || !detail.status) return;
  const target = devices.value.find((item) => item.id === detail.id);
  if (!target) return;
  target.status = detail.status;
  if (detail.lastSeenAt) target.lastSeenAt = detail.lastSeenAt;
}

onMounted(async () => {
  await loadDevices();
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  refreshTimer = setInterval(loadDevices, 15000);
});

onBeforeUnmount(() => {
  window.removeEventListener('device-status-change', handleDeviceStatusEvent);
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">设备管理</h1>
        <p class="page-description">维护设备档案和当前连接状态</p>
      </div>
      <el-button v-if="canManage" type="primary" :icon="Plus" @click="openCreate">新增设备</el-button>
    </div>
    <section class="table-panel">
      <el-alert class="topic-tip" type="info" :closable="false" show-icon>
        <template #title>
          心跳：<code>weikong/devices/&lt;设备编号&gt;/heartbeat</code>
          <span class="topic-divider">·</span>
          遥测：<code>weikong/devices/&lt;设备编号&gt;/telemetry</code>
          <span class="topic-divider">·</span>
          建议每 30 秒发送一次心跳
        </template>
      </el-alert>
      <div class="table-toolbar">
        <el-input v-model="query.keyword" clearable placeholder="搜索设备名称或编号" style="max-width: 300px" @keyup.enter="loadDevices">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-button @click="loadDevices">查询</el-button>
      </div>
      <el-table v-loading="loading" :data="devices">
        <el-table-column prop="deviceKey" label="设备编号" min-width="150" />
        <el-table-column label="设备名称" min-width="150"><template #default="{ row }"><router-link :to="`/devices/${row.id}`">{{ row.name }}</router-link></template></el-table-column>
        <el-table-column v-if="isSuperAdmin" prop="tenant.name" label="所属租户" min-width="160" />
        <el-table-column prop="protocol" label="协议" width="90" />
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="statusType(row.status)">{{ statusLabel(row.status) }}</el-tag></template></el-table-column>
        <el-table-column prop="location" label="位置" min-width="140" />
        <el-table-column label="最后上报" min-width="180"><template #default="{ row }">{{ row.lastSeenAt ? new Date(row.lastSeenAt).toLocaleString() : '-' }}</template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }"><el-button link type="primary" @click="openTelemetry(row)">查看数据</el-button><el-button v-if="canManage" link type="primary" @click="openEdit(row)">编辑</el-button><el-button v-if="canManage" link type="danger" @click="remove(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
      <div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadDevices" /></div>
    </section>
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑设备' : '新增设备'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="86px">
        <el-form-item label="设备编号" prop="deviceKey"><el-input v-model="form.deviceKey" placeholder="例如 WK-SENSOR-001" /></el-form-item>
        <el-form-item label="设备名称" prop="name"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="接入协议"><el-select v-model="form.protocol" class="full-width"><el-option label="MQTT" value="MQTT" /><el-option label="HTTP" value="HTTP" /><el-option label="Modbus" value="MODBUS" /></el-select></el-form-item>
        <el-form-item label="安装位置"><el-input v-model="form.location" /></el-form-item>
        <el-form-item v-if="!editingId" label="复用物模型">
          <el-select v-model="form.templateDeviceId" clearable class="full-width" placeholder="可选：从已有设备复制字段配置">
            <el-option v-for="template in modelTemplates" :key="template.id" :label="`${template.name} (${template.deviceKey}) · ${template._count.metrics} 个字段`" :value="template.id" />
          </el-select>
          <p class="form-tip">仅复制物模型字段，不复制遥测数据和告警规则。</p>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" @click="submit">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="telemetryVisible" :title="`${telemetryDevice?.name ?? ''} · 最新遥测`" width="620px">
      <el-table v-loading="telemetryLoading" :data="telemetryItems" empty-text="暂无遥测数据">
        <el-table-column label="上报时间" width="190"><template #default="{ row }">{{ new Date(row.time).toLocaleString() }}</template></el-table-column>
        <el-table-column label="指标"><template #default="{ row }">{{ formatMetrics(row.metrics) }}</template></el-table-column>
      </el-table>
      <template #footer><el-button @click="telemetryVisible = false">关闭</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.topic-tip { margin-bottom: 16px; }
.topic-divider { margin: 0 8px; color: #9aa4b2; }
code { color: #2563eb; font-family: Consolas, monospace; }
.form-tip { margin: 6px 0 0; color: #94a3b8; font-size: 12px; line-height: 1.5; }
</style>
