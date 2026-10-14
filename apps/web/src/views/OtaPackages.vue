<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { Delete, RefreshRight, Search, UploadFilled } from '@element-plus/icons-vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import request, { apiRequest } from '../api/request';

interface OtaPackageItem {
  id: string;
  model: string;
  release: string;
  version: string;
  keyId: string;
  artifactSha256: string;
  packageSha256: string;
  size: number;
  fileAvailable: boolean;
  purgedAt?: string;
  currentDeviceCount: number;
  activeTaskCount: number;
  canPurge: boolean;
  createdAt: string;
  _count?: { tasks: number };
}

const packages = ref<OtaPackageItem[]>([]);
const loading = ref(false);
const uploading = ref(false);
const deletingId = ref('');
const keyword = ref('');
const model = ref('');
const activeTab = ref<'available' | 'purged'>('available');
const fileInput = ref<HTMLInputElement>();
const readiness = ref({ enabled: false, reason: '正在检查 OTA 配置' });

const models = computed(() => [...new Set(packages.value.map(item => item.model))].sort());
const availablePackages = computed(() => packages.value.filter(item => item.fileAvailable));
const purgedPackages = computed(() => packages.value.filter(item => !item.fileAvailable));
const filtered = computed(() => {
  const term = keyword.value.trim().toLowerCase();
  const source = activeTab.value === 'available' ? availablePackages.value : purgedPackages.value;
  return source.filter(item => (!model.value || item.model === model.value)
    && (!term || `${item.model} ${item.version} ${item.release} ${item.keyId}`.toLowerCase().includes(term)));
});
const currentDeviceCount = computed(() => packages.value.reduce((sum, item) => sum + (item.currentDeviceCount || 0), 0));
const totalSize = computed(() => packages.value.reduce((sum, item) => sum + (item.fileAvailable ? item.size : 0), 0));
const latestReleases = computed(() => availablePackages.value.reduce<Record<string, bigint>>((result, item) => {
  if (!/^\d+$/.test(item.release)) return result;
  const release = BigInt(item.release);
  if (result[item.model] === undefined || release > result[item.model]) result[item.model] = release;
  return result;
}, {}));

async function load() {
  loading.value = true;
  try {
    [packages.value, readiness.value] = await Promise.all([
      apiRequest<OtaPackageItem[]>({ url: '/api/ota/packages', method: 'GET' }),
      apiRequest<{ enabled: boolean; reason: string }>({ url: '/api/ota/readiness', method: 'GET' }),
    ]);
  } finally { loading.value = false; }
}

async function upload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (!file.name.toLowerCase().endsWith('.wku')) {
    ElMessage.warning('请选择 .wku 签名升级包');
    input.value = '';
    return;
  }
  uploading.value = true;
  try {
    await request.post('/api/ota/packages', file, {
      headers: { 'Content-Type': 'application/octet-stream' },
      timeout: 120000,
    });
    ElMessage.success('升级包签名验证通过，已保存到升级包库');
    activeTab.value = 'available';
    await load();
  } finally {
    uploading.value = false;
    input.value = '';
  }
}

async function purge(item: OtaPackageItem) {
  if (!item.fileAvailable) return;
  if (item.currentDeviceCount) {
    ElMessage.info(`仍有 ${item.currentDeviceCount} 台设备正在使用该版本，不能清理`);
    return;
  }
  if (item.activeTaskCount) {
    ElMessage.info('该升级包存在进行中的升级任务，不能清理');
    return;
  }
  try {
    await ElMessageBox.confirm(
      `确认清理 ${item.model} ${item.version}（发布序号 ${item.release}）的升级包文件吗？版本信息和历史下发记录会保留，需要时可重新上传同一签名包恢复使用。`,
      '清理升级包文件',
      { type: 'warning', confirmButtonText: '确认清理', cancelButtonText: '取消' },
    );
  } catch { return; }
  deletingId.value = item.id;
  try {
    await apiRequest({ url: `/api/ota/packages/${item.id}`, method: 'DELETE' });
    ElMessage.success('升级包文件已清理，已移入“已清理升级包”');
    await load();
    activeTab.value = 'purged';
  } finally { deletingId.value = ''; }
}

function isLatest(item: OtaPackageItem) {
  return /^\d+$/.test(item.release) && latestReleases.value[item.model] === BigInt(item.release);
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${(value / 1024 / 1024).toFixed(2)} MiB`;
}

function formatTime(value?: string) { return value ? new Date(value).toLocaleString() : '-'; }
function shortHash(value: string) { return value ? `${value.slice(0, 10)}…${value.slice(-8)}` : '-'; }

onMounted(load);
</script>

<template>
  <div class="ota-package-page">
    <div class="page-actions">
      <div><h1 class="page-title">升级包管理</h1><p class="page-description">统一保存、验证和管理 EG100 应用升级包，下发操作仍在出厂网关页面完成</p></div>
      <div class="page-buttons">
        <el-button :icon="RefreshRight" :loading="loading" @click="load">刷新</el-button>
        <input ref="fileInput" type="file" accept=".wku" hidden @change="upload" />
        <el-button type="primary" :icon="UploadFilled" :loading="uploading" @click="fileInput?.click()">上传升级包</el-button>
      </div>
    </div>

    <el-alert v-if="!readiness.enabled" class="readiness-alert" type="warning" :closable="false" show-icon :title="readiness.reason" description="可继续查看已有升级包；修复 OTA 配置后才能远程下发。" />

    <div class="package-summary">
      <div><span>可用升级包</span><strong>{{ availablePackages.length }}</strong></div>
      <div><span>已清理升级包</span><strong>{{ purgedPackages.length }}</strong></div>
      <div><span>当前使用设备</span><strong>{{ currentDeviceCount }}</strong></div>
      <div><span>占用空间</span><strong>{{ formatBytes(totalSize) }}</strong></div>
    </div>

    <section class="table-panel">
      <el-tabs v-model="activeTab" class="package-tabs">
        <el-tab-pane name="available">
          <template #label><span class="package-tab-label">可用升级包 <em>{{ availablePackages.length }}</em></span></template>
        </el-tab-pane>
        <el-tab-pane name="purged">
          <template #label><span class="package-tab-label">已清理升级包 <em>{{ purgedPackages.length }}</em></span></template>
        </el-tab-pane>
      </el-tabs>
      <div class="package-toolbar">
        <el-input v-model="keyword" clearable :prefix-icon="Search" placeholder="搜索型号、版本、发布序号或签名密钥" aria-label="搜索升级包" />
        <el-select v-model="model" clearable placeholder="全部型号" aria-label="按型号筛选">
          <el-option v-for="item in models" :key="item" :label="item" :value="item" />
        </el-select>
      </div>
      <el-table v-loading="loading" :data="filtered" :empty-text="activeTab === 'available' ? '暂无可用升级包，请上传已签名的 .wku 文件' : '暂无已清理升级包'">
        <el-table-column label="版本" min-width="180">
          <template #default="{ row }"><div class="package-name"><div><strong>{{ row.version }}</strong><el-tag v-if="isLatest(row)" size="small" type="success">最新发布</el-tag></div><span>{{ row.model }}</span></div></template>
        </el-table-column>
        <el-table-column prop="release" label="发布序号" min-width="145" />
        <el-table-column label="包文件" width="105"><template #default="{ row }"><el-tag :type="row.fileAvailable ? 'success' : 'info'">{{ row.fileAvailable ? '可下发' : '已清理' }}</el-tag></template></el-table-column>
        <el-table-column label="文件大小" width="120"><template #default="{ row }">{{ formatBytes(row.size) }}</template></el-table-column>
        <el-table-column label="签名密钥" min-width="155"><template #default="{ row }"><code>{{ row.keyId }}</code></template></el-table-column>
        <el-table-column label="包摘要" min-width="210"><template #default="{ row }"><el-tooltip :content="row.packageSha256" placement="top"><code>{{ shortHash(row.packageSha256) }}</code></el-tooltip></template></el-table-column>
        <el-table-column label="当前使用" width="105"><template #default="{ row }"><el-tag :type="row.currentDeviceCount ? 'primary' : 'info'">{{ row.currentDeviceCount || 0 }} 台</el-tag></template></el-table-column>
        <el-table-column label="历史下发" width="105"><template #default="{ row }">{{ row._count?.tasks || 0 }} 次</template></el-table-column>
        <el-table-column :label="activeTab === 'available' ? '上传时间' : '清理时间'" min-width="175"><template #default="{ row }">{{ formatTime(activeTab === 'available' ? row.createdAt : row.purgedAt) }}</template></el-table-column>
        <el-table-column v-if="activeTab === 'available'" label="操作" width="100" fixed="right">
          <template #default="{ row }"><el-tooltip :disabled="row.canPurge || !row.fileAvailable" :content="row.currentDeviceCount ? `仍有 ${row.currentDeviceCount} 台设备使用` : '存在进行中的升级任务'" placement="top"><span><el-button link type="danger" :icon="Delete" :disabled="!row.canPurge" :loading="deletingId === row.id" @click="purge(row)">{{ row.fileAvailable ? '清理' : '已清理' }}</el-button></span></el-tooltip></template>
        </el-table-column>
      </el-table>
      <p v-if="activeTab === 'available'" class="package-note">平台以每台设备最近一次成功升级的版本判断“当前使用”。无人使用且没有进行中任务时可清理包文件。</p>
      <p v-else class="package-note">已清理包仅保留版本信息与历史记录，不占用升级包存储空间；重新上传同一签名包即可恢复到“可用升级包”。</p>
    </section>
  </div>
</template>

<style scoped>
.page-buttons { display: flex; align-items: center; gap: 10px; }
.page-buttons :deep(.el-button) { margin-left: 0; }
.readiness-alert { margin-bottom: 18px; }
.package-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; margin-bottom: 20px; }
.package-summary > div { display: flex; align-items: center; justify-content: space-between; padding: 18px 22px; border: 1px solid #e3eaf4; border-radius: 12px; background: #fff; }
.package-summary span { color: #8290a8; font-size: 13px; }
.package-summary strong { color: #172b4d; font-size: 24px; }
.package-summary > div:last-child strong { font-size: 18px; }
.package-tabs { margin: -2px 0 16px; }
.package-tabs :deep(.el-tabs__header) { margin-bottom: 0; }
.package-tabs :deep(.el-tabs__content) { display: none; }
.package-tab-label { display: inline-flex; align-items: center; gap: 7px; }
.package-tab-label em { display: inline-grid; min-width: 20px; height: 20px; padding: 0 6px; place-items: center; border-radius: 999px; color: #64748b; background: #eef2f7; font-size: 11px; font-style: normal; }
.package-tabs :deep(.is-active) .package-tab-label em { color: #2563eb; background: #eaf2ff; }
.package-toolbar { display: grid; grid-template-columns: minmax(260px, 420px) 180px; gap: 12px; margin-bottom: 18px; }
.package-name { display: grid; gap: 5px; }
.package-name > div { display: flex; align-items: center; gap: 8px; }
.package-name strong { color: #172b4d; }
.package-name span { color: #8290a8; font-size: 12px; }
code { color: #43546b; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 12px; }
.package-note { margin: 16px 0 0; color: #8290a8; font-size: 12px; line-height: 1.7; }
@media (max-width: 900px) { .package-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 600px) {
  .page-actions { align-items: stretch; flex-direction: column; gap: 14px; }
  .page-buttons { width: 100%; }
  .page-buttons .el-button { flex: 1; }
  .package-summary { gap: 8px; }
  .package-summary > div { padding: 13px; }
  .package-toolbar { grid-template-columns: 1fr; }
}
</style>
