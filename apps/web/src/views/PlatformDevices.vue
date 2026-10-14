<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import { Refresh, Search } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { apiRequest } from '../api/request';
import { formatMetricValue } from '../utils/metric-value';
import { useRoute, useRouter } from 'vue-router';

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
  deviceType: string;
  status: string;
  cloudAccessEnabled: boolean;
  cloudDisabledAt?: string;
  cloudDisabledReason?: string;
  location?: string;
  lastSeenAt?: string;
  createdAt: string;
  gateway?: { id: string; name: string; deviceKey: string; cloudAccessEnabled?: boolean };
}

interface Summary {
  tenantCount: number;
  deviceCount: number;
  onlineDeviceCount: number;
  offlineDeviceCount: number;
}

const loading = ref(false);
const route = useRoute();
const router = useRouter();
const telemetryLoading = ref(false);
const telemetryVisible = ref(false);
const selectedDevice = ref<DeviceItem | null>(null);
const devices = ref<DeviceItem[]>([]);
const tenants = ref<TenantItem[]>([]);
const telemetryItems = ref<Array<{ time: string; metrics: Record<string, unknown> }>>([]);
const total = ref(0);
const refreshedAt = ref<Date>();
const cloudActionId = ref('');
const summary = ref<Summary>({ tenantCount: 0, deviceCount: 0, onlineDeviceCount: 0, offlineDeviceCount: 0 });
const query = reactive({
  page: 1,
  pageSize: 20,
  keyword: '',
  tenantId: String(route.query.tenantId || ''),
  status: '',
  deviceType: '',
  protocol: '',
});
let refreshTimer: ReturnType<typeof setInterval> | undefined;
const selectedTenantName = computed(() => tenants.value.find((tenant) => tenant.id === query.tenantId)?.name
  || String(route.query.tenantName || ''));

async function loadDevices(silent = false) {
  if (!silent) loading.value = true;
  try {
    const data = await apiRequest<{ items: DeviceItem[]; total: number }>({
      url: '/api/devices',
      method: 'GET',
      params: query,
      silentError: silent,
    });
    devices.value = data.items;
    total.value = data.total;
    refreshedAt.value = new Date();
  } finally {
    if (!silent) loading.value = false;
  }
}

async function loadBaseData() {
  const [tenantData, summaryData] = await Promise.all([
    apiRequest<TenantItem[]>({ url: '/api/tenants', method: 'GET' }),
    apiRequest<Summary>({ url: '/api/dashboard/summary', method: 'GET' }),
  ]);
  tenants.value = tenantData;
  summary.value = summaryData;
}

async function searchDevices() {
  query.page = 1;
  await syncFilterRoute();
  await loadDevices();
}

function resetFilters() {
  Object.assign(query, { page: 1, keyword: '', tenantId: '', status: '', deviceType: '', protocol: '' });
  void router.replace({ path: '/devices' });
  void loadDevices();
}

async function syncFilterRoute() {
  const tenantName = tenants.value.find((tenant) => tenant.id === query.tenantId)?.name;
  const nextQuery: Record<string, string> = {};
  if (query.tenantId) nextQuery.tenantId = query.tenantId;
  if (tenantName) nextQuery.tenantName = tenantName;
  if (query.keyword) nextQuery.keyword = query.keyword;
  if (query.status) nextQuery.status = query.status;
  if (query.deviceType) nextQuery.deviceType = query.deviceType;
  if (query.protocol) nextQuery.protocol = query.protocol;
  await router.replace({ path: '/devices', query: nextQuery });
}

function clearTenantFilter() {
  query.tenantId = '';
  void searchDevices();
}

function statusType(status: string) {
  return status === 'ONLINE' ? 'success' : status === 'DISABLED' ? 'danger' : 'info';
}

function statusLabel(status: string) {
  return { ONLINE: '在线', OFFLINE: '离线', DISABLED: '停用' }[status] ?? status;
}

function effectiveStatusType(device: DeviceItem) {
  if (!device.cloudAccessEnabled) return 'danger';
  if (device.gateway?.cloudAccessEnabled === false) return 'warning';
  return statusType(device.status);
}

function effectiveStatusLabel(device: DeviceItem) {
  if (!device.cloudAccessEnabled) return '已禁用';
  if (device.gateway?.cloudAccessEnabled === false) return '网关受限';
  return statusLabel(device.status);
}

function effectiveStatusTip(device: DeviceItem) {
  if (!device.cloudAccessEnabled) {
    return `${device.cloudDisabledReason || '管理平台已禁用该设备的云连接'}${device.cloudDisabledAt ? ` · ${formatTime(device.cloudDisabledAt)}` : ''}`;
  }
  if (device.gateway?.cloudAccessEnabled === false) return '所属网关已被管理平台禁用，该设备暂时不能连接云平台';
  return device.status === 'ONLINE' ? '设备当前在线' : '设备当前离线，但仍允许连接云平台';
}

function deviceTypeLabel(type: string) {
  return { GATEWAY: '网关', GATEWAY_CHILD: '子设备', DIRECT: '直连设备' }[type] ?? type;
}

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString() : '-';
}

function formatMetrics(metrics: Record<string, unknown>) {
  const entries = Object.entries(metrics);
  if (!entries.length) return '无数据';
  return entries.slice(0, 12).map(([key, value]) => `${key}: ${formatMetricValue(value)}`).join(' · ')
    + (entries.length > 12 ? ` · 另有 ${entries.length - 12} 项` : '');
}

async function openTelemetry(device: DeviceItem) {
  selectedDevice.value = device;
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

async function toggleCloudAccess(device: DeviceItem) {
  try {
    let reason: string | undefined;
    if (device.cloudAccessEnabled) {
      const answer = await ElMessageBox.prompt(
        `禁用后，“${device.name}”将立即被禁止连接云平台，不能上报数据或接收云端指令。请输入禁用原因。`,
        '禁用设备云连接',
        {
          type: 'warning',
          confirmButtonText: '确认禁用',
          cancelButtonText: '取消',
          inputPlaceholder: '例如：租户停用、设备异常或安全隔离',
          inputValidator: (value) => value.trim().length >= 2 || '请输入至少 2 个字符的禁用原因',
        },
      );
      reason = answer.value.trim();
    } else {
      await ElMessageBox.confirm(
        `解禁后，“${device.name}”可以重新连接云平台，并在设备自动重连后恢复上报。`,
        '恢复设备云连接',
        { type: 'warning', confirmButtonText: '确认解禁', cancelButtonText: '取消' },
      );
    }
    cloudActionId.value = device.id;
    const result = await apiRequest<{
      device: DeviceItem;
      disconnectResults: Array<{ clientId: string; result: string }>;
      sharedGatewayConnectionRetained?: boolean;
    }>({
      url: `/api/devices/${device.id}/cloud-access`,
      method: 'PATCH',
      data: { enabled: !device.cloudAccessEnabled, reason },
    });
    Object.assign(device, result.device);
    const kickUnavailable = result.disconnectResults.some((item) => item.result === 'unavailable');
    if (kickUnavailable) {
      ElMessage.warning('禁用策略已生效，当前会话已禁止交换数据；EMQX 主动断开接口尚未就绪');
    } else if (result.sharedGatewayConnectionRetained) {
      ElMessage.success('子设备云连接已禁用；所属网关连接保持不变');
    } else {
      ElMessage.success(device.cloudAccessEnabled ? '设备云连接已恢复，等待设备自动重连' : '设备云连接已禁用');
    }
    await loadDevices(true);
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error;
  } finally {
    cloudActionId.value = '';
  }
}

onMounted(async () => {
  query.keyword = String(route.query.keyword || '');
  query.status = String(route.query.status || '');
  query.deviceType = String(route.query.deviceType || '');
  query.protocol = String(route.query.protocol || '');
  await Promise.all([loadBaseData(), loadDevices()]);
  refreshTimer = setInterval(() => void loadDevices(true), 15000);
});

watch(() => route.query.tenantId, (value) => {
  const tenantId = String(value || '');
  if (tenantId === query.tenantId) return;
  query.tenantId = tenantId;
  query.page = 1;
  void loadDevices();
});

onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>

<template>
  <div>
    <div class="page-actions">
      <div>
        <h1 class="page-title">全部设备</h1>
        <p class="page-description">集中查看所有租户的网关、子设备和直连设备运行状态</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="loadDevices()">刷新</el-button>
    </div>

    <section class="summary-grid">
      <div class="summary-card"><span>平台设备</span><strong>{{ summary.deviceCount }}</strong></div>
      <div class="summary-card online"><span>在线设备</span><strong>{{ summary.onlineDeviceCount }}</strong></div>
      <div class="summary-card"><span>离线设备</span><strong>{{ summary.offlineDeviceCount }}</strong></div>
      <div class="summary-card"><span>租户数量</span><strong>{{ summary.tenantCount }}</strong></div>
    </section>

    <section class="table-panel">
      <div v-if="query.tenantId" class="tenant-device-context">
        <div><span>当前租户设备</span><strong>{{ selectedTenantName || query.tenantId }}</strong></div>
        <el-button text type="primary" @click="clearTenantFilter">查看全部设备</el-button>
      </div>
      <div class="filter-panel">
        <el-input v-model="query.keyword" clearable placeholder="搜索名称或设备编号" @keyup.enter="searchDevices">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.tenantId" clearable filterable placeholder="全部租户" @change="searchDevices">
          <el-option v-for="tenant in tenants" :key="tenant.id" :label="tenant.name" :value="tenant.id" />
        </el-select>
        <el-select v-model="query.status" clearable placeholder="全部状态" @change="searchDevices">
          <el-option label="在线" value="ONLINE" />
          <el-option label="离线" value="OFFLINE" />
          <el-option label="停用" value="DISABLED" />
        </el-select>
        <el-select v-model="query.deviceType" clearable placeholder="全部类型" @change="searchDevices">
          <el-option label="网关" value="GATEWAY" />
          <el-option label="子设备" value="GATEWAY_CHILD" />
          <el-option label="直连设备" value="DIRECT" />
        </el-select>
        <el-select v-model="query.protocol" clearable filterable allow-create placeholder="全部协议" @change="searchDevices">
          <el-option label="MQTT" value="MQTT" />
          <el-option label="Modbus TCP" value="MODBUS-TCP" />
          <el-option label="IEC 104" value="IEC104" />
          <el-option label="OPC UA" value="OPC-UA" />
          <el-option label="IEC 61850" value="IEC61850" />
        </el-select>
        <div class="filter-actions">
          <el-button type="primary" @click="searchDevices">查询</el-button>
          <el-button @click="resetFilters">重置</el-button>
        </div>
      </div>

      <div class="result-meta">
        <span>共 {{ total }} 台设备</span>
        <span>最近刷新：{{ refreshedAt ? refreshedAt.toLocaleTimeString() : '-' }}</span>
      </div>

      <el-table v-loading="loading" :data="devices" empty-text="没有符合条件的设备">
        <el-table-column prop="tenant.name" label="所属租户" min-width="160" show-overflow-tooltip />
        <el-table-column label="设备" min-width="220">
          <template #default="{ row }">
            <div class="device-cell">
              <strong>{{ row.name }}</strong>
              <span>{{ row.deviceKey }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="105"><template #default="{ row }">{{ deviceTypeLabel(row.deviceType) }}</template></el-table-column>
        <el-table-column label="上级网关" min-width="150">
          <template #default="{ row }">{{ row.gateway?.name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="protocol" label="协议" width="115" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tooltip :content="effectiveStatusTip(row)" placement="top">
              <el-tag :type="effectiveStatusType(row)">{{ effectiveStatusLabel(row) }}</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="location" label="安装位置" min-width="145" show-overflow-tooltip>
          <template #default="{ row }">{{ row.location || '-' }}</template>
        </el-table-column>
        <el-table-column label="最后上报" min-width="175"><template #default="{ row }">{{ formatTime(row.lastSeenAt) }}</template></el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="openTelemetry(row)">查看数据</el-button>
            <el-button
              link
              :type="row.cloudAccessEnabled ? 'danger' : 'success'"
              :loading="cloudActionId === row.id"
              @click="toggleCloudAccess(row)"
            >
              {{ row.cloudAccessEnabled ? '禁用云连接' : '恢复云连接' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          :total="total"
          @current-change="loadDevices()"
          @size-change="searchDevices"
        />
      </div>
    </section>

    <el-drawer v-model="telemetryVisible" :title="`${selectedDevice?.name || ''} · 最近数据`" size="680px">
      <div v-if="selectedDevice" class="drawer-device">
        <span>{{ selectedDevice.tenant.name }}</span>
        <strong>{{ selectedDevice.deviceKey }}</strong>
        <el-tag :type="effectiveStatusType(selectedDevice)">{{ effectiveStatusLabel(selectedDevice) }}</el-tag>
      </div>
      <el-table v-loading="telemetryLoading" :data="telemetryItems" empty-text="暂无遥测数据">
        <el-table-column label="时间" width="180"><template #default="{ row }">{{ formatTime(row.time) }}</template></el-table-column>
        <el-table-column label="数据"><template #default="{ row }"><span class="metric-text">{{ formatMetrics(row.metrics) }}</span></template></el-table-column>
      </el-table>
    </el-drawer>
  </div>
</template>

<style scoped>
.summary-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin-bottom: 18px; }
.summary-card { padding: 18px 20px; border: 1px solid #e4ebf5; border-radius: 14px; background: #fff; }
.summary-card span { display: block; color: #64748b; font-size: 13px; }
.summary-card strong { display: block; margin-top: 8px; color: #172033; font-size: 27px; }
.summary-card.online { border-color: #bbf7d0; background: #f6fff9; }
.summary-card.online strong { color: #16a34a; }
.tenant-device-context { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; padding: 12px 14px; border: 1px solid #bfdbfe; border-radius: 10px; background: #f5f9ff; }
.tenant-device-context > div { display: flex; align-items: center; gap: 10px; }
.tenant-device-context span { color: #64748b; font-size: 13px; }
.tenant-device-context strong { color: #185fae; font-size: 14px; }
.filter-panel { display: grid; grid-template-columns: minmax(220px, 1.4fr) repeat(4, minmax(130px, .8fr)) auto; gap: 10px; align-items: center; }
.filter-actions { display: flex; white-space: nowrap; }
.result-meta { display: flex; justify-content: space-between; margin: 16px 0 10px; color: #8b96a8; font-size: 13px; }
.device-cell strong, .device-cell span { display: block; }
.device-cell strong { color: #172033; }
.device-cell span { margin-top: 4px; color: #8b96a8; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 12px; }
.drawer-device { display: flex; align-items: center; gap: 10px; margin-bottom: 16px; padding: 14px; border-radius: 12px; background: #f6f9fd; }
.drawer-device strong { flex: 1; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; }
.metric-text { color: #334155; line-height: 1.6; word-break: break-word; }
@media (max-width: 1200px) {
  .filter-panel { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 760px) {
  .summary-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .filter-panel { grid-template-columns: 1fr; }
  .filter-actions .el-button { flex: 1; }
  .result-meta { gap: 8px; }
}
</style>
