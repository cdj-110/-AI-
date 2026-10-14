<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { Connection, Plus, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus';
import { apiRequest } from '../api/request';
import { formatMetricValue } from '../utils/metric-value';
import GatewayBindDialog from '../components/GatewayBindDialog.vue';
import DeviceGroupPanel, { type DeviceGroupItem } from '../components/DeviceGroupPanel.vue';
import { useUserStore } from '../stores/user';
import { hasPermission } from '../utils/permissions';

interface TenantItem {
  id: string;
  name: string;
  deviceCloudAccessDisabled?: boolean;
}

interface DeviceItem {
  id: string;
  tenantId: string;
  tenant: TenantItem;
  gatewayId?: string;
  gateway?: GatewayOption;
  deviceKey: string;
  name: string;
  protocol: string;
  deviceType: string;
  status: string;
  cloudAccessEnabled: boolean;
  cloudDisabledReason?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
  lastSeenAt?: string;
  groups?: Array<{ id: string; name: string }>;
}

interface GatewayOption {
  id: string;
  name: string;
  deviceKey: string;
}

interface ModelTemplateItem {
  id: string;
  name: string;
  deviceType: string;
  _count: { metrics: number };
}

const userStore = useUserStore();
const loading = ref(false);
const gatewayBindVisible = ref(false);
const dialogVisible = ref(false);
const telemetryVisible = ref(false);
const telemetryLoading = ref(false);
const coordinatePickerVisible = ref(false);
const telemetryDevice = ref<DeviceItem | null>(null);
const telemetryItems = ref<Array<{ time: string; metrics: Record<string, unknown> }>>([]);
const coordinateMapRef = ref<HTMLDivElement>();
const editingId = ref('');
const formRef = ref<FormInstance>();
const devices = ref<DeviceItem[]>([]);
const modelTemplates = ref<ModelTemplateItem[]>([]);
const gatewayOptions = ref<GatewayOption[]>([]);
const total = ref(0);
const query = reactive({ page: 1, pageSize: 10, keyword: '', deviceType: '', groupId: '' });
const groups = ref<DeviceGroupItem[]>([]), groupsLoading = ref(false);
const groupTotals = reactive({ totalDevices: 0, ungroupedDevices: 0 });
const deviceTable = ref<{ clearSelection: () => void }>();
const selectedDevices = ref<DeviceItem[]>([]), assignVisible = ref(false), assigning = ref(false);
const assignIds = ref<string[]>([]), targetGroupIds = ref<string[]>([]);
const assignmentMode = ref<'ADD' | 'REMOVE' | 'REPLACE'>('ADD');
const tenantGroupsEnabled = computed(() => ['TENANT_ADMIN', 'TENANT_USER'].includes(userStore.userInfo?.role ?? ''));
const canManageGroups = computed(() => hasPermission(userStore.userInfo, 'DEVICE_GROUP_MANAGE'));
let devicesRequest = 0, groupsRequest = 0;

async function loadGroups(silentError = false) {
  if (!tenantGroupsEnabled.value) return;
  const request = ++groupsRequest;
  groupsLoading.value = !silentError;
  try {
    const data = await apiRequest<{ items: DeviceGroupItem[]; totalDevices: number; ungroupedDevices: number }>({ url: '/api/device-groups', method: 'GET', silentError });
    if (request !== groupsRequest) return;
    groups.value = data.items;
    Object.assign(groupTotals, { totalDevices: data.totalDevices, ungroupedDevices: data.ungroupedDevices });
    if (query.groupId && query.groupId !== 'ungrouped' && !data.items.some(group => group.id === query.groupId)) {
      query.groupId = '';
      query.page = 1;
      await loadDevices();
    }
  } finally { if (request === groupsRequest) groupsLoading.value = false; }
}

async function selectGroup(id: string) { query.groupId = id; await searchDevices(); }
async function refreshGroups(deletedId?: string) {
  if (deletedId === query.groupId) { query.groupId = 'ungrouped'; query.page = 1; }
  await Promise.all([loadGroups(), loadDevices()]);
}
function openAssign(device?: DeviceItem) {
  assignIds.value = device ? [device.id] : selectedDevices.value.map(item => item.id);
  if (!assignIds.value.length) return;
  assignmentMode.value = device ? 'REPLACE' : 'ADD';
  targetGroupIds.value = device?.groups?.map(group => group.id) ?? [];
  assignVisible.value = true;
}
async function assignGroup() {
  if (assigning.value) return;
  if (assignmentMode.value !== 'REPLACE' && !targetGroupIds.value.length) {
    ElMessage.warning(assignmentMode.value === 'ADD' ? '请选择要加入的分组' : '请选择要移出的分组');
    return;
  }
  assigning.value = true;
  try {
    await apiRequest({ url: '/api/device-groups/membership', method: 'PATCH', data: { deviceIds: assignIds.value, groupIds: targetGroupIds.value, mode: assignmentMode.value } });
    assignVisible.value = false;
    ElMessage.success(assignmentMode.value === 'ADD' ? '已加入选中分组' : assignmentMode.value === 'REMOVE' ? '已从选中分组移出' : '设备分组已更新');
    await refreshGroups();
  } catch { /* Keep the dialog and selected devices when the request fails. */ }
  finally { assigning.value = false; }
}
const form = reactive({
  deviceKey: '',
  name: '',
  protocol: 'MQTT',
  deviceType: 'DIRECT',
  gatewayId: '',
  location: '',
  latitude: undefined as number | undefined,
  longitude: undefined as number | undefined,
  modelTemplateId: '',
});
const protocolOptions = [
  { label: 'MQTT', value: 'MQTT' },
  { label: 'HTTP', value: 'HTTP' },
  { label: 'Modbus（通用）', value: 'MODBUS' },
  { label: 'Modbus TCP', value: 'modbus-tcp' },
  { label: 'Modbus RTU', value: 'modbus-rtu' },
  { label: 'DL/T 645', value: 'dlt645' },
  { label: 'IEC 104', value: 'iec104' },
  { label: 'IEC 61850 MMS', value: 'iec61850' },
  { label: 'IEC 61850 GOOSE', value: 'iec61850-goose' },
  { label: 'Siemens S7', value: 'siemens-s7' },
  { label: 'OPC UA', value: 'opcua' },
  { label: 'OPC DA', value: 'opcda' },
];
let refreshTimer: ReturnType<typeof setInterval> | undefined;
let coordinateMap: any;
let coordinateMarker: any;
let amapLoader: Promise<any> | undefined;

const isSuperAdmin = computed(() => userStore.userInfo?.role === 'SUPER_ADMIN');
const canCreateDevice = computed(() => hasPermission(userStore.userInfo, 'DEVICE_CREATE'));
const canEditDevice = computed(() => hasPermission(userStore.userInfo, 'DEVICE_EDIT'));
const canDeleteDevice = computed(() => hasPermission(userStore.userInfo, 'DEVICE_DELETE'));
const rules: FormRules = {
  deviceKey: [{ required: true, message: '请输入设备编号', trigger: 'blur' }],
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
  deviceType: [{ required: true, message: '请选择设备类型', trigger: 'change' }],
};

async function loadDevices(silentError = false) {
  // 列表定时刷新用于兜底；上线/离线的实时变化由 MainLayout 的 SSE 事件同步。
  if (silentError && loading.value) return;
  const request = ++devicesRequest;
  if (!silentError) { deviceTable.value?.clearSelection(); selectedDevices.value = []; }
  loading.value = !silentError;
  try {
    const data = await apiRequest<{ items: DeviceItem[]; total: number }>({ url: '/api/devices', method: 'GET', params: query, silentError });
    if (request !== devicesRequest) return;
    if (!silentError) { deviceTable.value?.clearSelection(); selectedDevices.value = []; }
    devices.value = data.items;
    total.value = data.total;
    if (!data.items.length && query.page > 1) {
      query.page = Math.max(1, Math.ceil(data.total / query.pageSize));
      await loadDevices(silentError);
    }
  } finally {
    if (request === devicesRequest) loading.value = false;
  }
}

async function searchDevices() {
  query.page = 1;
  await loadDevices();
}

function resetForm() {
  Object.assign(form, { deviceKey: '', name: '', protocol: 'MQTT', deviceType: 'DIRECT', gatewayId: '', location: '', latitude: undefined, longitude: undefined, modelTemplateId: '' });
}

async function loadGatewayOptions() {
  gatewayOptions.value = await apiRequest<GatewayOption[]>({ url: '/api/devices/gateways', method: 'GET' });
}

async function openCreate() {
  // 新建设备时预加载网关和物模型模板，便于选择设备类型后直接筛选。
  editingId.value = '';
  resetForm();
  [modelTemplates.value, gatewayOptions.value] = await Promise.all([
    apiRequest<ModelTemplateItem[]>({ url: '/api/model-templates/options', method: 'GET' }),
    apiRequest<GatewayOption[]>({ url: '/api/devices/gateways', method: 'GET' }),
  ]);
  dialogVisible.value = true;
}

async function openEdit(device: DeviceItem) {
  editingId.value = device.id;
  await loadGatewayOptions();
  Object.assign(form, {
    deviceKey: device.deviceKey,
    name: device.name,
    protocol: device.protocol,
    deviceType: device.deviceType ?? 'DIRECT',
    gatewayId: device.gatewayId ?? '',
    location: device.location ?? '',
    latitude: device.latitude,
    longitude: device.longitude,
    modelTemplateId: '',
  });
  dialogVisible.value = true;
}

async function submit() {
  if (!(await formRef.value?.validate())) return;
  if (form.deviceType === 'GATEWAY_CHILD' && !form.gatewayId) {
    ElMessage.warning('请选择所属网关');
    return;
  }
  const latitude = typeof form.latitude === 'number' && Number.isFinite(form.latitude) ? form.latitude : undefined;
  const longitude = typeof form.longitude === 'number' && Number.isFinite(form.longitude) ? form.longitude : undefined;
  const data = {
    ...form,
    gatewayId: form.deviceType === 'GATEWAY_CHILD' ? form.gatewayId : undefined,
    location: form.location || undefined,
    latitude,
    longitude,
    modelTemplateId: !editingId.value && form.modelTemplateId ? form.modelTemplateId : undefined,
  };
  await apiRequest({ url: editingId.value ? `/api/devices/${editingId.value}` : '/api/devices', method: editingId.value ? 'PATCH' : 'POST', data });
  ElMessage.success(editingId.value ? '设备已更新' : '设备已创建');
  dialogVisible.value = false;
  await Promise.all([loadDevices(), loadGroups()]);
}

async function remove(device: DeviceItem) {
  const gatewayTip = device.deviceType === 'GATEWAY'
    ? ' 如果它来自出厂网关绑定，删除后会同时解除绑定，可重新测试绑定流程。'
    : '';
  await ElMessageBox.confirm(`确定删除设备“${device.name}”吗？${gatewayTip}`, '删除确认', {
    type: 'warning',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消',
  });
  const result = await apiRequest<{ message: string; factoryGateway?: { sn: string } }>({
    url: `/api/devices/${device.id}`,
    method: 'DELETE',
  });
  ElMessage.success(result.factoryGateway
    ? `设备已删除，${result.factoryGateway.sn} 已解除绑定，可到出厂网关重置绑定码`
    : '设备已删除');
  await Promise.all([loadDevices(), loadGroups()]);
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

function loadAmapSdk() {
  const targetWindow = window as Window & { AMap?: any; _AMapSecurityConfig?: { securityJsCode: string } };
  if (targetWindow.AMap) return Promise.resolve(targetWindow.AMap);
  amapLoader ??= new Promise((resolve, reject) => {
    targetWindow._AMapSecurityConfig = {
      securityJsCode: import.meta.env.VITE_AMAP_SECURITY_CODE ?? '02eb711b9457def0e71bb5933c068d6f',
    };
    const script = document.createElement('script');
    script.src = `https://webapi.amap.com/maps?v=2.0&key=${import.meta.env.VITE_AMAP_KEY ?? '99b5c6fb59ffb2024fbccc5747595fde'}`;
    script.async = true;
    script.onload = () => targetWindow.AMap ? resolve(targetWindow.AMap) : reject(new Error('高德地图加载失败'));
    script.onerror = () => reject(new Error('高德地图加载失败，请检查网络或 Key 配置'));
    document.head.appendChild(script);
  });
  return amapLoader;
}

async function openCoordinatePicker() {
  coordinatePickerVisible.value = true;
  await new Promise((resolve) => setTimeout(resolve));
  const AMap = await loadAmapSdk();
  const center = [
    typeof form.longitude === 'number' ? form.longitude : 116.397428,
    typeof form.latitude === 'number' ? form.latitude : 39.90923,
  ];
  if (!coordinateMap) {
    coordinateMap = new AMap.Map(coordinateMapRef.value, {
      center,
      zoom: 16,
      viewMode: '2D',
      resizeEnable: true,
    });
    coordinateMap.on('click', (event: { lnglat: { getLng: () => number; getLat: () => number } }) => {
      setPickedCoordinate(event.lnglat.getLng(), event.lnglat.getLat());
    });
  } else {
    coordinateMap.setCenter(center);
  }
  if (typeof form.longitude === 'number' && typeof form.latitude === 'number') {
    setPickedCoordinate(form.longitude, form.latitude);
  }
}

function setPickedCoordinate(longitude: number, latitude: number) {
  form.longitude = Number(longitude.toFixed(6));
  form.latitude = Number(latitude.toFixed(6));
  const AMap = (window as Window & { AMap?: any }).AMap;
  if (!AMap || !coordinateMap) return;
  const position = [form.longitude, form.latitude];
  if (!coordinateMarker) {
    coordinateMarker = new AMap.Marker({ position, anchor: 'bottom-center' });
    coordinateMap.add(coordinateMarker);
    return;
  }
  coordinateMarker.setPosition(position);
}

function formatMetrics(metrics: Record<string, unknown>) {
  return Object.entries(metrics)
    .filter(([key]) => key.trim())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}: ${formatMetricValue(value)}`)
    .join(' · ');
}

function statusType(status: string) {
  return status === 'ONLINE' ? 'success' : 'info';
}

function statusLabel(status: string) {
  return { ONLINE: '在线', OFFLINE: '离线', DISABLED: '停用' }[status] ?? status;
}

function deviceTypeLabel(type: string) {
  return { GATEWAY: '网关', GATEWAY_CHILD: '子设备', DIRECT: '直连' }[type] ?? type;
}

function deviceTypeTagType(_type: string) {
  return '';
}

function deviceTypeTagClass(type: string) {
  return {
    GATEWAY: 'gateway-device-tag',
    GATEWAY_CHILD: 'gateway-child-tag',
    DIRECT: 'direct-device-tag',
  }[type] ?? '';
}

const availableGatewayOptions = computed(() => gatewayOptions.value.filter((gateway) => gateway.id !== editingId.value));
const availableModelTemplates = computed(() => modelTemplates.value.filter((template) => template.deviceType === form.deviceType));

watch(() => form.deviceType, (type) => {
  if (type !== 'GATEWAY_CHILD') form.gatewayId = '';
  if (!availableModelTemplates.value.some((template) => template.id === form.modelTemplateId)) form.modelTemplateId = '';
});

function handleDeviceStatusEvent(event: Event) {
  // 只修正当前页已有设备的状态，不打断分页/搜索条件。
  const detail = (event as CustomEvent<Partial<DeviceItem>>).detail;
  if (!detail?.id || !detail.status) return;
  const target = devices.value.find((item) => item.id === detail.id);
  if (!target) return;
  target.status = detail.status;
  if (detail.lastSeenAt) target.lastSeenAt = detail.lastSeenAt;
}

onMounted(async () => {
  void Promise.all([loadDevices(), loadGroups()]).catch(() => {});
  window.addEventListener('device-status-change', handleDeviceStatusEvent);
  refreshTimer = setInterval(() => { void Promise.all([loadDevices(true), loadGroups(true)]).catch(() => {}); }, 30000);
});

onBeforeUnmount(() => {
  devicesRequest++; groupsRequest++;
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
      <div v-if="userStore.userInfo?.role === 'TENANT_ADMIN' || canCreateDevice" class="page-action-buttons">
        <el-button v-if="userStore.userInfo?.role === 'TENANT_ADMIN'" :icon="Connection" @click="gatewayBindVisible = true">新增网关</el-button>
        <el-button v-if="canCreateDevice" type="primary" :icon="Plus" @click="openCreate">新增设备</el-button>
      </div>
    </div>
    <div class="devices-workspace">
    <section class="table-panel devices-table-panel">
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
        <div class="device-filter-controls">
          <DeviceGroupPanel v-if="tenantGroupsEnabled" :items="groups" :selected="query.groupId" :total="groupTotals.totalDevices" :ungrouped="groupTotals.ungroupedDevices" :can-manage="canManageGroups" :loading="groupsLoading" @select="selectGroup" @changed="refreshGroups" />
          <el-input v-model="query.keyword" clearable placeholder="搜索设备名称或编号" class="device-search" @keyup.enter="searchDevices">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="query.deviceType" clearable placeholder="全部类型" class="device-type-filter" @change="searchDevices">
            <el-option label="网关" value="GATEWAY" />
            <el-option label="子设备" value="GATEWAY_CHILD" />
            <el-option label="直连" value="DIRECT" />
          </el-select>
          <el-button @click="searchDevices">查询</el-button>
        </div>
        <el-button v-if="canManageGroups && selectedDevices.length" type="primary" plain @click="openAssign()">调整分组（{{ selectedDevices.length }}）</el-button>
      </div>
      <el-table ref="deviceTable" v-loading="loading" :data="devices" row-key="id" @selection-change="selectedDevices = $event">
        <el-table-column v-if="canManageGroups" type="selection" width="44" :reserve-selection="true" />
        <el-table-column prop="deviceKey" label="设备编号" min-width="150" />
        <el-table-column label="设备名称" min-width="150"><template #default="{ row }"><router-link :to="`/devices/${row.id}`">{{ row.name }}</router-link></template></el-table-column>
        <el-table-column v-if="isSuperAdmin" prop="tenant.name" label="所属租户" min-width="160" />
        <el-table-column v-if="tenantGroupsEnabled" label="设备分组" min-width="210">
          <template #default="{ row }">
            <button v-if="canManageGroups" type="button" class="device-groups-cell" :class="{ empty: !row.groups?.length }" @click="openAssign(row)">
              <template v-if="row.groups?.length"><el-tag v-for="group in row.groups.slice(0, 2)" :key="group.id" size="small" effect="plain">{{ group.name }}</el-tag><span v-if="row.groups.length > 2">+{{ row.groups.length - 2 }}</span></template>
              <template v-else>未分组</template>
            </button>
            <div v-else class="device-groups-cell readonly"><template v-if="row.groups?.length"><el-tag v-for="group in row.groups.slice(0, 2)" :key="group.id" size="small" effect="plain">{{ group.name }}</el-tag><span v-if="row.groups.length > 2">+{{ row.groups.length - 2 }}</span></template><template v-else>未分组</template></div>
          </template>
        </el-table-column>
        <el-table-column label="设备类型" min-width="150">
          <template #default="{ row }">
            <el-tag class="device-type-tag" :class="deviceTypeTagClass(row.deviceType)" :type="deviceTypeTagType(row.deviceType)">{{ deviceTypeLabel(row.deviceType) }}</el-tag>
            <span v-if="row.deviceType === 'GATEWAY_CHILD' && row.gateway" class="gateway-name">归属 {{ row.gateway.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="protocol" label="协议" width="90" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tooltip :disabled="row.cloudAccessEnabled && !row.tenant?.deviceCloudAccessDisabled" :content="row.tenant?.deviceCloudAccessDisabled ? '所属租户已停用全部设备云连接' : row.cloudDisabledReason || '管理平台已禁用该设备的云连接'">
              <el-tag :type="row.tenant?.deviceCloudAccessDisabled ? 'info' : statusType(row.status)">{{ row.tenant?.deviceCloudAccessDisabled ? '租户禁用' : row.cloudAccessEnabled ? statusLabel(row.status) : '平台禁用' }}</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="最后上报" min-width="180"><template #default="{ row }">{{ row.lastSeenAt ? new Date(row.lastSeenAt).toLocaleString() : '-' }}</template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }"><el-button link type="primary" @click="openTelemetry(row)">查看数据</el-button><el-button v-if="canEditDevice" link type="primary" @click="openEdit(row)">编辑</el-button><el-button v-if="canDeleteDevice" link type="danger" @click="remove(row)">删除</el-button></template>
        </el-table-column>
      </el-table>
      <div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="loadDevices()" /></div>
    </section>
    </div>
    <el-dialog v-model="assignVisible" title="调整设备分组" width="min(560px, 94vw)" :close-on-click-modal="!assigning" :close-on-press-escape="!assigning" :show-close="!assigning">
      <p class="form-tip">已选择 {{ assignIds.length }} 台设备。一台设备可同时属于多个分组，不影响网关与子设备关系。</p>
      <el-form label-position="top" class="assign-group-form">
        <el-form-item v-if="assignIds.length > 1" label="批量操作">
          <el-radio-group v-model="assignmentMode" :disabled="assigning"><el-radio-button value="ADD">加入分组</el-radio-button><el-radio-button value="REMOVE">移出分组</el-radio-button></el-radio-group>
        </el-form-item>
        <el-alert v-if="assignIds.length > 1" :closable="false" type="info" show-icon :title="assignmentMode === 'ADD' ? '保留每台设备原有分组，只新增下方选中的分组。' : '只移除下方选中的分组，其他分组保持不变。'" />
        <el-form-item :label="assignIds.length === 1 ? '所属分组（可多选）' : '选择分组'">
          <el-checkbox-group v-model="targetGroupIds" class="assignment-group-list" :disabled="assigning">
            <el-checkbox v-for="group in groups" :key="group.id" :value="group.id" border>{{ group.name }}<small>{{ group._count.devices }} 台</small></el-checkbox>
          </el-checkbox-group>
          <el-empty v-if="!groups.length" description="暂无可用分组" :image-size="52" />
        </el-form-item>
        <p v-if="assignIds.length === 1" class="assignment-hint">取消全部勾选后保存，该设备将变为“未分组”。</p>
      </el-form>
      <template #footer><el-button :disabled="assigning" @click="assignVisible = false">取消</el-button><el-button type="primary" :loading="assigning" @click="assignGroup">确定</el-button></template>
    </el-dialog>
    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑设备' : '新增设备'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="86px">
        <el-form-item label="设备编号" prop="deviceKey"><el-input v-model="form.deviceKey" placeholder="例如 WK-SENSOR-001" /></el-form-item>
        <el-form-item label="设备名称" prop="name"><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="设备类型" prop="deviceType">
          <el-select v-model="form.deviceType" class="full-width">
            <el-option label="直连设备" value="DIRECT" />
            <el-option label="网关" value="GATEWAY" />
            <el-option label="网关子设备" value="GATEWAY_CHILD" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.deviceType === 'GATEWAY_CHILD'" label="所属网关">
          <el-select v-model="form.gatewayId" filterable clearable class="full-width" placeholder="请选择上级网关">
            <el-option v-for="gateway in availableGatewayOptions" :key="gateway.id" :label="`${gateway.name} (${gateway.deviceKey})`" :value="gateway.id" />
          </el-select>
          <p class="form-tip">只有类型为“网关”的设备会出现在这里。</p>
        </el-form-item>
        <el-form-item label="接入协议">
          <el-select v-model="form.protocol" filterable class="full-width">
            <el-option v-for="option in protocolOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="安装位置"><el-input v-model="form.location" /></el-form-item>
        <el-form-item label="地图坐标">
          <div class="coordinate-row">
            <el-input-number v-model="form.longitude" class="coordinate-input" :min="-180" :max="180" :precision="6" controls-position="right" placeholder="经度" />
            <el-input-number v-model="form.latitude" class="coordinate-input" :min="-90" :max="90" :precision="6" controls-position="right" placeholder="纬度" />
          </div>
          <el-button class="pick-coordinate-button" plain @click="openCoordinatePicker">地图选点</el-button>
          <p class="form-tip">填写经纬度后，设备会显示在设备地图中；没有坐标的设备不会生成地图点位。</p>
        </el-form-item>
        <el-form-item v-if="!editingId" label="物模型模板">
          <el-select v-model="form.modelTemplateId" clearable class="full-width" placeholder="可选：选择模板自动生成字段">
            <el-option v-for="template in availableModelTemplates" :key="template.id" :label="`${template.name} · ${template._count.metrics} 个字段`" :value="template.id" />
          </el-select>
          <p class="form-tip">模板字段会复制到新设备，后续仍可在设备配置里单独调整。</p>
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" @click="submit">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="coordinatePickerVisible" title="地图选点" width="820px">
      <p class="form-tip">点击地图任意位置，系统会自动回填经纬度。</p>
      <div ref="coordinateMapRef" class="coordinate-map" />
      <template #footer>
        <span class="coordinate-preview">经度 {{ form.longitude ?? '-' }}，纬度 {{ form.latitude ?? '-' }}</span>
        <el-button type="primary" @click="coordinatePickerVisible = false">确定</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="telemetryVisible" :title="`${telemetryDevice?.name ?? ''} · 最新遥测`" width="620px">
      <el-table v-loading="telemetryLoading" :data="telemetryItems" empty-text="暂无遥测数据">
        <el-table-column label="上报时间" width="190"><template #default="{ row }">{{ new Date(row.time).toLocaleString() }}</template></el-table-column>
        <el-table-column label="指标"><template #default="{ row }">{{ formatMetrics(row.metrics) }}</template></el-table-column>
      </el-table>
      <template #footer><el-button @click="telemetryVisible = false">关闭</el-button></template>
    </el-dialog>
    <GatewayBindDialog v-model:visible="gatewayBindVisible" @bound="refreshGroups()" />
  </div>
</template>

<style scoped>
.devices-workspace,.devices-table-panel{min-width:0}.device-filter-controls{display:flex;align-items:center;gap:12px;min-width:0}.device-search{width:300px}.device-type-filter{width:140px}.assign-group-form{margin-top:18px}.device-groups-cell{display:flex;max-width:100%;align-items:center;gap:5px;padding:2px 0;border:0;background:transparent;color:#1677ff;cursor:pointer}.device-groups-cell.empty{color:#8a98ad}.device-groups-cell.readonly{cursor:default}.device-groups-cell>span{flex:0 0 auto;color:#6b7f9b;font-size:12px}.assignment-group-list{display:grid;width:100%;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.assignment-group-list .el-checkbox{width:100%;height:38px;margin:0}.assignment-group-list :deep(.el-checkbox__label){display:flex;min-width:0;flex:1;align-items:center;justify-content:space-between;gap:8px}.assignment-group-list small{color:#94a3b8;font-size:11px}.assignment-hint{margin:-8px 0 0;color:#8492a6;font-size:12px}
.page-action-buttons { display: flex; align-items: center; gap: 10px; }
.page-action-buttons .el-button { margin-left: 0; }
.topic-tip { margin-bottom: 16px; }
.topic-divider { margin: 0 8px; color: #9aa4b2; }
code { color: #2563eb; font-family: Consolas, monospace; }
.device-type-tag.gateway-device-tag { --el-tag-bg-color: #e6f4ff; --el-tag-border-color: #91caff; --el-tag-text-color: #1677ff; }
.device-type-tag.gateway-child-tag { --el-tag-bg-color: #fff7e6; --el-tag-border-color: #ffd591; --el-tag-text-color: #fa8c16; }
.device-type-tag.direct-device-tag { --el-tag-bg-color: #f6ffed; --el-tag-border-color: #b7eb8f; --el-tag-text-color: #52c41a; }
.gateway-name { margin-left: 8px; color: #94a3b8; font-size: 12px; }
.coordinate-row { display: grid; width: 100%; grid-template-columns: 1fr 1fr; gap: 10px; }
.coordinate-input { width: 100%; }
.pick-coordinate-button { margin-top: 10px; }
.coordinate-map { height: 460px; margin-top: 12px; overflow: hidden; border: 1px solid #e9eef5; border-radius: 12px; }
.coordinate-preview { float: left; color: #64748b; font-size: 13px; line-height: 32px; }
.form-tip { margin: 6px 0 0; color: #94a3b8; font-size: 12px; line-height: 1.5; }
@media (max-width: 560px) {
  .page-actions { flex-wrap: wrap; gap: 12px; }
  .devices-table-panel .table-toolbar { flex-wrap: wrap; justify-content: flex-start; }
  .device-filter-controls { width: 100%; flex-wrap: wrap; }
  .device-search { flex-basis: 100%; width: 100%; }
  .device-type-filter { flex: 1; min-width: 140px; }
  .devices-table-panel :deep(.el-table__cell.el-table-fixed-column--right) { position: static!important; right: auto!important; }
  .devices-table-panel :deep(.el-table__cell.el-table-fixed-column--right::before) { display: none; }
  .page-action-buttons { width: 100%; }
  .page-action-buttons .el-button { flex: 1; }
  .coordinate-row { grid-template-columns: 1fr; }
}
</style>
